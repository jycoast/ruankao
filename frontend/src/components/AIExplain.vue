<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { streamPost } from '@/utils/aiStream'
import { renderMd } from '@/utils/markdown'
import { useAI } from '@/composables/useAI'

// 可复用的「AI 解析」按钮 + 结果面板。放在任意已展示答案/解析的题目区域。
// 传入 question（含 title/content/options），可选 answer 覆盖（否则从选项 is_correct 推导）。
const props = defineProps<{
  question: any
  answer?: string
}>()

const { aiEnabled, showTrial, trialRemaining, ensureAIStatus, refreshAIStatus } = useAI()
onMounted(ensureAIStatus)

const loading = ref(false)
const explanation = ref('')
let abort: (() => void) | null = null

function explain() {
  const q = props.question
  if (!q || loading.value) return
  loading.value = true
  explanation.value = ''

  const opts = (q.options || []) as { label: string; content: string; is_correct: number }[]
  const options = opts.map((o) => `${o.label}. ${o.content}`).join('\n')
  const answer =
    props.answer ||
    opts.filter((o) => o.is_correct === 1).map((o) => o.label).join('')

  const finish = () => {
    loading.value = false
    abort = null
    if (showTrial.value) refreshAIStatus()
  }
  abort = streamPost(
    '/ai/explain-stream',
    { title: q.title || '', content: q.content || '', options, answer },
    {
      onDelta: (text) => {
        explanation.value += text
      },
      onError: (msg) => {
        if (!explanation.value) explanation.value = msg
        finish()
      },
      onDone: finish,
    },
  )
}

// 组件卸载时中止未完成的流
onBeforeUnmount(() => abort?.())
</script>

<template>
  <div v-if="aiEnabled" class="mt-1">
    <button
      class="inline-flex items-center gap-1 rounded-lg border border-violet-200 bg-violet-50 px-3 py-1.5 text-sm font-medium text-violet-700 transition hover:bg-violet-100 disabled:opacity-60 dark:border-violet-500/30 dark:bg-violet-500/10 dark:text-violet-300"
      :disabled="loading"
      @click="explain"
    >
      <el-icon><MagicStick /></el-icon>
      {{ loading ? 'AI 解析中…' : (explanation ? '重新解析' : '看不懂？让 AI 解析') }}
    </button>
    <span v-if="showTrial" class="ml-2 text-xs text-gray-400">
      免费试用剩 {{ trialRemaining }} 次
    </span>

    <div
      v-if="explanation"
      class="mt-2 rounded-lg border border-violet-200 bg-violet-50/60 p-3 dark:border-violet-500/30 dark:bg-violet-500/10"
    >
      <div class="mb-1 flex items-center gap-1 text-sm font-medium text-violet-700 dark:text-violet-300">
        <el-icon><MagicStick /></el-icon> AI 解析
      </div>
      <div class="md-content text-sm leading-relaxed text-slate-700 dark:text-slate-200" v-html="renderMd(explanation)" />
    </div>
  </div>
</template>
