<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { reportApi, questionApi } from '@/api'
import { stripMd } from '@/utils/markdown'
import type { QuestionReport, Question } from '@/types'

const list = ref<QuestionReport[]>([])
const total = ref(0)
const page = ref(1)
const statusFilter = ref(-1) // -1=全部

const detailVisible = ref(false)
const current = ref<QuestionReport | null>(null)
const adminNote = ref('')
const saving = ref(false)

// 「编辑题目」弹窗：从报告定位到题，直接改题后回填
const editVisible = ref(false)
const editQuestion = ref<Question | null>(null)
const editLoadingId = ref<number | null>(null)

const REPORT_TYPE_LABELS: Record<number, string> = {
  1: '题目表述有误',
  2: '答案有误',
  3: '解析/分析有误',
  4: '选项有误',
  5: '其他问题',
}

const QUESTION_TYPE_LABELS: Record<number, string> = { 1: '单选', 2: '多选', 3: '案例', 4: '论文' }

const STATUS_LABELS: Record<number, { text: string; type: string }> = {
  0: { text: '待处理', type: 'danger' },
  1: { text: '已处理', type: 'success' },
  2: { text: '已忽略', type: 'info' },
}

async function load() {
  const res: any = await reportApi.adminList({
    page: page.value,
    page_size: 20,
    status: statusFilter.value,
  })
  list.value = res.data.items || []
  total.value = res.data.total || 0
}

onMounted(load)

function changeStatus(val: number) {
  statusFilter.value = val
  page.value = 1
  load()
}

function openDetail(row: QuestionReport) {
  current.value = row
  adminNote.value = row.admin_note || ''
  detailVisible.value = true
}

async function handleUpdate(status: 1 | 2) {
  if (!current.value) return
  saving.value = true
  try {
    await reportApi.updateStatus(current.value.id, { status, admin_note: adminNote.value })
    ElMessage.success(status === 1 ? '已标记为处理完成' : '已忽略该报告')
    detailVisible.value = false
    load()
  } finally {
    saving.value = false
  }
}

function truncate(text: string, len = 60) {
  return text.length > len ? text.slice(0, len) + '…' : text
}

// 题干摘要是 markdown，去格式后展示纯文本，管理员一眼认出是第几题
function snippetOf(r: { question_content?: string; question_preview?: string }): string {
  const t = stripMd(r.question_content || r.question_preview || '')
  if (!t) return '（无题干）'
  return t.length > 80 ? t.slice(0, 80) + '…' : t
}

// 从报告拉完整题目（含选项）→ 交给公共编辑弹窗；题目可能已被删除，失败时提示。
async function openEditQuestion(row?: QuestionReport | null) {
  const r = row || current.value
  if (!r) return
  editLoadingId.value = r.question_id
  try {
    const res: any = await questionApi.getById(r.question_id)
    editQuestion.value = res.data
    editVisible.value = true
  } catch {
    ElMessage.error('加载题目失败，题目可能已被删除')
  } finally {
    editLoadingId.value = null
  }
}

// 保存后：刷新列表，处理弹窗里同步最新题干摘要
async function handleSaved() {
  await load()
  if (current.value && detailVisible.value) {
    current.value = list.value.find((r) => r.id === current.value!.id) || null
  }
}
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-bold">题目错误报告</h2>
      <div class="flex gap-2">
        <el-radio-group :model-value="statusFilter" @change="changeStatus">
          <el-radio-button :value="-1">全部</el-radio-button>
          <el-radio-button :value="0">待处理</el-radio-button>
          <el-radio-button :value="1">已处理</el-radio-button>
          <el-radio-button :value="2">已忽略</el-radio-button>
        </el-radio-group>
      </div>
    </div>

    <el-table :data="list" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="题目" min-width="200">
        <template #default="{ row }: { row: any }">
          <div class="flex flex-wrap items-center gap-1.5">
            <el-tag v-if="row.question_type" size="small">{{ QUESTION_TYPE_LABELS[row.question_type] || '其他' }}</el-tag>
            <el-tag v-if="row.question_year" size="small" type="info">{{ row.question_year }}年</el-tag>
            <a
              :href="`/questions/${row.question_id}`"
              target="_blank"
              class="text-blue-600 hover:underline"
            >#{{ row.question_id }}</a>
          </div>
          <div v-if="row.question_title" class="mt-1 text-sm text-gray-700 dark:text-gray-300">{{ row.question_title }}</div>
        </template>
      </el-table-column>
      <el-table-column label="科目" width="120">
        <template #default="{ row }: { row: any }">
          <span class="text-sm">{{ row.subject_name || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="题干摘要" min-width="280">
        <template #default="{ row }: { row: any }">
          <span class="text-sm text-gray-600 dark:text-gray-400">{{ snippetOf(row) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="问题类型" width="130">
        <template #default="{ row }: { row: any }">
          <el-tag size="small">{{ REPORT_TYPE_LABELS[row.report_type] || '其他' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="描述" min-width="180">
        <template #default="{ row }: { row: any }">
          <span class="text-sm">{{ truncate(row.description) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="提交用户" width="100" prop="username" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }: { row: any }">
          <el-tag :type="STATUS_LABELS[row.status]?.type as any" size="small">
            {{ STATUS_LABELS[row.status]?.text }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="提交时间" width="170">
        <template #default="{ row }: { row: any }">
          <span class="text-sm text-gray-500">{{ row.created_at?.slice(0, 16) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180">
        <template #default="{ row }: { row: any }">
          <el-button size="small" type="primary" plain @click="openDetail(row)">处理</el-button>
          <el-button size="small" :loading="editLoadingId === row.question_id" @click="openEditQuestion(row)">编辑题目</el-button>
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

    <!-- 处理弹窗 -->
    <el-dialog v-model="detailVisible" title="处理报告" width="560px">
      <template v-if="current">
        <el-descriptions :column="1" border size="small" class="mb-4">
          <el-descriptions-item label="题目">
            <div class="flex flex-wrap items-center gap-1.5">
              <el-tag v-if="current.question_type" size="small">{{ QUESTION_TYPE_LABELS[current.question_type] || '其他' }}</el-tag>
              <el-tag v-if="current.question_year" size="small" type="info">{{ current.question_year }}年</el-tag>
              <a :href="`/questions/${current.question_id}`" target="_blank" class="text-blue-600 hover:underline">
                #{{ current.question_id }} 查看题目
              </a>
            </div>
          </el-descriptions-item>
          <el-descriptions-item label="题目标题">{{ current.question_title || '—' }}</el-descriptions-item>
          <el-descriptions-item label="题干摘要">{{ snippetOf(current) }}</el-descriptions-item>
          <el-descriptions-item label="问题类型">{{ REPORT_TYPE_LABELS[current.report_type] }}</el-descriptions-item>
          <el-descriptions-item label="问题描述">
            <span class="whitespace-pre-wrap">{{ current.description }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="提交用户">{{ current.username }}</el-descriptions-item>
          <el-descriptions-item label="当前状态">
            <el-tag :type="STATUS_LABELS[current.status]?.type as any" size="small">
              {{ STATUS_LABELS[current.status]?.text }}
            </el-tag>
          </el-descriptions-item>
        </el-descriptions>
        <el-form-item label="处理备注">
          <el-input
            v-model="adminNote"
            type="textarea"
            :rows="3"
            placeholder="可填写处理说明（选填）"
            maxlength="500"
            show-word-limit
          />
        </el-form-item>
      </template>
      <template #footer>
        <el-button @click="detailVisible = false">取消</el-button>
        <el-button type="info" :loading="saving" @click="handleUpdate(2)">忽略</el-button>
        <el-button type="success" :loading="saving" @click="handleUpdate(1)">标记已处理</el-button>
        <el-button
          type="primary"
          :loading="editLoadingId === (current?.question_id ?? -1)"
          @click="openEditQuestion()"
        >编辑题目</el-button>
      </template>
    </el-dialog>

    <!-- 编辑题目弹窗：与题库管理共用；保存后回填报告页 -->
    <QuestionEditDialog
      v-model="editVisible"
      :question="editQuestion"
      :subject-id="0"
      @saved="handleSaved"
    />
  </div>
</template>
