<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { renderMd, renderMdInline } from '@/utils/markdown'
import { examApi } from '@/api'
import { useUserStore } from '@/stores/user'
import AnswerCard from '@/components/AnswerCard.vue'
import { useQuizShortcuts } from '@/composables/useQuizShortcuts'
import type { Question } from '@/types'
import { isRecentPaperYear } from '@/utils/memberGate'
import { setEssayPrefill, essayTopicOf, essayPointsOf, GENERATE_ESSAY_PATH } from '@/utils/essayPrefill'
import {
  groupOptionsByBlank,
  annotateBlanksInStem,
  replaceBlankMarkers,
  correctLabelsByBlank,
} from '@/utils/blanks'
import { groupQuestions } from '@/utils/questionGroup'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const paperId = Number(route.params.id)

const questions = ref<Question[]>([])
const currentIndex = ref(0)
const loading = ref(true)
// 已浏览过的题号（答题卡「已看 X/N」进度用），初始含第 1 题
const visited = ref<Record<number, boolean>>({ 0: true })

onMounted(async () => {
  try {
    const res: any = await examApi.getPaper(paperId)
    const paper = res.data
    // 近三年真题会员专属：直进背题路由时兜底拦截（与考试模式同文案）
    if (isRecentPaperYear(paper?.year || 0) && paper?.source === 0 && !userStore.isMember) {
      ElMessage.warning('近三年真题为会员专属功能，开通会员后即可练习/考试')
      router.replace('/exams')
      return
    }
    questions.value = paper.questions || []
  } catch {
    router.push('/exams')
  } finally {
    loading.value = false
  }
})

const currentQuestion = computed(() => questions.value[currentIndex.value])
// 答题卡按大题分组：背题页与答题页读同一份卷子，分组口径必须一致
const questionGroups = computed(() => groupQuestions(questions.value))
// 论文题（question_type=4）：背题时可以直接拿题面 + 写作要点去生成一篇范文
const isEssay = computed(() => currentQuestion.value?.question_type === 4)
const blankGroups = computed(() => groupOptionsByBlank(currentQuestion.value?.options || []))
const totalBlanks = computed(() => blankGroups.value.length)
// 每空正确选项 label（数组下标即空位编号），供选项高亮与答案行拼接
const correctLabels = computed(() => correctLabelsByBlank(currentQuestion.value?.options || []))
// 是否至少有一空标注了标准答案：有则显示「正确答案」行，否则提示看解析
const hasStandardAnswer = computed(() => correctLabels.value.some((l) => !!l))

// 答题卡进度：跳到哪里标哪里为「已看」
watch(currentIndex, (i) => {
  visited.value[i] = true
})

// 把题干里的空白用「正确答案」填上，背题时一眼看清每空答案。
// currentBlank 传一个不会命中的值，让所有空位都填上答案而非高亮某空。
function reviewStemHtml(q: Question): string {
  const raw = q.content || q.title || ''
  const labels = correctLabelsByBlank(q.options || [])
  const groups = groupOptionsByBlank(q.options || [])
  if (groups.length <= 1) return renderMd(raw)
  return replaceBlankMarkers(
    annotateBlanksInStem(raw, groups.length + 1, groups.length, labels as Record<number, string>),
  )
}

function prev() {
  if (currentIndex.value > 0) currentIndex.value--
}
function next() {
  if (currentIndex.value < questions.value.length - 1) currentIndex.value++
}
function gotoQuestion(index: number) {
  currentIndex.value = index
}

// 一键生成范文：题目当主题，写作要点当论点，跳到 AI 生成范文页预填。
// 要点很长（单题 5000+ 字符），走 sessionStorage 交接，见 utils/essayPrefill。
function generateEssayFromQuestion() {
  const q = currentQuestion.value
  if (!q) return
  setEssayPrefill({ topic: essayTopicOf(q), points: essayPointsOf(q) })
  router.push(GENERATE_ESSAY_PATH)
}

useQuizShortcuts({
  enabled: () => !!currentQuestion.value,
  onPrev: prev,
  onNext: next,
})
</script>

<template>
  <div class="max-w-6xl mx-auto px-4">
    <div v-if="loading" class="text-center py-20 text-gray-500">
      <div class="animate-spin inline-block w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full mb-4"></div>
      <div>加载中...</div>
    </div>

    <template v-else-if="currentQuestion">
      <!-- Header -->
      <div class="flex items-center justify-between mb-4 bg-white dark:bg-gray-800 p-3 rounded shadow transition-colors">
        <span class="text-lg font-bold text-gray-900 dark:text-gray-100">
          软考真题<span class="text-emerald-500 text-base font-medium"> · 背题模式</span>
        </span>
        <div class="flex items-center gap-4">
          <span class="text-gray-500 dark:text-gray-400">第 {{ currentIndex + 1 }} / {{ questions.length }} 题</span>
          <el-button size="small" @click="router.push('/exams')">返回试卷列表</el-button>
        </div>
      </div>

      <div class="flex gap-6">
        <!-- Question Area -->
        <div class="flex-1 min-w-0">
          <div class="bg-white dark:bg-gray-800 p-6 rounded-lg shadow transition-colors">
            <div class="text-sm text-gray-500 dark:text-gray-400 mb-3">第 {{ currentIndex + 1 }} / {{ questions.length }} 题</div>

            <!-- 题干（多空题空位填正确答案） -->
            <div class="text-lg font-medium mb-4 md-content leading-relaxed text-gray-900 dark:text-gray-100" v-html="reviewStemHtml(currentQuestion)"></div>

            <!-- 论文题专属：一键生成范文。放题干下方而不是解析区之后 ——
                 论文题的写作要点动辄五千字，放下面得滚过整段才看得到。 -->
            <div v-if="isEssay" class="mb-4">
              <el-button type="primary" plain @click="generateEssayFromQuestion">一键生成范文</el-button>
              <span class="ml-3 text-xs text-gray-400 dark:text-gray-500">把本题题目与写作要点带到 AI 生成范文</span>
            </div>

            <!-- 选项：分空展示，正确项绿框 + ✓（客观题） -->
            <template v-if="totalBlanks > 0">
              <div v-for="(group, gi) in blankGroups" :key="gi" class="mb-4">
                <div v-if="totalBlanks > 1" class="mb-2 text-sm font-semibold text-gray-500 dark:text-gray-400">
                  第 {{ gi + 1 }} 空
                </div>
                <div class="space-y-3">
                  <div
                    v-for="opt in group"
                    :key="opt.id"
                    class="p-3 border rounded-lg transition text-gray-800 dark:text-gray-200"
                    :class="correctLabels[gi] === opt.label
                      ? 'border-green-500 bg-green-50 dark:bg-green-900/30'
                      : 'border-gray-200 dark:border-gray-600'"
                  >
                    <span class="font-medium mr-2">{{ opt.label }}.</span>
                    <span v-html="renderMdInline(opt.content)"></span>
                    <el-tag v-if="correctLabels[gi] === opt.label" type="success" size="small" effect="plain" class="ml-2">
                      ✓ 正确答案
                    </el-tag>
                  </div>
                </div>
              </div>
            </template>

            <!-- 正确答案行 / 主观题提示 -->
            <div v-if="hasStandardAnswer" class="mb-3 text-green-600 dark:text-green-400 font-medium">
              正确答案：{{ correctLabels.join('') }}
            </div>
            <div v-else class="mb-3 text-sm text-gray-500 dark:text-gray-400">参考答案见解析</div>

            <!-- 解析 -->
            <div v-if="currentQuestion.explanation" class="mb-3 p-3 bg-gray-50 dark:bg-gray-700/50 rounded text-gray-600 dark:text-gray-400">
              <div class="font-medium mb-1">解析：</div>
              <div class="md-content" v-html="renderMd(currentQuestion.explanation)"></div>
            </div>
            <!-- 分析 -->
            <div v-if="currentQuestion.analysis" class="mb-3 p-3 bg-gray-50 dark:bg-gray-700/50 rounded text-gray-600 dark:text-gray-400">
              <div class="font-medium mb-1">分析：</div>
              <div class="md-content" v-html="renderMd(currentQuestion.analysis)"></div>
            </div>
            <!-- 无解析 -->
            <div v-if="!currentQuestion.explanation && !currentQuestion.analysis" class="mb-3 text-sm text-gray-400 dark:text-gray-500">
              暂无解析
            </div>

            <!-- 上/下一题 -->
            <div class="flex justify-between mt-6">
              <el-button :disabled="currentIndex === 0" @click="prev">上一题</el-button>
              <el-button :disabled="currentIndex >= questions.length - 1" type="primary" @click="next">下一题</el-button>
            </div>
          </div>
        </div>

        <!-- Answer Card (Right Side) -->
        <div class="w-72 flex-shrink-0 hidden lg:block">
          <AnswerCard
            :total="questions.length"
            :current-index="currentIndex"
            :groups="questionGroups"
            :answered-map="visited"
            class="sticky top-20"
            @jump="gotoQuestion"
          />
        </div>
      </div>

      <!-- Mobile Answer Card (Bottom) -->
      <div class="lg:hidden mt-4">
        <AnswerCard
          :total="questions.length"
          :current-index="currentIndex"
          :groups="questionGroups"
          :answered-map="visited"
          @jump="gotoQuestion"
        />
      </div>
    </template>
  </div>
</template>
