<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { membershipApi, aiApi } from '@/api'
import { useUserStore } from '@/stores/user'

interface Plan {
  key: string
  name: string
  days: number
  amount: number // 人民币，分
  ldc_amount: number // LDC 积分，0.01 积分
}

interface PayChannel {
  type: string
  name: string
  credit: boolean // 以积分计价（后端下发），前端据此切换 ¥ / 积分展示
}

const route = useRoute()
const userStore = useUserStore()

const plans = ref<Plan[]>([])
const channels = ref<PayChannel[]>([])
const selectedPlan = ref<string>('')
const payType = ref<string>('')
const loading = ref(false)
const paying = ref(false)
const cardCode = ref('')
const redeeming = ref(false)

const PENDING_KEY = 'pending_order_no'

const membership = computed(() => userStore.membership)
// 到期前 7 天开始提醒续费
const expiringSoon = computed(() => {
  const m = membership.value
  return !!m?.is_member && m.days_left > 0 && m.days_left <= 7
})

const yuan = (fen: number) => (fen / 100).toFixed(2).replace(/\.00$/, '')

const payingWithCredit = computed(
  () => channels.value.find((c) => c.type === payType.value)?.credit === true,
)
const selected = computed(() => plans.value.find((p) => p.key === selectedPlan.value))

// 整页价格随支付方式走：选积分通道就全用积分计价，
// 避免卡片标 ¥99、结算栏却写 99 积分这种前后不一致。
const amountOf = (p: Plan) => (payingWithCredit.value ? p.ldc_amount : p.amount)
const payableText = computed(() => {
  const p = selected.value
  if (!p) return ''
  return payingWithCredit.value ? `${yuan(amountOf(p))} 积分` : `¥${yuan(amountOf(p))}`
})

// 卡片副价：主价按当前支付方式计，副价给出另一种计价，
// 让用户不切换支付方式也能看到积分要多少。积分价同样由后台配置，
// 未配置（0）或后台没开积分通道时不展示，避免出现「0 积分」。
const hasCreditChannel = computed(() => channels.value.some((c) => c.credit))
const altAmountOf = (p: Plan) => (payingWithCredit.value ? p.amount : p.ldc_amount)
const altPriceText = (p: Plan) =>
  payingWithCredit.value ? `¥${yuan(p.amount)}` : `${yuan(p.ldc_amount)} 积分`
const showAltPrice = (p: Plan) => hasCreditChannel.value && altAmountOf(p) > 0

// 均摊到每天的价格，让长周期套餐的性价比一眼可见
const perDay = (p: Plan) => (amountOf(p) / 100 / p.days).toFixed(2)
const perDayText = (p: Plan) =>
  payingWithCredit.value ? `${perDay(p)} 积分/天` : `¥${perDay(p)}/天`

// 以「天数最短的套餐」为基准算省了多少，而不是写死月付
const baseline = computed(() => {
  if (!plans.value.length) return null
  return plans.value.reduce((a, b) => (a.days <= b.days ? a : b))
})
const savePercent = (p: Plan) => {
  const base = baseline.value
  if (!base || base.key === p.key || !base.days || !p.days) return 0
  const baseRate = amountOf(base) / base.days
  if (!baseRate) return 0
  return Math.round((1 - amountOf(p) / p.days / baseRate) * 100)
}

// 单位天价最低的那个套餐打「最划算」标，套餐改价后标记自动跟着走
const bestKey = computed(() => {
  if (plans.value.length < 2) return ''
  return plans.value.reduce((a, b) => (amountOf(a) / a.days <= amountOf(b) / b.days ? a : b)).key
})

// 支付方式配色：按通道给一个识别色，避免一排灰按钮分不清
const CHANNEL_TONE: Record<string, string> = {
  alipay: 'text-blue-600 dark:text-blue-400',
  wxpay: 'text-green-600 dark:text-green-400',
  yt_alipay: 'text-blue-600 dark:text-blue-400',
  yt_wxpay: 'text-green-600 dark:text-green-400',
  ldcpay: 'text-amber-600 dark:text-amber-500',
}
const toneOf = (t: string) => CHANNEL_TONE[t] || 'text-slate-600 dark:text-slate-300'

// 会员/非会员功能对比。
// 内容必须与后端实际门禁一致：premium 中间件（cmd/server/main.go:293,406）是真·会员专属，
// AI 功能走 aiQuota（免费额度用尽才拦），其余接口只需登录。
interface FeatureRow {
  name: string
  desc: string
  free: boolean | string
  vip: boolean | string
}
interface FeatureGroup {
  title: string
  rows: FeatureRow[]
}

// 非会员 AI 累计免费次数，由管理员后台配置，从 /ai/status 读取真实值。
// 会员/管理员访问该接口时后端恒返回 0（额度对其无意义），此时值不可信，用 null 表示未知。
const aiTrialLimit = ref<number | null>(null)
const aiTrialText = computed(() => {
  if (aiTrialLimit.value === null) return '免费体验'
  return aiTrialLimit.value > 0 ? `免费 ${aiTrialLimit.value} 次` : '不可用'
})

const featureGroups = computed<FeatureGroup[]>(() => [
  {
    title: '题库与练习',
    rows: [
      { name: '分类刷题', desc: '按知识点分类练习历年真题', free: true, vip: true },
      { name: '历年真题试卷', desc: '整卷模拟实战，自动判分（近三年卷会员专属）', free: '近三年', vip: true },
      { name: '真题解析 PDF 导出', desc: '一键导出整年真题及解析 PDF', free: false, vip: true },
      { name: '每日一练 · 签到', desc: '每天 15 道精选题，连续打卡', free: true, vip: true },
      { name: '搜题 · 知识点库', desc: '全站题库检索与知识点速查', free: true, vip: true },
      { name: '范文库', desc: '论文范文阅读与投稿', free: true, vip: true },
      {
        name: 'Anki 遗忘曲线复习',
        desc: '按记忆规律自动安排每日复习队列',
        free: false,
        vip: true,
      },
    ],
  },
  {
    title: 'AI 能力',
    rows: [
      {
        name: 'AI 题目解析',
        desc: '大模型逐题拆解思路与考点',
        free: aiTrialText.value,
        vip: '无限',
      },
      { name: 'AI 智能助手', desc: '随时提问，多轮对话答疑', free: aiTrialText.value, vip: '无限' },
      {
        name: 'AI 生成练习题',
        desc: '按知识点定制新题，举一反三',
        free: aiTrialText.value,
        vip: '无限',
      },
      {
        name: 'AI 论文生成 · 批改',
        desc: '生成范文并智能评分给出改进建议',
        free: aiTrialText.value,
        vip: '无限',
      },
      {
        name: '自定义 AI 配置',
        desc: '接入自己的模型与 API Key，不占用平台额度',
        free: false,
        vip: true,
      },
    ],
  },
  {
    title: '学习管理',
    rows: [
      { name: '错题本', desc: '自动收集错题，支持重练', free: true, vip: true },
      { name: '收藏夹', desc: '收藏重点题目随时回看', free: true, vip: true },
      { name: '我的笔记', desc: '为题目记录个人笔记', free: true, vip: true },
      { name: '学习数据报告', desc: '趋势图与知识点掌握度分析', free: true, vip: true },
    ],
  },
])

const aiTrialHint = computed(() => {
  if (aiTrialLimit.value === null) return 'AI 功能对非会员提供累计免费体验，会员无限使用。'
  return aiTrialLimit.value > 0
    ? `AI 功能对非会员提供 ${aiTrialLimit.value} 次累计免费体验（各功能共用），用完后开通会员即可无限使用。`
    : 'AI 功能目前为会员专属。'
})

async function loadPlans() {
  loading.value = true
  try {
    const [planRes, chRes] = await Promise.all([membershipApi.plans(), membershipApi.channels()])
    plans.value = (planRes as any).data || []
    // QQ 钱包不对外开放，后端仍下发该通道，这里只在展示层剔除
    channels.value = ((chRes as any).data || []).filter((c: PayChannel) => c.type !== 'yt_qqpay')
    if (plans.value.length && !selectedPlan.value) {
      // 默认选中「考季会员」，否则第一个
      selectedPlan.value = plans.value.find((p) => p.key === 'exam')?.key || plans.value[0].key
    }
    if (channels.value.length && !payType.value) {
      payType.value = channels.value[0].type
    }
  } finally {
    loading.value = false
  }
}

async function pay() {
  if (!selectedPlan.value) {
    ElMessage.warning('请先选择套餐')
    return
  }
  if (!payType.value) {
    ElMessage.warning('请选择支付方式')
    return
  }
  paying.value = true
  try {
    const res: any = await membershipApi.createOrder({
      plan: selectedPlan.value,
      type: payType.value,
    })
    localStorage.setItem(PENDING_KEY, res.data.out_trade_no)
    // 跳转到支付网关
    window.location.href = res.data.pay_url
  } catch {
    paying.value = false
  }
}

// 卡密兑换
async function redeem() {
  const code = cardCode.value.trim()
  if (!code) {
    ElMessage.warning('请输入卡密')
    return
  }
  redeeming.value = true
  try {
    const res: any = await membershipApi.redeem({ code })
    await userStore.fetchMembership()
    cardCode.value = ''
    ElMessage.success(`兑换成功，${res.data.plan} 已到账（+${res.data.days} 天）！`)
  } finally {
    redeeming.value = false
  }
}

// 支付完成后同步跳转回来（return_url 带 ?paid=1），轮询订单状态
async function pollPaidOrder() {  const outTradeNo = localStorage.getItem(PENDING_KEY)
  if (!outTradeNo) return
  for (let i = 0; i < 6; i++) {
    try {
      const res: any = await membershipApi.queryOrder(outTradeNo)
      if (res.data.paid) {
        localStorage.removeItem(PENDING_KEY)
        await userStore.fetchMembership()
        ElMessage.success('开通成功，会员权益已到账！')
        return
      }
    } catch {
      break
    }
    await new Promise((r) => setTimeout(r, 1500))
  }
  ElMessage.info('若已完成支付，会员可能稍有延迟，请稍后刷新页面')
}

onMounted(async () => {
  await loadPlans()
  await userStore.fetchMembership()
  // 试用额度是后台可调的，写死数字会和实际不符
  // 后端只对非会员填充 trial_limit，会员恒返回 0，此时保持 null 走通用文案
  if (!membership.value?.is_member) {
    try {
      const res: any = await aiApi.status()
      const limit = res.data?.trial_limit
      if (typeof limit === 'number') aiTrialLimit.value = limit
    } catch {
      aiTrialLimit.value = null
    }
  }
  if (route.query.paid === '1') {
    pollPaidOrder()
  }
})
</script>

<template>
  <div class="mx-auto max-w-4xl space-y-6 px-4 py-8">
    <!-- 头图：会员态与非会员态共用，只换文案与配色。
         会员态的底色是浅金，白字（尤其 text-white/85）在黄底上几乎不可见，
         这里改用深琥珀色文字，保证「剩余 N 天」这类关键信息看得清。 -->
    <section
      class="relative overflow-hidden rounded-2xl p-6 shadow-sm"
      :class="[
        membership?.is_member ? 'text-amber-950' : 'text-white',
        membership?.is_member
          ? expiringSoon
            ? 'bg-gradient-to-br from-amber-500 to-orange-500'
            : 'bg-gradient-to-br from-amber-400 to-yellow-500'
          : 'bg-gradient-to-br from-violet-600 to-indigo-500',
      ]"
    >
      <div class="relative z-10">
        <template v-if="membership?.is_member">
          <div class="flex items-center gap-2 text-lg font-semibold">
            <el-icon><GoldMedal /></el-icon>
            {{ expiringSoon ? '会员即将到期' : '您是尊贵的会员' }}
          </div>
          <p class="mt-1.5 text-sm text-amber-950">
            <template v-if="expiringSoon">
              还有 <b class="text-base">{{ membership.days_left }}</b> 天到期（{{
                new Date(membership.expire_at as string).toLocaleDateString()
              }}），现在续费时长叠加，权益不断档
            </template>
            <template v-else>
              到期时间 {{ new Date(membership.expire_at as string).toLocaleDateString() }} · 剩余
              {{ membership.days_left }} 天，续费可叠加时长
            </template>
          </p>
        </template>
        <template v-else>
          <div class="text-lg font-semibold">开通会员</div>
          <p class="mt-1.5 text-sm text-violet-100">
            解锁 Anki 遗忘曲线复习、无限次 AI 解析与答疑、自定义 AI 配置
          </p>
        </template>
      </div>
      <!-- 装饰图标：浅金底上白图标等于隐形，会员态换成低透明度深琥珀 -->
      <el-icon
        :class="membership?.is_member ? 'text-amber-900/15' : 'text-white/10'"
        class="pointer-events-none absolute -right-4 -bottom-5"
        :size="140"
      >
        <GoldMedal />
      </el-icon>
    </section>

    <!-- 套餐选择 -->
    <section
      class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
    >
      <div class="mb-4 flex items-baseline gap-2">
        <span
          class="flex h-5 w-5 items-center justify-center rounded-full bg-violet-600 text-xs font-bold text-white"
          >1</span
        >
        <h2 class="text-sm font-semibold text-slate-900 dark:text-white">选择套餐</h2>
      </div>

      <div v-loading="loading" class="grid grid-cols-2 gap-3 md:grid-cols-4">
        <button
          v-for="p in plans"
          :key="p.key"
          type="button"
          class="group relative rounded-xl border-2 p-4 text-center transition hover:-translate-y-0.5"
          :class="
            selectedPlan === p.key
              ? 'border-violet-500 bg-violet-50 shadow-md dark:border-violet-400 dark:bg-violet-500/10'
              : 'border-slate-200 bg-white hover:border-violet-300 dark:border-slate-700 dark:bg-slate-900 dark:hover:border-violet-500'
          "
          @click="selectedPlan = p.key"
        >
          <!-- 选中态角标，替代额外文字说明 -->
          <span
            v-if="selectedPlan === p.key"
            class="absolute -right-px -top-px flex h-6 w-6 items-center justify-center rounded-bl-lg rounded-tr-[10px] bg-violet-500 text-white"
          >
            <el-icon :size="13"><Select /></el-icon>
          </span>
          <span
            v-else-if="p.key === bestKey"
            class="absolute -right-px -top-px rounded-bl-lg rounded-tr-[10px] bg-amber-400 px-2 py-0.5 text-[10px] font-bold text-white"
          >
            最划算
          </span>

          <div class="text-sm font-medium text-slate-700 dark:text-slate-300">{{ p.name }}</div>
          <div class="mt-1.5 font-bold text-slate-900 dark:text-white">
            <template v-if="payingWithCredit">
              <span class="text-2xl">{{ yuan(amountOf(p)) }}</span
              ><span class="ml-1 text-xs">积分</span>
            </template>
            <template v-else>
              <span class="text-sm">¥</span><span class="text-2xl">{{ yuan(amountOf(p)) }}</span>
            </template>
          </div>
          <div class="mt-1 text-xs text-slate-400">{{ p.days }} 天 · {{ perDayText(p) }}</div>
          <div v-if="showAltPrice(p)" class="mt-0.5 text-[11px] text-slate-400">
            或 {{ altPriceText(p) }}
          </div>
          <div
            v-if="savePercent(p) > 0"
            class="mt-1.5 inline-block rounded bg-red-50 px-1.5 py-0.5 text-[11px] font-medium text-red-500 dark:bg-red-500/10 dark:text-red-400"
          >
            省 {{ savePercent(p) }}%
          </div>
        </button>
      </div>
    </section>

    <!-- 支付方式 -->
    <section
      v-if="channels.length"
      class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
    >
      <div class="mb-4 flex items-baseline gap-2">
        <span
          class="flex h-5 w-5 items-center justify-center rounded-full bg-violet-600 text-xs font-bold text-white"
          >2</span
        >
        <h2 class="text-sm font-semibold text-slate-900 dark:text-white">支付方式</h2>
      </div>

      <div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
        <button
          v-for="ch in channels"
          :key="ch.type"
          type="button"
          class="flex items-center justify-center gap-2 rounded-xl border-2 px-3 py-3 text-sm font-medium transition"
          :class="
            payType === ch.type
              ? 'border-violet-500 bg-violet-50 text-violet-700 dark:border-violet-400 dark:bg-violet-500/10 dark:text-violet-300'
              : 'border-slate-200 bg-white text-slate-600 hover:border-violet-300 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300'
          "
          @click="payType = ch.type"
        >
          <el-icon :size="17" :class="payType === ch.type ? '' : toneOf(ch.type)">
            <Coin v-if="ch.credit" />
            <Wallet v-else />
          </el-icon>
          {{ ch.name }}
        </button>
      </div>

      <p v-if="payingWithCredit" class="mt-3 text-xs text-slate-400">
        该方式以积分结算，将跳转至对应平台完成支付，需登录该平台账号确认。
      </p>

      <!-- 结算条：金额与操作收在一起，减少视线跳动 -->
      <div
        class="mt-5 flex flex-wrap items-center justify-between gap-4 border-t border-slate-100 pt-5 dark:border-slate-800"
      >
        <div class="min-w-0">
          <div class="text-xs text-slate-400">
            {{ selected?.name }}{{ selected ? ` · ${selected.days} 天` : '' }}
          </div>
          <div class="mt-0.5 text-2xl font-bold text-slate-900 dark:text-white">
            {{ payableText || '—' }}
          </div>
        </div>
        <el-button
          type="primary"
          size="large"
          :loading="paying"
          :disabled="!selectedPlan || !payType"
          @click="pay"
        >
          立即支付
        </el-button>
      </div>
      <p class="mt-3 text-xs text-slate-400">
        点击支付即表示同意会员服务协议。虚拟商品，开通后不支持退款。
      </p>
    </section>

    <!-- 卡密兑换：折叠起来，不与主付费路径抢注意力 -->
    <section
      class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900"
    >
      <details class="group">
        <summary
          class="flex cursor-pointer list-none items-center justify-between px-5 py-4 text-sm font-medium text-slate-700 transition hover:bg-slate-50 dark:text-slate-300 dark:hover:bg-slate-800/50"
        >
          <span class="flex items-center gap-2">
            <el-icon class="text-slate-400"><Ticket /></el-icon>
            已有卡密？点此兑换
          </span>
          <el-icon class="text-slate-400 transition group-open:rotate-180"><ArrowDown /></el-icon>
        </summary>
        <div class="border-t border-slate-100 px-5 py-4 dark:border-slate-800">
          <p class="mb-3 text-xs text-slate-400">
            在闲鱼等平台购买卡密后，在此输入即可直接开通对应套餐（可与已有会员时长叠加）。
          </p>
          <div class="flex max-w-lg items-center gap-3">
            <el-input
              v-model="cardCode"
              placeholder="请输入卡密，如 XXXX-XXXX-XXXX-XXXX"
              clearable
              @keyup.enter="redeem"
            />
            <el-button type="success" :loading="redeeming" @click="redeem">立即兑换</el-button>
          </div>
        </div>
      </details>
    </section>

    <div class="text-sm">
      <router-link
        to="/membership/orders"
        class="inline-flex items-center gap-1 text-slate-500 transition hover:text-violet-600 dark:text-slate-400"
      >
        查看我的订单
        <el-icon :size="13"><ArrowRight /></el-icon>
      </router-link>
    </div>

    <!-- 会员权益对比 -->
    <section class="pt-4">
      <h2 class="mb-1 text-sm font-semibold text-slate-900 dark:text-white">权益对比</h2>
      <p class="mb-3 text-xs text-slate-400">{{ aiTrialHint }}</p>

      <div
        class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900"
      >
        <table class="w-full table-fixed text-sm">
          <colgroup>
            <col />
            <col class="w-20 md:w-28" />
            <col class="w-20 md:w-28" />
          </colgroup>
          <thead>
            <tr class="bg-slate-50 text-slate-500 dark:bg-slate-800/60 dark:text-slate-400">
              <th class="px-4 py-3 text-left text-xs font-medium">功能</th>
              <th class="px-2 py-3 text-center text-xs font-medium">免费用户</th>
              <th
                class="border-b-2 border-amber-400 bg-amber-50/70 px-2 py-3 text-center text-xs font-semibold dark:bg-amber-500/10"
              >
                <span class="inline-flex items-center gap-1 text-amber-600 dark:text-amber-500">
                  <el-icon><GoldMedal /></el-icon>会员
                </span>
              </th>
            </tr>
          </thead>
          <tbody v-for="g in featureGroups" :key="g.title">
            <tr>
              <td
                colspan="3"
                class="border-y border-slate-100 bg-slate-50/80 px-4 py-2 text-xs font-semibold tracking-wide text-slate-500 dark:border-slate-800 dark:bg-slate-800/40 dark:text-slate-400"
              >
                {{ g.title }}
              </td>
            </tr>
            <tr
              v-for="row in g.rows"
              :key="row.name"
              class="border-t border-slate-100 transition-colors hover:bg-violet-50/40 dark:border-slate-800/60 dark:hover:bg-slate-800/40"
            >
              <td class="px-4 py-3">
                <div class="font-medium text-slate-800 dark:text-slate-200">{{ row.name }}</div>
                <div class="mt-0.5 text-xs leading-relaxed text-slate-400">{{ row.desc }}</div>
              </td>
              <td class="px-2 py-3 text-center align-middle">
                <el-icon v-if="row.free === true" class="text-green-500" :size="18">
                  <Select />
                </el-icon>
                <el-icon
                  v-else-if="row.free === false"
                  class="text-slate-300 dark:text-slate-600"
                  :size="18"
                >
                  <CloseBold />
                </el-icon>
                <span v-else class="text-xs text-slate-500 dark:text-slate-400">{{ row.free }}</span>
              </td>
              <td
                class="bg-amber-50/40 px-2 py-3 text-center align-middle dark:bg-amber-500/[0.06]"
              >
                <el-icon v-if="row.vip === true" class="text-green-500" :size="18">
                  <Select />
                </el-icon>
                <el-icon
                  v-else-if="row.vip === false"
                  class="text-slate-300 dark:text-slate-600"
                  :size="18"
                >
                  <CloseBold />
                </el-icon>
                <span v-else class="text-xs font-semibold text-amber-600 dark:text-amber-500">
                  {{ row.vip }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
