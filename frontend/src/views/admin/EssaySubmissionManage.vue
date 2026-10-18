<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { essaySubmissionApi } from '@/api'
import type { EssaySubmission } from '@/types'

const list = ref<EssaySubmission[]>([])
const total = ref(0)
const page = ref(1)
const statusFilter = ref(-1)
const loading = ref(false)

const detailVisible = ref(false)
const reviewVisible = ref(false)
const current = ref<EssaySubmission & { content?: string } | null>(null)
const reviewForm = ref({ action: 'approve' as 'approve' | 'reject', note: '', article_type: 1 })
const reviewing = ref(false)

async function load() {
  loading.value = true
  try {
    const res: any = await essaySubmissionApi.adminList({ page: page.value, page_size: 20, status: statusFilter.value })
    list.value = res.data.items || []
    total.value = res.data.total || 0
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function openDetail(row: EssaySubmission) {
  const res: any = await essaySubmissionApi.adminGetOne(row.id)
  current.value = res.data
  detailVisible.value = true
}

function openReview(row: EssaySubmission, action: 'approve' | 'reject') {
  current.value = row
  reviewForm.value = { action, note: '', article_type: 1 }
  reviewVisible.value = true
}

async function submitReview() {
  if (!current.value) return
  reviewing.value = true
  try {
    await essaySubmissionApi.adminReview(current.value.id, {
      action: reviewForm.value.action,
      note: reviewForm.value.note,
      article_type: reviewForm.value.article_type,
    })
    ElMessage.success(reviewForm.value.action === 'approve' ? '已通过，论文已发布到范文库' : '已拒绝')
    reviewVisible.value = false
    load()
  } finally {
    reviewing.value = false
  }
}

function statusTag(s: number) {
  return { 0: { label: '待审核', type: 'warning' }, 1: { label: '已通过', type: 'success' }, 2: { label: '已拒绝', type: 'danger' } }[s] || { label: '未知', type: '' }
}
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-semibold">论文投稿审核</h2>
    </div>

    <el-card>
      <div class="mb-4 flex gap-2">
        <el-radio-group v-model="statusFilter" @change="() => { page = 1; load() }">
          <el-radio-button :label="-1">全部</el-radio-button>
          <el-radio-button :label="0">待审核</el-radio-button>
          <el-radio-button :label="1">已通过</el-radio-button>
          <el-radio-button :label="2">已拒绝</el-radio-button>
        </el-radio-group>
      </div>

      <el-table :data="list" v-loading="loading" stripe>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="title" label="标题" min-width="200" show-overflow-tooltip />
        <el-table-column prop="username" label="投稿人" width="120" show-overflow-tooltip />
        <el-table-column prop="category" label="分类" width="120" show-overflow-tooltip />
        <el-table-column prop="year" label="年份" width="80" />
        <el-table-column prop="score" label="得分" width="80" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }: { row: any }">
            <el-tag :type="(statusTag(row.review_status) as any).type" size="small">
              {{ statusTag(row.review_status).label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="投稿时间" width="120">
          <template #default="{ row }: { row: any }">{{ row.created_at?.slice(0, 10) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }: { row: any }">
            <el-button size="small" @click="openDetail(row)">查看</el-button>
            <template v-if="row.review_status === 0">
              <el-button size="small" type="success" @click="openReview(row, 'approve')">通过</el-button>
              <el-button size="small" type="danger" @click="openReview(row, 'reject')">拒绝</el-button>
            </template>
          </template>
        </el-table-column>
      </el-table>

      <div class="mt-4 flex justify-center">
        <el-pagination
          v-model:current-page="page"
          :page-size="20"
          :total="total"
          layout="prev, pager, next"
          @current-change="load"
        />
      </div>
    </el-card>

    <!-- 论文内容详情弹窗 -->
    <el-dialog v-model="detailVisible" title="论文详情" width="800px" draggable>
      <template v-if="current">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="标题" :span="2">{{ current.title }}</el-descriptions-item>
          <el-descriptions-item label="投稿人">{{ current.username }}</el-descriptions-item>
          <el-descriptions-item label="投稿时间">{{ current.created_at?.slice(0, 10) }}</el-descriptions-item>
          <el-descriptions-item label="分类">{{ current.category || '—' }}</el-descriptions-item>
          <el-descriptions-item label="年份">{{ current.year || '—' }}</el-descriptions-item>
          <el-descriptions-item label="得分">{{ current.score || '—' }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="(statusTag(current.review_status) as any).type" size="small">
              {{ statusTag(current.review_status).label }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item v-if="current.reviewer_note" label="审核意见" :span="2">
            {{ current.reviewer_note }}
          </el-descriptions-item>
        </el-descriptions>
        <div class="mt-4">
          <div class="text-sm text-gray-500 mb-2 font-medium">论文正文</div>
          <div class="bg-gray-50 dark:bg-gray-800 p-4 rounded text-sm whitespace-pre-wrap max-h-96 overflow-y-auto">
            {{ current.content }}
          </div>
        </div>
      </template>
      <template #footer>
        <div class="flex gap-2 justify-end">
          <template v-if="current?.review_status === 0">
            <el-button type="success" @click="() => { detailVisible = false; openReview(current!, 'approve') }">通过</el-button>
            <el-button type="danger" @click="() => { detailVisible = false; openReview(current!, 'reject') }">拒绝</el-button>
          </template>
          <el-button @click="detailVisible = false">关闭</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 审核操作弹窗 -->
    <el-dialog
      v-model="reviewVisible"
      :title="reviewForm.action === 'approve' ? '通过投稿' : '拒绝投稿'"
      width="480px"
    >
      <el-form label-width="90px">
        <template v-if="reviewForm.action === 'approve'">
          <el-form-item label="发布类型">
            <el-select v-model="reviewForm.article_type">
              <el-option :value="1" label="论文范文" />
              <el-option :value="2" label="写作模板" />
              <el-option :value="3" label="评分标准" />
            </el-select>
          </el-form-item>
        </template>
        <el-form-item label="审核意见">
          <el-input v-model="reviewForm.note" type="textarea" :rows="3" placeholder="可选，会展示给投稿人" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="reviewVisible = false">取消</el-button>
        <el-button
          :type="reviewForm.action === 'approve' ? 'success' : 'danger'"
          :loading="reviewing"
          @click="submitReview"
        >
          确认{{ reviewForm.action === 'approve' ? '通过' : '拒绝' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>
