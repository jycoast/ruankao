<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { useEditor, EditorContent } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Underline from '@tiptap/extension-underline'
import Image from '@tiptap/extension-image'
// TipTap 3 把 Placeholder 合进了 @tiptap/extensions 单包（v2 是独立包），按子路径引入。
import { Placeholder } from '@tiptap/extensions/placeholder'
import { uploadApi } from '@/api'

// 通用富文本编辑器：笔记编辑器（QuestionNoteEditor.vue）的编辑器内核抽出来复用。
// 两者完整共享：TipTap 扩展、剪贴板粘贴图片→上传通道、工具栏图片按钮、.rt-content 排版样式、
// 以及「前端不上传 base64、返回 URL 只放行 https:/data:image/」的安全约束。
// 与 QuestionNoteEditor 的差异只在壳：这里没有题目/笔记的概念，content 用 v-model 双向绑定，
// sanitize（DOMPurify / 后端兜底）由调用方在提交时做，本组件不负责保存。
const props = withDefaults(defineProps<{
  // 富文本 HTML（原始、未 sanitize）；父组件提交时自行 sanitizeNoteHtml
  modelValue: string
  placeholder?: string
  minHeight?: number
  maxHeight?: number
}>(), {
  placeholder: '请输入内容…',
  minHeight: 180,
  maxHeight: 360,
})

const emit = defineEmits<{ (e: 'update:modelValue', value: string): void }>()

const uploading = ref(false)
// 图片上传用的隐藏 input ref（用 ref 绑而不是 document.getElementById，
// 避免同一页多个 RichTextEditor 实例时 id 冲突）
const imageInput = ref<HTMLInputElement | null>(null)

const editor = useEditor({
  extensions: [
    StarterKit,
    Underline,
    Image.configure({ inline: false, allowBase64: false }),
    // showOnlyCurrent:false —— 表单场景下未聚焦时也显示占位提示（默认只在编辑器聚焦时显示）
    Placeholder.configure({ placeholder: props.placeholder, showOnlyCurrent: false }),
  ],
  content: props.modelValue,
  onUpdate() {
    emit('update:modelValue', editor.value?.getHTML() || '')
  },
  editorProps: {
    attributes: { class: 'rt-content' },
    // 剪贴板粘贴：含图片就走上传通道，纯文本/HTML 让 TipTap 默认接管。
    // 返回 true = 已处理(阻止 TipTap 默认插入)，返回 false = 让 TipTap 自己处理。
    handlePaste: (_view, event) => {
      const items = Array.from(event.clipboardData?.items || [])
      const imageItems = items.filter((it) => it.kind === 'file' && it.type.startsWith('image/'))
      if (!imageItems.length) return false // 普通文本粘贴，走 TipTap 默认

      // 拦下默认行为，避免 TipTap 把图片当成 base64 src 塞进 <img>(我们 allowBase64:false)。
      event.preventDefault()
      const files = imageItems
        .map((it) => it.getAsFile())
        .filter((f): f is File => !!f)
      if (!files.length) return true

      // fire-and-forget：handlePaste 必须同步返回 true，async 工作在后台跑。
      // 顺序上传便于逐张给用户反馈；并发会让多张上传的失败 toast 堆在一起。
      void (async () => {
        for (const f of files) {
          await uploadAndInsertImage(f)
        }
        if (files.length > 1) {
          ElMessage.success(`已粘贴 ${files.length} 张图片`)
        }
      })()
      return true
    },
  },
})

// 父组件在外部把 modelValue 改回去（典型场景：提交成功后清空表单）时同步进编辑器。
// onUpdate 里也已经把当前值 emit 给父组件了，这里用「值相同」短路——
// 用户自己打字这条路径上 v 恒等于编辑器现内容，直接跳过，不会来回 setContent。
watch(() => props.modelValue, (v) => {
  if (v === editor.value?.getHTML()) return
  editor.value?.commands.setContent(v || '')
})

onBeforeUnmount(() => {
  editor.value?.destroy()
})

// placeholder 会随父组件切换（如反馈类型联动），编辑器建好后不能重配扩展 ——
// 原地改扩展 options 即可：Placeholder 插件的闭包持有同一个 options 对象，
// 下次 decoration 计算会读到新值并写进 data-placeholder。
watch(() => props.placeholder, (p) => {
  const ext = editor.value?.extensionManager?.extensions.find((e) => e.name === 'placeholder')
  if (ext) ext.options.placeholder = p
})

// 触发隐藏的文件选择器。比 el-upload 轻量、与现有工具栏按钮风格一致。
function pickImage() {
  if (uploading.value) return
  imageInput.value?.click()
}

// 共享：上传一张图片并在光标处插入。被工具栏文件按钮和剪贴板粘贴两条入口共用。
// 前端预检 5MB / image/*；返回 URL 只放行 https:/data:image/；后端 sanitize 会再过一道。
async function uploadAndInsertImage(file: File) {
  if (!file.type.startsWith('image/')) {
    ElMessage.warning('请选择图片文件')
    return
  }
  if (file.size > 5 * 1024 * 1024) {
    ElMessage.warning('图片超过 5MB')
    return
  }

  uploading.value = true
  try {
    const res: any = await uploadApi.noteImage(file, file.name || 'note.png')
    const url = res?.data?.url
    if (!url) {
      ElMessage.error('上传失败：未拿到图片地址')
      return
    }
    if (!/^https:\/\//i.test(url) && !/^data:image\//i.test(url)) {
      ElMessage.error('上传返回的 URL 协议不被允许')
      return
    }
    editor.value?.chain().focus().setImage({ src: url, alt: file.name }).run()
  } catch {
    // 拦截器已弹错，这里不再重复
  } finally {
    uploading.value = false
  }
}

// 监听文件选择 → 委托给 uploadAndInsertImage。
async function onImageSelected(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  // 清空 value，否则同一张图连选两次不触发 change
  input.value = ''
  if (file) await uploadAndInsertImage(file)
}

// 工具栏按钮配置
type BtnDef =
  | { type: 'sep' }
  | { type: 'btn'; title: string; label: string; active: () => boolean; action: () => void; disabled?: () => boolean }
  | { type: 'file' }

function btn(title: string, label: string, active: () => boolean, action: () => void, disabled?: () => boolean): BtnDef {
  return { type: 'btn', title, label, active, action, disabled }
}

const toolbar = (): BtnDef[] => [
  btn('加粗', 'B', () => !!editor.value?.isActive('bold'), () => editor.value?.chain().focus().toggleBold().run()),
  btn('斜体', 'I', () => !!editor.value?.isActive('italic'), () => editor.value?.chain().focus().toggleItalic().run()),
  btn('下划线', 'U', () => !!editor.value?.isActive('underline'), () => editor.value?.chain().focus().toggleUnderline().run()),
  btn('删除线', 'S̶', () => !!editor.value?.isActive('strike'), () => editor.value?.chain().focus().toggleStrike().run()),
  { type: 'sep' },
  btn('行内代码', '</>', () => !!editor.value?.isActive('code'), () => editor.value?.chain().focus().toggleCode().run()),
  btn('代码块', '{ }', () => !!editor.value?.isActive('codeBlock'), () => editor.value?.chain().focus().toggleCodeBlock().run()),
  { type: 'sep' },
  btn('无序列表', '• —', () => !!editor.value?.isActive('bulletList'), () => editor.value?.chain().focus().toggleBulletList().run()),
  btn('有序列表', '1. —', () => !!editor.value?.isActive('orderedList'), () => editor.value?.chain().focus().toggleOrderedList().run()),
  btn('引用', '" "', () => !!editor.value?.isActive('blockquote'), () => editor.value?.chain().focus().toggleBlockquote().run()),
  { type: 'sep' },
  { type: 'file' },
  btn('撤销', '↩', () => false, () => editor.value?.chain().focus().undo().run(), () => !editor.value?.can().undo()),
  btn('重做', '↪', () => false, () => editor.value?.chain().focus().redo().run(), () => !editor.value?.can().redo()),
]
</script>

<template>
  <div class="w-full rounded-lg border border-gray-200 border-l-[3px] border-l-blue-500 overflow-hidden dark:border-gray-700 dark:border-l-blue-500">
    <!-- 工具栏 -->
    <div class="flex items-center gap-0.5 px-2 py-1.5 bg-gray-50 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 flex-wrap">
      <template v-for="(item, i) in toolbar()" :key="i">
        <div v-if="item.type === 'sep'" class="w-px h-5 bg-gray-300 dark:bg-gray-600 mx-1" />
        <button
          v-else-if="item.type === 'file'"
          class="min-w-[28px] h-7 px-1.5 rounded text-xs text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors disabled:opacity-40 disabled:cursor-not-allowed flex items-center gap-1"
          :disabled="uploading"
          :title="uploading ? '图片上传中…' : '插入图片'"
          @click.prevent="pickImage"
        >
          <span v-if="uploading" class="inline-block w-3 h-3 border-2 border-current border-t-transparent rounded-full animate-spin" />
          <span v-else>🖼</span>
          <span>图片</span>
        </button>
        <button
          v-else
          class="min-w-[28px] h-7 px-1 rounded text-xs text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
          :class="{ 'bg-gray-200 dark:bg-gray-700 text-blue-600 dark:text-blue-400': item.active() }"
          :disabled="item.disabled?.() ?? false"
          :title="item.title"
          @click.prevent="item.action()"
        >{{ item.label }}</button>
      </template>
    </div>

    <!-- 隐藏文件 input：用于工具栏图片按钮触发选图 -->
    <input
      ref="imageInput"
      type="file"
      accept="image/jpeg,image/png,image/gif,image/webp,image/bmp"
      class="hidden"
      @change="onImageSelected"
    >

    <!-- 编辑区内容 -->
    <div
      class="bg-white dark:bg-gray-900 overflow-y-auto"
      :style="{ minHeight: `${minHeight}px`, maxHeight: `${maxHeight}px` }"
    >
      <EditorContent :editor="editor" />
    </div>
  </div>
</template>

<style>
/* ProseMirror 基础排版：与 QuestionNoteEditor 的 .note-prose 同源，展示端复用同类名渲染富文本内容 */
.rt-content {
  padding: 12px 16px;
  outline: none;
  min-height: inherit;
  font-size: 14px;
  line-height: 1.75;
  color: #374151;
  word-break: break-word;
}
.dark .rt-content { color: #d1d5db; }

.rt-content p { margin: 0 0 0.4em; }
.rt-content p:last-child { margin-bottom: 0; }

.rt-content strong { font-weight: 600; }
.rt-content em { font-style: italic; }
.rt-content u { text-decoration: underline; }
.rt-content s { text-decoration: line-through; }

.rt-content code {
  background: #f3f4f6;
  padding: 1px 5px;
  border-radius: 3px;
  font-family: ui-monospace, monospace;
  font-size: 0.85em;
  color: #ef4444;
}
.dark .rt-content code { background: #374151; color: #f87171; }

.rt-content pre {
  background: #1e293b;
  color: #e2e8f0;
  padding: 10px 14px;
  border-radius: 6px;
  overflow-x: auto;
  font-family: ui-monospace, monospace;
  font-size: 0.85em;
  margin: 0.5em 0;
}
.rt-content pre code { background: none; color: inherit; padding: 0; }

.rt-content ul { list-style: disc; padding-left: 1.4em; margin: 0.4em 0; }
.rt-content ol { list-style: decimal; padding-left: 1.4em; margin: 0.4em 0; }
.rt-content li { margin: 0.15em 0; }

.rt-content blockquote {
  border-left: 3px solid #d1d5db;
  padding-left: 12px;
  margin: 0.5em 0;
  color: #6b7280;
}
.dark .rt-content blockquote { border-color: #4b5563; color: #9ca3af; }

/* 图片：最大宽度受限于容器，不允许撑破排版；为光感更好加一点圆角阴影 */
.rt-content img {
  display: block;
  max-width: 100%;
  height: auto;
  margin: 0.5em 0;
  border-radius: 4px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
}
.rt-content img.ProseMirror-selected {
  outline: 2px solid rgb(var(--tw-blue-500-rgb, 59 130 246));
  outline-offset: 2px;
}

/* 空内容占位提示：Placeholder 扩展给空段落加上 is-empty/is-editor-empty 与 data-placeholder，
   与内容同色的浅灰，未聚焦也显示（showOnlyCurrent:false 的效果）。 */
.rt-content p.is-empty:first-child::before {
  content: attr(data-placeholder);
  color: #6b7280;
  pointer-events: none;
  float: left;
  height: 0;
}
.dark .rt-content p.is-empty:first-child::before {
  color: #9ca3af;
}
</style>