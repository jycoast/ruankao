<script setup lang="ts">
import AuthLayout from '@/components/AuthLayout.vue'
import { authApi } from '@/api'
import { ref, reactive, onMounted } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { useUserStore } from '@/stores/user'
import { useRouter } from 'vue-router'

const userStore = useUserStore()
const router = useRouter()
const formRef = ref<FormInstance>()
const loading = ref(false)

const form = reactive({
  username: '',
  email: '',
  password: '',
  nickname: '',
  invite_code: '',
  captcha_id: '',
  captcha_code: '',
})

// —— 图形验证码：进页加载，点图刷新，验证码错误时自动换图 ——
const captchaImg = ref('')

async function loadCaptcha() {
  try {
    const res: any = await authApi.getCaptcha()
    const data = res.data
    form.captcha_id = data.captcha_id
    // 后端返回的 image_base64 已是完整 data URI（base64Captcha v1.3.1 EncodeB64string）；
    // 下面防御性兼容两种形态：已是 data: 前缀则直接用，否则拼前缀。
    const raw = data.image_base64
    captchaImg.value = raw.startsWith('data:') ? raw : 'data:image/png;base64,' + raw
    form.captcha_code = ''
  } catch {}
}
onMounted(loadCaptcha)

const rules: FormRules = {
  username: [{ required: true, message: '用户名不能为空', trigger: 'blur' }],
  email: [
    { required: true, message: '邮箱不能为空', trigger: 'blur' },
    { type: 'email', message: '邮箱格式不正确', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '密码不能为空', trigger: 'blur' },
    { min: 6, message: '密码长度不能少于6位', trigger: 'blur' },
  ],
  captcha_code: [{ required: true, message: '请输入验证码', trigger: 'blur' }],
}

async function handleRegister() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return
  loading.value = true
  try {
    await userStore.register(form.username, form.email, form.password, form.nickname, form.invite_code, form.captcha_id, form.captcha_code)
  } catch (e: any) {
    // 验证码错误/过期：自动刷新一张新图，清掉已输的验证码
    if (String(e?.response?.data?.message || e?.message || '').includes('验证码')) {
      loadCaptcha()
    }
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AuthLayout>
    <div class="auth-card rounded-2xl border border-slate-200 bg-white p-8 dark:border-slate-800 dark:bg-slate-900">
      <h1 class="text-lg font-semibold text-slate-950 dark:text-white">创建账号</h1>
      <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">加入软考星图，开启刷题之旅</p>
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-position="top"
        @submit.prevent="handleRegister"
        class="mt-5 space-y-2"
      >
        <el-form-item prop="username" label="用户名">
          <el-input v-model="form.username" placeholder="用户名" prefix-icon="User" size="large" autocomplete="username" />
        </el-form-item>
        <el-form-item prop="email" label="邮箱">
          <el-input v-model="form.email" placeholder="邮箱" prefix-icon="Message" size="large" autocomplete="email" />
        </el-form-item>
        <el-form-item label="昵称（可选）">
          <el-input v-model="form.nickname" placeholder="昵称（可选）" prefix-icon="UserFilled" size="large" />
        </el-form-item>
        <el-form-item prop="password" label="密码">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="密码（至少6位）"
            prefix-icon="Lock"
            show-password
            size="large"
            autocomplete="new-password"
          />
        </el-form-item>
        <el-form-item label="邀请码（可选）">
          <el-input v-model="form.invite_code" placeholder="邀请码（可选）" prefix-icon="Ticket" size="large" />
        </el-form-item>
        <el-form-item prop="captcha_code" label="验证码">
          <div class="flex w-full gap-2">
            <el-input
              v-model="form.captcha_code"
              placeholder="验证码"
              prefix-icon="Key"
              size="large"
              maxlength="4"
            />
            <button
              type="button"
              aria-label="刷新验证码"
              title="看不清？点击刷新"
              class="h-10 w-28 shrink-0 cursor-pointer overflow-hidden rounded-lg border border-slate-200 p-0 transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 hover:border-blue-300 dark:border-slate-700"
              @click="loadCaptcha"
            >
              <img :src="captchaImg" alt="验证码图片" class="h-full w-full object-cover" />
            </button>
          </div>
        </el-form-item>
        <el-button native-type="submit" class="auth-submit w-full" :loading="loading">
          注册
        </el-button>
        <div class="pt-1 text-center text-sm">
          <el-link type="primary" :underline="false" @click="router.push('/login')">已有账号？去登录</el-link>
        </div>
      </el-form>
    </div>
  </AuthLayout>
</template>
