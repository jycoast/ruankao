<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { cardKeyApi, membershipApi } from '@/api'
import type { CardKey } from '@/types'

interface Plan {
  key: string
  name: string
  days: number
  amount: number
}

const list = ref<CardKey[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const keyword = ref('')
const status = ref<number>(-1) // -1=全部 0=未使用 1=已使用
const plan = ref('')

const plans = ref<Plan[]>([])
const planName = (key: string) => plans.value.find((p) => p.key === key)?.name || key

// 生成弹窗
const genVisible = ref(false)
const genForm = ref({ count: 10, plan: '', note: '' })
const generating = ref(false)
// 最新一批生成的卡密，用于一键复制
const latestCodes = ref<string[]>([])
const latestPlan = ref('')

async function loadPlans() {
  const res: any = await membershipApi.plans()
  plans.value = res.data || []
  if (plans.value.length && !genForm.value.plan) {
    genForm.value.plan = plans.value[0].key
  }
}

async function load() {
  loading.value = true
  try {
    const res: any = await cardKeyApi.list({
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value || undefined,
      plan: plan.value || undefined,
      status: status.value,
    })
    list.value = res.data.items || []
    total.value = res.data.total || 0
    // 当前页越界时自动回到第 1 页，避免停在一页空结果上
    if (!list.value.length && page.value > 1) {
      page.value = 1
      load()
    }
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await loadPlans()
  await load()
})

function search() {
  page.value = 1
  load()
}

function reset() {
  keyword.value = ''
  plan.value = ''
  status.value = -1
  page.value = 1
  load()
}

function onSizeChange(size: number) {
  pageSize.value = size
  page.value = 1
  load()
}

async function handleGenerate() {
  if (!genForm.value.plan) {
    ElMessage.warning('请选择套餐')
    return
  }
  generating.value = true
  try {
    const res: any = await cardKeyApi.generate({
      count: genForm.value.count,
      plan: genForm.value.plan,
      note: genForm.value.note,
    })
    latestCodes.value = res.data.codes || []
    latestPlan.value = genForm.value.plan
    ElMessage.success(`已生成 ${latestCodes.value.length} 张卡密`)
    genVisible.value = false
    page.value = 1
    load()
  } finally {
    generating.value = false
  }
}

async function handleDelete(row: CardKey) {
  await ElMessageBox.confirm(`确认删除卡密 ${row.code}？`, '删除确认', { type: 'warning' })
  await cardKeyApi.delete(row.id)
  ElMessage.success('已删除')
  load()
}

function copyText(text: string) {
  navigator.clipboard.writeText(text).then(() => ElMessage.success('已复制'))
}

// 复制格式「套餐名 卡密」，方便闲鱼发货
function copyLatest() {
  if (!latestCodes.value.length) return
  const name = planName(latestPlan.value)
  copyText(latestCodes.value.map((c) => `${name} ${c}`).join('\n'))
  ElMessage.success(`已复制 ${latestCodes.value.length} 张卡密`)
}

function copyAll() {
  const unused = list.value.filter((c) => c.status === 0)
  if (!unused.length) {
    ElMessage.warning('没有可用的卡密')
    return
  }
  copyText(unused.map((c) => `${planName(c.plan)} ${c.code}`).join('\n'))
  ElMessage.success(`已复制 ${unused.length} 张未使用的卡密`)
}

const unusedCount = computed(() => list.value.filter((c) => c.status === 0).length)
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-semibold">卡密管理</h2>
      <div class="flex gap-2">
        <el-button @click="copyAll">一键复制未使用</el-button>
        <el-button type="primary" @click="genVisible = true">批量生成</el-button>
      </div>
    </div>

    <!-- 最近生成结果 -->
    <el-card v-if="latestCodes.length" class="mb-4">
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
        <span class="font-medium text-gray-700">
          最新生成的卡密 · {{ planName(latestPlan) }}（{{ latestCodes.length }} 张）
        </span>
        <el-button size="small" type="primary" @click="copyLatest">一键复制全部</el-button>
      </div>
      <div class="bg-gray-50 rounded p-3 font-mono text-sm leading-7 flex flex-wrap gap-2">
        <el-tag
          v-for="code in latestCodes"
          :key="code"
          class="cursor-pointer"
          @click="copyText(code)"
        >
          {{ code }}
        </el-tag>
      </div>
      <div class="text-xs text-gray-400 mt-1">点击单张卡密可单独复制；一键复制含「套餐名 卡密」，方便闲鱼发货</div>
    </el-card>

    <el-card>
      <div class="mb-3 text-sm text-gray-500">
        共 {{ total }} 张卡密，当前页未使用 {{ unusedCount }} 张
      </div>

      <div class="mb-4 flex flex-wrap items-center gap-3">
        <el-input
          v-model="keyword"
          placeholder="搜索卡密 / 备注"
          clearable
          style="width: 220px"
          @keyup.enter="search"
          @clear="search"
        />
        <el-select
          v-model="plan"
          placeholder="套餐"
          clearable
          style="width: 160px"
          @change="search"
        >
          <el-option
            v-for="p in plans"
            :key="p.key"
            :label="p.name"
            :value="p.key"
          />
        </el-select>
        <el-select
          v-model="status"
          placeholder="状态"
          style="width: 140px"
          @change="search"
        >
          <el-option label="状态：全部" :value="-1" />
          <el-option label="未使用" :value="0" />
          <el-option label="已使用" :value="1" />
        </el-select>
        <el-button type="primary" @click="search">搜索</el-button>
        <el-button @click="reset">重置</el-button>
      </div>

      <el-table :data="list" v-loading="loading" stripe>
        <el-table-column prop="code" label="卡密" min-width="220">
          <template #default="{ row }: { row: any }">
            <span class="font-mono font-medium">{{ row.code }}</span>
            <el-button
              size="small"
              text
              :icon="'CopyDocument'"
              class="ml-1"
              @click="copyText(row.code)"
            />
          </template>
        </el-table-column>
        <el-table-column label="套餐" width="110">
          <template #default="{ row }: { row: any }">
            <el-tag size="small" type="warning">{{ planName(row.plan) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }: { row: any }">
            <el-tag :type="row.status === 0 ? 'success' : 'info'" size="small">
              {{ row.status === 0 ? '未使用' : '已使用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="note" label="备注" min-width="120" show-overflow-tooltip />
        <el-table-column prop="used_at" label="使用时间" width="120">
          <template #default="{ row }: { row: any }">{{ row.used_at?.slice(0, 10) || '—' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="120">
          <template #default="{ row }: { row: any }">{{ row.created_at?.slice(0, 10) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="80">
          <template #default="{ row }: { row: any }">
            <el-button
              v-if="row.status === 0"
              size="small"
              type="danger"
              text
              @click="handleDelete(row)"
            >删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="mt-4 flex justify-center">
        <el-pagination
          v-model:current-page="page"
          :page-size="pageSize"
          :page-sizes="[20, 50, 100]"
          :total="total"
          layout="total, sizes, prev, pager, next, jumper"
          @current-change="load"
          @size-change="onSizeChange"
        />
      </div>
    </el-card>

    <!-- 批量生成弹窗 -->
    <el-dialog v-model="genVisible" title="批量生成卡密" width="400px">
      <el-form label-width="80px">
        <el-form-item label="套餐">
          <el-select v-model="genForm.plan" placeholder="请选择套餐" class="w-full">
            <el-option
              v-for="p in plans"
              :key="p.key"
              :label="`${p.name}（${p.days} 天）`"
              :value="p.key"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="生成数量">
          <el-input-number v-model="genForm.count" :min="1" :max="100" />
          <span class="ml-2 text-gray-400 text-sm">最多100张</span>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="genForm.note" placeholder="可选，用于标识这批卡密的用途" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="genVisible = false">取消</el-button>
        <el-button type="primary" :loading="generating" @click="handleGenerate">
          生成
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>
