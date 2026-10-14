<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useUserStore } from '@/stores/user'
import { useAppearance, type ThemeMode } from '@/stores/appearance'
import { useRouter } from 'vue-router'
import { userApi } from '@/api'
import { ElMessage } from 'element-plus'

const router = useRouter()

const userStore = useUserStore()
const appearance = useAppearance()
const form = ref({ nickname: '', phone: '' })

onMounted(() => {
  userStore.fetchProfile()
  if (userStore.user) {
    form.value.nickname = userStore.user.nickname || ''
    form.value.phone = userStore.user.phone || ''
  }
})

async function updateProfile() {
  try {
    await userApi.updateProfile(form.value)
    await userStore.fetchProfile()
    ElMessage.success('保存成功')
  } catch {
    // 拦截器已提示
  }
}

// 色板来自 appearance store 的单一事实源，加颜色只改那边
const colorOptions = appearance.colorOptions

const modeOptions: { value: ThemeMode; label: string; icon: string }[] = [
  { value: 'light', label: '浅色模式', icon: 'Sunny' },
  { value: 'dark', label: '深色模式', icon: 'Moon' },
  { value: 'system', label: '跟随系统', icon: 'Monitor' },
]
</script>

<template>
  <div class="mx-auto max-w-2xl space-y-6">
    <h1 class="text-xl font-bold text-slate-950 dark:text-white">个人中心</h1>

    <!-- 基本资料卡：与首页/登录页统一的手写卡片语言 -->
    <section
      class="anim-item rounded-2xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900"
    >
      <div class="border-b border-slate-100 px-5 py-4 dark:border-slate-800">
        <h2 class="font-semibold text-slate-950 dark:text-white">基本资料</h2>
        <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
          昵称会显示在首页问候语，手机号用于联系方式的补充登记
        </p>
      </div>

      <div v-if="userStore.user" class="space-y-6 px-5 py-5">
        <div class="flex items-center gap-4">
          <el-avatar
            :size="64"
            :style="{ backgroundColor: 'var(--el-color-primary, #3b82f6)' }"
            class="shrink-0 text-lg font-bold"
          >{{ userStore.user.nickname?.[0] || userStore.user.username?.[0]?.toUpperCase() || 'U' }}</el-avatar>
          <div class="min-w-0">
            <!-- 有昵称时：大字显示昵称，下面单独标出用户名（登录要用，必须可见） -->
            <template v-if="userStore.user.nickname">
              <div class="truncate text-lg font-bold text-slate-950 dark:text-white">{{ userStore.user.nickname }}</div>
              <div class="truncate text-sm text-slate-500 dark:text-slate-400">用户名：{{ userStore.user.username }}</div>
            </template>
            <!-- 无昵称时：用户名就是展示名，不重复显示「用户名：」一行 -->
            <template v-else>
              <div class="truncate text-lg font-bold text-slate-950 dark:text-white">{{ userStore.user.username }}</div>
            </template>
            <div v-if="userStore.user.email" class="truncate text-sm text-slate-500 dark:text-slate-400">{{ userStore.user.email }}</div>
          </div>
        </div>

        <el-form :model="form" label-width="80px">
          <el-form-item label="昵称">
            <el-input v-model="form.nickname" autocomplete="nickname" />
          </el-form-item>
          <el-form-item label="手机">
            <el-input v-model="form.phone" type="tel" autocomplete="tel" />
          </el-form-item>
          <el-form-item>
            <div class="flex gap-3">
              <el-button type="primary" @click="updateProfile">保存</el-button>
              <el-button @click="router.push('/change-password')">{{ userStore.hasPassword ? '修改密码' : '设置密码' }}</el-button>
            </div>
          </el-form-item>
        </el-form>
      </div>

      <!-- 学习数据：统一品牌蓝语义色（原四色 rainbow 改为单一主色） -->
      <div
        v-if="userStore.profile"
        class="grid grid-cols-2 gap-4 border-t border-slate-100 px-5 py-4 sm:grid-cols-4 dark:border-slate-800"
      >
        <div class="text-center">
          <div class="text-xl font-semibold text-blue-600 dark:text-blue-400">{{ userStore.profile.total_practice }}</div>
          <div class="mt-0.5 text-sm text-slate-500 dark:text-slate-400">总练习</div>
        </div>
        <div class="text-center">
          <div class="text-xl font-semibold text-blue-600 dark:text-blue-400">{{ userStore.profile.total_correct }}</div>
          <div class="mt-0.5 text-sm text-slate-500 dark:text-slate-400">正确数</div>
        </div>
        <div class="text-center">
          <div class="text-xl font-semibold text-blue-600 dark:text-blue-400">{{ Math.round(userStore.profile.total_time / 60) }}分</div>
          <div class="mt-0.5 text-sm text-slate-500 dark:text-slate-400">学习时长</div>
        </div>
        <div class="text-center">
          <div class="text-xl font-semibold text-blue-600 dark:text-blue-400">{{ userStore.profile.streak_days }}天</div>
          <div class="mt-0.5 text-sm text-slate-500 dark:text-slate-400">连续学习</div>
        </div>
      </div>
    </section>

    <!-- 外观设置卡 -->
    <section
      class="anim-item rounded-2xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900"
      style="animation-delay: 90ms"
    >
      <div class="border-b border-slate-100 px-5 py-4 dark:border-slate-800">
        <h2 class="font-semibold text-slate-950 dark:text-white">外观设置</h2>
        <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
          主题颜色与外观模式即时生效
        </p>
      </div>

      <div class="space-y-6 px-5 py-5">
        <!-- Theme Color：选中态视觉 + aria-pressed 双通道标注 -->
        <div>
          <div class="mb-3 text-sm font-medium text-slate-700 dark:text-slate-300">主题颜色</div>
          <div class="flex flex-wrap gap-3">
            <button
              v-for="opt in colorOptions"
              :key="opt.value"
              type="button"
              :aria-label="`主题颜色：${opt.label}`"
              :aria-pressed="appearance.themeColor.value === opt.value"
              @click="appearance.setThemeColor(opt.value)"
              class="flex h-11 w-11 cursor-pointer items-center justify-center rounded-full border-2 transition-all focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500"
              :class="appearance.themeColor.value === opt.value
                ? 'border-slate-800 scale-110 shadow-lg dark:border-white'
                : 'border-slate-200 hover:scale-105 dark:border-slate-600'"
              :style="{ backgroundColor: opt.color }"
              :title="opt.label"
            >
              <el-icon v-if="appearance.themeColor.value === opt.value" class="text-white" :size="16">
                <Check />
              </el-icon>
            </button>
          </div>
        </div>

        <!-- Theme Mode -->
        <div>
          <div class="mb-3 text-sm font-medium text-slate-700 dark:text-slate-300">外观模式</div>
          <div class="flex flex-wrap gap-3">
            <button
              v-for="opt in modeOptions"
              :key="opt.value"
              type="button"
              :aria-pressed="appearance.themeMode.value === opt.value"
              @click="appearance.setThemeMode(opt.value)"
              class="flex cursor-pointer items-center gap-2 rounded-lg border px-4 py-3 text-sm transition-all focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500"
              :class="appearance.themeMode.value === opt.value
                ? 'border-blue-500 bg-blue-50 font-medium text-blue-600 dark:bg-blue-500/15 dark:text-blue-400'
                : 'border-slate-200 text-slate-600 hover:bg-slate-50 dark:border-slate-600 dark:text-slate-400 dark:hover:bg-slate-800/60'"
            >
              <el-icon :size="18"><component :is="opt.icon" /></el-icon>
              <span>{{ opt.label }}</span>
            </button>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
/* 入场动效：与首页/认证页同一套淡入上移（可关） */
@keyframes profile-fade-in-up {
  from {
    opacity: 0;
    transform: translateY(12px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
.anim-item {
  animation: profile-fade-in-up 0.4s ease both;
}

@media (prefers-reduced-motion: reduce) {
  .anim-item {
    animation: none;
  }
}
</style>
