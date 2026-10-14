<script setup lang="ts">
import { renderMd, renderMdInline } from '@/utils/markdown'
import { groupOptionsByBlank, annotateBlanksInStem, replaceBlankMarkers, extractTargetBlankNumber, correctLabelsByBlank, correctAnswerFromOptions } from '@/utils/blanks'
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { dailyApi, practiceApi } from '@/api'
import { useSubjectStore } from '@/stores/subject'
import AIExplain from '@/components/AIExplain.vue'
import QuestionNoteEditor from '@/components/QuestionNoteEditor.vue'
import ReportDialog from '@/components/ReportDialog.vue'
import type { Question } from '@/types'

const router = useRouter()
const subjectStore = useSubjectStore()

const loading = ref(true)
const checkedIn = ref(false)
const streakDays = ref(0)
const todayDate = ref('')
const calendar = ref<string[]>([])
const todayStat = ref({ question: 0, correct: 0 })

const questions = ref<Question[]>([])
const currentIndex = ref(0)

// 当前题作答状态
const answers = ref<Record<number, string>>({}) // 空号 -> 选项
const currentBlank = ref(0)
const showResult = ref(false)
const result = ref<any>({})
const questionStartTime = ref(Date.now())
const correctCount = ref(0)
const submitting = ref(false)
const reportDialogRef = ref<InstanceType<typeof ReportDialog> | null>(null)

// 服务端进度持久化：防抖定时器，避免每次点选都发 POST
let saveTimer: any = null
let persistStopped = false


// ------- 答题断点续答：刷新/误触版本刷新后从上次位置继续 -------
// 版本更新通知点击后整页重载，会把这里的答题状态清空；存 localStorage，
// 同一天、同学科的会话在下次进入时自动恢复，避免从头再来。
const SESSION_KEY = 'daily_practice_session'

function persistSession() {
  if (persistStopped) return
  try {
    localStorage.setItem(SESSION_KEY, JSON.stringify({
      date: todayDate.value,
      subjectId: subjectStore.currentSubjectId,
      questions: questions.value,
      currentIndex: currentIndex.value,
      answers: answers.value,
      currentBlank: currentBlank.value,
      showResult: showResult.value,
      result: result.value,
      correctCount: correctCount.value,
    }))
  } catch {}
  // 服务端持久化（防抖 800ms，失败静默）
  clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    dailyApi.saveSession({
      subject_id: subjectStore.currentSubjectId,
      current_index: currentIndex.value,
      current_blank: currentBlank.value,
      answers: answers.value,
      show_result: showResult.value,
      result: result.value,
      correct_count: correctCount.value,
    }).catch(() => {})
  }, 800)
}

function restoreSession() {
  try {
    const raw = localStorage.getItem(SESSION_KEY)
    if (!raw) return
    const s = JSON.parse(raw)
    // 只恢复同一天、同学科的会话；题目为空时不恢复
    if (!s || s.date !== todayDate.value || s.subjectId !== subjectStore.currentSubjectId) return
    if (s.currentIndex == null) return
    // 后端已按「日期+用户+科目」固定种子抽题，刷新/换设备拿到的都是同一组；
    // 这里仍把当时作答的题目一起恢复，答案/位置才与题面对得上（防题库中途变动串题）。
    if (Array.isArray(s.questions) && s.questions.length > 0) {
      questions.value = s.questions
    }
    if (questions.value.length === 0) return
    currentIndex.value = Math.min(s.currentIndex, questions.value.length - 1)
    answers.value = s.answers || {}
    currentBlank.value = Math.min(s.currentBlank || 0, Math.max(totalBlanks.value - 1, 0))
    showResult.value = !!s.showResult
    result.value = s.result || {}
    correctCount.value = s.correctCount || 0
  } catch {}
}

function clearSession() {
  persistStopped = true
  if (saveTimer) {
    clearTimeout(saveTimer)
    saveTimer = null
  }
  try { localStorage.removeItem(SESSION_KEY) } catch {}
}

/** 从服务端会话恢复进度（跨设备/清缓存用） */
function restoreFromServer(session: any) {
  if (!session) return false
  if (typeof session.current_index === 'number' && session.current_index >= 0) {
    currentIndex.value = Math.min(session.current_index, questions.value.length - 1)
  }
  if (session.answers && typeof session.answers === 'object') {
    answers.value = { ...session.answers }
  }
  if (typeof session.current_blank === 'number') {
    currentBlank.value = Math.min(session.current_blank, Math.max(totalBlanks.value - 1, 0))
  }
  if (typeof session.show_result === 'boolean') {
    showResult.value = session.show_result
  }
  if (session.result) {
    result.value = session.result
  }
  if (typeof session.correct_count === 'number') {
    correctCount.value = session.correct_count
  }
  return true
}

async function load() {
  loading.value = true
  try {
    const res: any = await dailyApi.today(subjectStore.currentSubjectId)
    const d = res.data || {}
    checkedIn.value = d.checked_in
    streakDays.value = d.streak_days || 0
    todayDate.value = d.date || ''
    calendar.value = d.calendar || []
    todayStat.value = { question: d.question_count || 0, correct: d.correct_count || 0 }
    questions.value = d.questions || []
    // 已打卡今天则直接展示日历，无会话可恢复；否则优先从服务端恢复，回落 localStorage
    if (!checkedIn.value && questions.value.length > 0) {
      const restored = d.session ? restoreFromServer(d.session) : false
      if (!restored) restoreSession()
    }
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => subjectStore.currentSubjectId, load)

const currentQuestion = computed(() =>
  currentIndex.value < questions.value.length ? questions.value[currentIndex.value] : null,
)

const blankGroups = computed(() => groupOptionsByBlank(currentQuestion.value?.options || []))

const totalBlanks = computed(() => blankGroups.value.length || 1)

/** 拆分后的单空题：题干仍含问题1/2/3，但红字「本题作答（_问题N_）」指定当前只答第 N 空 */
const targetBlankNumber = computed(() => {
  const q = currentQuestion.value
  if (!q) return null
  return extractTargetBlankNumber(q.content || q.title || '')
})

/** 题干中高亮当前空位，并回填已选答案，避免多空题「不知道在答哪一问」 */
const renderedStem = computed(() => {
  const q = currentQuestion.value
  if (!q) return ''
  const raw = q.content || q.title || ''
  const marked = annotateBlanksInStem(raw, currentBlank.value, totalBlanks.value, answers.value)
  return replaceBlankMarkers(renderMd(marked))
})
const currentOptions = computed(() => blankGroups.value[currentBlank.value] || [])
const selectedAnswer = computed(() => answers.value[currentBlank.value] || '')

const fullAnswer = computed(() => {
  let ans = ''
  for (let i = 0; i < totalBlanks.value; i++) ans += answers.value[i] || ''
  return ans
})

const canSubmit = computed(() => blankGroups.value.every((_, i) => answers.value[i]))

const answeredBlankCount = computed(
  () => blankGroups.value.filter((_, i) => !!answers.value[i]).length,
)

const correctLabels = computed(() => correctLabelsByBlank(currentQuestion.value?.options || []))
const correctAnswer = computed(() => correctLabels.value.join('') || correctAnswerFromOptions(currentQuestion.value?.options || []))

function selectOption(label: string) {
  if (showResult.value) return
  // 重新赋值触发依赖 answers 的计算属性（题干高亮/已选进度）
  answers.value = { ...answers.value, [currentBlank.value]: label }
  // 多空题选完当前空后自动跳到下一未作答空，减少“不知道在答哪一空”的困惑
  if (totalBlanks.value > 1 && currentBlank.value < totalBlanks.value - 1) {
    const nextUnanswered = blankGroups.value.findIndex((_, i) => i > currentBlank.value && !answers.value[i])
    if (nextUnanswered >= 0) {
      currentBlank.value = nextUnanswered
    } else if (!answers.value[currentBlank.value + 1]) {
      currentBlank.value++
    }
  }
  persistSession()
}

function isCurrentBlankCorrect(label: string) {
  return correctLabels.value[currentBlank.value] === label
}

async function submitAnswer() {
  const q = currentQuestion.value
  if (!q || !canSubmit.value) return
  const res: any = await practiceApi.submitAnswer({
    question_id: q.id,
    user_answer: fullAnswer.value,
    practice_type: 5,
    time_spent: Math.floor((Date.now() - questionStartTime.value) / 1000),
  })
  result.value = res.data
  if (res.data?.is_correct) correctCount.value++
  showResult.value = true
  persistSession()
}

async function nextQuestion() {
  if (currentIndex.value < questions.value.length - 1) {
    currentIndex.value++
    answers.value = {}
    currentBlank.value = 0
    showResult.value = false
    result.value = {}
    questionStartTime.value = Date.now()
    persistSession()
  } else {
    await doCheckin()
  }
}

async function doCheckin() {
  submitting.value = true
  try {
    clearSession() // 停防抖定时器 + 清 localStorage，避免迟到保存重建会话
    await dailyApi.checkin({
      subject_id: subjectStore.currentSubjectId,
      question_count: questions.value.length,
      correct_count: correctCount.value,
    })
    ElMessage.success('今日打卡成功！')
    await load()
  } finally {
    submitting.value = false
  }
}

// ------- 本月签到日历 -------
const checkedSet = computed(() => new Set(calendar.value))
const monthDays = computed(() => {
  if (!todayDate.value) return []
  const [y, m] = todayDate.value.split('-').map(Number)
  const first = new Date(y, m - 1, 1)
  const startWeekday = first.getDay() // 0=Sun
  const daysInMonth = new Date(y, m, 0).getDate()
  const cells: { day: number | null; date: string | null; done: boolean; isToday: boolean }[] = []
  for (let i = 0; i < startWeekday; i++) cells.push({ day: null, date: null, done: false, isToday: false })
  for (let d = 1; d <= daysInMonth; d++) {
    const ds = `${y}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`
    cells.push({ day: d, date: ds, done: checkedSet.value.has(ds), isToday: ds === todayDate.value })
  }
  return cells
})
const monthCheckins = computed(() => calendar.value.length)
const weekLabels = ['日', '一', '二', '三', '四', '五', '六']
</script>

<template>
  <div>
    <div v-if="loading" class="py-20 text-center text-gray-400">加载中...</div>

    <template v-else>
      <!-- 签到概览 -->
      <div class="mb-6 grid gap-4 sm:grid-cols-3">
        <div class="rounded-xl bg-gradient-to-br from-orange-500 to-amber-500 p-5 text-white shadow-sm">
          <div class="text-sm text-orange-100">连续打卡</div>
          <div class="mt-1 flex items-end gap-1">
            <span class="text-4xl font-bold leading-none">{{ streakDays }}</span>
            <span class="mb-1 text-sm">天</span>
          </div>
          <div class="mt-2 text-xs text-orange-100">🔥 坚持就是胜利</div>
        </div>
        <div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <div class="text-sm text-gray-500 dark:text-gray-400">本月打卡</div>
          <div class="mt-1 text-4xl font-bold text-gray-900 dark:text-gray-100">{{ monthCheckins }}<span class="ml-1 text-sm font-normal text-gray-400">天</span></div>
        </div>
        <div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <div class="text-sm text-gray-500 dark:text-gray-400">今日状态</div>
          <div class="mt-1 text-lg font-semibold" :class="checkedIn ? 'text-green-500' : 'text-orange-500'">
            {{ checkedIn ? '✓ 已完成' : '待完成' }}
          </div>
          <div v-if="checkedIn" class="mt-1 text-xs text-gray-400">
            答对 {{ todayStat.correct }}/{{ todayStat.question }}
          </div>
        </div>
      </div>

      <!-- 已完成今日：展示日历 -->
      <div v-if="checkedIn" class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-800">
        <div class="mb-4 flex items-center justify-between">
          <h2 class="font-semibold text-gray-900 dark:text-gray-100">本月签到日历</h2>
          <span class="text-sm text-gray-400">{{ todayDate.slice(0, 7) }}</span>
        </div>
        <div class="grid grid-cols-7 gap-1.5 text-center">
          <div v-for="w in weekLabels" :key="w" class="pb-2 text-xs text-gray-400">{{ w }}</div>
          <div v-for="(cell, i) in monthDays" :key="i" class="aspect-square">
            <div
              v-if="cell.day"
              class="flex h-full w-full items-center justify-center rounded-lg text-sm transition"
              :class="[
                cell.done
                  ? 'bg-orange-500 font-medium text-white'
                  : 'bg-gray-50 text-gray-400 dark:bg-gray-700/50',
                cell.isToday && !cell.done ? 'ring-2 ring-orange-400' : '',
              ]"
            >
              {{ cell.day }}
            </div>
          </div>
        </div>
        <div class="mt-6 text-center">
          <p class="mb-3 text-sm text-gray-500 dark:text-gray-400">今日已完成，明天再来保持连续！</p>
          <el-button type="primary" @click="router.push('/practice')">继续分类刷题</el-button>
        </div>
      </div>

      <!-- 未完成：答题 -->
      <div v-else-if="questions.length" class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-700 dark:bg-gray-800 sm:p-6">
        <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
          <span class="text-sm text-gray-500 dark:text-gray-400">第 {{ currentIndex + 1 }} / {{ questions.length }} 题</span>
          <div class="flex items-center gap-2">
            <el-tag v-if="totalBlanks > 1" type="warning" effect="plain" size="small" round>
              多空题 · 第 {{ currentBlank + 1 }}/{{ totalBlanks }} 空
            </el-tag>
            <el-tag v-else-if="targetBlankNumber" type="warning" effect="plain" size="small" round>
              本题作答：第 {{ targetBlankNumber }} 空
            </el-tag>
            <span class="text-sm text-gray-400">已答对 {{ correctCount }}</span>
            <el-button size="small" type="danger" plain @click="reportDialogRef?.open()">
              <el-icon><Warning /></el-icon> 报告问题
            </el-button>
          </div>
        </div>
        <el-progress :percentage="Math.round((currentIndex / questions.length) * 100)" class="mb-5" />

        <div v-if="currentQuestion">
          <ReportDialog v-if="currentQuestion" ref="reportDialogRef" :question-id="currentQuestion.id" />
          <!-- 直接展示题目内容；content 为空时回退到 title -->
          <div class="md-content mb-6 leading-relaxed text-gray-800 dark:text-gray-200 prose max-w-none" v-html="renderedStem"></div>

          <div
            v-if="totalBlanks > 1"
            class="mb-4 rounded-lg border border-orange-200 bg-orange-50 px-3 py-3 dark:border-orange-800/60 dark:bg-orange-950/30"
          >
            <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
              <div class="text-sm font-medium text-orange-700 dark:text-orange-300">
                多空题 · 本题共 {{ totalBlanks }} 问，请按空位逐一作答
              </div>
              <div class="text-xs text-orange-600/80 dark:text-orange-300/80">
                已选 {{ answeredBlankCount }}/{{ totalBlanks }}
              </div>
            </div>
            <div class="flex flex-wrap gap-2">
              <el-button
                v-for="(_, i) in blankGroups"
                :key="i"
                :type="currentBlank === i ? 'primary' : answers[i] ? 'success' : 'info'"
                size="small"
                @click="currentBlank = i"
              >
                第{{ i + 1 }}空{{ answers[i] ? `（${answers[i]}）` : '' }}
              </el-button>
            </div>
</div>

          <div
            v-if="totalBlanks > 1 || targetBlankNumber"
            class="mb-3 flex items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-700 dark:border-blue-800/60 dark:bg-blue-950/40 dark:text-blue-300"
          >
            <span class="inline-flex h-6 min-w-6 items-center justify-center rounded-full bg-blue-600 px-2 text-xs text-white">{{ targetBlankNumber || (currentBlank + 1) }}</span>
            <span v-if="totalBlanks > 1">当前选项对应：第 {{ currentBlank + 1 }} 空 / 共 {{ totalBlanks }} 空</span>
            <span v-else>当前选项对应：第 {{ targetBlankNumber }} 空</span>
            <span v-if="selectedAnswer" class="ml-auto text-xs font-normal text-blue-600/80 dark:text-blue-300/80">已选 {{ selectedAnswer }}</span>
          </div>

          <div
            class="mb-6 space-y-3"
            :aria-label="(totalBlanks > 1 || targetBlankNumber) ? `第 ${targetBlankNumber || (currentBlank + 1)} 空选项` : '选项'"
          >
<div
              v-for="opt in currentOptions"
              :key="opt.id"
              class="cursor-pointer rounded-lg border border-gray-200 p-3 text-gray-800 transition dark:border-gray-600 dark:text-gray-200"
              :class="{
                'border-blue-500 bg-blue-50 dark:bg-blue-900/30': !showResult && selectedAnswer === opt.label,
                'border-green-500 bg-green-50 dark:bg-green-900/30': showResult && isCurrentBlankCorrect(opt.label),
                'border-red-500 bg-red-50 dark:bg-red-900/30': showResult && selectedAnswer === opt.label && !isCurrentBlankCorrect(opt.label),
              }"
              @click="selectOption(opt.label)"
            >
<span class="mr-2 font-medium">{{ opt.label }}.</span><span v-html="renderMdInline(opt.content)"></span>
            </div>
          </div>

          <div v-if="!showResult">
            <div v-if="totalBlanks > 1" class="mb-3 flex flex-wrap items-center gap-2">
              <el-button :disabled="currentBlank === 0" @click="currentBlank--">上一空</el-button>
              <el-button v-if="currentBlank < totalBlanks - 1" @click="currentBlank++">下一空</el-button>
              <span class="text-xs text-gray-400">当前第 {{ currentBlank + 1 }}/{{ totalBlanks }} 空</span>
            </div>
            <el-button type="primary" :disabled="!canSubmit" @click="submitAnswer">
              {{ totalBlanks > 1 && answeredBlankCount < totalBlanks ? '请完成所有空位' : '提交' }}
            </el-button>
          </div>
          <div v-else>
            <el-alert
              :title="result.is_correct ? '回答正确！' : `回答错误，正确答案：${correctAnswer}`"
              :type="result.is_correct ? 'success' : 'error'"
              :closable="false"
              class="mb-4"
            />
            <div v-if="result.explanation" class="mb-4 rounded bg-gray-50 p-3 text-gray-600 dark:bg-gray-700/50 dark:text-gray-400">
              <div class="font-medium">解析：</div>
              <div class="md-content" v-html="renderMd(result.explanation)"></div>
            </div>
            <AIExplain v-if="currentQuestion" :question="currentQuestion" :answer="correctAnswer" class="mb-4" />
            <QuestionNoteEditor v-if="currentQuestion" :question-id="currentQuestion.id" class="mb-4" />
            <el-button type="primary" :loading="submitting" @click="nextQuestion">
              {{ currentIndex < questions.length - 1 ? '下一题' : '完成打卡' }}
            </el-button>
          </div>
        </div>
      </div>

      <div v-else class="rounded-xl border border-gray-200 bg-white p-12 text-center text-gray-500 dark:border-gray-700 dark:bg-gray-800">
        今日暂无可练习的题目，请稍后再来
      </div>
    </template>
  </div>
</template>
