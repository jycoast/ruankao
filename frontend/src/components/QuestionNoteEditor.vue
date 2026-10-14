<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useEditor, EditorContent } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Underline from '@tiptap/extension-underline'
import Image from '@tiptap/extension-image'
import { questionNoteApi, uploadApi } from '@/api'
import { sanitizeNoteHtml } from '@/utils/markdown'

const props = defineProps<{
  questionId: number
  // 默认 false —— 题目页（QuestionDetail.vue）不传，笔记区初始仍是收起的。
  // 但题目页并非「行为完全不变」：loadedFor 是两种用法共用的，上一次加载成功时，
  // 收起再展开不再重读服务端 —— 未保存的草稿因此留得住，代价是别处（例如另一个
  // 标签页）改过的内容不会再被这次展开拉过来；上一次加载失败时 loadedFor 仍是 null，
  // 展开会重新读取。这是有意接受的偏差。
  startOpen?: boolean
  // 传了就不再自取（父组件已经把正文交过来了，见 spec「为什么需要 initialContent」）。
  initialContent?: string
}>()

const emit = defineEmits<{ (e: 'saved'): void }>()

// 只决定**初始**开合；那个「📝 我的笔记」折叠按钮照常工作，用户仍可收起。
const open = ref(props.startOpen === true)

// 已经为哪道题灌过正文。注入只发生在「首次加载」与「切换题目」时，
// 收起再展开**不重新注入** —— 理由见 toggleOpen 的注释。
const loadedFor = ref<number | null>(null)
const hasNote = ref(false)
const saving = ref(false)
const uploading = ref(false)
const saveStatus = ref<'' | 'saved' | 'unsaved'>('')
// 避免首次 loadNote 时触发 onUpdate
const loadingNote = ref(false)
// 图片上传用的隐藏 input ref（用 ref 绑而不是 document.getElementById，
// 避免多个 QuestionNoteEditor 实例在同一页时 id 冲突）
const imageInput = ref<HTMLInputElement | null>(null)

const editor = useEditor({
  extensions: [StarterKit, Underline, Image.configure({ inline: false, allowBase64: false })],
  content: '',
  onUpdate() {
    if (!loadingNote.value) saveStatus.value = 'unsaved'
  },
  editorProps: {
    attributes: { class: 'note-prose' },
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

watch(() => props.questionId, () => {
  hasNote.value = false
  saveStatus.value = ''
  // 换题了：下次挂载/展开必须重新灌一次正文
  loadedFor.value = null
  editor.value?.commands.clearContent()
  if (open.value) loadNote()
})

async function loadNote() {
  // 注入路径：父组件已经拿到正文，不再打 GET /questions/:id/note
  //（那条路由继承 RateData 30次/分钟，每点一条笔记烧一次配额）。
  // 空判据与 handleSave 里那句保持一致，不用去标签的写法 ——
  // 纯图片笔记剥掉标签后是空串，会被误判成「没有笔记」。
  if (props.initialContent !== undefined) {
    const html = props.initialContent || ''
    editor.value?.commands.setContent(html)
    hasNote.value = !!html && html !== '<p></p>'
    saveStatus.value = hasNote.value ? 'saved' : ''
    loadedFor.value = props.questionId
    return
  }

  loadingNote.value = true
  // 记下发起请求时的题号：题目页（QuestionDetail / ExamSession / PracticeByCategory /
  // DailyPracticeCard）的编辑器实例会在原地换题 —— 它们不传 :key。快速连点
  // 「下一题」时两次请求会并发，旧响应可能后到。
  const id = props.questionId
  try {
    const res: any = await questionNoteApi.get(id)
    // 过期响应直接丢弃。少了这一条，先发的旧响应后到时会把**上一题**的正文写进编辑器，
    // 而界面上显示的是当前题 —— 用户此时一保存，就把上一题的笔记写到了这一题上。
    if (props.questionId !== id) return
    editor.value?.commands.setContent(res.data.content || '')
    hasNote.value = !!res.data.exists
    saveStatus.value = res.data.exists ? 'saved' : ''
    // 只在成功时记账：失败的话留 null，下次展开还能重试。
    loadedFor.value = id
  } catch {
    if (props.questionId !== id) return
    editor.value?.commands.clearContent()
    hasNote.value = false
  } finally {
    if (props.questionId === id) loadingNote.value = false
  }
}

function toggleOpen() {
  open.value = !open.value
  // 只在「还没为当前这道题灌过内容」时才注入。
  //
  // 为什么不能无条件 loadNote()：注入会把 props.initialContent 灌回编辑器，
  // 而父组件的种子只在**它自己知道内容变了**时才更新。编辑器内部有两处改动
  // 父组件不知道，无条件重注会把它们打回去：
  //   ① 用户点了「清除笔记」—— handleDelete 清了编辑器与服务端，但按设计不发事件，
  //      重注会让删掉的笔记复活，hasNote 翻回 true、状态栏写「已保存」，
  //      此时再点保存会把刚删的笔记写回库；
  //   ② 用户写了一半点收起 —— 草稿本就没保存，重注等于直接丢掉。
  // 记账之后，这两处都不会再被覆盖，而换题仍然会重新灌（见 4b）。
  if (open.value && loadedFor.value !== props.questionId) loadNote()
}

async function handleSave() {
  const html = editor.value?.getHTML() || ''
  if (!html || html === '<p></p>') {
    ElMessage.warning('笔记内容为空')
    return
  }
  // 前端 DOMPurify 兜底：剥危险协议 + 限制 <img src> 白名单。
  // 后端 question_note.Save 还会再走一次 sanitizeNoteHTML（regex）做保底。
  const safe = sanitizeNoteHtml(html)
  if (!safe || safe === '<p></p>') {
    ElMessage.warning('笔记内容被清洗后为空，请检查输入')
    return
  }
  saving.value = true
  try {
    await questionNoteApi.save(props.questionId, safe)
    hasNote.value = true
    saveStatus.value = 'saved'
    emit('saved')
    ElMessage.success('笔记已保存')
  } finally {
    saving.value = false
  }
}

async function handleDelete() {
  await ElMessageBox.confirm('确认清除本题笔记？', '清除笔记', { type: 'warning' })
  await questionNoteApi.delete(props.questionId)
  editor.value?.commands.clearContent()
  hasNote.value = false
  saveStatus.value = ''
  ElMessage.success('笔记已清除')
}

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
    saveStatus.value = 'unsaved'
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

// 初始灌内容必须走 onMounted，且**注册在 useEditor() 之后**。
// useEditor 在 setup 期就注册了自己的 onMounted（已核实 @tiptap/vue-3 的
// useEditor 实现：const editor = shallowRef(); onMounted(() => { editor.value = new Editor(options) })），
// 后注册的 onMounted 跑在它后面，所以这里 editor.value 一定已就绪。
//
// **不要**改成 watch(..., { immediate: true }) —— 那是在 mount 之前同步执行的，
// 彼时 editor.value 还是 undefined，下面那句 setContent 会被可选链静默吞掉，
// 表现是**笔记显示为空且不报任何错**。
onMounted(() => {
  if (open.value) loadNote()
})

onBeforeUnmount(() => {
  editor.value?.destroy()
})

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
  <div class="mt-4">
    <!-- 触发按钮 -->
    <button
      class="flex items-center gap-1.5 text-sm px-3 py-1.5 rounded-md border transition-colors"
      :class="open
        ? 'border-blue-300 bg-blue-50 text-blue-700 dark:bg-blue-500/10 dark:text-blue-300 dark:border-blue-500/50'
        : 'border-gray-200 dark:border-gray-600 text-gray-500 dark:text-gray-400 hover:border-blue-300 hover:text-blue-600'"
      @click="toggleOpen"
    >
      <span>📝</span>
      <span>我的笔记</span>
      <span v-if="hasNote" class="w-2 h-2 rounded-full bg-blue-500 flex-shrink-0" />
    </button>

    <!-- 编辑区 -->
    <div
      v-show="open"
      class="mt-2 border border-blue-200 border-l-[3px] border-l-blue-500 rounded-lg overflow-hidden dark:border-blue-500/30 dark:border-l-blue-500"
    >
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
      <div class="bg-white dark:bg-gray-900 overflow-y-auto" style="min-height:180px; max-height:360px;">
        <EditorContent :editor="editor" />
      </div>

      <!-- 底部状态栏 -->
      <div class="flex items-center justify-between px-3 py-2 bg-gray-50 dark:bg-gray-800 border-t border-gray-100 dark:border-gray-700">
        <span class="text-xs transition-colors" :class="{
          'text-green-500': saveStatus === 'saved',
          'text-orange-400': saveStatus === 'unsaved',
          'text-transparent': saveStatus === '',
        }">
          {{ saveStatus === 'saved' ? '✓ 已保存' : saveStatus === 'unsaved' ? '● 未保存' : '·' }}
        </span>
        <div class="flex gap-2">
          <el-button v-if="hasNote" size="small" type="danger" text @click="handleDelete">清除笔记</el-button>
          <el-button size="small" type="primary" :loading="saving" @click="handleSave">保存</el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<style>
/* ProseMirror 基础排版 */
.note-prose {
  padding: 12px 16px;
  outline: none;
  min-height: 180px;
  font-size: 14px;
  line-height: 1.75;
  color: #374151;
  word-break: break-word;
}
.dark .note-prose { color: #d1d5db; }

.note-prose p { margin: 0 0 0.4em; }
.note-prose p:last-child { margin-bottom: 0; }

.note-prose strong { font-weight: 600; }
.note-prose em { font-style: italic; }
.note-prose u { text-decoration: underline; }
.note-prose s { text-decoration: line-through; }

.note-prose code {
  background: #f3f4f6;
  padding: 1px 5px;
  border-radius: 3px;
  font-family: ui-monospace, monospace;
  font-size: 0.85em;
  color: #ef4444;
}
.dark .note-prose code { background: #374151; color: #f87171; }

.note-prose pre {
  background: #1e293b;
  color: #e2e8f0;
  padding: 10px 14px;
  border-radius: 6px;
  overflow-x: auto;
  font-family: ui-monospace, monospace;
  font-size: 0.85em;
  margin: 0.5em 0;
}
.note-prose pre code { background: none; color: inherit; padding: 0; }

.note-prose ul { list-style: disc; padding-left: 1.4em; margin: 0.4em 0; }
.note-prose ol { list-style: decimal; padding-left: 1.4em; margin: 0.4em 0; }
.note-prose li { margin: 0.15em 0; }

.note-prose blockquote {
  border-left: 3px solid #d1d5db;
  padding-left: 12px;
  margin: 0.5em 0;
  color: #6b7280;
}
.dark .note-prose blockquote { border-color: #4b5563; color: #9ca3af; }

/* 笔记图片：最大宽度受限于编辑器，不允许撑破排版；为光感更好加一点圆角阴影 */
.note-prose img {
  display: block;
  max-width: 100%;
  height: auto;
  margin: 0.5em 0;
  border-radius: 4px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
}
.note-prose img.ProseMirror-selected {
  outline: 2px solid rgb(var(--tw-blue-500-rgb, 59 130 246));
  outline-offset: 2px;
}

/* 空白提示 */
.note-prose.is-empty::before,
.note-prose p.is-empty:first-child::before {
  content: '在此记录本题笔记…';
  color: #6b7280;
  pointer-events: none;
  float: left;
  height: 0;
}
.dark .note-prose.is-empty::before,
.dark .note-prose p.is-empty:first-child::before {
  color: #9ca3af;
}
</style>
