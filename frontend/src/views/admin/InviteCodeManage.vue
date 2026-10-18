<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { inviteCodeApi } from '@/api'
import type { InviteCode } from '@/types'

const list = ref<InviteCode[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)

// 生成弹窗
const genVisible = ref(false)
const genForm = ref({ count: 10, note: '' })
const generating = ref(false)
// 最新一批生成的邀请码，用于一键复制
const latestCodes = ref<string[]>([])

async function load() {
  loading.value = true
  try {
    const res: any = await inviteCodeApi.list({ page: page.value, page_size: 50 })
    list.value = res.data.items || []
    total.value = res.data.total || 0
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function handleGenerate() {
  generating.value = true
  try {
    const res: any = await inviteCodeApi.generate({ count: genForm.value.count, note: genForm.value.note })
    latestCodes.value = res.data.codes || []
    ElMessage.success(`已生成 ${latestCodes.value.length} 个邀请码`)
    genVisible.value = false
    page.value = 1
    load()
  } finally {
    generating.value = false
  }
}

async function handleDelete(row: InviteCode) {
  await ElMessageBox.confirm(`确认删除邀请码 ${row.code}？`, '删除确认', { type: 'warning' })
  await inviteCodeApi.delete(row.id)
  ElMessage.success('已删除')
  load()
}

function copyText(text: string) {
  navigator.clipboard.writeText(text).then(() => ElMessage.success('已复制'))
}

function copyAll() {
  const unused = list.value.filter(c => c.status === 0).map(c => c.code)
  if (!unused.length) { ElMessage.warning('没有可用的邀请码'); return }
  copyText(unused.join('\n'))
  ElMessage.success(`已复制 ${unused.length} 个未使用的邀请码`)
}

function copyLatest() {
  if (!latestCodes.value.length) return
  copyText(latestCodes.value.join('\n'))
  ElMessage.success(`已复制 ${latestCodes.value.length} 个邀请码`)
}

const unusedCount = computed(() => list.value.filter(c => c.status === 0).length)
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-semibold">邀请码管理</h2>
      <div class="flex gap-2">
        <el-button @click="copyAll">一键复制未使用</el-button>
        <el-button type="primary" @click="genVisible = true">批量生成</el-button>
      </div>
    </div>

    <!-- 最近生成结果 -->
    <el-card v-if="latestCodes.length" class="mb-4">
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
        <span class="font-medium text-gray-700">最新生成的邀请码（{{ latestCodes.length }} 个）</span>
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
      <div class="text-xs text-gray-400 mt-1">点击单个邀请码可单独复制</div>
    </el-card>

    <el-card>
      <div class="mb-3 text-sm text-gray-500">
        共 {{ total }} 个邀请码，当前页未使用 {{ unusedCount }} 个
      </div>

      <el-table :data="list" v-loading="loading" stripe>
        <el-table-column prop="code" label="邀请码" min-width="180">
          <template #default="{ row }: { row: any }">
            <div class="flex min-w-0 items-center gap-1">
              <span class="truncate font-mono font-medium" :title="row.code">{{ row.code }}</span>
              <el-button
                size="small"
                text
                :icon="'CopyDocument'"
                class="shrink-0"
                @click="copyText(row.code)"
              />
            </div>
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
          :page-size="50"
          :total="total"
          layout="prev, pager, next"
          @current-change="load"
        />
      </div>
    </el-card>

    <!-- 批量生成弹窗 -->
    <el-dialog v-model="genVisible" title="批量生成邀请码" width="400px">
      <el-form label-width="80px">
        <el-form-item label="生成数量">
          <el-input-number v-model="genForm.count" :min="1" :max="100" />
          <span class="ml-2 text-gray-400 text-sm">最多100个</span>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="genForm.note" placeholder="可选，用于标识这批邀请码的用途" />
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
