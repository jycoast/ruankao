<script setup lang="ts">
import { renderMd, renderMdInline } from '@/utils/markdown'
import {
  groupOptionsByBlank,
  annotateBlanksInStem,
  replaceBlankMarkers,
  extractTargetBlankNumber,
  correctLabelsByBlank,
  correctAnswerFromOptions,
} from '@/utils/blanks'
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { questionApi, favoriteApi } from '@/api'
import { setEssayPrefill, essayTopicOf, essayPointsOf, GENERATE_ESSAY_PATH } from '@/utils/essayPrefill'
import type { Question } from '@/types'
import ReportDialog from '@/components/ReportDialog.vue'
import QuestionNoteEditor from '@/components/QuestionNoteEditor.vue'
import QuestionComments from '@/components/QuestionComments.vue'
import AIExplain from '@/components/AIExplain.vue'
import AIGradeCase from '@/components/AIGradeCase.vue'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const question = ref<Question | null>(null)
const answers = ref<Record<number, string>>({})
const currentBlank = ref(0)
const showAnswer = ref(false)
const isFavorited = ref(false)
const reportDialogRef = ref<InstanceType<typeof ReportDialog> | null>(null)

onMounted(async () => {
  const id = Number(route.params.id)
  const res: any = await questionApi.getById(id)
  question.value = res.data
  currentBlank.value = 0
  answers.value = {}
  showAnswer.value = false
  checkFavorite()
})

const hasOptions = computed(() => (question.value?.options?.length || 0) > 0)

/** 论文题（question_type=4）：参考答案里存的就是写作要点，可直接喂给范文生成器 */
const isEssay = computed(() => question.value?.question_type === 4)

function generateEssayFromQuestion() {
  if (!question.value) return
  // 要点很长，塞 URL query 会超长，走 sessionStorage 交接（见 utils/essayPrefill）
  setEssayPrefill({ topic: essayTopicOf(question.value), points: essayPointsOf(question.value) })
  router.push(GENERATE_ESSAY_PATH)
}

const blankGroups = computed(() => groupOptionsByBlank(question.value?.options || []))
const totalBlanks = computed(() => blankGroups.value.length || (hasOptions.value ? 1 : 0))
const currentOptions = computed(() => blankGroups.value[currentBlank.value] || [])

const targetBlankNumber = computed(() => {
  if (!question.value) return null
  return extractTargetBlankNumber(question.value.content || question.value.title || '')
})

/** 当前这组选项对应的空号（1-based），给用户看「A/B/C/D 属于第几空」 */
const activeBlankNo = computed(() => targetBlankNumber.value || (currentBlank.value + 1))

const renderedStem = computed(() => {
  if (!question.value) return ''
  const raw = question.value.content || question.value.title || ''
  const marked = annotateBlanksInStem(raw, currentBlank.value, totalBlanks.value || 1, answers.value)
  return replaceBlankMarkers(renderMd(marked))
})

const correctLabels = computed(() => correctLabelsByBlank(question.value?.options || []))
const correctAnswer = computed(
  () => correctLabels.value.join('') || correctAnswerFromOptions(question.value?.options || []),
)

const canShowAnswer = computed(() => {
  if (!hasOptions.value) return true
  if (totalBlanks.value <= 1) return !!answers.value[0] || !!targetBlankNumber.value
  return blankGroups.value.every((_, i) => answers.value[i])
})

function isCurrentBlankCorrect(label: string) {
  return correctLabels.value[currentBlank.value] === label
}

async function checkFavorite() {
  if (!question.value) return
  try {
    const res: any = await favoriteApi.check({ target_type: 1, target_id: question.value.id })
    isFavorited.value = res.data.exists
  } catch {}
}

async function toggleFavorite() {
  if (!question.value) return
  if (!userStore.isLoggedIn) {
    router.push('/login')
    return
  }
  if (isFavorited.value) {
    await favoriteApi.remove(1, question.value.id)
  } else {
    await favoriteApi.add({ target_type: 1, target_id: question.value.id })
  }
  isFavorited.value = !isFavorited.value
}

function selectOption(label: string) {
  if (showAnswer.value) return
  answers.value = { ...answers.value, [currentBlank.value]: label }
  if (totalBlanks.value > 1 && currentBlank.value < totalBlanks.value - 1) {
    const nextUnanswered = blankGroups.value.findIndex((_, i) => i > currentBlank.value && !answers.value[i])
    if (nextUnanswered >= 0) currentBlank.value = nextUnanswered
    else if (!answers.value[currentBlank.value + 1]) currentBlank.value++
  }
}

function typeLabel(t: number) {
  return { 1: '单选', 2: '多选', 3: '案例分析', 4: '论文' }[t] || '未知'
}
</script>

<template>
  <div v-if="question" class="mx-auto max-w-3xl px-3 sm:px-0">
    <div class="rounded-lg bg-white p-4 shadow transition-colors dark:bg-gray-800 sm:p-6">
      <div class="mb-4 flex flex-wrap items-center gap-2">
        <el-tag>{{ typeLabel(question.question_type) }}</el-tag>
        <span class="text-gray-400">{{ question.year }}年</span>
        <span v-if="question.knowledge_point" class="text-sm text-gray-400">{{ question.knowledge_point }}</span>
        <el-tag v-if="totalBlanks > 1" type="warning" effect="plain" size="small" round>
          多空题 · 第 {{ currentBlank + 1 }}/{{ totalBlanks }} 空
        </el-tag>
        <div class="ml-auto flex flex-wrap gap-2">
          <el-button :type="isFavorited ? 'warning' : 'info'" size="small" @click="toggleFavorite">
            <el-icon><Star /></el-icon> {{ isFavorited ? '已收藏' : '收藏' }}
          </el-button>
          <el-button type="danger" plain size="small" @click="reportDialogRef?.open()">
            <el-icon><Warning /></el-icon> 报告问题
          </el-button>
        </div>
      </div>

      <div class="md-content mb-4 leading-relaxed text-gray-700 dark:text-gray-300" v-html="renderedStem"></div>

      <div
        v-if="totalBlanks > 1"
        class="mb-4 flex items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-700 dark:border-blue-800/60 dark:bg-blue-950/40 dark:text-blue-300"
      >
        <span class="inline-flex h-6 min-w-6 items-center justify-center rounded-full bg-blue-600 px-2 text-xs text-white">{{ activeBlankNo }}</span>
        <span>当前选项对应：第 {{ currentBlank + 1 }} 空 / 共 {{ totalBlanks }} 空</span>
      </div>

      <template v-if="hasOptions">
        <div v-if="totalBlanks > 1" class="mb-3 flex flex-wrap gap-2">
          <el-button
            v-for="(_, i) in blankGroups"
            :key="i"
            :type="currentBlank === i ? 'primary' : (answers[i] ? 'success' : 'info')"
            size="small"
            @click="currentBlank = i"
          >
            第{{ i + 1 }}空 {{ answers[i] ? `(${answers[i]})` : '' }}
          </el-button>
        </div>

        <div class="mb-6 space-y-3">
<div
            v-for="opt in currentOptions"
            :key="opt.id"
            class="cursor-pointer rounded-lg border border-gray-200 p-3 text-gray-800 transition dark:border-gray-600 dark:text-gray-200"
            :class="{
              'border-blue-500 bg-blue-50 dark:bg-blue-900/30': !showAnswer && answers[currentBlank] === opt.label,
              'border-green-500 bg-green-50 dark:bg-green-900/30': showAnswer && isCurrentBlankCorrect(opt.label),
              'border-red-500 bg-red-50 dark:bg-red-900/30': showAnswer && answers[currentBlank] === opt.label && !isCurrentBlankCorrect(opt.label),
            }"
            @click="selectOption(opt.label)"
          >
            <span class="mr-2 font-medium">{{ opt.label }}.</span>
            <span v-html="renderMdInline(opt.content)"></span>
          </div>
        </div>

        <div v-if="totalBlanks > 1" class="mb-4 flex flex-wrap items-center gap-2">
          <el-button :disabled="currentBlank === 0" @click="currentBlank--">上一空</el-button>
          <el-button v-if="currentBlank < totalBlanks - 1" @click="currentBlank++">下一空</el-button>
          <span class="text-xs text-gray-400">当前第 {{ currentBlank + 1 }}/{{ totalBlanks }} 空</span>
        </div>
      </template>

      <el-button v-if="!showAnswer" type="primary" @click="showAnswer = true">
        {{ hasOptions ? '查看答案' : '查看参考答案' }}
      </el-button>

      <div v-if="showAnswer" class="mt-6 rounded-lg bg-gray-50 p-4 transition-colors dark:bg-gray-700/50">
        <div v-if="hasOptions" class="mb-3 font-medium text-green-600 dark:text-green-400">
          正确答案：{{ correctAnswer }}
          <span v-if="totalBlanks > 1" class="ml-2 text-xs font-normal text-gray-500">
            （按空位：{{ correctLabels.map((lab, i) => `第${i + 1}空=${lab}`).join('，') }}）
          </span>
        </div>

        <div v-if="question.explanation" class="mb-3">
          <div class="mb-1 font-medium text-gray-700 dark:text-gray-300">答案：</div>
          <div class="rounded border border-gray-200 bg-white p-3 leading-relaxed text-gray-600 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-400">
            <div class="md-content" v-html="renderMd(question.explanation)"></div>
          </div>
        </div>

        <div v-if="question.analysis">
          <div class="mb-1 font-medium text-gray-700 dark:text-gray-300">解析：</div>
          <div class="rounded border border-gray-200 bg-white p-3 leading-relaxed text-gray-600 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-400">
            <div class="md-content" v-html="renderMd(question.analysis)"></div>
          </div>
        </div>

        <div v-if="!question.explanation && !question.analysis" class="text-gray-400">暂无解析</div>

        <AIExplain :question="question" :answer="correctAnswer" class="mt-4" />
        <AIGradeCase v-if="!hasOptions" :question="question" class="mt-4" />
        <el-button v-if="isEssay" class="mt-4" type="primary" plain @click="generateEssayFromQuestion">
          用本题要点生成范文
        </el-button>
      </div>

      <QuestionNoteEditor :question-id="question.id" />
      <QuestionComments :question-id="question.id" />
    </div>

    <div class="mt-4">
      <el-button @click="$router.back()">返回</el-button>
    </div>

    <ReportDialog v-if="question" ref="reportDialogRef" :question-id="question.id" />
  </div>
</template>
