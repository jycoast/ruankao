<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { commentApi } from '@/api'

interface Row {
  id: number; content: string; nickname: string; question_id: number
  question_snippet: string; status: number; created_at: string
}

const list = ref<Row[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const loading = ref(false)
const statusFilter = ref(0)
const keyword = ref('')

async function load() {
  loading.value = true
  try {
    const res: any = await commentApi.adminList({
      status: statusFilter.value,
      keyword: keyword.value,
      page: page.value,
      page_size: pageSize,
    })
    list.value = res.data.items || []
    total.value = res.data.total || 0
  } catch {} finally {
    loading.value = false
  }
}

async function setStatus(row: Row, status: number) {
  const actionText = status === 2 ? '隐藏' : '恢复'
  try {
    await ElMessageBox.confirm(`确定${actionText}这条评论吗？`, '确认', { type: 'warning' })
  } catch { return }
  try {
    await commentApi.adminSetStatus(row.id, { status })
    ElMessage.success(`已${actionText}`)
    load()
  } catch {}
}

onMounted(load)
</script>

<template>
  <div class="p-4">
    <div class="mb-4 flex flex-wrap items-center gap-2">
      <el-select v-model="statusFilter" style="width: 140px" @change="page = 1; load()">
        <el-option label="正常+隐藏" :value="0" />
        <el-option label="仅隐藏" :value="2" />
        <el-option label="仅用户删除" :value="1" />
      </el-select>
      <el-input v-model="keyword" placeholder="搜索内容/昵称" clearable style="width: 220px" @keyup.enter="page = 1; load()" @clear="page = 1; load()" />
      <el-button type="primary" @click="page = 1; load()">搜索</el-button>
    </div>

    <el-table :data="list" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="nickname" label="用户" width="110" />
      <el-table-column label="题干" min-width="180">
        <template #default="{ row }">
          <span class="text-gray-500">#{{ row.question_id }}</span>
          <div class="truncate text-xs text-gray-400">{{ row.question_snippet || '（空题干）' }}</div>
        </template>
      </el-table-column>
      <el-table-column label="内容" min-width="260">
        <template #default="{ row }">
          <div class="whitespace-pre-wrap break-words">{{ row.content }}</div>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 0 ? 'success' : 'warning'">
            {{ row.status === 0 ? '正常' : row.status === 1 ? '用户删除' : '隐藏' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="时间" width="160">
        <template #default="{ row }">{{ new Date(row.created_at).toLocaleString('zh-CN') }}</template>
      </el-table-column>
      <el-table-column label="操作" width="140">
        <template #default="{ row }: { row: any }">
          <el-button v-if="row.status !== 2" link size="small" type="danger" @click="setStatus(row, 2)">隐藏</el-button>
          <el-button v-else link size="small" type="success" @click="setStatus(row, 0)">恢复</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="mt-3 flex justify-center">
      <el-pagination layout="prev, pager, next" :total="total" :page-size="pageSize" :current-page="page" @current-change="page = $event; load()" />
    </div>
  </div>
</template>
