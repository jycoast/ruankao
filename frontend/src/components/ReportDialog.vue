<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { reportApi } from '@/api'

const props = defineProps<{ questionId: number }>()

const visible = ref(false)
const submitting = ref(false)
const form = ref({ report_type: 1, description: '' })

const REPORT_TYPES = [
  { value: 1, label: '题目表述有误' },
  { value: 2, label: '答案有误' },
  { value: 3, label: '解析/分析有误' },
  { value: 4, label: '选项有误' },
  { value: 5, label: '其他问题' },
]

function open() {
  form.value = { report_type: 1, description: '' }
  visible.value = true
}

async function submit() {
  if (!form.value.description.trim() || form.value.description.trim().length < 5) {
    ElMessage.warning('请描述问题（至少5个字）')
    return
  }
  submitting.value = true
  try {
    await reportApi.submit(props.questionId, {
      report_type: form.value.report_type,
      description: form.value.description.trim(),
    })
    ElMessage.success('感谢反馈，我们会尽快处理')
    visible.value = false
  } finally {
    submitting.value = false
  }
}

defineExpose({ open })
</script>

<template>
  <el-dialog v-model="visible" title="报告题目问题" width="480px" :close-on-click-modal="false">
    <el-form label-width="90px">
      <el-form-item label="问题类型">
        <el-select v-model="form.report_type" style="width: 100%">
          <el-option
            v-for="t in REPORT_TYPES"
            :key="t.value"
            :label="t.label"
            :value="t.value"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="问题描述">
        <el-input
          v-model="form.description"
          type="textarea"
          :rows="4"
          placeholder="请详细描述题目存在的问题，例如：答案应为B而非C，因为……"
          maxlength="500"
          show-word-limit
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submit">提交报告</el-button>
    </template>
  </el-dialog>
</template>
