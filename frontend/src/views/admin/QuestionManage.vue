<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { questionApi } from '@/api'
import { useSubjectStore } from '@/stores/subject'
import { stripMd } from '@/utils/markdown'
import type { Question } from '@/types'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Rank, Search } from '@element-plus/icons-vue'

const subjectStore = useSubjectStore()

const TYPE_LABELS: Record<number, string> = { 1: '单选', 2: '多选', 3: '案例', 4: '论文' }

const questions = ref<Question[]>([])
const total = ref(0)
const loading = ref(false)

// 固定近 15 年可选，默认当前年份
const CURRENT_YEAR = new Date().getFullYear()
const yearOptions = Array.from({ length: 15 }, (_, i) => CURRENT_YEAR - i)

// 筛选（0=全部）
const filterYear = ref(CURRENT_YEAR)
const filterType = ref(0)
// 题干模糊搜索：用户只复制了题干片段时要能找回题目，按 content 全科搜
const filterKeyword = ref('')

const dialogVisible = ref(false)
const editingQuestion = ref<Question | null>(null)

const page = ref(1)
const pageSize = ref(50)

// 按选中年份/题型从后端加载
async function load() {
  loading.value = true
  try {
    const params: any = { page: page.value, page_size: pageSize.value, subject_id: subjectStore.currentSubjectId }
    if (filterKeyword.value.trim()) {
      // 有搜索词时做整个科目的全局搜索（title + content），忽略年份/题型筛选，
      // 保证能定位到用户只复制了题干片段的那道题
      params.keyword = filterKeyword.value.trim()
    } else {
      if (filterYear.value) params.year = filterYear.value
      if (filterType.value) params.question_type = filterType.value
    }
    const res: any = await questionApi.list(params)
    questions.value = res.data.items || []
    total.value = res.data.total || 0
  } finally {
    loading.value = false
  }
}

// 切筛选条件要回到第一页，否则会停在一个空白的尾页
function reload() {
  page.value = 1
  load()
}

onMounted(load)
// 切换科目后强制刷新数据(整页 reload 是首选;此处 watch 是兜底,防止 reload 因任何原因被吞
// 时还停留在旧科目数据上)。
// 走 reload() 而不是 load():换科目的数据量不同,停在原来的页码会请求到新科目的空白尾页
watch(() => subjectStore.currentSubjectId, () => reload())

// 按「年份 · 类型」分组
const groups = computed(() => {
  const map = new Map<string, { year: number; type: number; items: Question[] }>()
  for (const q of questions.value) {
    const key = `${q.year || 0}-${q.question_type}`
    if (!map.has(key)) map.set(key, { year: q.year || 0, type: q.question_type, items: [] })
    map.get(key)!.items.push(q)
  }
  const list = [...map.values()].sort((a, b) => b.year - a.year || a.type - b.type)
  // 组内：已拖拽排序的按 sort_order 升序在前；未排序的保持原 id 升序
  list.forEach((g) => g.items.sort((a, b) => (a.sort_order || 0) - (b.sort_order || 0) || a.id - b.id))
  return list
})

function openCreate() {
  editingQuestion.value = null
  dialogVisible.value = true
}

function openEdit(q: Question) {
  editingQuestion.value = q
  dialogVisible.value = true
}

async function handleDelete(id: number) {
  await ElMessageBox.confirm('确认删除该题目？', '提示', { type: 'warning' })
  await questionApi.delete(id)
  ElMessage.success('删除成功')
  load()
}

// 列表行：题干纯文本摘要，作为表格主列让管理员一眼认题，不必靠「第1题」这种重复标题。
const TYPE_LABELS_MAP = TYPE_LABELS
const DIFFICULTY_LABELS: Record<number, string> = { 1: '简单', 2: '中等', 3: '困难' }

function snippetOf(q: Question): string {
  const t = stripMd(q.content || q.title)
  if (!t) return '（无题干）'
  return t.length > 80 ? t.slice(0, 80) + '…' : t
}

function answerOf(q: Question): string {
  return (q.options || []).filter((o) => o.is_correct === 1).map((o) => o.label).join('')
}

// ---- 拖拽排序：按当前筛选条件加载全量题目，拖拽调整顺序，保存后整批重排 sort_order ----
const sortVisible = ref(false)
const sortList = ref<Question[]>([])
const sortLoading = ref(false)
const dragIndex = ref<number | null>(null)

async function openSort() {
  sortVisible.value = true
  sortLoading.value = true
  sortList.value = []
  try {
    const params: any = { page: 1, page_size: 10000, subject_id: subjectStore.currentSubjectId }
    if (filterYear.value) params.year = filterYear.value
    if (filterType.value) params.question_type = filterType.value
    const res: any = await questionApi.list(params)
    // 后端已按 sort_order 升序返回：已拖拽的在最前，未排序的按 id 倒序排后
    sortList.value = res.data.items || []
  } finally {
    sortLoading.value = false
  }
}

function onDragStart(i: number) {
  dragIndex.value = i
}

function onDrop(i: number) {
  const from = dragIndex.value
  dragIndex.value = null
  if (from === null || from === i) return
  const arr = [...sortList.value]
  const [moved] = arr.splice(from, 1)
  arr.splice(i, 0, moved)
  sortList.value = arr
}

function moveSort(i: number, offset: number) {
  const target = i + offset
  if (target < 0 || target >= sortList.value.length) return
  const arr = [...sortList.value]
  ;[arr[i], arr[target]] = [arr[target], arr[i]]
  sortList.value = arr
}

async function saveSort() {
  try {
    await questionApi.sort(sortList.value.map((q) => q.id))
    ElMessage.success('排序已保存')
    sortVisible.value = false
    load()
  } catch {
    ElMessage.error('保存失败，请重试')
  }
}
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-bold">
        题库管理
        <el-tag size="small" class="ml-2">{{ subjectStore.currentSubject()?.name || '当前科目' }}</el-tag>
      </h2>
      <el-button type="primary" @click="openCreate">新增题目</el-button>
    </div>

    <p class="mb-3 text-sm text-gray-400">当前仅展示 / 维护「{{ subjectStore.currentSubject()?.name }}」的题目，切换右上角科目可管理其他科目。</p>

    <!-- 筛选栏 -->
    <div class="mb-4 flex flex-wrap items-center gap-3">
      <el-select v-model="filterYear" placeholder="年份" style="width: 130px" @change="reload">
        <el-option label="全部年份" :value="0" />
        <el-option v-for="y in yearOptions" :key="y" :label="`${y}年`" :value="y" />
      </el-select>
      <el-select v-model="filterType" placeholder="题型" style="width: 130px" @change="reload">
        <el-option label="全部题型" :value="0" />
        <el-option label="单选" :value="1" />
        <el-option label="多选" :value="2" />
        <el-option label="案例" :value="3" />
        <el-option label="论文" :value="4" />
      </el-select>
      <el-input
        v-model="filterKeyword"
        placeholder="按题干模糊搜索（可粘贴用户复制的题干片段）"
        clearable
        style="width: 280px"
        @keyup.enter="reload"
        @clear="reload"
      >
        <template #suffix>
          <el-icon class="cursor-pointer" @click="reload"><Search /></el-icon>
        </template>
      </el-input>
      <span class="text-sm text-gray-400">共 {{ total }} 题</span>
      <el-button type="warning" plain class="ml-auto" @click="openSort">
        <el-icon class="mr-1"><Rank /></el-icon> 拖拽排序
      </el-button>
    </div>

    <!-- 分组列表 -->
    <div v-loading="loading" v-for="g in groups" :key="`${g.year}-${g.type}`" class="mb-6">
      <div class="mb-2 flex items-center gap-2 border-b border-gray-200 pb-1.5 dark:border-gray-700">
        <span class="text-sm font-semibold text-gray-700 dark:text-gray-200">
          {{ g.year || '未标注' }}年 · {{ TYPE_LABELS[g.type] || '其他' }}
        </span>
        <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-500 dark:bg-gray-700 dark:text-gray-300">本页 {{ g.items.length }}</span>
      </div>
      <el-table :data="g.items" size="small">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column label="题干摘要" min-width="320">
          <template #default="{ row }: { row: any }">
            <div class="text-sm text-gray-800 dark:text-gray-200">{{ snippetOf(row) }}</div>
            <div class="mt-0.5 text-xs text-gray-400">{{ row.title }}</div>
          </template>
        </el-table-column>
        <el-table-column label="题型" width="80">
          <template #default="{ row }: { row: any }">{{ TYPE_LABELS_MAP[row.question_type] || '-' }}</template>
        </el-table-column>
        <el-table-column label="答案" width="80">
          <template #default="{ row }: { row: any }">
            <span v-if="answerOf(row)" class="font-medium text-green-600 dark:text-green-400">{{ answerOf(row) }}</span>
            <span v-else class="text-gray-300">-</span>
          </template>
        </el-table-column>
        <el-table-column label="难度" width="80">
          <template #default="{ row }: { row: any }">{{ DIFFICULTY_LABELS[row.difficulty] || '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }: { row: any }">
            <el-button size="small" @click="openEdit(row)">编辑</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div v-if="!loading && !groups.length" class="py-12 text-center text-gray-400">暂无题目</div>

    <div v-if="total > pageSize" class="mt-4 flex justify-end">
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :page-sizes="[30, 50, 100]"
        :total="total"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="load"
        @size-change="reload"
      />
    </div>

    <!-- 编辑弹窗：与题目反馈「直接改题」共用同一组件 -->
    <QuestionEditDialog
      v-model="dialogVisible"
      :question="editingQuestion"
      :subject-id="subjectStore.currentSubjectId"
      @saved="load"
    />

    <!-- 拖拽排序弹窗 -->
    <el-dialog v-model="sortVisible" title="拖拽排序" width="760px" top="5vh">
      <div class="mb-3 text-sm text-gray-400">
        当前范围：{{ filterYear ? filterYear + '年' : '全部年份' }} · {{ filterType ? TYPE_LABELS[filterType] : '全部题型' }}
        （共 {{ sortList.length }} 题）—— 拖拽行首手柄调整顺序，保存后用户端题库列表按此顺序展示，未拖拽过的题目排在最后。
      </div>
      <div v-loading="sortLoading" class="max-h-[60vh] overflow-y-auto divide-y rounded-lg border dark:divide-gray-700 dark:border-gray-700">
        <div
          v-for="(q, i) in sortList"
          :key="q.id"
          draggable="true"
          class="flex cursor-move items-center gap-3 px-3 py-2 hover:bg-gray-50 dark:hover:bg-gray-800"
          :class="dragIndex === i ? 'bg-blue-50 dark:bg-blue-950/40' : ''"
          @dragstart="onDragStart(i)"
          @dragover.prevent
          @drop="onDrop(i)"
        >
          <span class="text-gray-400"><el-icon><Rank /></el-icon></span>
          <span class="w-8 text-xs text-gray-400">{{ i + 1 }}</span>
          <span class="w-16 text-xs text-gray-400">#{{ q.id }}</span>
          <span class="min-w-0 flex-1 truncate text-sm text-gray-700 dark:text-gray-200">{{ snippetOf(q) }}</span>
          <el-tag size="small" effect="plain">{{ q.year || '-' }}年</el-tag>
          <el-tag size="small" effect="plain" type="info">{{ TYPE_LABELS[q.question_type] || '-' }}</el-tag>
          <span v-if="answerOf(q)" class="w-8 text-center text-xs font-medium text-green-600 dark:text-green-400">{{ answerOf(q) }}</span>
          <div class="flex flex-col leading-none">
            <el-button size="small" text :disabled="i === 0" @click="moveSort(i, -1)">↑</el-button>
            <el-button size="small" text :disabled="i === sortList.length - 1" @click="moveSort(i, 1)">↓</el-button>
          </div>
        </div>
        <div v-if="!sortLoading && !sortList.length" class="py-12 text-center text-gray-400">当前筛选条件下暂无题目</div>
      </div>
      <template #footer>
        <el-button @click="sortVisible = false">取消</el-button>
        <el-button type="primary" :disabled="sortLoading || !sortList.length" @click="saveSort">保存排序</el-button>
      </template>
    </el-dialog>
  </div>
</template>
