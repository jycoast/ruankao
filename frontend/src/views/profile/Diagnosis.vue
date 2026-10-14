<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRouter } from 'vue-router'
import { diagnosisApi } from '@/api'
import { useSubjectStore } from '@/stores/subject'
import { requireMember } from '@/utils/memberGate'
import { streamPost } from '@/utils/aiStream'
import { renderMd } from '@/utils/markdown'

// 会员专属「AI 诊断报告」：错题本 + 答题流水聚合出薄弱点画像，AI 生成复习建议。
// 整页会员专属（后端 /api/diagnosis 挂 RequireMembership），不给免费试用；游客由路由守卫弹引导。

const router = useRouter()
const subjectStore = useSubjectStore()
const loading = ref(true)

interface DiagnosisCategory {
  category: string
  total: number
  done: number
  correct: number
  wrong: number
  accuracy: number
  level: number // 0强 1中 2弱 3极弱 4未开始
}

interface DiagnosisSummary {
  subject_id: number
  wrong_total: number
  due_count: number
  weak_category_n: number
  done: number
  accuracy: number
  has_data: boolean
  categories: DiagnosisCategory[]
}

const summary = ref<DiagnosisSummary>({
  subject_id: 0,
  wrong_total: 0,
  due_count: 0,
  weak_category_n: 0,
  done: 0,
  accuracy: 0,
  has_data: false,
  categories: [],
})

async function load() {
  loading.value = true
  try {
    const res: any = await diagnosisApi.summary(subjectStore.currentSubjectId)
    const d = res.data || {}
    summary.value = {
      subject_id: d.subject_id || 0,
      wrong_total: d.wrong_total || 0,
      due_count: d.due_count || 0,
      weak_category_n: d.weak_category_n || 0,
      done: d.done || 0,
      accuracy: d.accuracy || 0,
      has_data: !!d.has_data,
      categories: d.categories || [],
    }
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (!requireMember('diagnosis')) return
  load()
})
watch(() => subjectStore.currentSubjectId, load)

// ---------- 薄弱点画像 ----------
// 弱度分档与后端 diagnosisLevel / 报告页 accColor 同阈（>=80 强 / >=60 中 / >=40 弱 / <40 极弱）
function accColor(acc: number) {
  if (acc >= 80) return 'bg-green-500'
  if (acc >= 60) return 'bg-blue-500'
  if (acc >= 40) return 'bg-amber-500'
  return 'bg-red-500'
}

function levelLabel(level: number) {
  if (level === 4) return '未开始'
  if (level === 0) return '扎实'
  if (level === 1) return '一般'
  if (level === 2) return '薄弱'
  return '极弱'
}

// 进度条配色：未开始走灰条，其余按正确率分档
function barColor(c: DiagnosisCategory) {
  return c.level === 4 ? 'bg-gray-300 dark:bg-gray-600' : accColor(c.accuracy)
}

// ---------- AI 诊断建议 ----------
const aiText = ref('')
const aiLoading = ref(false)
let abort: (() => void) | null = null

function generate() {
  if (aiLoading.value) return
  aiLoading.value = true
  aiText.value = ''
  const finish = () => {
    aiLoading.value = false
    abort = null
  }
  abort = streamPost(
    '/diagnosis/ai',
    { subject_id: subjectStore.currentSubjectId },
    {
      onDelta: (text) => {
        aiText.value += text
      },
      onError: (msg) => {
        // AI 失败不影响画像（统计已独立加载），错误就地展示在 AI 卡内
        aiText.value = `⚠️ 生成失败：${msg}`
        finish()
      },
      onDone: finish,
    },
  )
}

// 组件卸载/切科目时中止未完成的流，避免切走后回调还在写已卸载的 ref
onBeforeUnmount(() => abort?.())
</script>

<template>
  <div class="mx-auto max-w-5xl px-4 py-6">
    <div class="mb-6">
      <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">AI 诊断报告</h1>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        错题本 + 答题流水专属分析，看清薄弱点再复习
      </p>
    </div>

    <div v-if="loading" class="py-20 text-center text-gray-400">加载中...</div>

    <template v-else>
      <!-- 空态：没做过客观题也没错题本存量 → 引导刷题 -->
      <div v-if="!summary.has_data" class="rounded-xl border border-gray-200 bg-white p-12 text-center shadow-sm dark:border-gray-700 dark:bg-gray-800">
        <p class="text-gray-500 dark:text-gray-300">先刷几组题再回来诊断，AI 需要你的作答数据来找出薄弱点</p>
        <el-button class="mt-4" type="primary" @click="router.push('/practice')">去刷题</el-button>
      </div>

      <template v-else>
        <!-- KPI 卡 -->
        <div class="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
          <div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
            <div class="text-sm text-gray-500 dark:text-gray-400">错题总数</div>
            <div class="mt-1 text-3xl font-bold text-gray-900 dark:text-gray-100">{{ summary.wrong_total }}</div>
            <div class="mt-1 text-xs text-gray-400">错题本存量（本科目）</div>
          </div>
          <div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
            <div class="text-sm text-gray-500 dark:text-gray-400">待复习</div>
            <div class="mt-1 text-3xl font-bold text-orange-500">{{ summary.due_count }}</div>
            <div class="mt-1 text-xs text-gray-400">到期的错题</div>
          </div>
          <div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
            <div class="text-sm text-gray-500 dark:text-gray-400">薄弱知识点</div>
            <div class="mt-1 text-3xl font-bold text-red-500">{{ summary.weak_category_n }}</div>
            <div class="mt-1 text-xs text-gray-400">正确率 &lt; 60% 的大类</div>
          </div>
          <div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
            <div class="text-sm text-gray-500 dark:text-gray-400">客观题正确率</div>
            <div class="mt-1 text-3xl font-bold" :class="summary.accuracy >= 60 ? 'text-green-500' : 'text-amber-500'">{{ summary.accuracy }}%</div>
            <div class="mt-1 text-xs text-gray-400">做 {{ summary.done }} 题</div>
          </div>
        </div>

        <!-- 薄弱点画像 -->
        <div class="mb-6 rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <div class="mb-4 flex items-center justify-between">
            <h2 class="font-semibold text-gray-900 dark:text-gray-100">薄弱点画像</h2>
            <span class="text-xs text-gray-400">按知识大类，最近一次作答正确率</span>
          </div>
          <div v-if="summary.categories.length" class="space-y-3">
            <div v-for="c in summary.categories" :key="c.category">
              <div class="mb-1 flex items-center justify-between text-sm">
                <span class="truncate pr-2 text-gray-700 dark:text-gray-300">{{ c.category }}</span>
                <span class="flex shrink-0 items-center gap-2">
                  <span v-if="c.level === 4" class="text-xs text-gray-400">{{ c.total }} 题未开始</span>
                  <template v-else>
                    <span class="text-gray-400">{{ c.done }}/{{ c.total }} · {{ c.accuracy }}%</span>
                    <span class="rounded px-1.5 py-0.5 text-xs font-medium" :class="{
                      'bg-green-100 text-green-700 dark:bg-green-500/15 dark:text-green-400': c.level === 0,
                      'bg-blue-100 text-blue-700 dark:bg-blue-500/15 dark:text-blue-400': c.level === 1,
                      'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400': c.level === 2,
                      'bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-400': c.level === 3,
                    }">{{ levelLabel(c.level) }}</span>
                  </template>
                </span>
              </div>
              <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                <div class="h-full rounded-full transition-all" :class="barColor(c)" :style="{ width: (c.level === 4 ? 0 : c.accuracy) + '%' }"></div>
              </div>
            </div>
          </div>
          <div v-else class="py-8 text-center text-sm text-gray-400">
            还没有客观题作答数据，先刷几组题再来看画像
          </div>
        </div>

        <!-- AI 诊断建议 -->
        <div class="rounded-xl border border-violet-200 bg-white p-5 shadow-sm dark:border-violet-500/30 dark:bg-gray-800">
          <div class="mb-4 flex items-center justify-between">
            <h2 class="flex items-center gap-1 font-semibold text-gray-900 dark:text-gray-100">
              <el-icon class="text-violet-500"><MagicStick /></el-icon>
              AI 复习建议
            </h2>
            <el-button type="primary" :loading="aiLoading" @click="generate">
              {{ aiLoading ? '诊断中…' : (aiText ? '重新生成' : '生成诊断建议') }}
            </el-button>
          </div>
          <div v-if="aiText" class="md-content rounded-lg bg-violet-50/60 p-4 text-sm leading-relaxed text-slate-700 dark:bg-violet-500/10 dark:text-slate-200" v-html="renderMd(aiText)" />
          <p v-else class="text-sm text-gray-400">
            基于上方薄弱点画像，让 AI 给你一份个性化复习建议：一句话诊断、薄弱点解读、分点复习计划与优先行动清单。
          </p>
        </div>
      </template>
    </template>
  </div>
</template>