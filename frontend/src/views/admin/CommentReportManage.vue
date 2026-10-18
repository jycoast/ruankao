<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { commentApi } from '@/api'

interface Row {
  id: number; comment_content: string; reason: string; author_nickname: string
  reporter_nickname: string; admin_note: string; status: number; created_at: string
}

const list = ref<Row[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const loading = ref(false)
const statusFilter = ref(0)

async function load() {
  loading.value = true
  try {
    const res: any = await commentApi.adminReportList({ status: statusFilter.value, page: page.value, page_size: pageSize })
    list.value = res.data.items || []
    total.value = res.data.total || 0
  } catch {} finally {
    loading.value = false
  }
}

async function handle(id: number, action: 'delete' | 'dismiss') {
  const tip = action === 'delete'
    ? '删除后该评论将被隐藏，且作者与举报人都会收到通知。确定？'
    : '驳回该举报，不做处理？'
  try {
    await ElMessageBox.confirm(tip, action === 'delete' ? '删除评论' : '驳回举报', { type: 'warning' })
  } catch { return }
  try {
    await commentApi.adminHandleReport(id, { action })
    ElMessage.success('已处理')
    load()
  } catch {}
}

onMounted(load)
</script>

<template>
  <div class="p-4">
    <div class="mb-4">
      <el-select v-model="statusFilter" style="width: 140px" @change="page = 1; load()">
        <el-option label="待处理" :value="0" />
        <el-option label="已处理" :value="1" />
        <el-option label="已驳回" :value="2" />
      </el-select>
    </div>

    <el-table :data="list" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column label="评论内容" min-width="260" show-overflow-tooltip>
        <template #default="{ row }">
          <span class="text-sm">{{ row.comment_content }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="reason" label="举报理由" width="120" show-overflow-tooltip />
      <el-table-column prop="author_nickname" label="评论作者" width="90" show-overflow-tooltip />
      <el-table-column prop="reporter_nickname" label="举报人" width="90" show-overflow-tooltip />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status === 0 ? 'danger' : 'info'">
            {{ row.status === 0 ? '待处理' : row.status === 1 ? '已处理' : '已驳回' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="时间" width="150">
        <template #default="{ row }">{{ new Date(row.created_at).toLocaleString('zh-CN') }}</template>
      </el-table-column>
      <el-table-column label="操作" width="150">
        <template #default="{ row }">
          <template v-if="row.status === 0">
            <el-button link size="small" type="danger" @click="handle(row.id, 'delete')">删除</el-button>
            <el-button link size="small" type="info" @click="handle(row.id, 'dismiss')">驳回</el-button>
          </template>
          <div v-else class="truncate text-xs text-gray-400" :title="row.admin_note || '—'">{{ row.admin_note || '—' }}</div>
        </template>
      </el-table-column>
    </el-table>

    <div class="mt-3 flex justify-center">
      <el-pagination layout="prev, pager, next" :total="total" :page-size="pageSize" :current-page="page" @current-change="page = $event; load()" />
    </div>
  </div>
</template>
