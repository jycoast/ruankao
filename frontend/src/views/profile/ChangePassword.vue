<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'
import { userApi } from '@/api'
import { useUserStore } from '@/stores/user'

const router = useRouter()
const userStore = useUserStore()
const hasPassword = computed(() => userStore.hasPassword)

onMounted(() => {
  if (!userStore.user) userStore.fetchProfile()
})
const formRef = ref<FormInstance>()
const loading = ref(false)

const form = reactive({
  old_password: '',
  new_password: '',
  confirm_password: '',
})

const validateConfirm = (_rule: any, value: string, callback: any) => {
  if (value !== form.new_password) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const rules = computed<FormRules>(() => ({
  old_password: hasPassword.value
    ? [{ required: true, message: '请输入原密码', trigger: 'blur' }]
    : [],
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码长度不能少于6位', trigger: 'blur' },
  ],
  confirm_password: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    { validator: validateConfirm, trigger: 'blur' },
  ],
}))

async function handleSubmit() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return
  loading.value = true
  try {
    await userApi.changePassword({ old_password: form.old_password, new_password: form.new_password })
    ElMessage.success(hasPassword.value ? '密码修改成功，请重新登录' : '密码设置成功，请重新登录')
    userStore.logout()
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="max-w-md mx-auto">
    <el-card shadow="never" class="border border-slate-200 dark:border-slate-800">
      <template #header>
        <div class="flex items-center gap-2">
          <el-button text @click="router.back()">
            <el-icon><ArrowLeft /></el-icon>
          </el-button>
          <h2 class="text-lg font-semibold text-slate-900 dark:text-white">{{ hasPassword ? '修改密码' : '设置密码' }}</h2>
        </div>
      </template>

      <el-alert
        v-if="!hasPassword"
        type="info"
        :closable="false"
        show-icon
        class="mb-4"
        title="当前账号通过第三方登录创建，尚未设置密码。设置后即可用用户名密码登录。"
      />

      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" class="space-y-1">
        <el-form-item v-if="hasPassword" label="原密码" prop="old_password">
          <el-input
            v-model="form.old_password"
            type="password"
            placeholder="请输入原密码"
            show-password
          />
        </el-form-item>

        <el-form-item label="新密码" prop="new_password">
          <el-input
            v-model="form.new_password"
            type="password"
            placeholder="至少6位"
            show-password
          />
        </el-form-item>

        <el-form-item label="确认新密码" prop="confirm_password">
          <el-input
            v-model="form.confirm_password"
            type="password"
            placeholder="请再次输入新密码"
            show-password
          />
        </el-form-item>

        <div class="pt-2 flex gap-3">
          <el-button class="flex-1" @click="router.back()">取消</el-button>
          <el-button type="primary" class="flex-1" :loading="loading" @click="handleSubmit">
            {{ hasPassword ? '确认修改' : '确认设置' }}
          </el-button>
        </div>
      </el-form>
    </el-card>
  </div>
</template>
