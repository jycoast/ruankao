<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { userApi } from '@/api'
import { useSubjectStore } from '@/stores/subject'
import { guardGuest } from '@/utils/guestGate'

const router = useRouter()
const subjectStore = useSubjectStore()
const loading = ref(true)

interface TrendPoint { date: string; total: number; correct: number }
interface KpPoint { knowledge_point: string; done: number; correct: number; accuracy: number }
interface SpeedStat {
  avg_seconds: number
  this_week_avg: number
  last_week_avg: number
  this_week_count: number
  faster_percent: number
  has_comparison: boolean
}

const report = ref({
  total_practice: 0,
  total_correct: 0,
  total_time: 0,
  streak_days: 0,
  accuracy: 0,
  trend: [] as TrendPoint[],
  knowledge: [] as KpPoint[],
  speed: null as SpeedStat | null,
})

function goDaily() {
  if (!guardGuest('每日一练')) return
  router.push('/daily')
}

async function load() {
  loading.value = true
  try {
    const res: any = await userApi.getReport(subjectStore.currentSubjectId)
    const d = res.data || {}
    report.value = {
      total_practice: d.total_practice || 0,
      total_correct: d.total_correct || 0,
      total_time: d.total_time || 0,
      streak_days: d.streak_days || 0,
      accuracy: d.accuracy || 0,
      trend: d.trend || [],
      knowledge: d.knowledge || [],
      speed: d.speed || null,
    }
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => subjectStore.currentSubjectId, load)

// 学习时长格式化（秒 -> 时/分）
const studyTime = computed(() => {
  const sec = report.value.total_time
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (h > 0) return `${h}h${m}m`
  return `${m}m`
})

// ---------- 折线图（近30日答题量） ----------
const CHART_W = 700
const CHART_H = 220
const PAD_L = 36
const PAD_R = 12
const PAD_T = 16
const PAD_B = 28

const trend = computed(() => report.value.trend)
const maxTotal = computed(() => Math.max(1, ...trend.value.map((t) => t.total)))

function xPos(i: number) {
  const n = trend.value.length
  if (n <= 1) return PAD_L
  return PAD_L + (i / (n - 1)) * (CHART_W - PAD_L - PAD_R)
}
function yPos(v: number) {
  return PAD_T + (1 - v / maxTotal.value) * (CHART_H - PAD_T - PAD_B)
}

const totalLine = computed(() =>
  trend.value.map((t, i) => `${xPos(i)},${yPos(t.total)}`).join(' '),
)
const correctLine = computed(() =>
  trend.value.map((t, i) => `${xPos(i)},${yPos(t.correct)}`).join(' '),
)
const totalArea = computed(() => {
  if (!trend.value.length) return ''
  const pts = trend.value.map((t, i) => `${xPos(i)},${yPos(t.total)}`).join(' ')
  const baseY = yPos(0)
  return `${xPos(0)},${baseY} ${pts} ${xPos(trend.value.length - 1)},${baseY}`
})

// Y 轴刻度（0, 中, 最大）
const yTicks = computed(() => {
  const mx = maxTotal.value
  return [0, Math.round(mx / 2), mx]
})
// X 轴标签：只显示首、中、尾日期（MM-DD）
const xLabels = computed(() => {
  const n = trend.value.length
  if (!n) return []
  const idxs = n === 1 ? [0] : [0, Math.floor(n / 2), n - 1]
  return idxs.map((i) => ({ x: xPos(i), text: (trend.value[i].date || '').slice(5) }))
})

const hasTrend = computed(() => trend.value.some((t) => t.total > 0))

// ---------- 做题速度 ----------
const speed = computed(() => report.value.speed)

function fmtSec(sec: number) {
  if (sec < 60) return `${sec} 秒`
  return `${Math.floor(sec / 60)} 分 ${sec % 60} 秒`
}

const speedTip = computed(() => {
  const s = speed.value
  if (!s) return ''
  if (!s.has_comparison) return '本周共 ' + s.this_week_count + ' 题，再练几天就能和上周对比'
  const p = s.faster_percent
  if (p >= 5) return `比上周快了 ${p}%，手感正好 🚀`
  if (p <= -5) return `比上周慢了 ${-p}%，可能在啃硬题`
  return '和上周节奏基本一致'
})

// ---------- 知识点掌握排行 ----------
const knowledge = computed(() => report.value.knowledge)
function accColor(acc: number) {
  if (acc >= 80) return 'bg-green-500'
  if (acc >= 60) return 'bg-blue-500'
  if (acc >= 40) return 'bg-amber-500'
  return 'bg-red-500'
}
</script>

<template>
  <div class="mx-auto max-w-5xl px-4 py-6">
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">学习报告</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">你的备考数据全景，用数据驱动复习</p>
      </div>
      <el-button @click="goDaily">去每日一练</el-button>
    </div>

    <div v-if="loading" class="py-20 text-center text-gray-400">加载中...</div>

    <template v-else>
      <!-- 概览卡片 -->
      <div class="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <div class="text-sm text-gray-500 dark:text-gray-400">累计练习</div>
          <div class="mt-1 text-3xl font-bold text-gray-900 dark:text-gray-100">{{ report.total_practice }}</div>
          <div class="mt-1 text-xs text-gray-400">答对 {{ report.total_correct }} 题</div>
        </div>
        <div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <div class="text-sm text-gray-500 dark:text-gray-400">正确率</div>
          <div class="mt-1 text-3xl font-bold" :class="report.accuracy >= 60 ? 'text-green-500' : 'text-amber-500'">{{ report.accuracy }}%</div>
          <div class="mt-1 text-xs text-gray-400">累计平均</div>
        </div>
        <div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <div class="text-sm text-gray-500 dark:text-gray-400">学习时长</div>
          <div class="mt-1 text-3xl font-bold text-gray-900 dark:text-gray-100">{{ studyTime }}</div>
          <div class="mt-1 text-xs text-gray-400">累计投入</div>
        </div>
        <div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <div class="text-sm text-gray-500 dark:text-gray-400">连续学习</div>
          <div class="mt-1 text-3xl font-bold text-orange-500">{{ report.streak_days }}<span class="ml-1 text-sm font-normal text-gray-400">天</span></div>
          <div class="mt-1 text-xs text-gray-400">🔥 保持节奏</div>
        </div>
      </div>

      <!-- 做题速度 -->
      <div v-if="speed" class="mb-6 rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
        <div class="mb-4 flex items-center justify-between">
          <h2 class="font-semibold text-gray-900 dark:text-gray-100">做题速度</h2>
          <span class="text-xs text-gray-400">{{ speedTip }}</span>
        </div>
        <div class="grid grid-cols-2 gap-4 sm:grid-cols-3">
          <div>
            <div class="text-xs text-gray-500 dark:text-gray-400">累计平均每题</div>
            <div class="mt-1 text-2xl font-bold text-gray-900 dark:text-gray-100">{{ fmtSec(speed.avg_seconds) }}</div>
          </div>
          <div>
            <div class="text-xs text-gray-500 dark:text-gray-400">本周平均每题</div>
            <div class="mt-1 flex items-baseline gap-2">
              <span class="text-2xl font-bold text-blue-500">{{ speed.this_week_avg ? fmtSec(speed.this_week_avg) : '—' }}</span>
              <span
                v-if="speed.has_comparison && speed.faster_percent !== 0"
                class="text-sm font-medium"
                :class="speed.faster_percent > 0 ? 'text-green-500' : 'text-amber-500'"
              >
                {{ speed.faster_percent > 0 ? '↓' : '↑' }}{{ Math.abs(speed.faster_percent) }}%
              </span>
            </div>
            <div class="mt-1 text-xs text-gray-400">共 {{ speed.this_week_count }} 题</div>
          </div>
          <div class="col-span-2 sm:col-span-1">
            <div class="text-xs text-gray-500 dark:text-gray-400">上周平均每题</div>
            <div class="mt-1 text-2xl font-bold text-gray-400">{{ speed.last_week_avg ? fmtSec(speed.last_week_avg) : '—' }}</div>
          </div>
        </div>
      </div>

      <!-- 近30日趋势折线图 -->
      <div class="mb-6 rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
        <div class="mb-4 flex items-center justify-between">
          <h2 class="font-semibold text-gray-900 dark:text-gray-100">近 30 日练习趋势</h2>
          <div class="flex items-center gap-4 text-xs text-gray-500">
            <span class="flex items-center gap-1"><span class="inline-block h-2 w-3 rounded bg-blue-500"></span>答题量</span>
            <span class="flex items-center gap-1"><span class="inline-block h-2 w-3 rounded bg-green-500"></span>答对数</span>
          </div>
        </div>
        <div v-if="hasTrend" class="overflow-x-auto">
          <svg :viewBox="`0 0 ${CHART_W} ${CHART_H}`" class="w-full" :style="{ minWidth: '480px' }">
            <!-- 网格 + Y 轴刻度 -->
            <g>
              <line
                v-for="(t, i) in yTicks"
                :key="'g' + i"
                :x1="PAD_L" :x2="CHART_W - PAD_R"
                :y1="yPos(t)" :y2="yPos(t)"
                stroke="currentColor" stroke-width="1"
                class="text-gray-200 dark:text-gray-700"
              />
              <text
                v-for="(t, i) in yTicks"
                :key="'yt' + i"
                :x="PAD_L - 6" :y="yPos(t) + 4"
                text-anchor="end" font-size="11"
                fill="currentColor" class="text-gray-400"
              >{{ t }}</text>
            </g>
            <!-- 面积 -->
            <polygon :points="totalArea" fill="rgb(59 130 246)" fill-opacity="0.08" />
            <!-- 答题量线 -->
            <polyline :points="totalLine" fill="none" stroke="rgb(59 130 246)" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />
            <!-- 答对数线 -->
            <polyline :points="correctLine" fill="none" stroke="rgb(34 197 94)" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />
            <!-- X 轴标签 -->
            <text
              v-for="(l, i) in xLabels"
              :key="'xl' + i"
              :x="l.x" :y="CHART_H - 8"
              text-anchor="middle" font-size="11"
              fill="currentColor" class="text-gray-400"
            >{{ l.text }}</text>
          </svg>
        </div>
        <div v-else class="py-12 text-center text-sm text-gray-400">近 30 日还没有练习记录，快去刷题吧</div>
      </div>

      <!-- 知识点掌握排行 -->
      <div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
        <h2 class="mb-4 font-semibold text-gray-900 dark:text-gray-100">知识点掌握排行</h2>
        <div v-if="knowledge.length" class="space-y-3">
          <div v-for="kp in knowledge" :key="kp.knowledge_point">
            <div class="mb-1 flex items-center justify-between text-sm">
              <span class="truncate pr-2 text-gray-700 dark:text-gray-300">{{ kp.knowledge_point }}</span>
              <span class="shrink-0 text-gray-400">{{ kp.correct }}/{{ kp.done }} · {{ kp.accuracy }}%</span>
            </div>
            <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
              <div class="h-full rounded-full transition-all" :class="accColor(kp.accuracy)" :style="{ width: kp.accuracy + '%' }"></div>
            </div>
          </div>
        </div>
        <div v-else class="py-12 text-center text-sm text-gray-400">还没有知识点练习数据</div>
      </div>
    </template>
  </div>
</template>
