<script setup lang="ts">
import { ref, watch, computed, nextTick } from 'vue'
import { renderMd } from '@/utils/markdown'
import { usePasteUpload } from '@/composables/usePasteUpload'

const props = withDefaults(defineProps<{
  modelValue: string
  label?: string
  rows?: number
  placeholder?: string
  /** 是否在全屏里展示 Markdown 实时预览（题干/解析/分析为 markdown，标题为纯文本） */
  preview?: boolean
  /** 是否显示 Markdown 快捷工具栏（论文/题干等 markdown 字段开启） */
  toolbar?: boolean
}>(), {
  label: '内容',
  rows: 4,
  placeholder: '',
  preview: true,
  toolbar: true,
})

const emit = defineEmits<{
  (e: 'update:modelValue', v: string): void
}>()

// 与父组件双向同步：外层 textarea 直接绑 modelValue，
// 全屏弹窗用一个 draft 快照，关闭时回写，避免弹窗里每次按键都触发外层重渲染。
const draft = ref(props.modelValue)
watch(() => props.modelValue, (v) => { draft.value = v })

const fullscreen = ref(false)
const colorPickerOpen = ref(false)
const inlineTa = ref<HTMLTextAreaElement | null>(null)
const fullTa = ref<HTMLTextAreaElement | null>(null)

function openFullscreen() {
  draft.value = props.modelValue
  fullscreen.value = true
}
function closeFullscreen() {
  emit('update:modelValue', draft.value)
  fullscreen.value = false
}

const { handlePaste } = usePasteUpload()
function onPaste(e: ClipboardEvent, target: 'inline' | 'full') {
  const value = target === 'inline' ? props.modelValue : draft.value
  const write = target === 'inline'
    ? (next: string) => emit('update:modelValue', next)
    : (next: string) => { draft.value = next }
  handlePaste(e, value, write)
}

const previewHtml = computed(() => (props.preview ? renderMd(draft.value) : ''))

type ToolbarTarget = 'inline' | 'full'
type MdAction =
  | 'bold' | 'italic' | 'strike'
  | 'h1' | 'h2' | 'h3'
  | 'ul' | 'ol' | 'quote' | 'code' | 'hr'
  | 'link' | 'image'
  | 'table' | 'sup' | 'sub' | 'details'

interface ToolbarItem {
  action: MdAction
  label: string
  title: string
}

// 工具栏内联分组；调色板单独走 popover，不进 toolbarItems
const toolbarItems: ToolbarItem[] = [
  { action: 'bold', label: 'B', title: '加粗 **文本**' },
  { action: 'italic', label: 'I', title: '斜体 *文本*' },
  { action: 'strike', label: 'S', title: '删除线 ~~文本~~' },
  { action: 'h1', label: 'H1', title: '一级标题' },
  { action: 'h2', label: 'H2', title: '二级标题' },
  { action: 'h3', label: 'H3', title: '三级标题' },
  { action: 'ul', label: '• 列表', title: '无序列表' },
  { action: 'ol', label: '1. 列表', title: '有序列表' },
  { action: 'quote', label: '“”', title: '引用' },
  { action: 'code', label: '</>', title: '代码块' },
  { action: 'hr', label: '—', title: '分隔线' },
  { action: 'link', label: '链接', title: '插入链接' },
  { action: 'image', label: '图片', title: '插入图片语法' },
  { action: 'table', label: '⊞ 表格', title: '插入表格' },
  { action: 'sup', label: 'x²', title: '上标' },
  { action: 'sub', label: 'x₂', title: '下标' },
  { action: 'details', label: '▼ 详情', title: '可折叠块' },
]

// 调色板：用户要"红色重点"所以这是高频入口；预定义 7 色够用，
// 避免开放自定义 hex 导致脱色与色盲场景失控。深色用更亮档位更可读。
const colorPalette = [
  { name: '红', hex: '#dc2626' },
  { name: '橙', hex: '#ea580c' },
  { name: '黄', hex: '#ca8a04' },
  { name: '绿', hex: '#16a34a' },
  { name: '青', hex: '#0891b2' },
  { name: '蓝', hex: '#2563eb' },
  { name: '紫', hex: '#9333ea' },
] as const

function resolveTextarea(target: ToolbarTarget): HTMLTextAreaElement | null {
  if (target === 'full') {
    if (fullTa.value) return fullTa.value
    return document.querySelector('.md-editor-full-textarea textarea') as HTMLTextAreaElement | null
  }
  if (inlineTa.value) return inlineTa.value
  return document.querySelector('.md-editor-inline-textarea textarea') as HTMLTextAreaElement | null
}

function readValue(target: ToolbarTarget): string {
  return target === 'full' ? draft.value : props.modelValue
}

function writeValue(target: ToolbarTarget, next: string) {
  if (target === 'full') draft.value = next
  else emit('update:modelValue', next)
}

function wrapSelection(
  value: string,
  start: number,
  end: number,
  before: string,
  after: string,
  placeholder = '文本',
) {
  const selected = value.slice(start, end)
  const inner = selected || placeholder
  const inserted = before + inner + after
  const next = value.slice(0, start) + inserted + value.slice(end)
  const selStart = start + before.length
  const selEnd = selStart + inner.length
  return { next, selStart, selEnd }
}

function prefixLines(
  value: string,
  start: number,
  end: number,
  makePrefix: (i: number) => string,
  placeholder = '列表项',
) {
  // 扩展到整行，方便对多行批量加列表/引用前缀
  const lineStart = value.lastIndexOf('\n', Math.max(0, start - 1)) + 1
  let lineEnd = value.indexOf('\n', end)
  if (lineEnd === -1) lineEnd = value.length

  const block = value.slice(lineStart, lineEnd) || placeholder
  const lines = block.split('\n')
  const replaced = lines.map((line, i) => {
    const bare = line.replace(/^\s{0,3}([-*+] |\d+\. |> )/, '')
    return makePrefix(i) + (bare || placeholder)
  }).join('\n')

  const next = value.slice(0, lineStart) + replaced + value.slice(lineEnd)
  return {
    next,
    selStart: lineStart,
    selEnd: lineStart + replaced.length,
  }
}

function applyMarkdown(target: ToolbarTarget, action: MdAction) {
  const el = resolveTextarea(target)
  const value = readValue(target)
  const start = el?.selectionStart ?? value.length
  const end = el?.selectionEnd ?? value.length

  let result: { next: string; selStart: number; selEnd: number }

  switch (action) {
    case 'bold':
      result = wrapSelection(value, start, end, '**', '**', '加粗文本')
      break
    case 'italic':
      result = wrapSelection(value, start, end, '*', '*', '斜体文本')
      break
    case 'strike':
      result = wrapSelection(value, start, end, '~~', '~~', '删除文本')
      break
    case 'h1':
      result = prefixLines(value, start, end, () => '# ', '一级标题')
      break
    case 'h2':
      result = prefixLines(value, start, end, () => '## ', '二级标题')
      break
    case 'h3':
      result = prefixLines(value, start, end, () => '### ', '三级标题')
      break
    case 'ul':
      result = prefixLines(value, start, end, () => '- ', '列表项')
      break
    case 'ol':
      result = prefixLines(value, start, end, (i) => `${i + 1}. `, '列表项')
      break
    case 'quote':
      result = prefixLines(value, start, end, () => '> ', '引用内容')
      break
    case 'code': {
      const selected = value.slice(start, end)
      if (selected.includes('\n') || start === end) {
        const inner = selected || 'code'
        const snippet = `\n\`\`\`\n${inner}\n\`\`\`\n`
        result = {
          next: value.slice(0, start) + snippet + value.slice(end),
          selStart: start + 5,
          selEnd: start + 5 + inner.length,
        }
      } else {
        result = wrapSelection(value, start, end, '`', '`', 'code')
      }
      break
    }
    case 'hr': {
      const snippet = '\n\n---\n\n'
      result = {
        next: value.slice(0, start) + snippet + value.slice(end),
        selStart: start + snippet.length,
        selEnd: start + snippet.length,
      }
      break
    }
    case 'link': {
      const selected = value.slice(start, end) || '链接文字'
      const snippet = `[${selected}](https://)`
      result = {
        next: value.slice(0, start) + snippet + value.slice(end),
        selStart: start + selected.length + 3,
        selEnd: start + selected.length + 3 + 'https://'.length,
      }
      break
    }
    case 'image': {
      const selected = value.slice(start, end) || '图片说明'
      const snippet = `![${selected}](https://)`
      result = {
        next: value.slice(0, start) + snippet + value.slice(end),
        selStart: start + selected.length + 4,
        selEnd: start + selected.length + 4 + 'https://'.length,
      }
      break
    }
    case 'table': {
      // GFM 表格：首列重命名留给用户，光标放在第一格内容处
      const snippet = '\n| 列1 | 列2 | 列3 |\n| --- | --- | --- |\n|  |  |  |\n|  |  |  |\n'
      result = {
        next: value.slice(0, start) + snippet + value.slice(end),
        selStart: start + 3, // 落在 "| 列" 里"列"字之后
        selEnd: start + 5,
      }
      break
    }
    case 'sup':
      result = wrapSelection(value, start, end, '<sup>', '</sup>', '上标')
      break
    case 'sub':
      result = wrapSelection(value, start, end, '<sub>', '</sub>', '下标')
      break
    case 'details': {
      // 折叠块：默认折叠，光标停在 summary 文本里
      const snippet = '\n<details>\n<summary>标题</summary>\n\n内容\n\n</details>\n'
      result = {
        next: value.slice(0, start) + snippet + value.slice(end),
        selStart: start + 18, // "<details>\n<summary>" 之后
        selEnd: start + 20,
      }
      break
    }
    default:
      return
  }

  writeValue(target, result.next)
  nextTick(() => {
    const ta = resolveTextarea(target)
    if (!ta) return
    ta.focus()
    ta.setSelectionRange(result.selStart, result.selEnd)
  })
}

function pickTextarea(el: any): HTMLTextAreaElement | null {
  if (!el) return null
  if (el instanceof HTMLTextAreaElement) return el
  // Element Plus el-input 不同版本字段略有差异
  if (el.textarea instanceof HTMLTextAreaElement) return el.textarea
  if (el.ref instanceof HTMLTextAreaElement) return el.ref
  const root = el.$el as HTMLElement | undefined
  return (root?.querySelector?.('textarea') as HTMLTextAreaElement | null) || null
}
function setInlineRef(el: any) {
  inlineTa.value = pickTextarea(el)
}
function setFullRef(el: any) {
  fullTa.value = pickTextarea(el)
}

// 调色板走专用入口：wrapSelection 直接拼 <span style="color:HEX">。
// 选区为空时插 placeholder='重点'，让用户立刻看到颜色效果。
function applyColor(target: ToolbarTarget, hex: string) {
  const el = resolveTextarea(target)
  const value = readValue(target)
  const start = el?.selectionStart ?? value.length
  const end = el?.selectionEnd ?? value.length
  const result = wrapSelection(
    value, start, end,
    `<span style="color:${hex}">`, '</span>', '重点',
  )
  writeValue(target, result.next)
  colorPickerOpen.value = false
  nextTick(() => {
    const ta = resolveTextarea(target)
    if (!ta) return
    ta.focus()
    ta.setSelectionRange(result.selStart, result.selEnd)
  })
}

// 输入符号自动包裹：有选区且按下 * / _ / ~ / ` 时把选区包起来。
// 不抢 Ctrl/Cmd/Alt 组合键，也不抢 IME composition 流程。
// 选区为空时不干预（用户可能就要输入裸字符，例如表格里 `|`）。
const WRAP_KEYS: Record<string, [string, string]> = {
  '*': ['*', '*'],   // 斜体
  _: ['_', '_'],     // 斜体（备选）
  '~': ['~', '~'],   // ~~ 删除线（单 ~ 是 GFM 不识别；用户连按两次才出 strike，行为略 quirky 但安全）
  '`': ['`', '`'],   // 行内代码
}

function onKeydown(e: Event, target: ToolbarTarget) {
  // el-input 的 @keydown 事件签名是 Event；强制收窄到 KeyboardEvent。
  // 不是 keyboard event 时（例如 CompositionEvent）直接放行。
  if (!(e instanceof KeyboardEvent)) return
  if (e.ctrlKey || e.metaKey || e.altKey) return
  if (e.isComposing) return // 中文/日文输入法进行中
  const pair = WRAP_KEYS[e.key]
  if (!pair) return
  const el = resolveTextarea(target)
  if (!el) return
  const start = el.selectionStart
  const end = el.selectionEnd
  if (start === end) return // 无选区 → 不拦，让原生输入走原路径
  e.preventDefault()
  const value = readValue(target)
  const result = wrapSelection(value, start, end, pair[0], pair[1], '文本')
  writeValue(target, result.next)
  nextTick(() => {
    el.focus()
    el.setSelectionRange(result.selStart, result.selEnd)
  })
}
</script>

<template>
  <div class="w-full">
    <div v-if="toolbar" class="md-toolbar mb-1.5">
      <button
        v-for="item in toolbarItems"
        :key="'inline-' + item.action"
        type="button"
        class="md-toolbar__btn"
        :title="item.title"
        @mousedown.prevent
        @click="applyMarkdown('inline', item.action)"
      >{{ item.label }}</button>
      <!-- 调色板单独走 popover：toolbar 已被 13 个按钮撑长，调色板再排一行会很乱。
           折叠进 popover 让"颜色"成为二级入口，命中频次低于其他按钮。 -->
      <el-popover
        :width="180"
        trigger="click"
        v-model:visible="colorPickerOpen"
        placement="bottom-start"
      >
        <template #reference>
          <button
            type="button"
            class="md-toolbar__btn"
            title="字体颜色"
            @mousedown.prevent
          >🎨 颜色</button>
        </template>
        <div class="color-palette">
          <button
            v-for="c in colorPalette"
            :key="c.hex"
            type="button"
            class="color-swatch"
            :title="c.name"
            :style="{ backgroundColor: c.hex }"
            @click="applyColor('inline', c.hex)"
          />
        </div>
      </el-popover>
    </div>

    <div class="relative">
      <el-input
        :ref="setInlineRef"
        class="md-editor-inline-textarea"
        :model-value="modelValue"
        @update:model-value="(v: string) => emit('update:modelValue', v)"
        type="textarea"
        :rows="rows"
        :placeholder="placeholder"
        @paste="onPaste($event, 'inline')"
        @keydown="onKeydown($event, 'inline')"
      />
      <el-button
        class="absolute right-1.5 top-1.5"
        size="small"
        circle
        title="全屏编辑"
        @click="openFullscreen"
      >
        <el-icon><FullScreen /></el-icon>
      </el-button>
    </div>

    <el-dialog
      v-model="fullscreen"
      :title="'全屏编辑 · ' + label"
      fullscreen
      destroy-on-close
      append-to-body
    >
      <div class="flex h-full flex-col gap-3">
        <div v-if="toolbar" class="md-toolbar">
          <button
            v-for="item in toolbarItems"
            :key="'full-' + item.action"
            type="button"
            class="md-toolbar__btn"
            :title="item.title"
            @mousedown.prevent
            @click="applyMarkdown('full', item.action)"
          >{{ item.label }}</button>
          <el-popover
            :width="180"
            trigger="click"
            v-model:visible="colorPickerOpen"
            placement="bottom-start"
          >
            <template #reference>
              <button
                type="button"
                class="md-toolbar__btn"
                title="字体颜色"
                @mousedown.prevent
              >🎨 颜色</button>
            </template>
            <div class="color-palette">
              <button
                v-for="c in colorPalette"
                :key="c.hex"
                type="button"
                class="color-swatch"
                :title="c.name"
                :style="{ backgroundColor: c.hex }"
                @click="applyColor('full', c.hex)"
              />
            </div>
          </el-popover>
        </div>

        <div class="flex min-h-0 flex-1 gap-4">
          <!-- 编辑区：占左 2/3，全高 -->
          <div class="flex min-h-0 flex-col" style="flex: 2 1 0; min-width: 0">
            <el-input
              :ref="setFullRef"
              v-model="draft"
              class="md-editor-full-textarea flex-1"
              type="textarea"
              :autosize="{ minRows: 20 }"
              :placeholder="placeholder || '在此输入内容，支持 Markdown；可直接粘贴图片自动上传'"
              @paste="onPaste($event, 'full')"
              @keydown="onKeydown($event, 'full')"
            />
          </div>
          <!-- 预览区：占右 1/3，题干/解析为 markdown，标题等纯文本不预览 -->
          <div
            v-if="preview"
            class="overflow-auto rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-gray-700 dark:bg-gray-800/50"
            style="flex: 1 1 0; min-width: 0"
          >
            <div class="mb-2 text-xs font-medium text-gray-400">实时预览</div>
            <div class="md-content leading-relaxed text-gray-700 dark:text-gray-300" v-html="previewHtml" />
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="closeFullscreen">取消</el-button>
        <el-button type="primary" @click="closeFullscreen">完成编辑</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.md-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 6px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #f8fafc;
}
.md-toolbar__btn {
  min-width: 34px;
  height: 28px;
  padding: 0 8px;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  background: #fff;
  color: #334155;
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
  cursor: pointer;
  transition: all .15s ease;
}
.md-toolbar__btn:hover {
  border-color: #93c5fd;
  color: #1d4ed8;
  background: #eff6ff;
}
:global(.dark) .md-toolbar {
  border-color: #374151;
  background: #111827;
}
:global(.dark) .md-toolbar__btn {
  border-color: #4b5563;
  background: #1f2937;
  color: #e5e7eb;
}
:global(.dark) .md-toolbar__btn:hover {
  border-color: #60a5fa;
  color: #93c5fd;
  background: #1e3a8a33;
}

/* 调色板：5 列 7 色网格。圆点配细描边，深色模式下边框变浅避免淹没 */
.color-palette {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 6px;
  padding: 4px;
}
.color-swatch {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  border: 1px solid rgba(0, 0, 0, 0.15);
  cursor: pointer;
  transition: transform .12s ease;
}
.color-swatch:hover {
  transform: scale(1.15);
}
:global(.dark) .color-swatch {
  border-color: rgba(255, 255, 255, 0.2);
}
</style>
