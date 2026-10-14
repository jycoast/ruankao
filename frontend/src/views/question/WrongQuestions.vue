<script setup lang="ts">
import { renderMd, renderMdInline } from '@/utils/markdown'
import { groupOptionsByBlank, annotateBlanksInStem, replaceBlankMarkers, correctLabelsByBlank, correctAnswerFromOptions } from '@/utils/blanks'
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { practiceApi } from '@/api'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { loginGuestRoute } from '@/utils/guestGate'
import { requireMember } from '@/utils/memberGate'
import AnswerCard from '@/components/AnswerCard.vue'
import ShortcutHint from '@/components/ShortcutHint.vue'
import AIExplain from '@/components/AIExplain.vue'
import AIGradeCase from '@/components/AIGradeCase.vue'
import { useQuizShortcuts } from '@/composables/useQuizShortcuts'
import type { Question, WrongQuestion } from '@/types'
import { isDueReview, matchKeyword, sortByLatestWrong } from '@/utils/wrongBook'
import WrongQuestionList from '@/components/wrongbook/WrongQuestionList.vue'
import WrongQuestionDetail from '@/components/wrongbook/WrongQuestionDetail.vue'
import { List, Search } from '@element-plus/icons-vue'

const wrongQuestions = ref<WrongQuestion[]>([])
const loading = ref(false)
const loadError = ref(false)
const keyword = ref('')
const dueOnly = ref(false)

// 分页加载（同笔记页 NoteList 的模式）：
// 首屏只拉第 1 页立即渲染，滚动接近底部由 WrongQuestionList 的哨兵触发续页。
// wrongQuestions 只持有「已加载」的增量前缀（后端按 created_at DESC 返回，
// 页码递增即时间倒序推进，跨页顺序天然正确）。
const PAGE_SIZE = 50 // 与后端 parsePage 对普通用户的上限一致
const page = ref(1)
const total = ref(0)
const hasMore = ref(false)
// 翻页专用 loading（区别于首屏 loading 的整屏 spinner）；兼作 loadMore/ensureAllLoaded 互斥锁
const listLoading = ref(false)
// 翻页失败锁：失败后不能再自动续页——loadMore 是先 page++ 再请求，若失败还继续翻
// 就会跳过失败那一页、永久缺一块（同 NoteList）。失败态由底部「重试」按钮显式推进。
const loadFailed = ref(false)
// 请求序号：loadWrong() 重置时自增；旧的在途响应回来时发现序号过期即丢弃，
// 避免把旧页数据拼到新列表上。
let reqSeq = 0
const redoLoading = ref(false)
// 左栏当前选中行（右栏详情随之联动）
const selectedId = ref<number | null>(null)

const mode = ref<'list' | 'practice'>('list')
const questions = ref<Question[]>([])
const currentIndex = ref(0)
// 答题记录按题存储：questionIndex -> { blankIndex: label }。
// 之前是单个 Record<blank, label> 只对"当前题"有效，切题后不清理就把上一题选中串到下一题。
const answers = ref<Record<number, Record<number, string>>>({})
const currentBlank = ref(0)
// 判题结果按题存储：questionIndex -> { showResult, result }。
// 之前 showResult/result 是当前题全局，切题/跳转即清掉，回头看不到已提交的对/错与解析，像没做过。
const resultStates = ref<Record<number, { showResult: boolean; result: any }>>({})
const currentResultState = computed(() => resultStates.value[currentIndex.value] || { showResult: false, result: {} })
const showResult = computed(() => currentResultState.value.showResult)
const result = computed(() => currentResultState.value.result || {})
const questionStartTime = ref(Date.now())

const router = useRouter()
const userStore = useUserStore()
const isGuest = computed(() => userStore.isGuest)

const dueCount = computed(() => wrongQuestions.value.filter((wq) => isDueReview(wq, new Date())).length)

const filtered = computed(() => {
  const now = new Date()
  return wrongQuestions.value.filter(
    (wq) => (!dueOnly.value || isDueReview(wq, now)) && matchKeyword(wq, keyword.value),
  )
})

// 左栏展示项：筛选后按最新答错排序（最新在前）；重做也按此顺序
const visibleItems = computed(() => sortByLatestWrong(filtered.value))

// 左栏选中 → 右栏详情联动；被选中的行若被移走，选下一题兜底
const selectedWq = computed(() => wrongQuestions.value.find((wq) => wq.id === selectedId.value) ?? null)

function selectRow(id: number) {
  selectedId.value = id
  drawer.value = false // 窄屏抽屉里点选后自动收起，露出详情
}

// 窄屏左栏收进抽屉。matchMedia 而非纯 CSS：抽屉开合是 JS 状态，需要知道当前断点。
// 断点与 Tailwind 的 lg（1024px）一致；加 typeof window 守卫兼容预渲染（同笔记页）。
const drawer = ref(false)
const isNarrow = ref(typeof window !== 'undefined' && window.matchMedia('(max-width: 1023px)').matches)
const mql = typeof window !== 'undefined' ? window.matchMedia('(max-width: 1023px)') : null
const onMql = (e: MediaQueryListEvent) => {
  isNarrow.value = e.matches
  if (!e.matches) drawer.value = false
}
mql?.addEventListener('change', onMql)
onUnmounted(() => {
  mql?.removeEventListener('change', onMql)
})

// 重置回第 1 页（首屏 / 「全部」重载 / 重试共用）。整屏 spinner 由 loading 控制，
// 只有这一个入口会整屏转圈——续页是静默的在底部追加。
async function loadWrong() {
  loading.value = true
  loadError.value = false
  reqSeq++
  page.value = 1
  hasMore.value = false
  loadFailed.value = false
  try {
    const res: any = await practiceApi.getWrongQuestions({ status: 0, page: 1, page_size: PAGE_SIZE })
    const items: WrongQuestion[] = res.data?.items || []
    total.value = Number(res.data?.total || 0)
    wrongQuestions.value = items
    hasMore.value = items.length < total.value
  } catch {
    loadError.value = true
    ElMessage.error('错题本加载失败')
  } finally {
    loading.value = false
    // 首屏完成时若已有过滤条件（比如列表还在飞时用户就输入了搜索词）
    // 就得把全集拉齐——否则 watch(keyword) 早触发、当时 hasMore 还是 false
    // 已经空跑过了，这里补上，过滤才不会漏掉后面的页。
    if (keyword.value.trim() || dueOnly.value) ensureAllLoaded()
  }
}

// 拉下一页追加到列表尾部（loadMore 与 ensureAllLoaded 都走这里）。
// hasMore / listLoading 双守卫：哨兵反复触发、或 ensureAllLoaded 并发进来时，
// 同一时刻只有一个请求在飞，页码不会乱。
async function loadOnePage() {
  if (!hasMore.value || listLoading.value || loadFailed.value) return
  const seq = reqSeq
  const nextPage = page.value + 1
  listLoading.value = true
  loadFailed.value = false
  try {
    const res: any = await practiceApi.getWrongQuestions({ status: 0, page: nextPage, page_size: PAGE_SIZE })
    if (seq !== reqSeq) return // loadWrong() 已重置，这批数据作废
    const items: WrongQuestion[] = res.data?.items || []
    total.value = Number(res.data?.total || 0)
    wrongQuestions.value = wrongQuestions.value.concat(items)
    page.value = nextPage
    hasMore.value = wrongQuestions.value.length < total.value
  } catch {
    loadFailed.value = true // 底部出「重试」，失败的那一页不跳过
  } finally {
    listLoading.value = false
  }
}

// 哨兵触发：追加下一页（守卫都在 loadOnePage 里）
function loadMore() {
  loadOnePage()
}

// 底部「重试」：清掉失败锁后重发失败的那一页（page 未推进，重试不跳过）
function loadRetry() {
  loadFailed.value = false
  loadOnePage()
}

// 把剩余页拉齐。过滤（搜索词 / 待复习）与重做都是作用于 wrongQuestions 的前端逻辑，
// 只对已加载的增量生效会漏掉后面的题，所以这些动作前先把全集拿全；
// 「惰性」在于默认浏览时并不拉齐——滚动续页即可。
// 失败时停下，等已有内容的底部失败态重试，不无限重试。
async function ensureAllLoaded() {
  while (hasMore.value) {
    await loadOnePage()
    if (loadFailed.value) break
    if (listLoading.value) await new Promise((r) => setTimeout(r, 30)) // 被并发拉页占用则让一轮
  }
}

onMounted(() => loadWrong())

// 输入搜索词后只过滤「已加载」会搜不到后面页的题，先拉齐再过滤。
watch(keyword, (v) => {
  if (v.trim()) ensureAllLoaded()
})

// 「全部」= 取消待复习筛选 + 重新拉数据（原「刷新」按钮功能并入，重复点也生效）。
// 不依赖 el-segmented 的 change——切到当前项不会触发事件。
async function showAll() {
  dueOnly.value = false
  await loadWrong()
}

// 「待复习」= 只看待复习的题：这是前端过滤（后端待复习是另一套接口），
// 同样要先拉齐才能保证不漏掉后面页里到期的题。
function showDue() {
  dueOnly.value = true
  ensureAllLoaded()
}

// 跳转会员专属 AI 诊断报告页：未达会员先弹引导，通过才跳。
function goDiagnosis() {
  if (!requireMember('diagnosis')) return
  router.push('/my/diagnosis')
}

async function handleDelete(wq: WrongQuestion) {
  await practiceApi.deleteWrongQuestion(wq.question_id)
  wrongQuestions.value = wrongQuestions.value.filter((x) => x.question_id !== wq.question_id)
  total.value = Math.max(0, total.value - 1) // 头部「共 N 条」跟随本地删除即时减一
  // 组、计数、筛选结果全部由 computed 派生，随数组即时更新，组空自动消失
  // 被删的是当前选中行 → 右栏自动切到下一题（保持有一题可见）
  if (selectedId.value === wq.id) {
    selectedId.value = wrongQuestions.value.find((x) => x.id !== wq.id)?.id ?? null
  }
}

// 重新做题要覆盖**全部**错题——若只拿已加载的增量，没滚到底的页就被漏掉了。
// 先静默拉齐，再按当前筛选取题；按钮 loading 期间用户在等待，不做无提示的假点击。
async function startRedo() {
  if (redoLoading.value) return
  redoLoading.value = true
  try {
    await ensureAllLoaded()
    const list = visibleItems.value.map((wq) => wq.question).filter(Boolean)
    if (list.length === 0) return
    questions.value = list
    currentIndex.value = 0
    answers.value = {}
    resultStates.value = {}
    currentBlank.value = 0
    mode.value = 'practice'
  } finally {
    redoLoading.value = false
  }
}

function backToList() {
  mode.value = 'list'
  questions.value = []
  resultStates.value = {}
  // 不清 selectedId：practice 结束后回到离开时的筛选/选中视图，位置不丢
}

const currentQuestion = computed(() => {
  if (questions.value.length > 0 && currentIndex.value < questions.value.length) {
    return questions.value[currentIndex.value]
  }
  return null
})

const blankGroups = computed(() => groupOptionsByBlank(currentQuestion.value?.options || []))

const totalBlanks = computed(() => blankGroups.value.length)
const currentOptions = computed(() => blankGroups.value[currentBlank.value] || [])
// 当前题的作答记录（blank index -> label），随题目切换自动落到对应题目的记录上
const currentAnswers = computed(() => answers.value[currentIndex.value] || {})
const selectedAnswer = computed(() => currentAnswers.value[currentBlank.value] || '')

const answeredBlankCount = computed(
  () => blankGroups.value.filter((_, i) => !!currentAnswers.value[i]).length,
)

/** Highlight the active blank in stem so multi-blank / split-blank redo is not ambiguous. */
const renderedStem = computed(() => {
  const q = currentQuestion.value
  if (!q) return ''
  const raw = q.content || q.title || ''
  const marked = annotateBlanksInStem(raw, currentBlank.value, totalBlanks.value || 1, currentAnswers.value)
  return replaceBlankMarkers(renderMd(marked))
})

const fullAnswer = computed(() => {
  let ans = ''
  for (let i = 0; i < totalBlanks.value; i++) {
    ans += currentAnswers.value[i] || ''
  }
  return ans
})

const canSubmit = computed(() => {
  if (totalBlanks.value <= 1) return !!selectedAnswer.value
  return blankGroups.value.every((_, i) => currentAnswers.value[i])
})

function selectOption(label: string) {
  if (showResult.value) return
  // 只写当前题的记录，不影响其他题
  const cur = { ...currentAnswers.value, [currentBlank.value]: label }
  answers.value = { ...answers.value, [currentIndex.value]: cur }
  if ((totalBlanks.value || 0) > 1 && currentBlank.value < totalBlanks.value - 1) {
    const nextUnanswered = blankGroups.value.findIndex((_, i) => i > currentBlank.value && !cur[i])
    if (nextUnanswered >= 0) {
      currentBlank.value = nextUnanswered
    } else if (!cur[currentBlank.value + 1]) {
      currentBlank.value++
    }
  }
}

async function submitAnswer() {
  const q = currentQuestion.value
  if (!q) return
  // 提交期间的题目索引固定下来，避免 await 期间用户切题导致结果写到错误题目的记录上
  const idx = currentIndex.value
  const res: any = await practiceApi.submitAnswer({
    question_id: q.id,
    user_answer: fullAnswer.value,
    practice_type: 5,
    time_spent: Math.floor((Date.now() - questionStartTime.value) / 1000),
  })
  resultStates.value = { ...resultStates.value, [idx]: { showResult: true, result: res.data } }
}

function nextQuestion() {
  questionStartTime.value = Date.now()
  if (currentIndex.value < questions.value.length - 1) {
    currentIndex.value++
    currentBlank.value = 0
    // 不重置判题结果：按题保留，回头跳转仍能看到已提交的对/错与解析
  } else {
    // 最后一题点"完成"：结束本轮重做，回到错题本列表
    ElMessage.success('已完成全部错题重做！')
    backToList()
  }
}

/** 重练中确认已掌握：把当前题移出错题本（硬删除），并处理删除后的索引位移 */
function removeCurrentFromWrongBook() {
  const q = currentQuestion.value
  if (!q) return
  ElMessageBox.confirm('确定将该题移出错题本？移出后将不再出现在错题本中。', '移出错题本', {
    type: 'warning',
    confirmButtonText: '移出',
    cancelButtonText: '取消',
  })
    .then(async () => {
      const idx = currentIndex.value
      await practiceApi.deleteWrongQuestion(q.id)
      questions.value.splice(idx, 1)
      // 本地数组同步修剪，否则 backToList 不重拉后会残留已移出的题
      wrongQuestions.value = wrongQuestions.value.filter((wq) => wq.question_id !== q.id)
      // answers/resultStates 以题目下标为键，删中间题后重建映射防止后续作答串位
      const shiftMap = <T>(map: Record<number, T>) => {
        const next: Record<number, T> = {}
        for (const [k, v] of Object.entries(map)) {
          const n = Number(k)
          if (n < idx) next[n] = v
          else if (n > idx) next[n - 1] = v
        }
        return next
      }
      answers.value = shiftMap(answers.value)
      resultStates.value = shiftMap(resultStates.value)
      currentBlank.value = 0
      questionStartTime.value = Date.now()
      ElMessage.success('已移出错题本')
      if (questions.value.length === 0) {
        backToList()
        return
      }
      if (currentIndex.value >= questions.value.length) {
        currentIndex.value = questions.value.length - 1
      }
    })
    .catch(() => {}) // 用户取消确认，不处理
}

function isCurrentBlankCorrect(label: string) {
  return correctLabels.value[currentBlank.value] === label
}

const correctLabels = computed(() => correctLabelsByBlank(currentQuestion.value?.options || []))
const correctAnswer = computed(() => correctLabels.value.join('') || correctAnswerFromOptions(currentQuestion.value?.options || []))

const answeredMap = computed(() => {
  const map: Record<number, boolean> = {}
  for (let i = 0; i < questions.value.length; i++) {
    if (answers.value[i] && Object.keys(answers.value[i]).length > 0) map[i] = true
  }
  return map
})

function jumpToQuestion(index: number) {
  currentIndex.value = index
  currentBlank.value = 0
  // 不重置判题结果：按题保留，回头跳转仍能看到已提交的对/错与解析
}

function renderContent(text: string) {
  return renderMd(text)
}

useQuizShortcuts({
  enabled: () => !!currentQuestion.value,
  onSelect: (label) => {
    if (currentOptions.value.some(o => o.label === label)) selectOption(label)
  },
  onEnter: () => {
    if (showResult.value) nextQuestion()
    else if (canSubmit.value) submitAnswer()
  },
  onPrev: () => {
    if (currentIndex.value > 0) jumpToQuestion(currentIndex.value - 1)
  },
  onNext: () => {
    if (currentIndex.value < questions.value.length - 1) jumpToQuestion(currentIndex.value + 1)
  },
})

</script>

<template>
  <!-- List Mode（双栏：左列表独立滚动 + 右详情，布局参考笔记页） -->
  <div v-if="mode === 'list'" class="wrongbook-shell flex flex-col px-3 sm:px-4">
    <div class="mb-3 flex shrink-0 flex-wrap items-center gap-3">
      <el-button v-if="isNarrow" :icon="List" @click="drawer = true">错题列表</el-button>
      <h2 class="text-lg font-bold text-gray-900 dark:text-gray-100">
        错题本
        <!-- 「共 N 条」用接口全量 total（分页加载下 wrongQuestions 只是已加载前缀）；
             待复习数是已加载范围里的（滚动加载后更接近全量） -->
        <span class="ml-1.5 text-xs font-normal text-gray-500 dark:text-gray-400">共 {{ total }} 条 · 待复习 {{ dueCount }}</span>
      </h2>
      <div class="ml-auto flex flex-wrap items-center gap-2">
        <el-input
          v-model="keyword"
          placeholder="搜题干 / 选项"
          clearable
          class="w-full sm:!w-56"
          :prefix-icon="Search"
        />
        <el-button type="primary" :plain="dueOnly" @click="showAll">全部</el-button>
        <el-button type="primary" :plain="!dueOnly" @click="showDue">待复习</el-button>
        <el-button type="primary" plain @click="goDiagnosis">AI 诊断报告</el-button>
        <el-button v-if="filtered.length > 0" type="primary" :loading="redoLoading" @click="startRedo">
          重新做题
        </el-button>
      </div>
    </div>

    <div v-if="loading" class="flex min-h-0 flex-1 flex-col items-center justify-center gap-3">
      <!-- 加载态：旋转环 + 主/副标题，同知识点导图页 -->
      <svg
        class="h-10 w-10 animate-spin text-blue-500"
        viewBox="0 0 24 24"
        fill="none"
        aria-hidden="true"
      >
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
      </svg>
      <div class="text-base font-semibold text-gray-700 dark:text-gray-200">正在加载中，请稍后</div>
      <div class="text-xs text-gray-400 dark:text-gray-500">错题数据加载中…</div>
    </div>

    <div v-else-if="loadError" class="flex min-h-0 flex-1 flex-col items-center justify-center py-12 text-center text-gray-500">
      <p>错题本加载失败</p>
      <el-button class="mt-3" type="primary" plain @click="loadWrong">重试</el-button>
    </div>

    <div v-else-if="wrongQuestions.length === 0" class="flex min-h-0 flex-1 flex-col items-center justify-center py-12 text-center">
      <template v-if="isGuest">
        <p class="text-gray-500">登录后即可同步你的错题本</p>
        <el-button class="mt-3" type="primary" plain size="small" @click="router.push(loginGuestRoute())">去登录</el-button>
      </template>
      <p v-else class="text-gray-500">暂无错题，继续加油！</p>
    </div>

    <div v-else-if="filtered.length === 0" class="flex min-h-0 flex-1 flex-col items-center justify-center py-12 text-center">
      <p class="text-gray-500">没有符合条件的错题</p>
      <el-button class="mt-3" type="primary" plain @click="keyword = ''; dueOnly = false">清除筛选</el-button>
    </div>

    <div v-else class="flex min-h-0 flex-1 gap-4">
      <!-- 宽屏：常驻左栏，列表自身独立滚动 -->
      <aside v-if="!isNarrow" class="flex w-[380px] shrink-0 flex-col">
        <WrongQuestionList
          :items="visibleItems"
          :selected-id="selectedId"
          :has-more="hasMore"
          :loading-more="listLoading"
          :load-failed="loadFailed"
          @select="selectRow"
          @load-more="loadMore"
          @retry="loadRetry"
        />
      </aside>

      <!-- 窄屏：左栏收进抽屉，点选后自动收起露出详情 -->
      <el-drawer v-else v-model="drawer" title="错题列表" direction="ltr" size="86%">
        <div class="flex h-full min-h-0 flex-col">
          <WrongQuestionList
            :items="visibleItems"
            :selected-id="selectedId"
            :has-more="hasMore"
            :loading-more="listLoading"
            :load-failed="loadFailed"
            @select="selectRow"
            @load-more="loadMore"
            @retry="loadRetry"
          />
        </div>
      </el-drawer>

      <!-- min-w-0：详情里一张宽图 / 一段不换行代码块不会撑破此行出现横向滚动。
           本页要求只有两栏各自纵向滚动、页面不滚。 -->
      <section class="min-h-0 min-w-0 flex-1 overflow-y-auto rounded-lg bg-white shadow dark:bg-gray-800">
        <WrongQuestionDetail
          v-if="selectedWq"
          :wq="selectedWq"
          @remove="handleDelete(selectedWq)"
        />
        <div v-else class="flex h-full flex-col items-center justify-center gap-3 p-6 text-center text-gray-400 dark:text-gray-500">
          <span class="text-4xl">👈</span>
          <p>从左侧选择一道错题查看详情</p>
        </div>
      </section>
    </div>
  </div>

  <!-- Practice Mode -->
  <div v-else class="max-w-6xl mx-auto px-4">
    <div class="mb-4 flex items-center gap-3">
      <el-button @click="backToList" size="small"><el-icon class="mr-1"><ArrowLeft /></el-icon> 返回</el-button>
      <span class="text-sm text-gray-500 dark:text-gray-400">错题重做</span>
      <div class="ml-auto flex items-center gap-2 whitespace-nowrap">
        <el-tag v-if="totalBlanks > 1" type="warning" effect="plain" size="small" round>
          多空题 · 第 {{ currentBlank + 1 }}/{{ totalBlanks }} 空
        </el-tag>
        <span class="text-sm text-gray-500 dark:text-gray-400">{{ currentIndex + 1 }}/{{ questions.length }}</span>
      </div>
    </div>
    <el-progress :percentage="Math.round((currentIndex / questions.length) * 100)" class="mb-4" />

    <div class="flex gap-6">
      <!-- Question Area -->
      <div class="flex-1 min-w-0">
        <div v-if="currentQuestion" class="bg-white dark:bg-gray-800 p-6 rounded-lg shadow transition-colors">
          <div class="md-content mb-6 leading-relaxed text-gray-800 dark:text-gray-200 prose max-w-none" v-html="renderedStem"></div>

          <div
            v-if="totalBlanks > 1"
            class="mb-3 flex items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-700 dark:border-blue-800/60 dark:bg-blue-950/40 dark:text-blue-300"
          >
            <span class="inline-flex h-6 min-w-6 items-center justify-center rounded-full bg-blue-600 px-2 text-xs text-white">{{ currentBlank + 1 }}</span>
            <span>当前选项对应：第 {{ currentBlank + 1 }} 空 / 共 {{ totalBlanks }} 空</span>
            <span v-if="selectedAnswer" class="ml-auto text-xs font-normal text-blue-600/80 dark:text-blue-300/80">已选 {{ selectedAnswer }}</span>
          </div>

          <div v-if="totalBlanks > 1" class="mb-4 flex gap-2 flex-wrap">
            <el-button
              v-for="(_, i) in blankGroups"
              :key="i"
              :type="currentBlank === i ? 'primary' : (currentAnswers[i] ? 'success' : 'info')"
              size="small"
              @click="currentBlank = i"
            >
              第{{ i + 1 }}空 {{ currentAnswers[i] ? `(${currentAnswers[i]})` : '' }}
            </el-button>
          </div>

          <div
            v-if="currentOptions.length > 0"
            class="mb-6 space-y-3"
            :aria-label="totalBlanks > 1 ? `第 ${currentBlank + 1} 空选项` : '选项'"
          >
<div v-for="opt in currentOptions" :key="opt.id"
              class="p-3 border border-gray-200 dark:border-gray-600 rounded-lg cursor-pointer transition text-gray-800 dark:text-gray-200"
              :class="{
                'border-blue-500 bg-blue-50 dark:bg-blue-900/30': !showResult && selectedAnswer === opt.label,
                'border-green-500 bg-green-50 dark:bg-green-900/30': showResult && isCurrentBlankCorrect(opt.label),
                'border-red-500 bg-red-50 dark:bg-red-900/30': showResult && selectedAnswer === opt.label && !isCurrentBlankCorrect(opt.label),
              }" @click="selectOption(opt.label)">
              <span class="font-medium mr-2">{{ opt.label }}.</span><span v-html="renderMdInline(opt.content)"></span>
            </div>
          </div>

          <div v-if="totalBlanks > 1 && showResult" class="mb-4 text-green-600 dark:text-green-400 font-medium">
            正确答案：{{ correctAnswer }}
          </div>

          <div v-if="!showResult">
            <div v-if="totalBlanks > 1" class="mb-3 flex flex-wrap items-center gap-2">
              <el-button :disabled="currentBlank === 0" @click="currentBlank--">上一空</el-button>
              <el-button v-if="currentBlank < totalBlanks - 1" @click="currentBlank++">下一空</el-button>
              <span class="text-xs text-gray-400">当前第 {{ currentBlank + 1 }}/{{ totalBlanks }} 空</span>
            </div>
            <el-button type="primary" @click="submitAnswer" :disabled="!canSubmit">
              {{ totalBlanks > 1 && answeredBlankCount < totalBlanks ? '请完成所有空位' : '提交' }}
            </el-button>
            <ShortcutHint />
          </div>
          <div v-else>
            <el-alert :title="result.is_correct ? '回答正确！' : '回答错误'" :type="result.is_correct ? 'success' : 'error'" class="mb-4" />
            <div v-if="result.explanation" class="text-gray-600 dark:text-gray-400 mb-4 p-3 bg-gray-50 dark:bg-gray-700/50 rounded">
              <div class="font-medium">解析：</div><div class="md-content" v-html="renderMd(result.explanation)"></div>
            </div>
            <div v-if="result.analysis" class="text-gray-600 dark:text-gray-400 mb-4 p-3 bg-gray-50 dark:bg-gray-700/50 rounded">
              <div class="font-medium">分析：</div><div class="md-content" v-html="renderMd(result.analysis)"></div>
            </div>
            <AIExplain v-if="currentQuestion" :question="currentQuestion" :answer="result.answer" class="mb-4" />
            <AIGradeCase
              v-if="currentQuestion && (currentQuestion.options || []).length === 0"
              :question="currentQuestion"
              class="mb-4"
            />
            <el-button type="warning" plain @click="removeCurrentFromWrongBook">已掌握，移出错题本</el-button>
            <el-button type="primary" @click="nextQuestion">{{ currentIndex < questions.length - 1 ? '下一题' : '完成' }}</el-button>
          </div>
        </div>
        <div v-else class="text-center py-12 text-gray-500 dark:text-gray-400">暂无题目</div>
      </div>

      <!-- Answer Card (Right Side) -->
      <div class="w-72 flex-shrink-0 hidden lg:block">
        <AnswerCard
          :total="questions.length"
          :current-index="currentIndex"
          :answered-map="answeredMap"
          class="sticky top-20"
          @jump="jumpToQuestion"
        />
      </div>
    </div>

    <!-- Mobile Answer Card -->
    <div class="lg:hidden mt-4">
      <AnswerCard
        :total="questions.length"
        :current-index="currentIndex"
        :answered-map="answeredMap"
        @jump="jumpToQuestion"
      />
    </div>
  </div>
</template>

<style scoped>
/* 顶栏 h-14 = 3.5rem，另需扣掉刘海安全区（AppLayout 的 pt-[env(safe-area-inset-top)]）。
   还要减掉 AppLayout <main> 的 py-6（上下各 1.5rem，合计 3rem）：不减的话整页会多出
   3rem 溢出，页面自身就出现滚动条，而本页要求左右两栏各自滚动、页面不滚。
   不写进 Tailwind 任意值是怕 env() 里的连字符被解析器切错。
   本规则是 ≥1280px（Tailwind xl）那一档：该宽度下 <header> 里只剩 h-14 那一行
   （实测 57px = 56 顶栏 + 1px 底边框；这里仍减 3.5rem，残余 1px 溢出）。
   更窄的档位 <header> 会多出下面的行，高度改由下面的 @media 覆盖。 */
.wrongbook-shell {
  height: calc(100vh - 3.5rem - 3rem - env(safe-area-inset-top, 0px));
}

/* < 1280px（Tailwind xl）：<header> 里还多一行移动端导航
   （AppLayout 的 nav.no-scrollbar.xl:hidden）。
   实测 <header> 高度 = 89px（56 顶栏 + 32 导航行 + 1px 底边框）= 5.5625rem，
   比 h-14 多 33px；不减就会把两栏底部切掉。 */
@media (max-width: 1279px) {
  .wrongbook-shell {
    height: calc(100vh - 5.5625rem - 3rem - env(safe-area-inset-top, 0px));
  }
}

/* < 640px（Tailwind sm）：再多一行移动端科目选择器
   （AppLayout 的 div.mb-1.block.sm:hidden）。
   实测 <header> 高度 = 131px（56 顶栏 + 4 选择器 mb-1 + 38 选择器行 + 32 导航行
   + 1px 底边框）= 8.1875rem。 */
@media (max-width: 639px) {
  .wrongbook-shell {
    height: calc(100vh - 8.1875rem - 3rem - env(safe-area-inset-top, 0px));
  }
}
</style>
