import { computed, ref } from 'vue'
import api from '@/api'

// 内置排期兜底：后端不可用/PWA 离线时倒计时仍可用，正常情况以下后台配置为准
const EXAM_DATES = [
  { label: '2026年秋季', date: '2026-11-07' },
  { label: '2027年春季', date: '2027-05-22' },
  { label: '2027年秋季', date: '2027-11-06' },
  { label: '2028年春季', date: '2028-05-27' },
]

function daysUntil(dateStr: string) {
  const d = new Date(dateStr + 'T00:00:00')
  return Math.ceil((d.getTime() - Date.now()) / (1000 * 60 * 60 * 24))
}

// 后台排期（模块级共享：多个消费方只拉一次）
const remoteSchedules = ref<{ label: string; date: string }[] | null>(null)
let fetched = false
function loadSchedules() {
  if (fetched) return
  fetched = true
  // silent 是必须的：拦截器会替调用方弹错误提示，而这里只是给倒计时取个更准的日期，
  // 失败本就该无声降级到内置表。有了它，维护期间打开登录页才不会因为这一发请求
  // 弹一个「系统维护中」的红条。
  api
    .get('/exam-schedules', { silent: true } as any)
    .then((res: any) => {
      remoteSchedules.value = (res.data || []).map((s: any) => ({
        label: s.label,
        date: String(s.exam_date).slice(0, 10),
      }))
    })
    .catch(() => {}) // 静默失败：走内置兜底
}

// 下一场考试的倒计时。导航栏 chip 与首页焦点卡共用，避免重复维护日期表。
// 日期在管理后台「考试排期」配置，前端内置表仅作离线兜底。
export function useExamCountdown() {
  loadSchedules() // fire-and-forget：加载完成后 computed 自动重算

  const countdown = computed(() => {
    const sources = remoteSchedules.value || EXAM_DATES
    for (const e of sources) {
      const days = daysUntil(e.date)
      if (days > 0) {
        return { days, label: e.label, date: e.date, note: e.label + ' · ' + e.date }
      }
    }
    return { days: 0, label: '暂无排期', date: '', note: '暂无排期' }
  })

  return { countdown }
}
