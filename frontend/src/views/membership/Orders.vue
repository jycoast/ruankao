<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { membershipApi } from '@/api'

interface Order {
  id: number
  out_trade_no: string
  plan: string
  channel: string // 下单通道：yipay / ldc / yuetongyun
  amount: number // 分（积分通道为 0.01 积分）
  status: number // 0待支付 1已支付 2已退款 3已关闭
  pay_channel: string
  trade_no: string
  paid_at: string | null
  created_at: string
}

const router = useRouter()
const list = ref<Order[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)

const PLAN_NAMES: Record<string, string> = {
  month: '月度会员',
  season: '季度会员',
  exam: '考季会员',
  year: '年度会员',
  manual: '管理员开通',
}
const STATUS_MAP: Record<number, { label: string; type: string }> = {
  0: { label: '待支付', type: 'warning' },
  1: { label: '已支付', type: 'success' },
  2: { label: '已退款', type: 'info' },
  3: { label: '已关闭', type: 'info' },
}
const CHANNEL_NAMES: Record<string, string> = {
  alipay: '支付宝',
  wxpay: '微信支付',
  qqpay: 'QQ钱包',
  epay: 'LINUX DO 积分',
}
// 积分类通道的 amount 单位是积分而非元，标 ¥ 会误导
const CREDIT_CHANNELS = ['ldc']

const planName = (k: string) => PLAN_NAMES[k] || k
const statusTag = (s: number) => STATUS_MAP[s] || { label: '未知', type: 'info' }
const yuan = (fen: number) => (fen / 100).toFixed(2)
const amountText = (o: Order) =>
  CREDIT_CHANNELS.includes(o.channel) ? `${yuan(o.amount)} 积分` : `¥${yuan(o.amount)}`
const fmtTime = (t?: string | null) => (t ? new Date(t).toLocaleString() : '—')

async function load() {
  loading.value = true
  try {
    const res: any = await membershipApi.listOrders({ page: page.value, page_size: 20 })
    list.value = res.data.items || []
    total.value = res.data.total || 0
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-3xl px-4 py-8">
    <div class="mb-6 flex items-center justify-between">
      <div class="flex items-center gap-3">
        <el-button text @click="router.back()">返回</el-button>
        <h2 class="text-xl font-bold text-gray-900 dark:text-gray-100">我的订单</h2>
      </div>
      <el-button type="primary" @click="router.push('/membership')">开通 / 续费</el-button>
    </div>

    <el-card v-loading="loading">
      <el-empty v-if="!list.length && !loading" description="还没有订单记录" />

      <div v-else class="space-y-3">
        <div
          v-for="o in list"
          :key="o.id"
          class="rounded-lg border p-4 dark:border-gray-700"
        >
          <div class="flex items-start justify-between gap-2">
            <div>
              <div class="font-medium text-gray-800 dark:text-gray-200">{{ planName(o.plan) }}</div>
              <div class="mt-0.5 font-mono text-xs text-gray-400">{{ o.out_trade_no }}</div>
            </div>
            <div class="text-right">
              <div class="text-lg font-bold text-red-500">{{ amountText(o) }}</div>
              <el-tag :type="(statusTag(o.status) as any).type" size="small" class="mt-1">
                {{ statusTag(o.status).label }}
              </el-tag>
            </div>
          </div>
          <div class="mt-3 flex flex-wrap gap-x-5 gap-y-1 text-xs text-gray-500">
            <span>下单时间：{{ fmtTime(o.created_at) }}</span>
            <span v-if="o.paid_at">支付时间：{{ fmtTime(o.paid_at) }}</span>
            <span v-if="o.pay_channel">支付方式：{{ CHANNEL_NAMES[o.pay_channel] || o.pay_channel }}</span>
          </div>
        </div>
      </div>

      <div v-if="total > 20" class="mt-4 flex justify-center">
        <el-pagination
          v-model:current-page="page"
          :page-size="20"
          :total="total"
          layout="prev, pager, next"
          @current-change="load"
        />
      </div>
    </el-card>

    <p class="mt-4 text-xs text-gray-400">
      卡密兑换不产生订单记录，仅在线支付的订单会显示在这里。
    </p>
  </div>
</template>
