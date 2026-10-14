<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { aiApi } from '@/api'
import { useAI } from '@/composables/useAI'

// 案例题（主观题）AI 判分。传入 question（含 title/content/analysis/explanation/score），
// 以及可选的 userAnswer。有 userAnswer 时点击直接判分（练习/刷题已作答场景）；
// 没有时组件内嵌输入框，用户粘贴自己的答案再判分（题目详情页场景）。
const props = defineProps<{
  question: any
  userAnswer?: string
}>()

const { aiEnabled, showTrial, trialRemaining, ensureAIStatus, refreshAIStatus } = useAI()
onMounted(ensureAIStatus)

const loading = ref(false)
const input = ref(props.userAnswer || '')
const result = ref<any>(null)

const scoreColor = computed(() => {
  const t = Number(result.value?.total) || 0
  const f = Number(result.value?.full) || 0
  const pct = f > 0 ? t / f : 0
  if (pct >= 0.7) return 'text-emerald-600 dark:text-emerald-400'
  if (pct >= 0.4) return 'text-blue-600 dark:text-blue-400'
  return 'text-orange-600 dark:text-orange-400'
})

function barColor(pct: number) {
  if (pct >= 0.7) return 'bg-emerald-500'
  if (pct >= 0.4) return 'bg-blue-500'
  return 'bg-orange-400'
}

function grade() {
  const q = props.question
  if (!q || loading.value) return
  const ans = input.value.trim()
  if (ans.length < 5) {
    return
  }
  loading.value = true
  result.value = null
  aiApi
    .gradeCase({
      title: q.title || '',
      content: q.content || '',
      reference: q.analysis || '',
      explanation: q.explanation || '',
      user_answer: ans,
      score: Number(q.score) || 0,
    })
    .then((res: any) => {
      result.value = res.data
      if (showTrial.value) refreshAIStatus()
    })
    .catch(() => {
      // 拦截器已提示
    })
    .finally(() => {
      loading.value = false
    })
}
</script>

<template>
  <div v-if="aiEnabled" class="mt-1">
    <!-- 无预填答案时，内嵌输入框让用户粘贴自己的作答 -->
    <textarea
      v-if="!userAnswer"
      v-model="input"
      rows="3"
      :disabled="loading"
      placeholder="粘贴你的答案，AI 帮你判断能不能得分…"
      class="mb-2 w-full resize-y rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm focus:border-violet-400 focus:outline-none disabled:opacity-60 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
    />

    <div class="flex items-center gap-2">
      <button
        class="inline-flex items-center gap-1 rounded-lg border border-violet-200 bg-violet-50 px-3 py-1.5 text-sm font-medium text-violet-700 transition hover:bg-violet-100 disabled:opacity-60 dark:border-violet-500/30 dark:bg-violet-500/10 dark:text-violet-300"
        :disabled="loading || !input.trim() || input.trim().length < 5"
        @click="grade"
      >
        <el-icon><MagicStick /></el-icon>
        {{ loading ? 'AI 判分中…' : (result ? '重新判分' : 'AI 判分') }}
      </button>
      <span v-if="showTrial" class="text-xs text-gray-400">
        免费试用剩 {{ trialRemaining }} 次
      </span>
      <span v-if="input && input.trim().length > 0 && input.trim().length < 5" class="text-xs text-gray-400">
        作答太短（至少 5 字）
      </span>
    </div>

    <!-- 评分卡 -->
    <div
      v-if="result"
      class="mt-2 rounded-xl border border-violet-200 bg-violet-50/60 p-4 dark:border-violet-500/30 dark:bg-violet-500/10"
    >
      <div class="mb-3 flex items-start gap-4">
        <div class="shrink-0 text-center">
          <div class="text-3xl font-bold leading-none" :class="scoreColor">
            {{ result.total ?? '—' }}
          </div>
          <div class="mt-1 text-xs text-slate-400">
            / {{ result.full ?? '满分' }} 分
          </div>
        </div>
        <p v-if="result.summary" class="min-w-0 flex-1 text-sm leading-relaxed text-slate-600 dark:text-slate-300">
          {{ result.summary }}
        </p>
      </div>

      <div v-if="result.dimensions?.length" class="mb-3 space-y-3">
        <div v-for="d in result.dimensions" :key="d.name" :class="{ 'pb-2 border-b border-violet-200/60 dark:border-violet-500/20': d.comment }">
          <div class="flex items-center justify-between text-sm">
            <span class="font-medium text-slate-700 dark:text-slate-200">{{ d.name }}</span>
            <span class="text-slate-500">{{ d.score ?? 0 }} / {{ d.full ?? 0 }}</span>
          </div>
          <div class="mt-1 h-2 overflow-hidden rounded-full bg-slate-200 dark:bg-slate-700">
            <div
              class="h-full rounded-full transition-all"
              :class="barColor(Number(d.full) > 0 ? Number(d.score) / Number(d.full) : 0)"
              :style="{ width: (Number(d.full) > 0 ? Math.min(100, Math.round(Number(d.score) / Number(d.full) * 100)) : 0) + '%' }"
            />
          </div>
          <p v-if="d.comment" class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ d.comment }}</p>
        </div>
      </div>

      <ul v-if="result.suggestions?.length" class="space-y-1.5">
        <li v-for="(s, i) in result.suggestions" :key="i" class="flex items-start gap-2 text-sm text-slate-600 dark:text-slate-300">
          <span class="mt-0.5 shrink-0 text-violet-500">•</span><span>{{ s }}</span>
        </li>
      </ul>
    </div>
  </div>
</template>
