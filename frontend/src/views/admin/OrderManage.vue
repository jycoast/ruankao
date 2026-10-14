<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { membershipApi } from '@/api'
import type { AdminOrder } from '@/types'

const list = ref<AdminOrder[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const loading = ref(false)

const statusFilter = ref(-1) // -1=全部 0=待支付 1=已支付
const planFilter = ref('')
const keyword = ref('')

const PLAN_OPTIONS = [
  { value: '', label: '全部套餐' },
  { value: 'month', label: '月度会员' },
  { value: 'season', label: '季度会员' },
  { value: 'exam', label: '考季会员' },
  { value: 'year', label: '年度会员' },
  { value: 'manual', label: '管理员开通' },
]

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
  ldc: 'LINUX DO 积分',
}

const CREDIT_CHANNELS = ['ldc']

const planName = (k: string) => PLAN_OPTIONS.find((p) => p.value === k)?.label || k || '-'
const statusTag = (s: number) => STATUS_MAP[s] || { label: '未知', type: 'info' }
const channelName = (ch?: string) => (ch ? CHANNEL_NAMES[ch] || ch : '-')
const yuan = (fen: number) => (fen / 100).toFixed(2)
const amountText = (o: AdminOrder) =>
  CREDIT_CHANNELS.includes(o.channel) ? `${yuan(o.amount)} 积分` : `¥${yuan(o.amount)}`
const fmtTime = (t?: string | null) => (t ? new Date(t).toLocaleString() : '-')

async function load() {
  loading.value = true
  try {
    const res: any = await membershipApi.adminListOrders({
      page: page.value,
      page_size: pageSize,
      status: statusFilter.value,
      plan: planFilter.value,
      keyword: keyword.value.trim(),
    })
    list.value = res.data.items || []
    total.value = res.data.total || 0
  } finally {
    loading.value = false
  }
}

onMounted(load)

function changeStatus(val: number) {
  statusFilter.value = val
  page.value = 1
  load()
}

function search() {
  page.value = 1
  load()
}

function resetFilters() {
  statusFilter.value = -1
  planFilter.value = ''
  keyword.value = ''
  page.value = 1
  load()
}
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center gap-3">
      <h2 class="text-xl font-bold">订单管理</h2>
      <el-radio-group :model-value="statusFilter" @change="changeStatus">
        <el-radio-button :value="-1">全部</el-radio-button>
        <el-radio-button :value="0">待支付</el-radio-button>
        <el-radio-button :value="1">已支付</el-radio-button>
      </el-radio-group>
      <el-select v-model="planFilter" placeholder="套餐" style="width: 140px" @change="search">
        <el-option v-for="p in PLAN_OPTIONS" :key="p.value" :label="p.label" :value="p.value" />
      </el-select>
      <el-input
        v-model="keyword"
        placeholder="订单号 / 交易号 / 用户名"
        clearable
        style="width: 240px"
        @keyup.enter="search"
        @clear="search"
      />
      <el-button type="primary" @click="search">搜索</el-button>
      <el-button @click="resetFilters">重置</el-button>
    </div>

    <el-table v-loading="loading" :data="list" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="商户订单号" min-width="200" show-overflow-tooltip>
        <template #default="{ row }: { row: any }">
          <span class="font-mono text-xs">{{ row.out_trade_no }}</span>
        </template>
      </el-table-column>
      <el-table-column label="用户" width="120">
        <template #default="{ row }: { row: any }">
          <span class="text-sm">{{ row.username || row.user_id }}</span>
        </template>
      </el-table-column>
      <el-table-column label="套餐" width="110">
        <template #default="{ row }: { row: any }">{{ planName(row.plan) }}</template>
      </el-table-column>
      <el-table-column label="下单通道" width="110">
        <template #default="{ row }: { row: any }">{{ channelName(row.channel) }}</template>
      </el-table-column>
      <el-table-column label="金额" width="110" align="right">
        <template #default="{ row }: { row: any }">
          <span class="font-medium text-red-500">{{ amountText(row) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }: { row: any }">
          <el-tag :type="statusTag(row.status).type as any" size="small">
            {{ statusTag(row.status).label }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="支付方式" width="110">
        <template #default="{ row }: { row: any }">{{ channelName(row.pay_channel) }}</template>
      </el-table-column>
      <el-table-column label="交易号" min-width="160" show-overflow-tooltip>
        <template #default="{ row }: { row: any }">
          <span v-if="row.trade_no" class="font-mono text-xs">{{ row.trade_no }}</span>
          <span v-else class="text-gray-400">-</span>
        </template>
      </el-table-column>
      <el-table-column label="下单时间" width="170">
        <template #default="{ row }: { row: any }">
          <span class="text-sm text-gray-500">{{ fmtTime(row.created_at) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="支付时间" width="170">
        <template #default="{ row }: { row: any }">
          <span class="text-sm text-gray-500">{{ fmtTime(row.paid_at) }}</span>
        </template>
      </el-table-column>
    </el-table>

    <div class="mt-4 flex justify-center">
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next, total"
        @current-change="load"
      />
    </div>
  </div>
</template>