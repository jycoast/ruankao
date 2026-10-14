import { ref, computed } from 'vue'
import { aiApi } from '@/api'

// 全局缓存 AI 启用状态，避免各模块重复探测。
const aiEnabled = ref(false)
const trialUsed = ref(0)
const trialLimit = ref(0)
const ownConfig = ref(false)
let loaded = false
let inflight: Promise<void> | null = null

async function fetchStatus() {
  try {
    const res: any = await aiApi.status()
    aiEnabled.value = !!res.data.enabled
    trialUsed.value = res.data.trial_used || 0
    trialLimit.value = res.data.trial_limit || 0
    ownConfig.value = !!res.data.own_config
  } catch {
    aiEnabled.value = false
  }
}

async function ensureAIStatus(): Promise<void> {
  if (loaded) return
  if (inflight) return inflight
  inflight = (async () => {
    await fetchStatus()
    loaded = true
    inflight = null
  })()
  return inflight
}

// AI 调用后刷新剩余额度
async function refreshAIStatus() {
  await fetchStatus()
}

export function useAI() {
  const trialRemaining = computed(() => Math.max(0, trialLimit.value - trialUsed.value))
  // 会员/管理员/自带 key 的用户后端不返额度（limit=0），不展示试用提示
  const showTrial = computed(() => trialLimit.value > 0 && !ownConfig.value)
  return { aiEnabled, trialUsed, trialLimit, trialRemaining, showTrial, ensureAIStatus, refreshAIStatus }
}
