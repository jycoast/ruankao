<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useExamCountdown } from '@/composables/useExamCountdown'
import { SPRINT_WORDS } from '@/data/sprintWords'
import { SPRINT_CALC } from '@/data/sprintCalc'
import { SPRINT_POINTS } from '@/data/sprintPoints'
import { useAI } from '@/composables/useAI'
import { useUserStore } from '@/stores/user'
import { guardGuest } from '@/utils/guestGate'

const router = useRouter()
const userStore = useUserStore()

// 供模板区分“游客点击即锁”的卡(模拟卷/ AI 出题)
interface SprintEntry {
  to: string
  label: string
  desc: string
  icon: string
  tone: string
  ai: boolean
  needLogin?: boolean
}
const { countdown } = useExamCountdown()
const { aiEnabled, ensureAIStatus } = useAI()

const masteredCount = ref(0)
onMounted(() => {
  ensureAIStatus()
  try {
    const raw = localStorage.getItem('sprint_words_mastered')
    masteredCount.value = raw ? (JSON.parse(raw) as string[]).length : 0
  } catch {}
})

const wordProgress = computed(() => {
  const total = SPRINT_WORDS.length
  const pct = total ? Math.round((masteredCount.value / total) * 100) : 0
  return { total, done: masteredCount.value, pct }
})

const pointCount = computed(() => SPRINT_POINTS.reduce((s, g) => s + g.items.length, 0))

const entries = computed<SprintEntry[]>(() => {
  const base: SprintEntry[] = [
    { to: '/sprint/mock-exam', label: '生成模拟卷', desc: '历年真题随机组卷 · 模拟考试', icon: 'Tickets', tone: 'text-indigo-600 bg-indigo-50 dark:bg-indigo-500/15', ai: false, needLogin: true },
    { to: '/sprint/calc', label: '计算题汇总', desc: `${SPRINT_CALC.length} 类公式+例题+易错`, icon: 'Histogram', tone: 'text-blue-600 bg-blue-50 dark:bg-blue-500/15', ai: false },
    { to: '/sprint/calc?tab=formula', label: '公式速查表', desc: '高频公式一页速览', icon: 'Odometer', tone: 'text-emerald-600 bg-emerald-50 dark:bg-emerald-500/15', ai: false },
    { to: '/sprint/words', label: '英语单词', desc: `${SPRINT_WORDS.length} 高频术语 · 翻卡背诵`, icon: 'Notebook', tone: 'text-orange-600 bg-orange-50 dark:bg-orange-500/15', ai: false },
    { to: '/sprint/points', label: '必考速记', desc: `${pointCount.value} 条分章要点`, icon: 'Collection', tone: 'text-blue-600 bg-blue-50 dark:bg-blue-500/15', ai: false },
    { to: '/sprint/points?tab=mistakes', label: '易错集锦', desc: '高频陷阱逐条排雷', icon: 'Warning', tone: 'text-rose-600 bg-rose-50 dark:bg-rose-500/15', ai: false },
    { to: '/sprint/skills', label: '答题套路', desc: '案例要点 + 论文模板', icon: 'Tickets', tone: 'text-emerald-600 bg-emerald-50 dark:bg-emerald-500/15', ai: false },
  ]
  // AI 出题:对游客 aiEnabled 恒 false,单独放开“带锁显示”(点击由 guardGuest 拦);
  // 对真用户仍只在 AI 全局启用时显示。
  if (aiEnabled.value || userStore.isGuest) {
    base.push({ to: '/sprint/ai-questions', label: 'AI 出题', desc: '按知识点即时生成练习题', icon: 'MagicStick', tone: 'text-violet-600 bg-violet-50 dark:bg-violet-500/15', ai: true, needLogin: true })
  }
  return base
})

// 游客点击带锁卡(生成模拟卷/ AI 出题):弹登录引导,不进入作答/AI 页
function openEntry(e: SprintEntry) {
  if (userStore.isGuest && e.needLogin) {
    guardGuest(e.label)
    return
  }
  router.push(e.to)
}

const checklist = [
  '每天背 1 组单词，翻卡巩固易忘词',
  '计算题按类型过一遍公式与例题',
  '必考速记分章默写，查漏补缺',
  '易错集锦考前一天快速扫读',
  '论文准备 1–2 个熟悉项目素材',
]
</script>

<template>
  <div class="space-y-6">
    <!-- 倒计时冲刺焦点 -->
    <section class="relative overflow-hidden rounded-2xl bg-gradient-to-br from-orange-500 to-amber-500 p-6 text-white shadow-sm">
      <div class="relative z-10 flex flex-wrap items-end justify-between gap-4">
        <div>
          <div class="text-lg font-semibold">🔥 考前冲刺</div>
          <p class="mt-1.5 text-sm text-orange-50">集中攻克计算题、英语单词、必考点与易错点，稳步冲线</p>
        </div>
        <div class="text-right">
          <div class="text-xs text-orange-50">距 {{ countdown.label }}</div>
          <div class="flex items-end justify-end gap-1">
            <span class="text-4xl font-bold leading-none">{{ countdown.days }}</span>
            <span class="mb-1 text-sm">天</span>
          </div>
        </div>
      </div>
      <el-icon class="pointer-events-none absolute -right-4 -bottom-4 text-white/10" :size="140"><Timer /></el-icon>
    </section>

    <!-- 板块入口 -->
    <section class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4">
      <button
        v-for="e in entries"
        :key="e.to"
        class="group flex items-start gap-3 rounded-xl border border-slate-200 bg-white p-4 text-left shadow-sm transition hover:-translate-y-0.5 hover:border-blue-200 hover:shadow-md dark:border-slate-800 dark:bg-slate-900"
        @click="openEntry(e)"
      >
        <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg" :class="e.tone">
          <el-icon :size="22"><component :is="e.icon" /></el-icon>
        </div>
        <div class="min-w-0">
          <div class="flex items-center gap-1.5">
            <span class="font-semibold text-slate-900 group-hover:text-blue-600 dark:text-white">{{ e.label }}</span>
            <span v-if="e.ai" class="rounded bg-violet-100 px-1.5 py-0.5 text-[10px] font-medium text-violet-700 dark:bg-violet-500/20 dark:text-violet-300">AI</span>
            <span
              v-if="userStore.isGuest && e.needLogin"
              class="rounded bg-orange-100 px-1.5 py-0.5 text-[10px] font-medium text-orange-600 dark:bg-orange-500/20 dark:text-orange-300"
            >登录</span>
          </div>
          <div class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ e.desc }}</div>
        </div>
      </button>
    </section>

    <div class="grid gap-6 lg:grid-cols-[1fr_340px]">
      <!-- 背单词进度 -->
      <section class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div class="flex items-center justify-between">
          <h2 class="font-semibold text-slate-950 dark:text-white">背单词进度</h2>
          <router-link to="/sprint/words" class="text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-300">去背单词</router-link>
        </div>
        <div class="mt-4 flex items-center justify-between text-sm">
          <span class="text-slate-500 dark:text-slate-400">已掌握</span>
          <span class="font-medium text-slate-700 dark:text-slate-200">{{ wordProgress.done }} / {{ wordProgress.total }} · {{ wordProgress.pct }}%</span>
        </div>
        <div class="mt-2 h-2.5 overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800">
          <div class="h-full rounded-full bg-emerald-500 transition-all" :style="{ width: wordProgress.pct + '%' }" />
        </div>
        <p class="mt-3 text-xs text-slate-400">翻卡时点「认识」即计入已掌握，进度自动保存在本机。</p>
      </section>

      <!-- 冲刺清单 -->
      <aside class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <h2 class="font-semibold text-slate-950 dark:text-white">冲刺清单</h2>
        <ul class="mt-3 space-y-2.5">
          <li v-for="(c, i) in checklist" :key="i" class="flex items-start gap-2 text-sm text-slate-600 dark:text-slate-300">
            <el-icon class="mt-0.5 shrink-0 text-orange-500"><CircleCheck /></el-icon>
            <span>{{ c }}</span>
          </li>
        </ul>
      </aside>
    </div>
  </div>
</template>
