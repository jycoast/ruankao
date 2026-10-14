<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { essaySubmissionApi } from '@/api'

const router = useRouter()
const submitting = ref(false)

const form = ref({
  title: '',
  content: '',
  category: '',
  year: new Date().getFullYear(),
  score: 0,
})

const rules = {
  title: [{ required: true, message: '请输入论文标题', trigger: 'blur' }],
  content: [
    { required: true, message: '请输入论文正文', trigger: 'blur' },
    { min: 50, message: '正文不得少于50字', trigger: 'blur' },
  ],
}

const formRef = ref()

async function handleSubmit() {
  await formRef.value?.validate()
  submitting.value = true
  try {
    await essaySubmissionApi.submit(form.value)
    ElMessage.success('投稿成功，等待管理员审核')
    router.push('/essays/my-submissions')
  } catch {
    // error handled by interceptor
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="max-w-3xl mx-auto">
    <div class="flex items-center gap-3 mb-6">
      <el-button :icon="'ArrowLeft'" @click="router.back()" text>返回</el-button>
      <h2 class="text-xl font-bold text-gray-900 dark:text-gray-100">提交论文</h2>
    </div>

    <el-card>
      <el-alert
        type="info"
        :closable="false"
        class="mb-6"
        description="投稿经管理员审核通过后将发布到论文范文库，供其他用户参考学习。"
      />

      <el-form ref="formRef" :model="form" :rules="rules" label-width="90px">
        <el-form-item label="论文标题" prop="title">
          <el-input v-model="form.title" placeholder="请输入论文标题" maxlength="200" show-word-limit />
        </el-form-item>
        <el-form-item label="分类">
          <el-input v-model="form.category" placeholder="如：数据流图、软件架构等（可选）" />
        </el-form-item>
        <el-form-item label="考试年份">
          <el-input-number v-model="form.year" :min="2000" :max="2099" />
        </el-form-item>
        <el-form-item label="论文得分">
          <el-input-number v-model="form.score" :min="0" :max="75" :step="0.5" />
          <span class="ml-2 text-gray-400 text-sm">分（可选）</span>
        </el-form-item>
        <el-form-item label="论文正文" prop="content">
          <el-input
            v-model="form.content"
            type="textarea"
            :rows="20"
            placeholder="请粘贴或输入论文全文，至少50字"
            show-word-limit
            maxlength="20000"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="submitting" @click="handleSubmit">提交投稿</el-button>
          <el-button @click="router.back()">取消</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>
