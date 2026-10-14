<script setup lang="ts">
import { renderMd, renderMdInline } from '@/utils/markdown'
import { ref, computed, onMounted } from 'vue'
import { practiceApi } from '@/api'
import AnswerCard from '@/components/AnswerCard.vue'
import AIExplain from '@/components/AIExplain.vue'
import AIGradeCase from '@/components/AIGradeCase.vue'
import type { WrongQuestion } from '@/types'
import {
  groupOptionsByBlank,
  annotateBlanksInStem,
  replaceBlankMarkers,
  extractTargetBlankNumber,
  correctLabelsByBlank,
  correctAnswerFromOptions,
  stemCoversTitle,
} from '@/utils/blanks'

const dueQuestions = ref<WrongQuestion[]>([])
const currentIndex = ref(0)
const currentBlank = ref(0)
const pageSize = 20
// sessionTotal = 首次拉取时的到期总数，整个复习会话冻结不动。复习是「从队首消耗」：
// 每次答完一题，后端都把 next_review_time 推到未来（或掌握毕业移出队列），到期集合
// 只会从最前面收缩。因此续拉永远重取 page=1 + 按 question_id 去重，就能拿到下一批、
// 且绝不跳题——若用 page+1 的 OFFSET 续拉，收缩后的集合会让中间一段被跳过去。
const sessionTotal = ref(0)
const loading = ref(false)
const loadError = ref(false)
// 已加载过的 question_id：page=1 重取会重复返回已加载题，用它去重，也防御跳题复习
const loadedIds = ref(new Set<number>())
// 服务端到期集合已取空（某次续拉没带回任何未加载的新题）。sessionTotal 冻结在首次
// 拉取值上，若期间有题目被并发毕业/移除，实际能拿到的会比它少——这时不能再空等续拉，
// 否则会永久停在"加载下一页…"。exhausted 兜底成已完成态。
const exhausted = ref(false)
// 翻卡（显示答案）状态按题存储：questionIndex -> true。
// 之前 showAnswer 是当前题全局，切题/跳转即清掉，回头看不到已翻开的答案，像没答过。
const answerStates = ref<Record<number, boolean>>({})
const showAnswer = computed(() => !!answerStates.value[currentIndex.value])
// 答题记录按题存储：questionIndex -> { blankIndex: label }。
// 之前是单个 Record<blank, label> 只对"当前题"有效，切题后清空导致回头看不到已选选项。
const answers = ref<Record<number, Record<number, string>>>({})
// 当前题的作答记录（blank index -> label），随题目切换自动落到对应题目的记录上
const currentAnswers = computed(() => answers.value[currentIndex.value] || {})

// 重取第 1 页并追加未加载过的题目。错误置 loadError，供模板给重试出口（Fix3）。
// 返回本次新增条数——续拉方据此判断服务端是否已取空。
async function loadDue() {
  if (loading.value) return
  loading.value = true
  loadError.value = false
  try {
    const res: any = await practiceApi.getDueReview({ page: 1, page_size: pageSize })
    const items = (res.data?.items || []) as WrongQuestion[]
    // 会话总数只在首次拉取时定型——后续重取返回的 total 是「剩余」数，会一直缩
    if (!sessionTotal.value) {
      sessionTotal.value = Number(res.data?.total || 0)
    }
    const fresh = items.filter((q) => !loadedIds.value.has(q.question_id))
    fresh.forEach((q) => loadedIds.value.add(q.question_id))
    dueQuestions.value.push(...fresh)
    // 续拉没带回任何新题 → 服务端到期集合已空（或已全被并发毕业），不必再等
    if (fresh.length === 0 && sessionTotal.value > 0) {
      exhausted.value = true
    }
  } catch (e) {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

onMounted(loadDue)

function getCurrent() {
  if (dueQuestions.value.length > 0 && currentIndex.value < dueQuestions.value.length) {
    return dueQuestions.value[currentIndex.value]
  }
  return null
}

const blankGroups = computed(() => groupOptionsByBlank(getCurrent()?.question?.options || []))
const totalBlanks = computed(() => blankGroups.value.length)
const currentOptions = computed(() => blankGroups.value[currentBlank.value] || [])
const targetBlankNumber = computed(() => {
  const q = getCurrent()?.question
  if (!q) return null
  return extractTargetBlankNumber(q.content || q.title || '')
})
const activeBlankNo = computed(() => targetBlankNumber.value || (currentBlank.value + 1))
const correctLabels = computed(() => correctLabelsByBlank(getCurrent()?.question?.options || []))
const correctAnswer = computed(
  () => correctLabels.value.join('') || correctAnswerFromOptions(getCurrent()?.question?.options || []),
)

const selectedAnswer = computed(() => currentAnswers.value[currentBlank.value] || '')
const answeredBlankCount = computed(
  () => blankGroups.value.filter((_, i) => !!currentAnswers.value[i]).length,
)
const canSubmit = computed(() => blankGroups.value.every((_, i) => currentAnswers.value[i]))

function selectOption(label: string) {
  if (showAnswer.value) return
  // 只写当前题的记录，不影响其他题
  const cur = { ...currentAnswers.value, [currentBlank.value]: label }
  answers.value = { ...answers.value, [currentIndex.value]: cur }
  // 多空题选完当前空自动跳到下一未作答空，减少"不知道在答哪一空"的困惑
  if (totalBlanks.value > 1 && currentBlank.value < totalBlanks.value - 1) {
    const nextUnanswered = blankGroups.value.findIndex((_, i) => i > currentBlank.value && !cur[i])
    if (nextUnanswered >= 0) {
      currentBlank.value = nextUnanswered
    } else if (!cur[currentBlank.value + 1]) {
      currentBlank.value++
    }
  }
}

/** 翻开当前题答案：只写当前题的记录，不影响其他题的翻卡状态 */
function revealAnswer() {
  answerStates.value = { ...answerStates.value, [currentIndex.value]: true }
}

const renderedStem = computed(() => {
  const q = getCurrent()?.question
  if (!q) return ''
  const raw = q.content || q.title || ''
  const marked = annotateBlanksInStem(raw, currentBlank.value, totalBlanks.value || 1, {})
  return replaceBlankMarkers(renderMd(marked))
})

function isCurrentBlankCorrect(label: string) {
  return correctLabels.value[currentBlank.value] === label
}

const answeredMap = computed(() => {
  const map: Record<number, boolean> = {}
  for (let i = 0; i < dueQuestions.value.length; i++) {
    if (answers.value[i] && Object.keys(answers.value[i]).length > 0) map[i] = true
  }
  return map
})

function jumpToQuestion(index: number) {
  currentIndex.value = index
  currentBlank.value = 0
  // 不清空 answers/answerStates：按题保留作答与翻卡记录，回头跳转仍能看到已选选项与已翻开的答案
}

// 已经加载完的题目全部答过、且还有未加载的到期题 → 从队首（page=1）续拉一批。
// 与 Fix1 设计配套：OFFSET 续拉会因到期集合收缩而跳题，这里永远从头取并去重。
async function maybeLoadMore() {
  if (loading.value || loadError.value) return
  if (currentIndex.value < dueQuestions.value.length) return // 手里还有未答的题
  if (dueQuestions.value.length >= sessionTotal.value) return // 全部到期题都已加载
  await loadDue()
}

async function review(correct: boolean) {
  const wq = getCurrent()
  if (!wq) return
  await practiceApi.review({ question_id: wq.question_id, is_correct: correct })
  currentIndex.value++
  currentBlank.value = 0
  // 不清空 answers/answerStates：该题作答与翻卡记录按题保留，回头跳转仍能看到已选选项与已翻开的答案
  await maybeLoadMore()
}

// 全部到期题都答完才算完成。答完当前已加载的最后一条、等待续拉或续拉失败时都不算完成
// （避免闪"复习完成！"、以及续拉失败时误判完成把用户困在死态里——Fix3 给重试出口）。
// 收尾条件是「指针越过全部已加载题」且「没有更多可加载」：已加载到 sessionTotal 封顶，
// 或服务端到期集合已取空（exhausted——部分题可能中途被并发毕业，实际少于首拉总数）。
const finished = computed(
  () =>
    sessionTotal.value > 0 &&
    !loadError.value &&
    currentIndex.value >= dueQuestions.value.length &&
    (dueQuestions.value.length >= sessionTotal.value || exhausted.value),
)

function nextBlank() {
  if (currentBlank.value < totalBlanks.value - 1) {
    currentBlank.value++
  }
}

function prevBlank() {
  if (currentBlank.value > 0) {
    currentBlank.value--
  }
}
</script>

<template>
  <div class="max-w-6xl mx-auto px-4">
    <div class="mb-4 text-gray-500 dark:text-gray-400">
      待复习 {{ sessionTotal }} 题
    </div>

    <div class="flex gap-6">
      <div class="flex-1 min-w-0">
        <div v-if="getCurrent()" class="bg-white dark:bg-gray-800 p-6 rounded-lg shadow transition-colors">
          <div class="mb-2 flex flex-wrap items-center gap-2">
            <el-tag v-if="totalBlanks > 1" type="warning" effect="plain" size="small" round>
              多空题 · 第 {{ currentBlank + 1 }}/{{ totalBlanks }} 空
            </el-tag>
          </div>

          <div
            v-if="!stemCoversTitle(getCurrent()!.question?.title, getCurrent()!.question?.content)"
            class="text-lg font-medium mb-4 whitespace-pre-wrap text-gray-900 dark:text-gray-100"
          >{{ getCurrent()!.question?.title }}</div>
          <div class="text-gray-600 dark:text-gray-400 mb-6 md-content leading-relaxed" v-html="renderedStem"></div>

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
              :type="currentBlank === i ? 'primary' : 'info'"
              size="small"
              @click="currentBlank = i"
            >
              第{{ i + 1 }}空
            </el-button>
          </div>

          <div class="space-y-3 mb-6">
            <div
              v-for="opt in currentOptions"
              :key="opt.id"
              class="cursor-pointer p-3 border border-gray-200 dark:border-gray-600 rounded-lg text-gray-800 dark:text-gray-200 transition-colors"
              :class="{
                'border-blue-500 bg-blue-50 dark:bg-blue-900/30': !showAnswer && selectedAnswer === opt.label,
                'border-green-500 bg-green-50 dark:bg-green-900/30': showAnswer && isCurrentBlankCorrect(opt.label),
                'border-red-500 bg-red-50 dark:bg-red-900/30': showAnswer && selectedAnswer === opt.label && !isCurrentBlankCorrect(opt.label),
              }"
              @click="selectOption(opt.label)"
            >
              <span class="font-medium mr-2">{{ opt.label }}.</span>
              <span v-html="renderMdInline(opt.content)"></span>
            </div>
          </div>

          <div v-if="totalBlanks > 1 && showAnswer" class="mb-4 text-green-600 dark:text-green-400 font-medium">
            正确答案：{{ correctAnswer }}
            <span class="ml-2 text-xs font-normal text-gray-500">
              （按空位：{{ correctLabels.map((lab, i) => `第${i + 1}空=${lab}`).join('，') }}）
            </span>
          </div>

          <div v-if="!showAnswer">
            <div v-if="totalBlanks > 1" class="flex gap-2 mb-4">
              <el-button :disabled="currentBlank === 0" @click="prevBlank">上一空</el-button>
              <el-button v-if="currentBlank < totalBlanks - 1" @click="nextBlank">下一空</el-button>
              <el-button
                type="primary"
                :disabled="!canSubmit"
                @click="revealAnswer"
              >{{ answeredBlankCount < totalBlanks ? '请完成所有空位' : '提交答案' }}</el-button>
            </div>
            <el-button
              v-else
              type="primary"
              :disabled="totalBlanks === 1 && !selectedAnswer"
              @click="revealAnswer"
            >{{ totalBlanks === 1 ? '提交答案' : '显示答案' }}</el-button>
          </div>

          <div v-else>
            <AIExplain :question="getCurrent()?.question" :answer="correctAnswer" class="mb-4" />
            <AIGradeCase
              v-if="getCurrent()?.question && (getCurrent()!.question.options || []).length === 0"
              :question="getCurrent()?.question"
              class="mb-4"
            />
            <div class="flex gap-4">
              <el-button type="success" @click="review(true)">记住了</el-button>
              <el-button type="danger" @click="review(false)">没记住</el-button>
            </div>
          </div>
        </div>

        <div v-else class="text-center py-12 text-gray-500 dark:text-gray-400">
          <template v-if="loadError">
            <div class="mb-3">加载失败，请检查网络后重试</div>
            <el-button type="primary" plain @click="loadDue">重试</el-button>
          </template>
          <template v-else-if="loading">加载中…</template>
          <template v-else-if="sessionTotal === 0">暂无待复习题目</template>
          <template v-else-if="finished">复习完成！</template>
          <template v-else>加载下一页…</template>
        </div>
      </div>

      <div class="w-72 flex-shrink-0 hidden lg:block">
        <AnswerCard
          :total="dueQuestions.length"
          :current-index="currentIndex"
          :answered-map="answeredMap"
          class="sticky top-20"
          @jump="jumpToQuestion"
        />
      </div>
    </div>

    <div class="lg:hidden mt-4">
      <AnswerCard
        :total="dueQuestions.length"
        :current-index="currentIndex"
        :answered-map="answeredMap"
        @jump="jumpToQuestion"
      />
    </div>
  </div>
</template>
