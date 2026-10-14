<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { ElMessage } from 'element-plus'
import { examApi } from '@/api'
import type { Paper } from '@/types'
import { hasResumableExam, clearExamCache, type ExamMode } from '@/utils/examCache'
import { isRecentPaperYear, requireMember } from '@/utils/memberGate'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const paper = ref<Paper | null>(null)
const loading = ref(true)

// 近三年真题为会员专属，用于按钮入口门禁与角标展示
const isRecentYear = computed(() => isRecentPaperYear(paper.value?.year || 0))

onMounted(async () => {
  const id = Number(route.params.id)
  try {
    const res: any = await examApi.getPaper(id)
    paper.value = res.data
  } catch {
    ElMessage.error('试卷不存在或加载失败')
    router.push('/exams')
  } finally {
    loading.value = false
  }
})

// 进入考试/练习前：若有未提交的本地进度，弹窗选择"继续答题"或"从头开始"。
const showResumeDialog = ref(false)
const resumableMode = ref<'exam' | 'practice'>('exam')

async function startExam(mode: 'exam' | 'practice') {
  if (!paper.value) return
  // 近三年真题为会员专属：非会员弹引导，不进入恢复进度/开始逻辑
  if (isRecentYear.value && !requireMember('recentExam')) return
  if (!userStore.isLoggedIn) {
    router.push('/login')
    return
  }
  // 练习模式的可恢复进度改查服务端（跨设备/清缓存后仍在）；请求失败回落本地
  let resumable = false
  if (mode === 'practice') {
    try {
      const s: any = await examApi.practiceSessionState({ paper_id: paper.value.id, mode: 'practice' })
      resumable = !!s?.data?.resumable
    } catch {
      resumable = hasResumableExam(paper.value.id, mode as ExamMode)
    }
  } else {
    resumable = hasResumableExam(paper.value.id, mode as ExamMode)
  }
  if (resumable) {
    resumableMode.value = mode
    showResumeDialog.value = true
    return
  }
  goToExamSession(mode)
}

function goToExamSession(mode: 'exam' | 'practice') {
  if (!paper.value) return
  router.push(`/exam/${paper.value.id}?mode=${mode}`)
}

// 背题模式：直接浏览题目、答案与解析，无需作答。近三年真题与练习/考试同门禁。
function startReview() {
  if (!paper.value) return
  if (isRecentYear.value && !requireMember('recentExam')) return
  router.push(`/exam/${paper.value.id}/review`)
}

function resumeExam() {
  showResumeDialog.value = false
  goToExamSession(resumableMode.value)
}

async function restartExam() {
  showResumeDialog.value = false
  if (paper.value) {
    // 从头开始：练习模式还要清掉服务端草稿，否则换设备后旧进度仍在
    if (resumableMode.value === 'practice') {
      await examApi.clearPracticeSession({ paper_id: paper.value.id, mode: 'practice' }).catch(() => {})
    }
    clearExamCache(paper.value.id, resumableMode.value as ExamMode)
  }
  goToExamSession(resumableMode.value)
}

function typeLabel(t: number) {
  return { 1: '综合知识', 2: '案例分析', 3: '论文' }[t] || '未知'
}


</script>

<template>
  <div class="max-w-3xl mx-auto px-4 py-8">
    <!-- Loading -->
    <div v-if="loading" class="text-center py-20">
      <div class="animate-spin inline-block w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full mb-4"></div>
      <div class="text-gray-500">加载中...</div>
    </div>

    <!-- Paper Detail -->
    <div v-else-if="paper" class="bg-white dark:bg-gray-800 rounded-2xl shadow-lg border border-gray-100 dark:border-gray-700 overflow-hidden">
      <!-- Header -->
      <div class="bg-gradient-to-r from-blue-600 to-purple-600 px-8 py-10 text-white">
        <div class="flex items-center gap-3 mb-4">
          <span 
            class="px-3 py-1 rounded-full text-sm font-medium bg-white/20 backdrop-blur"
          >
            {{ typeLabel(paper.paper_type) }}
          </span>
          <span class="text-white/80">{{ paper.year }}年</span>
        </div>
        <h1 class="text-2xl font-bold mb-2">{{ paper.title }}</h1>
        <p v-if="paper.description" class="text-white/80 text-sm">{{ paper.description }}</p>
      </div>

      <!-- Info Cards -->
      <div class="px-8 py-6">
        <div class="grid grid-cols-3 gap-4 mb-6">
          <div class="bg-gray-50 dark:bg-gray-700 rounded-xl p-4 text-center">
            <div class="text-2xl font-bold text-blue-600">{{ paper.duration }}</div>
            <div class="text-sm text-gray-500 mt-1">考试时长(分钟)</div>
          </div>
          <div class="bg-gray-50 dark:bg-gray-700 rounded-xl p-4 text-center">
            <div class="text-2xl font-bold text-green-600">{{ paper.total_score }}</div>
            <div class="text-sm text-gray-500 mt-1">满分(分)</div>
          </div>
          <div class="bg-gray-50 dark:bg-gray-700 rounded-xl p-4 text-center">
            <div class="text-2xl font-bold text-purple-600">{{ paper.year }}</div>
            <div class="text-sm text-gray-500 mt-1">考试年份</div>
          </div>
        </div>

        <!-- Description -->
        <div v-if="paper.description" class="mb-6 p-4 bg-blue-50 dark:bg-blue-900/20 rounded-xl">
          <h3 class="text-sm font-medium text-blue-800 dark:text-blue-300 mb-2">考试说明</h3>
          <p class="text-sm text-blue-700 dark:text-blue-400">{{ paper.description }}</p>
        </div>

        <!-- Actions -->
        <div v-if="isRecentYear" class="mb-3">
          <el-tag type="warning" effect="plain" size="small">🔒 会员专属 · 近三年真题</el-tag>
        </div>
        <div class="flex flex-col sm:flex-row gap-4">
          <button
            @click="startReview"
            class="flex-1 bg-white dark:bg-gray-700 border border-emerald-500 text-emerald-600 dark:text-emerald-300 py-3 px-6 rounded-xl font-medium hover:bg-emerald-50 dark:hover:bg-gray-600 transition-all"
          >
            背题模式
            <span class="block text-xs font-normal text-gray-400 mt-0.5">直接浏览题目、答案与解析</span>
          </button>
          <button
            @click="startExam('practice')"
            class="flex-1 bg-white dark:bg-gray-700 border border-blue-500 text-blue-600 dark:text-blue-300 py-3 px-6 rounded-xl font-medium hover:bg-blue-50 dark:hover:bg-gray-600 transition-all"
          >
            练习模式
            <span class="block text-xs font-normal text-gray-400 mt-0.5">逐题作答，立即看对错和解析</span>
          </button>
          <button
            @click="startExam('exam')"
            class="flex-1 bg-gradient-to-r from-blue-600 to-purple-600 text-white py-3 px-6 rounded-xl font-medium hover:from-blue-700 hover:to-purple-700 transition-all shadow-lg hover:shadow-xl"
          >
            开始考试
            <span class="block text-xs font-normal text-white/70 mt-0.5">限时模拟，交卷后出成绩</span>
          </button>
          <button
            @click="$router.back()"
            class="px-6 py-3 rounded-xl border border-gray-200 dark:border-gray-600 text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
          >
            返回
          </button>

        </div>
      </div>
    </div>

    <!-- 继续答题 / 从头开始 -->
    <el-dialog v-model="showResumeDialog" title="继续答题" width="400px" :close-on-click-modal="false">
      <div class="py-2">
        <p class="text-gray-700 dark:text-gray-300 mb-2">
          检测到上次未完成的答题进度。
        </p>
        <p class="text-xs text-gray-400 dark:text-gray-500">
          “继续答题”会恢复上次的题目与作答；“从头开始”会清空该卷进度，重新作答。
        </p>
      </div>
      <template #footer>
        <div class="flex gap-3 justify-end">
          <el-button @click="showResumeDialog = false">取消</el-button>
          <el-button type="primary" @click="resumeExam">继续答题</el-button>
          <el-button @click="restartExam">从头开始</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>
