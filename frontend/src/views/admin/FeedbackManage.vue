<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { feedbackApi, questionApi } from '@/api'
import { stripMd, stripHtml, feedbackContentHtml } from '@/utils/markdown'
import type { Feedback, Question } from '@/types'

const list = ref<Feedback[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const keyword = ref('')
const status = ref(-1) // -1=全部 0=待处理 1=已处理 2=已忽略
const typeFilter = ref(-1) // -1=全部 1=功能建议 2=问题反馈 3=内容纠错 4=其他

const detailVisible = ref(false)
const current = ref<Feedback | null>(null)
const adminNote = ref('')
const saving = ref(false)

// 「编辑题目」弹窗：内容纠错反馈直接改题，改完回填
const editVisible = ref(false)
const editQuestion = ref<Question | null>(null)
const editLoadingId = ref<number | null>(null)

const TYPE_LABELS: Record<number, string> = {
  1: '功能建议',
  2: '问题反馈',
  3: '内容纠错',
  4: '其他',
}

const QUESTION_TYPE_LABELS: Record<number, string> = { 1: '单选', 2: '多选', 3: '案例', 4: '论文' }

const STATUS_LABELS: Record<number, { text: string; type: string }> = {
  0: { text: '待处理', type: 'danger' },
  1: { text: '已处理', type: 'success' },
  2: { text: '已忽略', type: 'info' },
}

async function load() {
  loading.value = true
  try {
    const res: any = await feedbackApi.adminList({
      page: page.value,
      page_size: pageSize.value,
      status: status.value,
      type: typeFilter.value,
      keyword: keyword.value || undefined,
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

onMounted(load)

function search() {
  page.value = 1
  load()
}

function reset() {
  keyword.value = ''
  typeFilter.value = -1
  status.value = -1
  page.value = 1
  load()
}

function onSizeChange(size: number) {
  pageSize.value = size
  page.value = 1
  load()
}

function openDetail(row: Feedback) {
  current.value = row
  adminNote.value = row.admin_note || ''
  detailVisible.value = true
}

async function handleUpdate(status: 1 | 2) {
  if (!current.value) return
  saving.value = true
  try {
    await feedbackApi.updateStatus(current.value.id, { status, admin_note: adminNote.value })
    ElMessage.success(status === 1 ? '已标记为处理完成' : '已忽略该反馈')
    detailVisible.value = false
    load()
  } finally {
    saving.value = false
  }
}

// 反馈内容是富文本 HTML：表格摘要先剥标签，纯图片反馈给个提示；
// 长度交给列宽单行省略 + 悬停 tooltip，不在前端二次截断。
function contentSnippet(r: { content?: string }): string {
  const t = stripHtml(r.content || '')
  if (!t) return '（仅图片，点「处理」查看）'
  return t
}

// 题干摘要是 markdown，去格式后展示纯文本
function snippetOf(r: { question_content?: string }): string {
  const t = stripMd(r.question_content || '')
  if (!t) return '（无题干）'
  return t.length > 80 ? t.slice(0, 80) + '…' : t
}

// 从反馈拉完整题目（含选项）→ 交给公共编辑弹窗；题目可能已被删除，失败时提示。
async function openEditQuestion(row?: Feedback | null) {
  const r = row || current.value
  if (!r?.question_id) return
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
      <h2 class="text-xl font-bold">意见反馈</h2>
      <div class="flex flex-wrap items-center gap-3">
        <el-input
          v-model="keyword"
          placeholder="搜索反馈内容 / 用户名 / 联系方式"
          clearable
          style="width: 260px"
          @keyup.enter="search"
          @clear="search"
        />
        <el-select
          v-model="typeFilter"
          placeholder="反馈类型"
          style="width: 140px"
          @change="search"
        >
          <el-option label="类型：全部" :value="-1" />
          <el-option label="功能建议" :value="1" />
          <el-option label="问题反馈" :value="2" />
          <el-option label="内容纠错" :value="3" />
          <el-option label="其他" :value="4" />
        </el-select>
        <el-select
          v-model="status"
          placeholder="状态"
          style="width: 130px"
          @change="search"
        >
          <el-option label="状态：全部" :value="-1" />
          <el-option label="待处理" :value="0" />
          <el-option label="已处理" :value="1" />
          <el-option label="已忽略" :value="2" />
        </el-select>
        <el-button type="primary" @click="search">搜索</el-button>
        <el-button @click="reset">重置</el-button>
      </div>
    </div>

    <el-table :data="list" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column label="类型" width="100">
        <template #default="{ row }: { row: any }">
          <el-tag size="small">{{ TYPE_LABELS[row.feedback_type] || '其他' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="关联题目" min-width="220">
        <template #default="{ row }: { row: any }">
          <!-- 单行展示：标签 + 题号 + 标题；标题过长省略，悬停看全文 -->
          <div v-if="row.question_id" class="flex min-w-0 items-center gap-1.5">
            <el-tag v-if="row.question_type" size="small" class="shrink-0">{{ QUESTION_TYPE_LABELS[row.question_type] || '其他' }}</el-tag>
            <el-tag v-if="row.question_year" size="small" type="info" class="shrink-0">{{ row.question_year }}年</el-tag>
            <a
              :href="`/questions/${row.question_id}`"
              target="_blank"
              class="shrink-0 text-blue-600 hover:underline"
            >#{{ row.question_id }}</a>
            <span v-if="row.question_title" class="truncate text-sm text-gray-700 dark:text-gray-300" :title="row.question_title">{{ row.question_title }}</span>
          </div>
          <span v-else class="text-gray-300">—</span>
        </template>
      </el-table-column>
      <el-table-column label="反馈内容" min-width="240" show-overflow-tooltip>
        <template #default="{ row }: { row: any }">
          <span class="text-sm">{{ contentSnippet(row) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="联系方式" width="130" show-overflow-tooltip>
        <template #default="{ row }: { row: any }">
          <span class="text-sm text-gray-600">{{ row.contact || '—' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="提交用户" width="90" prop="username" show-overflow-tooltip />
      <el-table-column label="状态" width="80">
        <template #default="{ row }: { row: any }">
          <el-tag :type="STATUS_LABELS[row.status]?.type as any" size="small">
            {{ STATUS_LABELS[row.status]?.text }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="提交时间" width="150">
        <template #default="{ row }: { row: any }">
          <span class="text-sm text-gray-500">{{ row.created_at?.slice(0, 16) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="90">
        <template #default="{ row }: { row: any }">
          <el-button size="small" type="primary" plain @click="openDetail(row)">处理</el-button>
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

    <!-- 处理弹窗 -->
    <el-dialog v-model="detailVisible" title="处理反馈" width="560px">
      <template v-if="current">
        <el-descriptions :column="1" border size="small" class="mb-4">
          <el-descriptions-item label="反馈类型">{{ TYPE_LABELS[current.feedback_type] }}</el-descriptions-item>
          <el-descriptions-item v-if="current.question_id" label="关联题目">
            <div class="flex flex-wrap items-center gap-1.5">
              <el-tag v-if="current.question_type" size="small">{{ QUESTION_TYPE_LABELS[current.question_type] || '其他' }}</el-tag>
              <el-tag v-if="current.question_year" size="small" type="info">{{ current.question_year }}年</el-tag>
              <a :href="`/questions/${current.question_id}`" target="_blank" class="text-blue-600 hover:underline">
                #{{ current.question_id }} 查看题目
              </a>
            </div>
          </el-descriptions-item>
          <el-descriptions-item v-if="current.question_id" label="题目标题">{{ current.question_title || '—' }}</el-descriptions-item>
          <el-descriptions-item v-if="current.question_id" label="题干摘要">{{ snippetOf(current) }}</el-descriptions-item>
          <el-descriptions-item label="反馈内容">
            <div class="rt-content text-sm" v-html="feedbackContentHtml(current.content)"></div>
          </el-descriptions-item>
          <el-descriptions-item label="联系方式">{{ current.contact || '未填写' }}</el-descriptions-item>
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
        <el-button
          v-if="current?.question_id"
          type="primary"
          :loading="editLoadingId === (current?.question_id ?? -1)"
          @click="openEditQuestion()"
        >编辑题目</el-button>
        <el-button type="success" :loading="saving" @click="handleUpdate(1)">标记已处理</el-button>
      </template>
    </el-dialog>

    <!-- 编辑题目弹窗：与题库管理/题目报告共用；保存后回填反馈页 -->
    <QuestionEditDialog
      v-model="editVisible"
      :question="editQuestion"
      :subject-id="0"
      @saved="handleSaved"
    />
  </div>
</template>