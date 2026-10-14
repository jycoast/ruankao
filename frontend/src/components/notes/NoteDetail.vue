<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { questionNoteApi, favoriteApi } from '@/api'
import { useUserStore } from '@/stores/user'
import { renderMd, renderMdInline, sanitizeNoteHtml } from '@/utils/markdown'
import {
  groupOptionsByBlank,
  correctLabelsByBlank,
  correctAnswerFromOptions,
} from '@/utils/blanks'
import { setEssayPrefill, essayTopicOf, essayPointsOf, GENERATE_ESSAY_PATH } from '@/utils/essayPrefill'
import type { NoteWithQuestion, Question } from '@/types'
import QuestionNoteEditor from '@/components/QuestionNoteEditor.vue'
import QuestionComments from '@/components/QuestionComments.vue'
import AIExplain from '@/components/AIExplain.vue'
import AIGradeCase from '@/components/AIGradeCase.vue'
import ReportDialog from '@/components/ReportDialog.vue'

const props = defineProps<{ questionId?: number }>()

const router = useRouter()
const userStore = useUserStore()

const note = ref<NoteWithQuestion | null>(null)
const question = ref<Question | null>(null)
const loading = ref(false)
const failed = ref(false)

const isFavorited = ref(false)
const reportDialogRef = ref<InstanceType<typeof ReportDialog> | null>(null)

// 喂给编辑器的正文种子 —— 编辑器「每道题只灌一次」时灌的就是它。
// 两个写入点：① 本页加载这条笔记时（load）；② 保存成功后的 refreshTimestamp。
//
// 注意：改这个 ref 本身**不会**打断正在编辑的编辑器。注入只发生在
// loadNote()，也就是编辑器为某道题首次加载的那一刻（见 Task 1 的 loadedFor）。
const seedContent = ref('')

async function load(id: number) {
  loading.value = true
  failed.value = false
  try {
    const res: any = await questionNoteApi.myNoteDetail(id, { silent: true })
    // 快速连点两条笔记时，先发的请求可能后到，丢弃已不是当前选中项的结果。
    if (props.questionId !== id) return
    note.value = res.data.note
    question.value = res.data.question
    // 仍然过一遍 sanitize：XSS 防护不因为「换成编辑器」而放松。
    // 内容本来就是 handleSave 里 sanitize 过再存的，这里是幂等的第二道。
    seedContent.value = sanitizeNoteHtml(res.data.note?.content || '')
    checkFavorite()
  } catch {
    if (props.questionId !== id) return
    // 请求已用 silent 静音，这里自行落到空态，不弹提示。
    note.value = null
    question.value = null
    seedContent.value = ''
    isFavorited.value = false
    failed.value = true
  } finally {
    if (props.questionId === id) loading.value = false
  }
}

// 保存笔记后刷新「最后更新」，**并且**把 seedContent 同步成本次保存的正文。
//
// 为什么要同步：seedContent 是编辑器下次**重新挂载**时灌进去的内容，同步之后
// 「最后更新」与「下次灌什么」都跟得上这次保存。
//
// 这一步目前是保险带，不是承重结构：编辑器重建只有换题一条来路，而换题会先走
// load()，由它在把 loading.value 置回 false 之前重灌 seedContent（见 load() 的
// 过期响应守卫与 finally）；清除笔记后详情接口回 404，落的是空态分支，编辑器
// 根本不会重建。也就是说，现在的代码里没有哪条路径会拿旧 seedContent 去重建编辑器。
//
// 为什么在这里同步是安全的：本函数只由 @saved 触发，也就是编辑器里的当前
// 内容刚刚成功落库的那一刻；而且改 seedContent 不会打断正在编辑的编辑器
// （注入只在 loadNote() 里发生，而编辑器对同一道题只灌一次）。
async function refreshTimestamp(id: number) {
  try {
    const res: any = await questionNoteApi.myNoteDetail(id, { silent: true })
    if (props.questionId !== id) return
    note.value = res.data.note
    seedContent.value = sanitizeNoteHtml(res.data.note?.content || '')
  } catch {
    // 静默：刷不到就保持原时间戳与旧种子，不值得为此打扰用户
  }
}

watch(
  () => props.questionId,
  (id) => {
    if (id) {
      load(id)
    } else {
      note.value = null
      question.value = null
      failed.value = false
      seedContent.value = ''
      isFavorited.value = false
      // 必须一起复位：load() 的 finally 只在「仍是当前选中项」时才清 loading，
      // 若取消选中时正好有请求在途，它回来时会被判定为过期请求而不复位 ——
      // 不在这里清就会永久停在骨架屏，盖住「从左侧选择一条笔记」空态。
      loading.value = false
    }
  },
  { immediate: true },
)

const blankGroups = computed(() => groupOptionsByBlank(question.value?.options || []))
const correctLabels = computed(() => correctLabelsByBlank(question.value?.options || []))
const correctAnswer = computed(
  () => correctLabels.value.join('') || correctAnswerFromOptions(question.value?.options || []),
)

// 按**组**比对，不能用全局 includes：多空题里每组标号都从 A 开始，
// 全局匹配会让第 2 空的 A 被第 1 空的正确答案点亮。
// （与 QuestionDetail.vue:85-87 的 isCurrentBlankCorrect 同一套判据。）
function isCorrect(groupIndex: number, label: string) {
  return correctLabels.value[groupIndex] === label
}

const hasOptions = computed(() => (question.value?.options?.length || 0) > 0)
const isEssay = computed(() => question.value?.question_type === 4)

// 与 QuestionDetail.vue:89-109 同一套。本页正常路径下用户必然已登录
//（游客取不到笔记，走 404 空态），未登录那支是会话中途过期的兜底。
async function checkFavorite() {
  const id = question.value?.id
  if (!id) return
  try {
    const res: any = await favoriteApi.check({ target_type: 1, target_id: id })
    isFavorited.value = res.data.exists
  } catch {}
}

async function toggleFavorite() {
  const id = question.value?.id
  if (!id) return
  if (!userStore.isLoggedIn) {
    router.push('/login')
    return
  }
  if (isFavorited.value) {
    await favoriteApi.remove(1, id)
  } else {
    await favoriteApi.add({ target_type: 1, target_id: id })
  }
  isFavorited.value = !isFavorited.value
}

/** 论文题（question_type=4）：参考答案里存的就是写作要点，可直接喂给范文生成器 */
function generateEssayFromQuestion() {
  if (!question.value) return
  // 要点很长，塞 URL query 会超长，走 sessionStorage 交接（见 utils/essayPrefill）
  setEssayPrefill({ topic: essayTopicOf(question.value), points: essayPointsOf(question.value) })
  router.push(GENERATE_ESSAY_PATH)
}

function typeLabel(t: number) {
  return { 1: '单选', 2: '多选', 3: '案例分析', 4: '论文' }[t] || '未知'
}
</script>

<template>
  <div v-if="loading" class="space-y-3 p-4">
    <el-skeleton :rows="8" animated />
  </div>

  <el-empty v-else-if="failed" description="笔记不存在或已删除">
    <router-link to="/my/notes">
      <el-button type="primary" plain size="small">返回列表</el-button>
    </router-link>
  </el-empty>

  <el-empty v-else-if="!questionId" description="从左侧选择一条笔记" />

  <div v-else class="p-4 sm:p-6">
    <!-- 笔记：就地编辑，默认展开。正文由本页已取到的 note.content 注入，
         编辑器不再自己去打 GET /questions/:id/note（那条路由挂 RateData）。
         :key 与上面的 v-if="loading" **不可互替**，但两者不等重：
         · 承重的是 v-if="loading" —— 它保证「编辑器挂载时 seedContent 已经对应当前题目」；
         · `:key` 只是冗余的保险带 —— 换题时这个 watch 先同步置 loading=true，
           v-else 分支在新 key 生效之前就已经被销毁，新实例重挂时 loadedFor 本来就是 null。
         只保留 :key 而把编辑器挪出 loading 分支会造成**静默错位**：父组件会先以
         「新 questionId + 旧 seedContent」渲染，新实例灌进上一题的正文并记账，
         此后卡住不再重灌，用户一保存就把上一题的笔记写到了这一题上。 -->
    <SectionTitle>笔记</SectionTitle>

    <QuestionNoteEditor
      :key="questionId"
      :question-id="questionId"
      :start-open="true"
      :initial-content="seedContent"
      @saved="refreshTimestamp(questionId)"
    />

    <div v-if="question" class="mt-8">
      <SectionTitle>题目</SectionTitle>

      <div class="mb-4 flex flex-wrap items-center gap-2">
        <el-tag>{{ typeLabel(question.question_type) }}</el-tag>
        <span v-if="question.year" class="text-sm text-gray-500 dark:text-gray-400">{{ question.year }}年</span>
        <span v-if="question.knowledge_point" class="text-sm text-gray-500 dark:text-gray-400">
          {{ question.knowledge_point }}
        </span>
        <div class="ml-auto flex flex-wrap gap-2">
          <el-button :type="isFavorited ? 'warning' : 'info'" size="small" @click="toggleFavorite">
            <el-icon><Star /></el-icon> {{ isFavorited ? '已收藏' : '收藏' }}
          </el-button>
          <el-button type="danger" plain size="small" @click="reportDialogRef?.open()">
            <el-icon><Warning /></el-icon> 报告问题
          </el-button>
        </div>
      </div>

      <div class="md-content mb-4 leading-relaxed text-gray-700 dark:text-gray-300" v-html="renderMd(question.content || question.title)"></div>

      <!-- 只读选项：直接标出正确项，不需要「查看答案」这一步（这是复习，不是做题） -->
      <div v-if="blankGroups.length" class="mb-4 space-y-3">
        <div v-for="(group, gi) in blankGroups" :key="gi">
          <div v-if="blankGroups.length > 1" class="mb-1 text-xs text-gray-500 dark:text-gray-400">第 {{ gi + 1 }} 空</div>
          <div
            v-for="opt in group"
            :key="opt.id"
            class="mb-2 rounded-lg border p-3 text-gray-800 dark:text-gray-200"
            :class="
              isCorrect(gi, opt.label)
                ? 'border-green-500 bg-green-50 dark:border-green-600 dark:bg-green-900/30'
                : 'border-gray-200 dark:border-gray-600'
            "
          >
            <span class="mr-2 font-medium">{{ opt.label }}.</span>
            <span v-html="renderMdInline(opt.content)"></span>
          </div>
        </div>
      </div>

      <div class="mt-8 rounded-lg bg-gray-50/70 p-4 dark:bg-gray-700/50">
        <SectionTitle>答案与解析</SectionTitle>

        <div v-if="blankGroups.length" class="mb-3 font-medium text-green-600 dark:text-green-400">
          正确答案：{{ correctAnswer }}
        </div>

        <div v-if="question.explanation" class="mb-3">
          <div class="mb-1 font-medium text-gray-700 dark:text-gray-300">答案：</div>
          <div class="md-content rounded border border-gray-200 bg-white p-3 leading-relaxed text-gray-600 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-400" v-html="renderMd(question.explanation)"></div>
        </div>

        <div v-if="question.analysis">
          <div class="mb-1 font-medium text-gray-700 dark:text-gray-300">解析：</div>
          <div class="md-content rounded border border-gray-200 bg-white p-3 leading-relaxed text-gray-600 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-400" v-html="renderMd(question.analysis)"></div>
        </div>

        <div v-if="!question.explanation && !question.analysis && !blankGroups.length" class="text-gray-500 dark:text-gray-400">
          暂无解析
        </div>

        <AIExplain :question="question" :answer="correctAnswer" class="mt-4" />
        <!-- 案例/论文题（无选项）才有案例判分；单选题不渲染 -->
        <AIGradeCase v-if="!hasOptions" :question="question" class="mt-4" />
        <el-button v-if="isEssay" class="mt-4" type="primary" plain @click="generateEssayFromQuestion">
          用本题要点生成范文
        </el-button>
      </div>

      <SectionTitle class="mt-8">讨论</SectionTitle>

      <QuestionComments :question-id="question.id" />
    </div>

    <div class="mt-4 text-xs text-gray-500 dark:text-gray-400">
      最后更新 {{ (note?.updated_at || '').replace('T', ' ').slice(0, 16) }}
    </div>

    <ReportDialog v-if="question" ref="reportDialogRef" :question-id="question.id" />
  </div>
</template>
