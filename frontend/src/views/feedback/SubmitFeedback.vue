<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { feedbackApi, questionApi } from '@/api'
import { guardGuest } from '@/utils/guestGate'
import { sanitizeNoteHtml, stripHtml } from '@/utils/markdown'
import RichTextEditor from '@/components/RichTextEditor.vue'
import type { Question } from '@/types'

const router = useRouter()
const route = useRoute()

const TYPE_OPTIONS = [
  { value: 1, label: '功能建议' },
  { value: 2, label: '问题反馈（Bug）' },
  { value: 3, label: '内容纠错' },
  { value: 4, label: '其他' },
]

const QUESTION_TYPE_LABELS: Record<number, string> = { 1: '单选', 2: '多选', 3: '案例', 4: '论文' }

const form = ref({
  feedback_type: 1,
  question_id: 0,
  content: '',
  contact: '',
})
const submitting = ref(false)

// 内容纠错可关联题目：粘贴题目网址或编号 → 确认（后端拉取校验）→ 显示题目信息
const isCorrection = computed(() => form.value.feedback_type === 3)
const verifiedQuestion = ref<Question | null>(null)
const verifying = ref(false)
const questionInput = ref('')

// 用户可能在浏览器地址栏直接复制整条网址（.../questions/123），
// 也可能只填编号 —— 两种都收，网址里自动提取编号。
// 下面的正则按 `/questions/<数字>` 取段，故老的 hash 形式 /#/questions/123 也照样能解析。
function parseQuestionId(input: string): number | null {
  const s = input.trim()
  if (!s) return null
  if (/^\d+$/.test(s)) return Number(s)
  const m = s.match(/(?:^|\/)questions\/(\d+)/)
  return m ? Number(m[1]) : null
}

// 校验编号对应题目是否存在；成功返回 true 并缓存题目信息
async function verifyQuestion(): Promise<boolean> {
  const id = parseQuestionId(questionInput.value)
  if (!id) {
    ElMessage.warning('请粘贴题目网址或填写题目编号')
    return false
  }
  form.value.question_id = id
  verifying.value = true
  try {
    const res: any = await questionApi.getById(id)
    verifiedQuestion.value = res.data
    return true
  } catch {
    verifiedQuestion.value = null
    ElMessage.error('未找到该题目，请检查网址或编号是否正确')
    return false
  } finally {
    verifying.value = false
  }
}

function clearQuestion() {
  verifiedQuestion.value = null
  form.value.question_id = 0
  questionInput.value = ''
}

function goMyFeedbacks() {
  if (!guardGuest('我的反馈')) return
  router.push('/my/feedbacks')
}

async function handleSubmit() {
  if (!guardGuest('提交反馈')) return
  // 富文本内容带 <p>/<img> 等标签，按「剥离标签后的可见文字」计字数
  if (stripHtml(form.value.content).length < 5) {
    ElMessage.warning('请填写至少 5 个字的反馈内容')
    return
  }
  // 前端 DOMPurify 兜底：剥危险协议 + 限制 <img src> 白名单。
  // 后端 FeedbackService.Submit 还会再走一次 sanitizeNoteHTML（regex）做保底。
  const content = sanitizeNoteHtml(form.value.content)
  if (!content || stripHtml(content).length < 5) {
    ElMessage.warning('反馈内容被清洗后不完整，请调整后再提交')
    return
  }
  // 填了编号但没确认过，提交前先校验，避免反馈指向不存在的题目
  if (isCorrection.value && form.value.question_id > 0 && !verifiedQuestion.value) {
    const ok = await verifyQuestion()
    if (!ok) return
  }
  submitting.value = true
  try {
    const payload: any = {
      feedback_type: form.value.feedback_type,
      content,
      contact: form.value.contact.trim() || undefined,
    }
    if (form.value.question_id > 0) payload.question_id = form.value.question_id
    await feedbackApi.submit(payload)
    ElMessage.success('感谢你的反馈，处理进度可在「我的反馈」查看')
    form.value.content = ''
    form.value.contact = ''
    form.value.feedback_type = 1
    form.value.question_id = 0
    verifiedQuestion.value = null
    questionInput.value = ''
  } finally {
    submitting.value = false
  }
}

// 支持从题目页/做题页带 ?question_id=xxx 跳转过来，自动关联
onMounted(() => {
  const qid = Number(route.query.question_id)
  if (qid > 0) {
    form.value.feedback_type = 3
    questionInput.value = String(qid)
    verifyQuestion()
  }
})
</script>

<template>
  <div class="mx-auto max-w-2xl">
    <el-card>
      <template #header>
        <div class="flex items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <el-icon><ChatLineSquare /></el-icon>
            <span class="font-semibold">意见反馈</span>
          </div>
          <el-button text type="primary" @click="goMyFeedbacks"
            >我的反馈</el-button
          >
        </div>
      </template>

      <p class="mb-4 text-sm text-slate-500 dark:text-slate-400">
        对软考星图有任何建议、问题或吐槽，都欢迎告诉我们。你的反馈是产品变得更好的动力。
      </p>

      <el-form label-position="top">
        <el-form-item label="反馈类型">
          <el-radio-group v-model="form.feedback_type">
            <el-radio-button
              v-for="opt in TYPE_OPTIONS"
              :key="opt.value"
              :value="opt.value"
            >{{ opt.label }}</el-radio-button>
          </el-radio-group>
        </el-form-item>

        <el-form-item v-if="isCorrection" label="关联题目（选填，便于我们直接定位是哪一题）">
          <div v-if="verifiedQuestion" class="flex w-full flex-wrap items-center gap-2 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-gray-700 dark:bg-gray-800/50">
            <el-tag size="small">{{ QUESTION_TYPE_LABELS[verifiedQuestion.question_type] || '其他' }}</el-tag>
            <el-tag v-if="verifiedQuestion.year" size="small" type="info">{{ verifiedQuestion.year }}年</el-tag>
            <span class="text-sm font-medium text-blue-600">#{{ verifiedQuestion.id }}</span>
            <span class="text-sm text-gray-600 dark:text-gray-400">{{ verifiedQuestion.title || '（无标题）' }}</span>
            <el-button size="small" text type="danger" class="ml-auto" @click="clearQuestion">清除</el-button>
          </div>
          <div v-else class="w-full">
            <div class="flex flex-wrap items-center gap-2">
              <el-input
                v-model="questionInput"
                clearable
                placeholder="粘贴题目网址，或直接填编号，如 123"
                style="width: 320px"
                @keyup.enter="verifyQuestion"
              />
              <el-button :loading="verifying" @click="verifyQuestion">确认题目</el-button>
            </div>
            <p class="mt-1.5 text-xs text-gray-400">
              在题目页浏览器地址栏复制网址粘贴到这里即可，系统自动提取编号，
              例如 <code>https://…/questions/123</code>
            </p>
          </div>
        </el-form-item>

        <el-form-item label="反馈内容">
          <div class="w-full">
            <RichTextEditor
              v-model="form.content"
              :placeholder="isCorrection ? '请指出题目具体哪里有问题，方便我们修正……' : '请详细描述你的建议或遇到的问题……'"
            />
            <p class="mt-1.5 text-xs text-gray-400">支持直接粘贴截图，单张不超过 5MB</p>
          </div>
        </el-form-item>

        <el-form-item label="联系方式（选填）">
          <el-input
            v-model="form.contact"
            maxlength="100"
            placeholder="邮箱 / 微信 / QQ，方便我们回复你（选填）"
          />
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            :loading="submitting"
            @click="handleSubmit"
          >提交反馈</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>