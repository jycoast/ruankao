import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/api'

// 维护开关的客户端缓存时长。这直接界定了「维护开启后，还在浏览的用户
// 最多多久会被守卫发现」——60 秒内开启的维护靠 axios 拦截器兜住，
// 两条路径互补，不是二选一。
const CACHE_TTL = 60_000

export const useSiteStore = defineStore('site', () => {
  const maintenance = ref(false)
  let fetchedAt = 0
  // 导航守卫可能被连续触发（连点链接、重定向链），并发去重避免打出一串重复请求
  let inflight: Promise<boolean> | null = null

  // fetchMaintenance 读维护开关。
  // 任何失败都按「未维护」放行：一次网络抖动就弹整站维护页太吓人，
  // 用户自己的网络问题也会误触发。
  async function fetchMaintenance(force = false): Promise<boolean> {
    if (!force && Date.now() - fetchedAt < CACHE_TTL) return maintenance.value
    if (inflight) return inflight

    inflight = api
      .get('/site-status', { silent: true } as any)
      .then((res: any) => {
        maintenance.value = !!res.data?.maintenance
        fetchedAt = Date.now()
        return maintenance.value
      })
      .catch(() => {
        // 放行，但**不**更新时间戳：不然一次失败会被缓存 60 秒，
        // 等于把一次抖动放大成一分钟的静默失效。
        return maintenance.value
      })
      .finally(() => {
        inflight = null
      })
    return inflight
  }

  return { maintenance, fetchMaintenance }
})
