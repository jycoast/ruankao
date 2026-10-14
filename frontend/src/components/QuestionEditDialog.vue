<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { questionApi } from '@/api'
import { renderMd, renderMdInline } from '@/utils/markdown'
import { usePasteUpload } from '@/composables/usePasteUpload'
import type { Question } from '@/types'
import { ElMessage } from 'element-plus'

// 题目编辑弹窗：题库管理「编辑/新增」与题目反馈「直接改题」共用。
// 传 question=null 为新增；传具体题目为编辑（只改标题/题干/选项等，不换科目）。
const props = defineProps<{
  modelValue: boolean
  question: Question | null
  subjectId: number
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void
  (e: 'saved'): void
}>()

const TYPE_LABELS: Record<number, string> = { 1: '单选', 2: '多选', 3: '案例', 4: '论文' }

const saving = ref(false)

function emptyForm() {
  return {
    title: '', content: '', question_type: 1, difficulty: 1, year: new Date().getFullYear(),
    explanation: '', analysis: '', score: 0,
    options: [
      { label: 'A', content: '', is_correct: 0 },
      { label: 'B', content: '', is_correct: 0 },
      { label: 'C', content: '', is_correct: 0 },
      { label: 'D', content: '', is_correct: 0 },
    ],
  }
}
const form = ref(emptyForm())

// 题干/解析/分析都支持粘贴图片，统一走 markdown 图片语法。
// 全屏编辑组件内部已统一处理粘贴上传，此处仅保留 uploading 用于表单 loading 提示。
const { uploading } = usePasteUpload()

// 每次打开时按传入的题目重新初始化表单；关闭后再次打开同一题也能拿到干净状态。
watch(
  () => [props.modelValue, props.question] as const,
  ([visible, q]) => {
    if (!visible) return
    if (q) {
      form.value = {
        title: q.title, content: q.content, question_type: q.question_type,
        difficulty: q.difficulty, year: q.year, explanation: q.explanation || '',
        analysis: q.analysis || '', score: q.score,
        options: (q.options || []).map((o) => ({ label: o.label, content: o.content, is_correct: o.is_correct })),
      }
    } else {
      form.value = emptyForm()
    }
  },
  { immediate: true },
)

const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit('update:modelValue', v),
})

const hasOptions = computed(() => [1, 2].includes(form.value.question_type))

// 编辑时保留题目原科目（报告页跨科目改题不能把题挪到当前科目）；新增才用传入科目。
const effectiveSubjectId = computed(() => props.question?.subject_id || props.subjectId)

function addOption() {
  const label = String.fromCharCode(65 + form.value.options.length)
  form.value.options.push({ label, content: '', is_correct: 0 })
}
function removeOption(i: number) {
  form.value.options.splice(i, 1)
  // 重排 label
  form.value.options.forEach((o, idx) => (o.label = String.fromCharCode(65 + idx)))
}

async function save() {
  saving.value = true
  try {
    const payload = { ...form.value, subject_id: effectiveSubjectId.value }
    if (props.question?.id) {
      await questionApi.update(props.question.id, payload)
    } else {
      await questionApi.create(payload)
    }
    ElMessage.success('保存成功')
    visible.value = false
    emit('saved')
  } finally {
    saving.value = false
  }
}

// 预览用正确答案
const previewAnswer = computed(() =>
  form.value.options.filter((o) => o.is_correct === 1).map((o) => o.label).join(''),
)
</script>

<template>
  <el-dialog v-model="visible" :title="question ? '编辑题目' : '新增题目'" width="960px" top="5vh">
    <div class="grid gap-5 md:grid-cols-2">
      <!-- 编辑区 -->
      <el-form v-loading="uploading" element-loading-text="图片上传中…" :model="form" label-width="72px" label-position="top">
        <div class="grid grid-cols-2 gap-3">
          <el-form-item label="题型">
            <el-select v-model="form.question_type" class="w-full">
              <el-option label="单选" :value="1" />
              <el-option label="多选" :value="2" />
              <el-option label="案例" :value="3" />
              <el-option label="论文" :value="4" />
            </el-select>
          </el-form-item>
          <el-form-item label="年份">
            <el-input-number v-model="form.year" :min="2000" :max="2030" class="w-full" />
          </el-form-item>
        </div>
        <el-form-item label="标题">
          <FullScreenEditor v-model="form.title" label="标题" :rows="2" :preview="false" :toolbar="false" />
        </el-form-item>
        <el-form-item label="内容（题干，支持 Markdown，可直接粘贴图片）">
          <FullScreenEditor v-model="form.content" label="题干" :rows="4" :preview="true" placeholder="输入题干内容，支持 Markdown；可直接粘贴图片自动上传" />
        </el-form-item>
        <el-form-item v-if="hasOptions" label="选项">
          <div class="w-full space-y-2">
            <div v-for="(opt, i) in form.options" :key="i" class="flex items-center gap-2">
              <span class="w-5 shrink-0 text-gray-500">{{ opt.label }}.</span>
              <el-input v-model="opt.content" class="flex-1" placeholder="选项内容" />
              <el-checkbox v-model="opt.is_correct" :true-value="1" :false-value="0">正确</el-checkbox>
              <el-button size="small" text type="danger" :disabled="form.options.length <= 2" @click="removeOption(i)">删</el-button>
            </div>
            <el-button size="small" @click="addOption">+ 添加选项</el-button>
          </div>
        </el-form-item>
        <el-form-item label="解析">
          <FullScreenEditor v-model="form.explanation" label="解析" :rows="3" :preview="true" placeholder="输入解析，支持 Markdown；可直接粘贴图片自动上传" />
        </el-form-item>
        <el-form-item v-if="!hasOptions" label="参考答案 / 分析">
          <FullScreenEditor v-model="form.analysis" label="参考答案 / 分析" :rows="3" :preview="true" placeholder="输入参考答案 / 分析，支持 Markdown；可直接粘贴图片自动上传" />
        </el-form-item>
      </el-form>

      <!-- 预览区 -->
      <div class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-gray-700 dark:bg-gray-800/50">
        <div class="mb-3 text-xs font-medium text-gray-400">实时预览</div>
        <div class="mb-2 flex items-center gap-2">
          <el-tag size="small">{{ TYPE_LABELS[form.question_type] }}</el-tag>
          <span v-if="form.year" class="text-xs text-gray-400">{{ form.year }}年</span>
        </div>
        <div v-if="form.title" class="mb-2 font-medium text-gray-900 dark:text-gray-100" v-html="renderMdInline(form.title)" />
        <div v-if="form.content" class="md-content mb-4 leading-relaxed text-gray-700 dark:text-gray-300" v-html="renderMd(form.content)" />

        <div v-if="hasOptions" class="space-y-2">
          <div
            v-for="opt in form.options"
            :key="opt.label"
            class="rounded-lg border p-2.5 text-sm"
            :class="opt.is_correct === 1
              ? 'border-green-500 bg-green-50 dark:bg-green-900/30'
              : 'border-gray-200 dark:border-gray-600'"
          >
            <span class="font-medium mr-1.5">{{ opt.label }}.</span>
            <span v-html="renderMdInline(opt.content)" />
          </div>
          <div v-if="previewAnswer" class="pt-1 text-sm font-medium text-green-600 dark:text-green-400">正确答案：{{ previewAnswer }}</div>
        </div>

        <div v-if="form.explanation" class="mt-4 rounded bg-white p-3 text-sm text-gray-600 dark:bg-gray-800 dark:text-gray-300">
          <div class="mb-1 font-medium">解析</div>
          <div class="md-content" v-html="renderMd(form.explanation)" />
        </div>
        <div v-if="!hasOptions && form.analysis" class="mt-4 rounded bg-white p-3 text-sm text-gray-600 dark:bg-gray-800 dark:text-gray-300">
          <div class="mb-1 font-medium">参考答案 / 分析</div>
          <div class="md-content" v-html="renderMd(form.analysis)" />
        </div>
      </div>
    </div>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
    </template>
  </el-dialog>
</template>
