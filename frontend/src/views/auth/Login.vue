<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useUserStore } from '@/stores/user'
import { passwordResetApi } from '@/api'
import AuthLayout from '@/components/AuthLayout.vue'

const userStore = useUserStore()
const route = useRoute()
const router = useRouter()
const loginForm = ref({ username: '', password: '' })
// 登录请求进行中：后端响应慢时按钮转圈 + 禁用，防止用户以为卡死或重复点击
const loading = ref(false)

const linuxdoLoginUrl = (import.meta.env.VITE_API_BASE_URL || '/api') + '/auth/oauth/linuxdo'

const oauthErrorMap: Record<string, string> = {
  state_mismatch: 'OAuth 状态验证失败，请重试',
  token_exchange_failed: 'LinuxDO 令牌获取失败，请检查应用回调地址配置',
  userinfo_failed: '获取 LinuxDO 用户信息失败',
  account_inactive: 'LinuxDO 账号未激活',
  account_silenced: 'LinuxDO 账号已被禁言',
  db_error: '数据库错误，请稍后重试',
  missing_code: '授权码缺失',
}

onMounted(() => {
  const err = route.query.oauth_error as string
  if (err) {
    ElMessage.error(oauthErrorMap[err] || `OAuth 错误：${err}`)
  }
})

async function handleLogin() {
  loading.value = true
  try {
    await userStore.login(loginForm.value.username, loginForm.value.password)
  } finally {
    // 成功会跳转走；失败由全局拦截器提示，这里只负责兜底关掉 loading
    loading.value = false
  }
}

const guestLoading = ref(false)
async function handleGuest() {
  guestLoading.value = true
  try {
    await userStore.guestLogin() // 成功后由 store 跳转/路由守卫改道
  } catch {
    /* 拦截器已提示 */
  } finally {
    guestLoading.value = false
  }
}

// ---- 找回密码（管理员人工审核，站点没有 SMTP）----

const resetVisible = ref(false)
const resetting = ref(false)
const resetForm = ref({ account: '', contact: '', reason: '' })

async function submitReset() {
  if (!resetForm.value.account.trim()) {
    ElMessage.warning('请填写用户名或邮箱')
    return
  }
  if (!resetForm.value.contact.trim()) {
    ElMessage.warning('请填写联系方式，否则管理员无法把新密码告知你')
    return
  }
  resetting.value = true
  try {
    await passwordResetApi.submit({
      account: resetForm.value.account.trim(),
      contact: resetForm.value.contact.trim(),
      reason: resetForm.value.reason.trim(),
    })
    // 后端对不存在的账号也返回成功，这里的提示不能透露账号是否存在
    ElMessage.success('申请已提交，管理员核实后会通过你留的联系方式告知新密码')
    resetVisible.value = false
    resetForm.value = { account: '', contact: '', reason: '' }
  } catch {
    // 拦截器已提示
  } finally {
    resetting.value = false
  }
}
</script>

<template>
  <AuthLayout>
    <div class="auth-card rounded-2xl border border-slate-200 bg-white p-8 dark:border-slate-800 dark:bg-slate-900">
      <h1 class="text-lg font-semibold text-slate-950 dark:text-white">欢迎回来</h1>
      <el-form :model="loginForm" @submit.prevent="handleLogin" label-position="top" class="mt-5 space-y-3">
        <el-form-item label="用户名">
          <el-input
            v-model="loginForm.username"
            placeholder="用户名"
            prefix-icon="User"
            size="large"
            autocomplete="username"
          />
        </el-form-item>
        <el-form-item label="密码">
          <el-input
            v-model="loginForm.password"
            type="password"
            placeholder="密码"
            prefix-icon="Lock"
            show-password
            size="large"
            autocomplete="current-password"
          />
        </el-form-item>
        <el-button native-type="submit" class="auth-submit w-full" :loading="loading" :disabled="loading">
          登录
        </el-button>
        <div class="flex items-center justify-between pt-1 text-sm">
          <el-link type="primary" :underline="false" @click="router.push('/register')">还没有账号？去注册</el-link>
          <el-link type="info" :underline="false" @click="resetVisible = true">忘记密码？</el-link>
        </div>
      </el-form>

      <div class="relative my-5 flex items-center justify-center">
        <div class="w-full border-t border-slate-200 dark:border-slate-700" />
        <span class="absolute bg-white px-3 text-xs text-slate-500 dark:bg-slate-900 dark:text-slate-400">或</span>
      </div>

      <a
        :href="linuxdoLoginUrl"
        class="flex w-full items-center justify-center gap-2 rounded-lg border border-slate-200 bg-white py-3 text-sm font-medium text-slate-700 shadow-sm transition hover:-translate-y-0.5 hover:border-blue-300 hover:bg-blue-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
      >
        <img src="/linuxdo.svg" alt="Linux.do" width="18" height="18" />
        使用 Linux.do 账号登录
      </a>

      <button
        class="mt-3 flex w-full items-center justify-center gap-2 rounded-lg border border-slate-200 bg-white py-2.5 text-sm text-slate-400 transition hover:border-slate-300 hover:text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-500"
        :disabled="guestLoading"
        @click="handleGuest"
      >
        游客模式试用
      </button>
    </div>
  </AuthLayout>

  <el-dialog v-model="resetVisible" title="找回密码" width="420px" class="max-sm:!w-[calc(100vw-24px)]">
    <el-alert
      type="info"
      :closable="false"
      class="mb-4"
      title="站点未接入邮件服务，重置需管理员人工核实。请留下能联系到你的方式，核实后新密码会通过该方式告知，登录后请立即修改。"
    />
    <el-form label-position="top">
      <el-form-item label="用户名或邮箱">
        <el-input v-model="resetForm.account" placeholder="注册时使用的用户名或邮箱" />
      </el-form-item>
      <el-form-item label="联系方式">
        <el-input v-model="resetForm.contact" placeholder="QQ / 微信 / 其他邮箱，管理员据此联系你" />
      </el-form-item>
      <el-form-item label="补充说明（选填）">
        <el-input v-model="resetForm.reason" type="textarea" :rows="3" placeholder="便于核实身份的信息，如注册时间、常用设备等" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="resetVisible = false">取消</el-button>
      <el-button type="primary" :loading="resetting" @click="submitReset">提交申请</el-button>
    </template>
  </el-dialog>
</template>
