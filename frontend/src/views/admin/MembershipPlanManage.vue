<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { membershipApi } from '@/api'

interface PlanRow {
  key: string
  name: string
  days: number
  amount: number // 人民币，分
  ldc_amount: number // LDC 积分，0.01 积分
  sort_order: number
  enabled: number
}

const rows = ref<PlanRow[]>([])
const loading = ref(false)
const savingKey = ref('')

// 表单以「元 / 积分」为单位编辑，落库统一按分。
// 浮点乘 100 会出现 29.99 → 2998.9999，必须四舍五入。
const toFen = (v: number) => Math.round(v * 100)
const toYuan = (fen: number) => fen / 100

const form = ref<Record<string, { name: string; amount: number; ldc: number; sort: number; enabled: number }>>({})

async function load() {
  loading.value = true
  try {
    const res: any = await membershipApi.adminListPlans()
    rows.value = res.data || []
    const next: typeof form.value = {}
    for (const r of rows.value) {
      next[r.key] = {
        name: r.name,
        amount: toYuan(r.amount),
        ldc: toYuan(r.ldc_amount),
        sort: r.sort_order,
        enabled: r.enabled,
      }
    }
    form.value = next
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function save(key: string) {
  const f = form.value[key]
  if (!f) return
  if (!f.name.trim()) {
    ElMessage.warning('套餐名称不能为空')
    return
  }
  savingKey.value = key
  try {
    await membershipApi.adminUpdatePlan(key, {
      name: f.name.trim(),
      amount: toFen(f.amount),
      ldc_amount: toFen(f.ldc),
      sort_order: f.sort,
      enabled: f.enabled,
    })
    ElMessage.success('已保存')
    await load()
  } finally {
    savingKey.value = ''
  }
}

// 活动常用操作：把积分价按人民币价整体打折
const discount = ref(80)
function applyLDCDiscount() {
  for (const r of rows.value) {
    const f = form.value[r.key]
    if (f) f.ldc = Math.round(f.amount * discount.value) / 100
  }
  ElMessage.info('已填入折后积分价，逐行点击「保存」后生效')
}
</script>

<template>
  <div class="max-w-5xl">
    <h2 class="mb-1 text-xl font-bold text-gray-900 dark:text-gray-100">会员套餐与价格</h2>
    <p class="mb-6 text-sm text-slate-500 dark:text-slate-400">
      人民币价与 LINUX DO 积分价互相独立，可只调其中一条做活动（默认 1 积分 = 1 元）。
      改价只影响新订单，已下单的订单按下单时的价格结算。套餐天数与标识不可修改，以免已售卡密对不上。
    </p>

    <el-card class="mb-4">
      <div class="flex flex-wrap items-center gap-3">
        <span class="text-sm text-gray-700 dark:text-gray-300">积分价批量设为人民币价的</span>
        <el-input-number v-model="discount" :min="1" :max="200" :step="5" size="small" />
        <span class="text-sm text-gray-700 dark:text-gray-300">%</span>
        <el-button size="small" @click="applyLDCDiscount">填入</el-button>
        <span class="text-xs text-slate-400">仅填入表单，需逐行保存</span>
      </div>
    </el-card>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column label="标识" prop="key" width="100" />
      <el-table-column label="名称" width="160">
        <template #default="{ row }">
          <el-input v-model="form[row.key].name" size="small" />
        </template>
      </el-table-column>
      <el-table-column label="天数" prop="days" width="80" align="center" />
      <el-table-column label="人民币（元）" width="160">
        <template #default="{ row }">
          <el-input-number v-model="form[row.key].amount" :min="0" :precision="2" :step="1" size="small" controls-position="right" />
        </template>
      </el-table-column>
      <el-table-column label="LDC 积分" width="160">
        <template #default="{ row }">
          <el-input-number v-model="form[row.key].ldc" :min="0" :precision="2" :step="1" size="small" controls-position="right" />
        </template>
      </el-table-column>
      <el-table-column label="排序" width="110">
        <template #default="{ row }">
          <el-input-number v-model="form[row.key].sort" :min="0" size="small" controls-position="right" />
        </template>
      </el-table-column>
      <el-table-column label="上架" width="90" align="center">
        <template #default="{ row }">
          <el-switch v-model="form[row.key].enabled" :active-value="1" :inactive-value="0" />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="90" align="center">
        <template #default="{ row }">
          <el-button type="primary" size="small" :loading="savingKey === row.key" @click="save(row.key)">
            保存
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>
