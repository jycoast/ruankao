<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { ElMessage } from 'element-plus'

const route = useRoute()
const userStore = useUserStore()
const statusMsg = ref('正在完成登录，请稍候...')

const ERROR_MESSAGES: Record<string, string> = {
  state_mismatch: '登录状态验证失败，请重试',
  missing_code: '授权码缺失，请重试',
  token_exchange_failed: '获取访问令牌失败，请重试',
  userinfo_failed: '获取用户信息失败，请重试',
  account_inactive: '您的 Linux.do 账号尚未激活',
  account_silenced: '您的 Linux.do 账号已被禁言，无法登录',
  db_error: '账号处理失败，请联系管理员',
  access_denied: '您取消了授权',
}

onMounted(async () => {
  const token = route.query.token as string
  const oauthError = route.query.oauth_error as string

  if (oauthError) {
    const msg = ERROR_MESSAGES[oauthError] || '登录失败，请重试'
    statusMsg.value = msg
    ElMessage.error(msg)
    setTimeout(() => userStore.logout(), 1500)
    return
  }

  if (!token) {
    statusMsg.value = '登录失败：未收到令牌'
    ElMessage.error('登录失败，请重试')
    setTimeout(() => userStore.logout(), 1500)
    return
  }

  await userStore.loginWithToken(token)
  ElMessage.success('Linux.do 登录成功')
})
</script>

<template>
  <div class="min-h-screen flex flex-col items-center justify-center bg-gradient-to-br from-blue-500 to-purple-600">
    <div class="bg-white rounded-xl shadow-lg p-10 flex flex-col items-center gap-4">
      <div class="w-12 h-12 border-4 border-blue-500 border-t-transparent rounded-full animate-spin" />
      <p class="text-slate-600 text-sm">{{ statusMsg }}</p>
    </div>
  </div>
</template>
