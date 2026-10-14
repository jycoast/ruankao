<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'

const router = useRouter()
const userStore = useUserStore()

// 提醒档位：进入 7 / 3 / 1 天时各提醒一次，关掉后下一档再弹
const TIERS = [7, 3, 1]
const DISMISS_KEY = 'membership_expiry_dismissed_tier'

const dismissedTier = ref(Number(localStorage.getItem(DISMISS_KEY) || 0))

const daysLeft = computed(() => userStore.membership?.days_left ?? 0)

const tier = computed(() => {
  if (!userStore.membership?.is_member || daysLeft.value <= 0) return 0
  return TIERS.find((t) => daysLeft.value <= t) ?? 0
})

const visible = computed(() => tier.value > 0 && tier.value !== dismissedTier.value)

// 续费后（脱离提醒区间）清掉记录，下个会员周期能重新提醒
watch(tier, (t) => {
  if (t === 0 && dismissedTier.value) {
    dismissedTier.value = 0
    localStorage.removeItem(DISMISS_KEY)
  }
})

function dismiss() {
  dismissedTier.value = tier.value
  localStorage.setItem(DISMISS_KEY, String(tier.value))
}

function renew() {
  dismiss()
  router.push('/membership')
}
</script>

<template>
  <div
    v-if="visible"
    class="border-b border-amber-200 bg-amber-50 dark:border-amber-500/30 dark:bg-amber-500/10"
  >
    <div
      class="mx-auto flex w-full max-w-screen-2xl flex-wrap items-center gap-3 px-4 py-2 text-sm text-amber-800 sm:px-6 lg:px-8 lg:w-[86%] dark:text-amber-200"
    >
      <span class="flex-1">
        你的会员还有 <b>{{ daysLeft }}</b> 天到期，续费可叠加时长，AI 解析与 Anki 复习不断档。
      </span>
      <el-button size="small" type="warning" @click="renew">立即续费</el-button>
      <button
        class="text-xs text-amber-600 hover:underline dark:text-amber-300"
        @click="dismiss"
      >
        知道了
      </button>
    </div>
  </div>
</template>
