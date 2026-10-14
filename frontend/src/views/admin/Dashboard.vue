<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { adminApi } from '@/api'
import { ElMessage, ElMessageBox } from 'element-plus'

const router = useRouter()
const stats = ref<any>({})
const loading = ref(true)

onMounted(async () => {
  try {
    const res: any = await adminApi.getDashboard()
    stats.value = res.data || {}
  } finally {
    loading.value = false
  }
})

// 在线用户：接口很轻（只查 Redis），单独轮询，不拖累整份仪表盘
const online = ref(0)
const onlineUsers = ref<any[]>([])
const onlineWindowMin = ref(5)
let onlineTimer: number | undefined
// 服务器时钟偏差（秒）与本地滚动时钟：算出「N 秒前活跃」且能自己走，不必等轮询
const serverOffsetSec = ref(0)
const nowSec = ref(Math.floor(Date.now() / 1000))
let clockTimer: number | undefined

const loadOnline = async () => {
  try {
    const res: any = await adminApi.getOnline()
    online.value = res.data?.online || 0
    onlineUsers.value = res.data?.users || []
    const w = res.data?.window
    if (w) onlineWindowMin.value = Math.max(1, Math.round(w / 60))
    if (res.data?.now) serverOffsetSec.value = res.data.now - Math.floor(Date.now() / 1000)
  } catch {
    // 轮询失败静默，下次重试
  }
}

onMounted(() => {
  loadOnline()
  onlineTimer = window.setInterval(loadOnline, 30_000)
  clockTimer = window.setInterval(() => { nowSec.value = Math.floor(Date.now() / 1000) }, 15_000)
})

onBeforeUnmount(() => {
  if (onlineTimer) window.clearInterval(onlineTimer)
  if (clockTimer) window.clearInterval(clockTimer)
})

const displayName = (u: any) => u.nickname || u.username || '用户'
const relTime = (unix?: number) => {
  if (!unix) return '在线' // 旧后端无该字段时兜底
  const age = Math.max(0, nowSec.value - unix + serverOffsetSec.value)
  if (age < 60) return age + ' 秒前'
  if (age < 3600) return Math.floor(age / 60) + ' 分钟前'
  return Math.floor(age / 3600) + ' 小时前'
}
const onlineMemberCount = computed(() => onlineUsers.value.filter((u) => u.is_member).length)

// 系统维护开关。放在仪表盘最上方：出故障要开维护时，
// 最不该发生的事情是「还得先找那个开关在哪个二级菜单里」。
const maintenance = ref(false)
const maintenanceSaving = ref(false)
// 读设置失败标记。失败时 maintenance 仍是 false —— 开关必须保持可用，
// 出故障正是要打开维护的时候 —— 但界面不能据此宣称「站点正常」：
// 读失败与事故高度相关，正是管理员会打开仪表盘的时刻。
// 卡片据此走第三态（未知），而不是说一句可能为假的「所有用户正常访问」。
const maintenanceUnknown = ref(false)

onMounted(async () => {
  try {
    const res: any = await adminApi.getSiteSettings()
    maintenance.value = !!res.data?.maintenance_enabled
    maintenanceUnknown.value = false
  } catch {
    // 读失败：不弹窗、不阻塞仪表盘其它内容，但状态标为未知
    maintenanceUnknown.value = true
  }
})

const onMaintenanceChange = async (val: boolean) => {
  try {
    await ElMessageBox.confirm(
      val
        ? '开启后，所有非管理员用户将无法访问站点（前端显示维护页，接口返回 503）。确认开启？'
        : '确认关闭维护模式、恢复全站访问？',
      '系统维护',
      { type: 'warning', confirmButtonText: '确认', cancelButtonText: '取消' },
    )
  } catch {
    maintenance.value = !val // 取消：把开关拨回原位
    return
  }
  maintenanceSaving.value = true
  try {
    await adminApi.updateSiteSettings(val)
    // 保存成功 = 服务端确实按这个值落库了，状态从未知变为已知
    maintenanceUnknown.value = false
    ElMessage.success(val ? '已开启系统维护' : '已关闭系统维护')
  } catch {
    maintenance.value = !val // 保存失败：开关不能停在骗人的位置上
  } finally {
    maintenanceSaving.value = false
  }
}

const kpis = computed(() => [
  { label: '用户总数', value: stats.value.total_users || 0, note: `今日 +${stats.value.new_users_today || 0} · 近7天 +${stats.value.new_users_week || 0}`, icon: 'User', color: 'text-blue-600', bg: 'bg-blue-50 dark:bg-blue-500/15' },
  { label: '题目总数', value: stats.value.total_questions || 0, note: '题库累计', icon: 'List', color: 'text-emerald-600', bg: 'bg-emerald-50 dark:bg-emerald-500/15' },
  { label: '今日做题', value: stats.value.today_practice || 0, note: `活跃用户 ${stats.value.active_users_today || 0}`, icon: 'EditPen', color: 'text-orange-600', bg: 'bg-orange-50 dark:bg-orange-500/15' },
  { label: '总正确率', value: (stats.value.overall_accuracy || 0) + '%', note: `累计 ${stats.value.total_practice || 0} 次作答`, icon: 'TrendCharts', color: 'text-blue-600', bg: 'bg-blue-50 dark:bg-blue-500/15' },
])

const trend = computed<{ date: string; count: number }[]>(() => stats.value.practice_trend || [])
const trendMax = computed(() => Math.max(1, ...trend.value.map((t) => t.count)))

// 后端金额一律以「分」返回，只在这里除一次
const yuan = (cents: number) => ((cents || 0) / 100).toFixed(2)
// LDC 积分同为 0.01 积分记价，展示格式与人民币一致，只是带积分单位
const credit = (cents: number) => yuan(cents) + ' 积分'
const pct = (n: number, d: number) => (d > 0 ? ((n / d) * 100).toFixed(1) : '0.0')
const compact = (n: number) => {
  n = n || 0
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return String(n)
}

const bizKpis = computed(() => {
  const s = stats.value
  return [
    {
      // 现金与 LDC 积分是两套定价、量纲不同，必须分开展示，不能加在一起
      label: '现金收入',
      value: '¥' + yuan(s.revenue_cash_total),
      note: `本月 ¥${yuan(s.revenue_cash_month)} · ${s.revenue_cash_orders || 0} 笔`,
      icon: 'CreditCard', color: 'text-emerald-600', bg: 'bg-emerald-50 dark:bg-emerald-500/15',
    },
    {
      label: '积分收入',
      value: credit(s.revenue_credit_total),
      note: `本月 ${credit(s.revenue_credit_month)} · ${s.revenue_credit_orders || 0} 笔`,
      icon: 'Coin', color: 'text-amber-600', bg: 'bg-amber-50 dark:bg-amber-500/15',
    },
    {
      label: '有效会员',
      value: s.active_members || 0,
      note: `已到期 ${s.expired_members || 0} 人待续费`,
      icon: 'GoldMedal', color: 'text-amber-600', bg: 'bg-amber-50 dark:bg-amber-500/15',
    },
    {
      label: '付费转化',
      value: pct(s.paid_users || 0, s.total_users || 0) + '%',
      note: `${s.paid_users || 0} 人曾付费 · 在期占比 ${pct(s.active_members || 0, s.total_users || 0)}%`,
      icon: 'TrendCharts', color: 'text-violet-600', bg: 'bg-violet-50 dark:bg-violet-500/15',
    },
    {
      label: 'AI 用量（30天）',
      value: compact(s.ai_tokens_30d) + ' tokens',
      note: `${s.ai_calls_30d || 0} 次调用 · 失败 ${pct(s.ai_failed_30d || 0, s.ai_calls_30d || 0)}%`
        + ((s.ai_estimated_ratio || 0) > 0 ? ` · ${s.ai_estimated_ratio}% 为估算` : ''),
      icon: 'MagicStick', color: 'text-blue-600', bg: 'bg-blue-50 dark:bg-blue-500/15',
    },
  ]
})

const revTrend = computed<{ date: string; count: number }[]>(() => stats.value.revenue_trend || [])
const revTrendCredit = computed<{ date: string; count: number }[]>(() => stats.value.revenue_trend_credit || [])
// 现金逐日 = 全部渠道 - 积分渠道（两条序列同日期、单位同为 0.01 分，可直接相减）
const revTrendCash = computed<{ date: string; count: number }[]>(() =>
  revTrend.value.map((t, i) => ({ date: t.date, count: t.count - (revTrendCredit.value[i]?.count || 0) })),
)
const revMax = computed(() =>
  Math.max(1, ...revTrendCash.value.map((t) => t.count), ...revTrendCredit.value.map((t) => t.count)),
)

const TYPE_LABELS: Record<number, string> = { 1: '单选', 2: '多选', 3: '案例', 4: '论文' }
const TYPE_COLORS: Record<number, string> = { 1: 'bg-blue-500', 2: 'bg-violet-500', 3: 'bg-emerald-500', 4: 'bg-orange-500' }
const typeDist = computed<{ type: number; count: number }[]>(() => stats.value.type_dist || [])
const typeTotal = computed(() => Math.max(1, typeDist.value.reduce((s, d) => s + d.count, 0)))
</script>

<template>
  <div v-loading="loading" class="space-y-6">
    <h2 class="text-xl font-bold text-gray-900 dark:text-gray-100">仪表盘</h2>

    <!-- 待办提醒 -->
    <div v-if="(stats.pending_reports || 0) + (stats.pending_essays || 0) + (stats.pending_feedbacks || 0) > 0" class="flex flex-wrap gap-3">
      <button
        v-if="stats.pending_reports > 0"
        class="flex items-center gap-2 rounded-lg border border-rose-200 bg-rose-50 px-4 py-2 text-sm font-medium text-rose-700 transition hover:bg-rose-100 dark:border-rose-500/30 dark:bg-rose-500/10 dark:text-rose-300"
        @click="router.push('/admin/reports')"
      >
        <el-icon><Warning /></el-icon> 纠错待处理 {{ stats.pending_reports }} 条
      </button>
      <button
        v-if="stats.pending_essays > 0"
        class="flex items-center gap-2 rounded-lg border border-orange-200 bg-orange-50 px-4 py-2 text-sm font-medium text-orange-700 transition hover:bg-orange-100 dark:border-orange-500/30 dark:bg-orange-500/10 dark:text-orange-300"
        @click="router.push('/admin/essay-submissions')"
      >
        <el-icon><EditPen /></el-icon> 投稿待审 {{ stats.pending_essays }} 篇
      </button>
      <button
        v-if="stats.pending_feedbacks > 0"
        class="flex items-center gap-2 rounded-lg border border-violet-200 bg-violet-50 px-4 py-2 text-sm font-medium text-violet-700 transition hover:bg-violet-100 dark:border-violet-500/30 dark:bg-violet-500/10 dark:text-violet-300"
        @click="router.push('/admin/feedbacks')"
      >
        <el-icon><ChatDotRound /></el-icon> 反馈待处理 {{ stats.pending_feedbacks }} 条
      </button>
    </div>

    <!-- 系统维护开关：异常时要一眼找到，所以放最上方，不塞进二级菜单 -->
    <div
      class="flex flex-wrap items-center justify-between gap-4 rounded-xl border p-4 shadow-sm"
      :class="maintenanceUnknown
        ? 'border-amber-300 bg-amber-50 dark:border-amber-500/40 dark:bg-amber-500/10'
        : maintenance
          ? 'border-red-300 bg-red-50 dark:border-red-500/40 dark:bg-red-500/10'
          : 'border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800'"
    >
      <div>
        <div class="flex items-center gap-2 font-medium text-gray-900 dark:text-gray-100">
          <el-icon><Tools /></el-icon> 系统维护中
        </div>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ maintenanceUnknown
            ? '状态未知：设置读取失败，请刷新后确认'
            : maintenance
              ? '已开启：普通用户看到维护页，后端接口返回 503。管理员不受影响。'
              : '已关闭：所有用户正常访问。' }}
        </p>
      </div>
      <el-switch
        v-model="maintenance"
        :loading="maintenanceSaving"
        :disabled="maintenanceSaving"
        active-text="开启"
        inactive-text="关闭"
        @change="onMaintenanceChange"
      />
    </div>

    <!-- KPI 卡片 -->
    <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <div
        v-for="k in kpis"
        :key="k.label"
        class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-700 dark:bg-gray-800"
      >
        <div class="flex items-center justify-between">
          <div class="min-w-0">
            <div class="text-sm text-gray-500 dark:text-gray-400">{{ k.label }}</div>
            <div class="mt-1 text-2xl font-bold text-gray-900 dark:text-gray-100">{{ k.value }}</div>
            <div class="mt-0.5 truncate text-xs text-gray-400">{{ k.note }}</div>
          </div>
          <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg" :class="[k.bg, k.color]">
            <el-icon :size="20"><component :is="k.icon" /></el-icon>
          </div>
        </div>
      </div>
    </div>

    <!-- 实时在线 -->
    <div>
      <h3 class="mb-3 text-sm font-semibold text-gray-500 dark:text-gray-400">实时</h3>
      <div class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-700 dark:bg-gray-800">
        <div class="flex flex-col gap-4 lg:flex-row">
          <!-- 在线数 + 会员数 -->
          <div class="flex shrink-0 items-center gap-3 lg:w-60">
            <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg bg-emerald-50 text-emerald-600 dark:bg-emerald-500/15">
              <el-icon :size="22"><Monitor /></el-icon>
            </div>
            <div class="min-w-0">
              <div class="text-sm text-gray-500 dark:text-gray-400">在线用户</div>
              <div class="flex items-baseline gap-2">
                <span class="text-2xl font-bold text-gray-900 dark:text-gray-100">{{ online }}</span>
                <span class="text-xs text-gray-400">近 {{ onlineWindowMin }} 分钟活跃</span>
              </div>
              <div class="mt-1 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs">
                <span class="inline-flex items-center gap-1 text-emerald-600 dark:text-emerald-400">
                  <span class="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-500"></span>实时
                </span>
                <span v-if="onlineMemberCount > 0" class="inline-flex items-center gap-1 text-amber-600 dark:text-amber-400">
                  <el-icon :size="12"><GoldMedal /></el-icon>会员 {{ onlineMemberCount }}
                </span>
              </div>
            </div>
          </div>

          <!-- 最近活跃用户列表 -->
          <div class="min-w-0 flex-1 border-t border-gray-100 pt-3 dark:border-gray-700 lg:border-l lg:border-t-0 lg:pl-4 lg:pt-0">
            <div class="mb-2 text-xs text-gray-400">最近活跃 · 每 30 秒刷新</div>
            <div v-if="onlineUsers.length" class="grid max-h-44 gap-1 overflow-y-auto pr-1 sm:grid-cols-2 xl:grid-cols-3">
              <div
                v-for="u in onlineUsers"
                :key="u.id"
                class="flex items-center gap-2 rounded-lg px-2 py-1.5 transition hover:bg-gray-50 dark:hover:bg-gray-700/40"
              >
                <el-avatar :size="26" :src="u.avatar || undefined" class="shrink-0 bg-gray-200 text-xs dark:bg-gray-600">
                  {{ displayName(u)[0] }}
                </el-avatar>
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-1.5">
                    <span class="truncate text-sm text-gray-700 dark:text-gray-200" :title="displayName(u)">{{ displayName(u) }}</span>
                    <el-tag v-if="u.is_member" size="small" type="warning" effect="plain" round>会员</el-tag>
                  </div>
                  <div class="text-xs text-gray-400">{{ relTime(u.last_active_at) }}</div>
                </div>
              </div>
            </div>
            <div v-else class="py-4 text-center text-sm text-gray-400">当前暂无活跃用户</div>
          </div>
        </div>
      </div>
    </div>

    <!-- 商业化 -->
    <div>
      <h3 class="mb-3 text-sm font-semibold text-gray-500 dark:text-gray-400">商业化</h3>
      <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
        <div
          v-for="k in bizKpis"
          :key="k.label"
          class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-700 dark:bg-gray-800"
        >
          <div class="flex items-center justify-between">
            <div class="min-w-0">
              <div class="text-sm text-gray-500 dark:text-gray-400">{{ k.label }}</div>
              <div class="mt-1 text-2xl font-bold text-gray-900 dark:text-gray-100">{{ k.value }}</div>
              <div class="mt-0.5 truncate text-xs text-gray-400" :title="k.note">{{ k.note }}</div>
            </div>
            <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg" :class="[k.bg, k.color]">
              <el-icon :size="20"><component :is="k.icon" /></el-icon>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 运营与 AI -->
    <div class="grid gap-4 sm:grid-cols-3">
      <!-- 题目纠错 -->
      <button
        class="rounded-xl border border-gray-200 bg-white p-4 text-left shadow-sm transition hover:border-blue-200 hover:shadow-md dark:border-gray-700 dark:bg-gray-800"
        @click="router.push('/admin/reports')"
      >
        <div class="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400">
          <el-icon class="text-rose-500"><Warning /></el-icon> 题目纠错
        </div>
        <div class="mt-2 flex items-baseline gap-2">
          <span class="text-2xl font-bold text-gray-900 dark:text-gray-100">{{ stats.total_reports || 0 }}</span>
          <span class="text-xs text-gray-400">累计提交</span>
        </div>
        <div class="mt-1 text-sm" :class="(stats.pending_reports || 0) > 0 ? 'font-medium text-rose-600 dark:text-rose-400' : 'text-gray-400'">
          待处理 {{ stats.pending_reports || 0 }}
        </div>
      </button>

      <!-- 论文投稿 -->
      <button
        class="rounded-xl border border-gray-200 bg-white p-4 text-left shadow-sm transition hover:border-blue-200 hover:shadow-md dark:border-gray-700 dark:bg-gray-800"
        @click="router.push('/admin/essay-submissions')"
      >
        <div class="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400">
          <el-icon class="text-orange-500"><EditPen /></el-icon> 论文投稿
        </div>
        <div class="mt-2 flex items-baseline gap-2">
          <span class="text-2xl font-bold text-gray-900 dark:text-gray-100">{{ stats.total_essays || 0 }}</span>
          <span class="text-xs text-gray-400">累计提交</span>
        </div>
        <div class="mt-1 text-sm" :class="(stats.pending_essays || 0) > 0 ? 'font-medium text-orange-600 dark:text-orange-400' : 'text-gray-400'">
          待审核 {{ stats.pending_essays || 0 }}
        </div>
      </button>

      <!-- AI 使用 -->
      <div class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-700 dark:bg-gray-800">
        <div class="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400">
          <el-icon class="text-violet-500"><MagicStick /></el-icon> AI 使用
        </div>
        <div class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1.5 text-sm">
          <div class="flex items-baseline gap-1"><span class="text-lg font-bold text-gray-900 dark:text-gray-100">{{ stats.ai_conversations || 0 }}</span><span class="text-xs text-gray-400">对话</span></div>
          <div class="flex items-baseline gap-1"><span class="text-lg font-bold text-gray-900 dark:text-gray-100">{{ stats.ai_messages || 0 }}</span><span class="text-xs text-gray-400">消息</span></div>
          <div class="flex items-baseline gap-1"><span class="text-lg font-bold text-gray-900 dark:text-gray-100">{{ stats.ai_today_messages || 0 }}</span><span class="text-xs text-gray-400">今日</span></div>
          <div class="flex items-baseline gap-1"><span class="text-lg font-bold text-gray-900 dark:text-gray-100">{{ stats.ai_users || 0 }}</span><span class="text-xs text-gray-400">用户</span></div>
        </div>
      </div>
    </div>

    <div class="grid gap-6 lg:grid-cols-[1fr_360px]">
      <!-- 近 7 天做题趋势 -->
      <section class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
        <h3 class="mb-4 font-semibold text-gray-900 dark:text-gray-100">近 7 天做题趋势</h3>
        <div class="flex h-48 items-end gap-2">
          <div v-for="t in trend" :key="t.date" class="group flex flex-1 flex-col items-center gap-1">
            <div class="text-xs font-medium text-gray-500 opacity-0 transition group-hover:opacity-100 dark:text-gray-300">{{ t.count }}</div>
            <div
              class="w-full rounded-t bg-blue-500 transition-all dark:bg-blue-500/80"
              :style="{ height: Math.round((t.count / trendMax) * 160) + 'px' }"
            />
            <div class="text-xs text-gray-400">{{ t.date }}</div>
          </div>
        </div>
      </section>

      <!-- 题型分布 -->
      <section class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
        <h3 class="mb-4 font-semibold text-gray-900 dark:text-gray-100">题型分布</h3>
        <div v-if="typeDist.length" class="space-y-3">
          <div v-for="d in typeDist" :key="d.type">
            <div class="mb-1 flex items-center justify-between text-sm">
              <span class="text-gray-700 dark:text-gray-200">{{ TYPE_LABELS[d.type] || '其他' }}</span>
              <span class="text-gray-400">{{ d.count }} · {{ Math.round((d.count / typeTotal) * 100) }}%</span>
            </div>
            <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
              <div class="h-full rounded-full transition-all" :class="TYPE_COLORS[d.type] || 'bg-gray-400'" :style="{ width: Math.round((d.count / typeTotal) * 100) + '%' }" />
            </div>
          </div>
        </div>
        <div v-else class="py-8 text-center text-sm text-gray-400">暂无数据</div>
      </section>
    </div>

    <!-- 近 30 天营收：现金 / 积分分列展示，量纲不同不合并 -->
    <section class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
      <h3 class="mb-4 font-semibold text-gray-900 dark:text-gray-100">近 30 天营收（现金 / 积分）</h3>
      <!-- 窄屏横向滚动保证柱宽；逐日数值用 el-tooltip（桌面 hover / 移动端 tap 均可查看） -->
      <div class="rev-chart overflow-x-auto">
        <div class="flex h-40 min-w-[560px] items-end gap-1">
          <el-tooltip
            v-for="(t, i) in revTrend"
            :key="t.date"
            :content="`${t.date} 现金 ¥${yuan(revTrendCash[i]?.count)} · 积分 ${yuan(revTrendCredit[i]?.count)}`"
            placement="top"
            :show-after="100"
          >
            <div class="flex min-w-0 cursor-pointer flex-1 flex-col items-center justify-end">
              <div class="flex w-full items-end justify-center gap-0.5">
                <div
                  class="w-1/3 rounded-t bg-emerald-500 transition-all dark:bg-emerald-500/80"
                  :style="{ height: Math.round(((revTrendCash[i]?.count || 0) / revMax) * 120) + 'px' }"
                />
                <div
                  class="w-1/3 rounded-t bg-amber-400 transition-all dark:bg-amber-400/80"
                  :style="{ height: Math.round(((revTrendCredit[i]?.count || 0) / revMax) * 120) + 'px' }"
                />
              </div>
            </div>
          </el-tooltip>
        </div>
      </div>
      <div class="mt-1 flex justify-between text-xs text-gray-400">
        <span>{{ revTrend[0]?.date }}</span>
        <span>{{ revTrend[revTrend.length - 1]?.date }}</span>
      </div>
      <div class="mt-2 flex items-center justify-center gap-4 text-xs text-gray-400">
        <span class="flex items-center gap-1"><i class="inline-block h-2.5 w-2.5 rounded-sm bg-emerald-500"></i>现金（¥）</span>
        <span class="flex items-center gap-1"><i class="inline-block h-2.5 w-2.5 rounded-sm bg-amber-400"></i>积分</span>
      </div>
    </section>
  </div>
</template>

<style scoped>
/* 近30天营收图：el-tooltip 包裹层是 <span class="el-tooltip__trigger">，
   需在 flex 行里随 flex-1 伸缩，柱底对齐 */
.rev-chart :deep(.el-tooltip__trigger) {
  flex: 1 1 0;
  min-width: 0;
}
</style>
