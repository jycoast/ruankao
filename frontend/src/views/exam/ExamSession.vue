<script setup lang="ts">
import { renderMd, renderMdInline } from '@/utils/markdown'
import { ref, onMounted, onUnmounted, computed, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { examApi, practiceApi } from '@/api'
import { useUserStore } from '@/stores/user'
import AnswerCard from '@/components/AnswerCard.vue'
import ShortcutHint from '@/components/ShortcutHint.vue'
import ReportDialog from '@/components/ReportDialog.vue'
import QuestionNoteEditor from '@/components/QuestionNoteEditor.vue'
import AIExplain from '@/components/AIExplain.vue'
import AIGradeCase from '@/components/AIGradeCase.vue'
import { useQuizShortcuts } from '@/composables/useQuizShortcuts'
import type { Question } from '@/types'
import { loadExamCache, saveExamCache, clearExamCache, type ExamMode } from '@/utils/examCache'
import { isRecentPaperYear } from '@/utils/memberGate'
import { setEssayPrefill, essayTopicOf, essayPointsOf, GENERATE_ESSAY_PATH } from '@/utils/essayPrefill'
import {
  groupOptionsByBlank,
  annotateBlanksInStem,
  replaceBlankMarkers,
  extractTargetBlankNumber,
  correctLabelsByBlank,
} from '@/utils/blanks'
import { groupQuestions } from '@/utils/questionGroup'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const paperId = Number(route.params.id)
// 模式：exam=考试模式(限时、交卷出分)；practice=练习模式(逐题即时判题)
const isPractice = route.query.mode === 'practice'

const questions = ref<Question[]>([])
const paperType = ref(0)
const currentIndex = ref(0)
const answers = ref<Record<number, Record<number, string>>>({})
const currentBlank = ref(0)
const currentInput = ref('')
const timer = ref(0)
const timerInterval = ref<any>(null)
const submitting = ref(false)
const finished = ref(false)
const result = ref<any>(null)
const examRecordId = ref(0)

// 练习模式：逐题判题结果 { is_correct: true/false/null(主观题不判), correctAnswer }
const practiceResults = ref<Record<number, { is_correct: boolean | null; correctAnswer: string }>>({})
const questionStartTime = ref(Date.now())

// ---- 进度持久化：localStorage 兜底 + 服务端草稿（仅练习模式，防抖） ----
const examMode: ExamMode = isPractice ? 'practice' : 'exam'
let cachedTimer = 0
let persistStopped = false
// 练习模式服务端保存的防抖定时器：watch 每次点选都触发，合并成一次 POST
let saveTimer: any = null

function saveCache() {
  if (persistStopped) return
  saveExamCache(paperId, examMode, {
    answers: answers.value,
    practiceResults: practiceResults.value,
    currentIndex: currentIndex.value,
    timer: timer.value,
  })
  // 服务端持久化（仅练习模式）：防抖 800ms，失败静默，localStorage 兜底本设备
  if (isPractice && userStore.isLoggedIn) {
    clearTimeout(saveTimer)
    saveTimer = setTimeout(() => {
      examApi
        .savePracticeSession({
          paper_id: paperId,
          mode: 'practice',
          answers: answers.value,
          practiceResults: practiceResults.value,
          currentIndex: currentIndex.value,
        })
        .catch(() => {})
    }, 800)
  }
}

async function restoreCache() {
  // 服务端权威：有会话则全量恢复并返回；无会话或请求失败回落本地兜底
  if (isPractice && userStore.isLoggedIn) {
    try {
      const s: any = await examApi.practiceSessionState({ paper_id: paperId, mode: 'practice' })
      const sess = s?.data?.session
      if (sess) {
        if (sess.answers && typeof sess.answers === 'object') answers.value = sess.answers
        if (sess.practiceResults && typeof sess.practiceResults === 'object') practiceResults.value = sess.practiceResults
        if (typeof sess.currentIndex === 'number' && sess.currentIndex >= 0 && sess.currentIndex < questions.value.length) {
          currentIndex.value = sess.currentIndex
        }
        return
      }
    } catch {
      /* 服务端查询失败，回落本地兜底 */
    }
  }
  const c = loadExamCache(paperId, examMode)
  if (!c) return
  if (c.answers && typeof c.answers === 'object') answers.value = c.answers
  if (c.practiceResults && typeof c.practiceResults === 'object') practiceResults.value = c.practiceResults
  if (typeof c.currentIndex === 'number' && c.currentIndex >= 0 && c.currentIndex < questions.value.length) {
    currentIndex.value = c.currentIndex
  }
  cachedTimer = typeof c.timer === 'number' ? c.timer : 0
}

function clearCache() {
  persistStopped = true
  // 清掉防抖中的待发 POST，避免完成/从头开始后 DELETE 与迟到的保存竞态重建会话
  if (saveTimer) {
    clearTimeout(saveTimer)
    saveTimer = null
  }
  clearExamCache(paperId, examMode)
  // 服务端草稿同步清除（练习模式完成/从头开始时）
  if (isPractice) {
    examApi.clearPracticeSession({ paper_id: paperId, mode: 'practice' }).catch(() => {})
  }
}

// 答案/判题结果/当前题变化即写入本地缓存
watch([answers, practiceResults, currentIndex], saveCache, { deep: true })

const currentQuestion = computed(() => questions.value[currentIndex.value])
// 答题卡按大题分组：案例分析卷一行往往是一道大题的一问（见 utils/questionGroup）
const questionGroups = computed(() => groupQuestions(questions.value))
const reportDialogRef = ref<InstanceType<typeof ReportDialog> | null>(null)

const blankGroups = computed(() => groupOptionsByBlank(currentQuestion.value?.options || []))
const totalBlanks = computed(() => blankGroups.value.length)
const currentOptions = computed(() => blankGroups.value[currentBlank.value] || [])
const targetBlankNumber = computed(() => {
  const q = currentQuestion.value
  if (!q) return null
  return extractTargetBlankNumber(q.content || q.title || '')
})
const activeBlankNo = computed(() => targetBlankNumber.value || (currentBlank.value + 1))
const correctLabels = computed(() => correctLabelsByBlank(currentQuestion.value?.options || []))

const currentQAnswers = computed(() => {
  if (!currentQuestion.value) return {}
  return answers.value[currentQuestion.value.id] || {}
})

const renderedStem = computed(() => {
  const q = currentQuestion.value
  if (!q) return ''
  const raw = q.content || q.title || ''
  const marked = annotateBlanksInStem(raw, currentBlank.value, totalBlanks.value || 1, currentQAnswers.value as Record<number, string>)
  return replaceBlankMarkers(renderMd(marked))
})

// 练习模式派生状态
const currentResult = computed(() =>
  currentQuestion.value ? practiceResults.value[currentQuestion.value.id] : undefined,
)
const isJudged = computed(() => !!currentResult.value)
const practiceCorrectCount = computed(
  () => Object.values(practiceResults.value).filter((r) => r.is_correct === true).length,
)
const practiceAnsweredCount = computed(() => Object.keys(practiceResults.value).length)
// 论文题（question_type=4）：判题后可直接拿题面 + 写作要点去生成一篇范文
const isEssay = computed(() => currentQuestion.value?.question_type === 4)

const canSubmitPractice = computed(() => {
  const q = currentQuestion.value
  if (!q) return false
  if ((q.options || []).length === 0) return !!(currentQAnswers.value[0] || currentInput.value.trim())
  return blankGroups.value.every((_, i) => (currentQAnswers.value as Record<number, string>)[i])
})

onMounted(async () => {
  try {
    const res: any = await examApi.getPaper(paperId)
    const paper = res.data
    // 兜底：非会员直进近三年真题（绕过详情页入口）时拦截；生成模拟卷（source==1）不受影响
    if (isRecentPaperYear(paper?.year || 0) && paper?.source === 0 && !userStore.isMember) {
      ElMessage.warning('近三年真题为会员专属功能，开通会员后即可练习/考试')
      router.replace('/exams')
      return
    }
    questions.value = paper.questions || []
    paperType.value = paper.paper_type || 0
    await restoreCache() // 恢复未提交的答题记录（服务端优先）
    syncCurrentInput()
    if (!isPractice) {
      // 考试模式：建立考试记录并倒计时。
      // 后端对同一套卷子的未交卷记录会复用，刷新不会新增记录。
      const recordRes: any = await examApi.startExam(paperId)
      examRecordId.value = recordRes.data.id
      const duration = res.data.duration || 150
      // 剩余时间以服务端 started_at 为准；localStorage 会被清、也会被改，
      // 刷新一次就重置倒计时等于送无限时间。
      const startedAt = Date.parse(recordRes.data.started_at)
      if (startedAt) {
        const elapsed = Math.floor((Date.now() - startedAt) / 1000)
        timer.value = Math.max(0, duration * 60 - elapsed)
      } else {
        timer.value = cachedTimer > 0 ? cachedTimer : duration * 60
      }
      timerInterval.value = setInterval(() => {
        if (timer.value > 0) {
          timer.value--
          if (timer.value % 10 === 0) saveCache() // 定期持久化剩余时间
        } else submitExam()
      }, 1000)
    }
  } catch {
    router.push('/exams')
  }
})

watch(currentIndex, () => {
  syncCurrentInput()
  questionStartTime.value = Date.now()
})

onUnmounted(() => {
  if (timerInterval.value) clearInterval(timerInterval.value)
})

function formatTime(s: number) {
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  return `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}:${sec.toString().padStart(2, '0')}`
}

const answeredCount = computed(() => {
  return Object.values(answers.value).filter(a => Object.keys(a).length > 0).length
})

const answeredMap = computed(() => {
  const map: Record<number, boolean> = {}
  for (let i = 0; i < questions.value.length; i++) {
    const q = questions.value[i]
    if (q && answers.value[q.id] && Object.keys(answers.value[q.id]).length > 0) {
      map[i] = true
    }
  }
  return map
})

// 练习模式答题卡对错状态：true=正确 false=错误 null=已答未判(主观题)
const resultMap = computed(() => {
  const map: Record<number, boolean | null> = {}
  for (let i = 0; i < questions.value.length; i++) {
    const q = questions.value[i]
    const r = q && practiceResults.value[q.id]
    if (r) map[i] = r.is_correct
  }
  return map
})

// 考试模式交卷后：本地逐题判分，列出错题（含未作答）供回顾。
// 判分口径与练习模式一致（correctLabelsByBlank 逐空取标准答案），
// 主观题/无标准答案的题跳过 —— 与后端「不计分也不进错题本」保持一致。
const examReviewItems = computed(() => {
  if (isPractice || !finished.value) return []
  const items: {
    index: number
    q: Question
    myAnswer: string
    correctAnswer: string
    answered: boolean
  }[] = []
  for (let i = 0; i < questions.value.length; i++) {
    const q = questions.value[i]
    if (!q || (q.options || []).length === 0) continue
    const labels = correctLabelsByBlank(q.options || [])
    if (labels.length === 0) continue
    const myAnswer = buildAnswerStr(q)
    const answered = myAnswer !== ''
    const isCorrect = answered && myAnswer === labels.join('')
    if (isCorrect) continue
    items.push({ index: i + 1, q, myAnswer, correctAnswer: labels.join(''), answered })
  }
  return items
})

// 把题干里的空白用「用户答案」填上，错题回顾时能一眼看清每空选了什么。
function reviewStemHtml(q: Question): string {
  const raw = q.content || q.title || ''
  const qAnswers = answers.value[q.id] || {}
  const groups = groupOptionsByBlank(q.options || [])
  if (groups.length <= 1) return renderMd(raw)
  // currentBlank 传一个不会命中的值，让所有空位都填上答案而非高亮某空
  return replaceBlankMarkers(annotateBlanksInStem(raw, groups.length + 1, groups.length, qAnswers as Record<number, string>))
}

function jumpToQuestion(index: number) {
  currentIndex.value = index
  currentBlank.value = 0
  syncCurrentInput()
}

function syncCurrentInput() {
  if (!currentQuestion.value) { currentInput.value = ''; return }
  const opts = currentQuestion.value.options || []
  if (opts.length === 0) {
    currentInput.value = answers.value[currentQuestion.value.id]?.[0] || ''
  }
}

function saveCurrentInput() {
  if (!currentQuestion.value) return
  const opts = currentQuestion.value.options || []
  if (opts.length === 0) {
    const qid = currentQuestion.value.id
    if (!answers.value[qid]) answers.value[qid] = {}
    answers.value[qid][0] = currentInput.value.trim()
  }
}

function selectOption(label: string) {
  if (!currentQuestion.value) return
  if (isPractice && isJudged.value) return // 练习模式判题后锁定
  const qid = currentQuestion.value.id
  if (!answers.value[qid]) answers.value[qid] = {}
  answers.value[qid][currentBlank.value] = label
}

function buildAnswerStr(q: Question) {
  const qAnswers = answers.value[q.id] || {}
  const opts = q.options || []
  if (opts.length === 0) return (qAnswers[0] || '').trim()
  const groups = groupOptionsByBlank(opts)
  let s = ''
  for (let i = 0; i < groups.length; i++) s += qAnswers[i] || ''
  return s
}

function submitCurrentPractice() {
  const q = currentQuestion.value
  if (!q) return
  saveCurrentInput()
  const qAns = answers.value[q.id] || {}
  let isCorrect: boolean | null
  const labels = correctLabelsByBlank(q.options || [])
  if ((q.options || []).length === 0) {
    isCorrect = null
  } else {
    isCorrect = labels.length > 0
    for (let i = 0; i < labels.length; i++) {
      if ((qAns[i] || '') !== labels[i]) isCorrect = false
    }
  }
  practiceResults.value[q.id] = { is_correct: isCorrect, correctAnswer: labels.join('') }
  // 记录练习（错题本/统计），失败静默忽略，不影响判题展示
  if (userStore.isLoggedIn && (q.options || []).length > 0) {
    practiceApi
      .submitAnswer({
        question_id: q.id,
        user_answer: buildAnswerStr(q),
        practice_type: paperType.value || 1,
        time_spent: Math.floor((Date.now() - questionStartTime.value) / 1000),
      })
      .catch(() => {})
  }
}

function practicePrev() {
  if (currentIndex.value === 0) return
  currentIndex.value--
  currentBlank.value = 0
}

function practiceNext() {
  saveCurrentInput()
  if (currentIndex.value < questions.value.length - 1) {
    currentIndex.value++
    currentBlank.value = 0
    nextTick(syncCurrentInput)
  } else {
    finished.value = true
    clearCache()
  }
}

// 一键生成范文：题目当主题，写作要点当论点，跳到 AI 生成范文页预填。
// 要点很长（单题 5000+ 字符），走 sessionStorage 交接，见 utils/essayPrefill。
function generateEssayFromQuestion() {
  const q = currentQuestion.value
  if (!q) return
  setEssayPrefill({ topic: essayTopicOf(q), points: essayPointsOf(q) })
  router.push(GENERATE_ESSAY_PATH)
}

async function submitExam() {
  if (timerInterval.value) clearInterval(timerInterval.value)
  submitting.value = true
  const ans: { question_id: number; answer: string }[] = []
  for (const q of questions.value) {
    const answerStr = buildAnswerStr(q)
    if (answerStr) {
      ans.push({ question_id: q.id, answer: answerStr })
    }
  }
  try {
    const res: any = await examApi.submitExam({ exam_record_id: examRecordId.value, answers: ans })
    result.value = res.data
    finished.value = true
    clearCache()
  } catch {
    // 重复交卷等业务错误已由拦截器提示；清掉缓存避免卡在已交卷的考试里
    clearCache()
    router.push('/exams')
  } finally {
    submitting.value = false
  }
}

function gotoQuestion(index: number) {
  if (index < 0 || index >= questions.value.length) return
  saveCurrentInput()
  currentIndex.value = index
  currentBlank.value = 0
  nextTick(syncCurrentInput)
}

useQuizShortcuts({
  enabled: () => !finished.value && !!currentQuestion.value,
  onSelect: (label) => {
    if (currentOptions.value.some((o: any) => o.label === label)) selectOption(label)
  },
  // 考试模式不绑定 Enter 交卷，避免误触；只在练习模式提交/翻页
  onEnter: () => {
    if (!isPractice) return
    if (isJudged.value) practiceNext()
    else if (canSubmitPractice.value) submitCurrentPractice()
  },
  onPrev: () => gotoQuestion(currentIndex.value - 1),
  onNext: () => gotoQuestion(currentIndex.value + 1),
})

</script>

<template>
  <div class="max-w-6xl mx-auto px-4">
    <template v-if="!finished">
      <div class="flex items-center justify-between mb-4 bg-white dark:bg-gray-800 p-3 rounded shadow transition-colors">
        <span class="text-lg font-bold text-gray-900 dark:text-gray-100">
          软考真题<span v-if="isPractice" class="text-blue-500 text-base font-medium"> · 练习模式</span>
        </span>
        <div class="flex items-center gap-4">
          <span class="text-gray-500 dark:text-gray-400">已答 {{ answeredCount }}/{{ questions.length }}</span>
          <span v-if="!isPractice" class="text-red-500 text-xl font-mono">{{ formatTime(timer) }}</span>
          <span v-else class="text-green-600 dark:text-green-400 font-medium">答对 {{ practiceCorrectCount }}</span>
        </div>
      </div>

      <div class="flex gap-6">
        <!-- Question Area -->
        <div class="flex-1 min-w-0">
          <div v-if="currentQuestion" class="bg-white dark:bg-gray-800 p-6 rounded-lg shadow transition-colors">
            <div class="flex items-center justify-between mb-2">
              <div class="text-sm text-gray-500 dark:text-gray-400">第 {{ currentIndex + 1 }} / {{ questions.length }} 题</div>
              <el-button size="small" type="danger" plain @click="reportDialogRef?.open()">
                <el-icon><Warning /></el-icon> 报告问题
              </el-button>
            </div>
            <div class="mb-2 flex flex-wrap items-center gap-2">
              <el-tag v-if="totalBlanks > 1" type="warning" effect="plain" size="small" round>
                多空题 · 第 {{ currentBlank + 1 }}/{{ totalBlanks }} 空
              </el-tag>
            </div>
            <div class="text-lg font-medium mb-4 md-content leading-relaxed text-gray-900 dark:text-gray-100" v-html="renderedStem"></div>

            <div
              v-if="totalBlanks > 1"
              class="mb-3 flex items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-700 dark:border-blue-800/60 dark:bg-blue-950/40 dark:text-blue-300"
            >
              <span class="inline-flex h-6 min-w-6 items-center justify-center rounded-full bg-blue-600 px-2 text-xs text-white">{{ activeBlankNo }}</span>
              <span>当前选项对应：第 {{ currentBlank + 1 }} 空 / 共 {{ totalBlanks }} 空</span>
            </div>

            <div v-if="totalBlanks > 1" class="mb-4 flex gap-2 flex-wrap">
              <el-button
                v-for="(_, i) in blankGroups"
                :key="i"
                :type="currentBlank === i ? 'primary' : (currentQAnswers[i] ? 'success' : 'info')"
                size="small"
                @click="currentBlank = i"
              >
                第{{ i + 1 }}空 {{ currentQAnswers[i] ? `(${currentQAnswers[i]})` : '' }}
              </el-button>
            </div>

            <template v-if="currentOptions.length > 0">

              <div class="mb-6 space-y-3">
<div
                  v-for="opt in currentOptions"
                  :key="opt.id"
                  class="p-3 border border-gray-200 dark:border-gray-600 rounded-lg cursor-pointer transition text-gray-800 dark:text-gray-200"
                  :class="{
                    'border-blue-500 bg-blue-50 dark:bg-blue-900/30': (!isPractice || !isJudged) && currentQAnswers[currentBlank] === opt.label,
                    'border-green-500 bg-green-50 dark:bg-green-900/30': isPractice && isJudged && correctLabels[currentBlank] === opt.label,
                    'border-red-500 bg-red-50 dark:bg-red-900/30': isPractice && isJudged && currentQAnswers[currentBlank] === opt.label && correctLabels[currentBlank] !== opt.label,
                  }"
                  @click="selectOption(opt.label)"
                >
                  <span class="font-medium mr-2">{{ opt.label }}.</span>
                  <span v-html="renderMdInline(opt.content)"></span>
                </div>
              </div>
            </template>
            <div v-else class="mb-6">
              <el-input v-model="currentInput" type="textarea" :rows="5" placeholder="请输入你的答案" :disabled="isPractice && isJudged" />
            </div>

            <!-- 考试模式：分空 + 上/下一题 + 交卷 -->
            <template v-if="!isPractice">
              <div v-if="totalBlanks > 1" class="flex gap-2 mb-4">
                <el-button :disabled="currentBlank === 0" @click="currentBlank--">上一空</el-button>
                <el-button v-if="currentBlank < totalBlanks - 1" @click="currentBlank++">下一空</el-button>
              </div>
              <QuestionNoteEditor :question-id="currentQuestion.id" class="mb-4" />
              <div class="flex justify-between">
                <el-button :disabled="currentIndex === 0" @click="currentIndex--; currentBlank = 0">上一题</el-button>
                <el-button v-if="currentIndex < questions.length - 1" type="primary" @click="saveCurrentInput(); currentIndex++; currentBlank = 0; $nextTick(syncCurrentInput)">下一题</el-button>
                <el-button v-else type="success" @click="submitExam" :loading="submitting">交卷</el-button>
              </div>
              <ShortcutHint :show-enter="false" />
            </template>

            <!-- 练习模式：提交判题 → 显示对错/正确答案/解析 → 下一题 -->
            <template v-else>
              <div v-if="totalBlanks > 1 && !isJudged" class="flex gap-2 mb-4">
                <el-button :disabled="currentBlank === 0" @click="currentBlank--">上一空</el-button>
                <el-button v-if="currentBlank < totalBlanks - 1" @click="currentBlank++">下一空</el-button>
              </div>

              <div v-if="!isJudged">
                <el-button type="primary" :disabled="!canSubmitPractice" @click="submitCurrentPractice">提交</el-button>
                <ShortcutHint />
              </div>
              <div v-else>
                <el-alert
                  v-if="currentResult && currentResult.is_correct !== null"
                  :title="currentResult.is_correct ? '回答正确！' : '回答错误'"
                  :type="currentResult.is_correct ? 'success' : 'error'"
                  :closable="false"
                  class="mb-3"
                />
                <div v-if="currentResult && currentResult.correctAnswer" class="mb-3 text-green-600 dark:text-green-400 font-medium">
                  正确答案：{{ currentResult.correctAnswer }}
                </div>
                <!-- 论文题专属：一键生成范文。放判题块开头而不是解析区之后 ——
                     论文题的写作要点动辄五千字，放下面得滚过整段才看得到。 -->
                <div v-if="isEssay" class="mb-3">
                  <el-button type="primary" plain @click="generateEssayFromQuestion">一键生成范文</el-button>
                  <span class="ml-3 text-xs text-gray-400 dark:text-gray-500">把本题题目与写作要点带到 AI 生成范文</span>
                </div>
                <div v-if="currentQuestion.explanation" class="mb-3 p-3 bg-gray-50 dark:bg-gray-700/50 rounded text-gray-600 dark:text-gray-400">
                  <div class="font-medium mb-1">解析：</div>
                  <div class="md-content" v-html="renderMd(currentQuestion.explanation)"></div>
                </div>
                <div v-if="currentQuestion.analysis" class="mb-3 p-3 bg-gray-50 dark:bg-gray-700/50 rounded text-gray-600 dark:text-gray-400">
                  <div class="font-medium mb-1">分析：</div>
                  <div class="md-content" v-html="renderMd(currentQuestion.analysis)"></div>
                </div>
                <AIExplain :question="currentQuestion" :answer="currentResult?.correctAnswer" class="mb-3" />
                <AIGradeCase
                  v-if="(currentQuestion.options || []).length === 0"
                  :question="currentQuestion"
                  :user-answer="answers[currentQuestion.id]?.[0] || ''"
                  class="mb-3"
                />
                <QuestionNoteEditor :question-id="currentQuestion.id" class="mb-3" />
                <div class="flex justify-between">
                  <el-button :disabled="currentIndex === 0" @click="practicePrev">上一题</el-button>
                  <el-button type="primary" @click="practiceNext">
                    {{ currentIndex < questions.length - 1 ? '下一题' : '完成' }}
                  </el-button>
                </div>
              </div>
            </template>
          </div>
          <div v-else class="text-center py-12 text-gray-500">加载中...</div>
        </div>

        <!-- Answer Card (Right Side) -->
        <div class="w-72 flex-shrink-0 hidden lg:block">
          <AnswerCard
            :total="questions.length"
            :current-index="currentIndex"
            :groups="questionGroups"
            :answered-map="answeredMap"
            :result-map="isPractice ? resultMap : undefined"
            class="sticky top-20"
            @jump="jumpToQuestion"
          />
        </div>
      </div>

      <!-- Mobile Answer Card (Bottom) -->
      <div class="lg:hidden mt-4">
        <AnswerCard
          :total="questions.length"
          :current-index="currentIndex"
          :groups="questionGroups"
          :answered-map="answeredMap"
          :result-map="isPractice ? resultMap : undefined"
          @jump="jumpToQuestion"
        />
      </div>
    </template>

    <template v-else>
      <div class="bg-white dark:bg-gray-800 p-8 rounded-lg shadow text-center transition-colors">
        <!-- 考试模式完成 -->
        <template v-if="!isPractice">
          <h2 class="text-2xl font-bold mb-6 text-gray-900 dark:text-gray-100">考试完成</h2>
          <!-- 整卷均为论文/案例等主观题时无法机判，不展示 0 分误导用户 -->
          <p v-if="!result?.total_count" class="mb-8 text-gray-500">
            本卷均为主观题，系统无法自动判分，请对照解析自行评估。
          </p>
          <div v-else class="grid grid-cols-3 gap-6 mb-8">
            <div>
              <div class="text-3xl font-bold text-blue-500">{{ result?.score?.toFixed(1) || 0 }}</div>
              <div class="text-gray-500 mt-1">得分</div>
            </div>
            <div>
              <div class="text-3xl font-bold text-green-500">{{ result?.correct_count || 0 }}</div>
              <div class="text-gray-500 mt-1">正确</div>
            </div>
            <div>
              <div class="text-3xl font-bold text-red-500">{{ (result?.total_count || 0) - (result?.correct_count || 0) }}</div>
              <div class="text-gray-500 mt-1">错误</div>
            </div>
          </div>

          <!-- 错题回顾：交卷后本地逐题判分列出错题，含题干/我的答案/正确答案/解析 -->
          <div v-if="examReviewItems.length > 0" class="text-left mt-6 mb-8">
            <h3 class="text-lg font-bold mb-4 text-gray-900 dark:text-gray-100">错题回顾（{{ examReviewItems.length }} 题）</h3>
            <div class="space-y-4">
              <div
                v-for="item in examReviewItems"
                :key="item.q.id"
                class="border border-gray-200 dark:border-gray-600 rounded-lg p-4"
              >
                <div class="flex items-center gap-2 mb-2">
                  <span class="font-medium text-gray-900 dark:text-gray-100">第 {{ item.index }} 题</span>
                  <el-tag v-if="!item.answered" type="info" size="small" effect="plain">未作答</el-tag>
                  <el-tag v-else type="danger" size="small" effect="plain">答错</el-tag>
                </div>
                <div class="md-content mb-3 text-gray-800 dark:text-gray-200 leading-relaxed" v-html="reviewStemHtml(item.q)"></div>
                <div class="text-sm mb-1 text-red-600 dark:text-red-400">你的答案：{{ item.myAnswer || '（未作答）' }}</div>
                <div class="text-sm mb-2 text-green-600 dark:text-green-400 font-medium">正确答案：{{ item.correctAnswer }}</div>
                <div v-if="item.q.explanation" class="mb-3 p-3 bg-gray-50 dark:bg-gray-700/50 rounded text-gray-600 dark:text-gray-400">
                  <div class="font-medium mb-1">解析：</div>
                  <div class="md-content" v-html="renderMd(item.q.explanation)"></div>
                </div>
                <div v-if="item.q.analysis" class="mb-3 p-3 bg-gray-50 dark:bg-gray-700/50 rounded text-gray-600 dark:text-gray-400">
                  <div class="font-medium mb-1">分析：</div>
                  <div class="md-content" v-html="renderMd(item.q.analysis)"></div>
                </div>
              </div>
            </div>
          </div>
          <div v-else-if="result?.total_count" class="mb-8">
            <el-alert title="全部答对，太棒了！" type="success" :closable="false" />
          </div>
        </template>
        <!-- 练习模式完成 -->
        <template v-else>
          <h2 class="text-2xl font-bold mb-6 text-gray-900 dark:text-gray-100">练习完成</h2>
          <div class="grid grid-cols-3 gap-6 mb-8">
            <div>
              <div class="text-3xl font-bold text-green-500">{{ practiceCorrectCount }}</div>
              <div class="text-gray-500 mt-1">答对</div>
            </div>
            <div>
              <div class="text-3xl font-bold text-blue-500">{{ practiceAnsweredCount }}</div>
              <div class="text-gray-500 mt-1">已作答</div>
            </div>
            <div>
              <div class="text-3xl font-bold text-gray-500">{{ questions.length }}</div>
              <div class="text-gray-500 mt-1">总题数</div>
            </div>
          </div>
        </template>
        <el-button type="primary" @click="$router.push('/exams')">返回试卷列表</el-button>
      </div>
    </template>
  </div>

  <ReportDialog
    v-if="currentQuestion"
    ref="reportDialogRef"
    :question-id="currentQuestion.id"
  />
</template>
