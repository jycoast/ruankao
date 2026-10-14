<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { examApi, questionApi } from '@/api'
import { useSubjectStore } from '@/stores/subject'
import type { Paper, Question } from '@/types'

const subjectStore = useSubjectStore()

// 试卷类型：1上午题 2下午题I 3下午题II 4综合（与 papers 表注释一致）
const paperTypeLabels: Record<number, string> = { 1: '上午题', 2: '下午题I', 3: '下午题II', 4: '综合' }
// 考试批次：2024 软考改革后一年两考
const batchLabels: Record<number, string> = { 0: '单次', 1: '上半年', 2: '下半年' }
const questionTypeLabels: Record<number, string> = { 1: '单选', 2: '多选', 3: '案例', 4: '论文' }

// 年份选项：题库从 2009 年开始，到今年为止
const currentYear = new Date().getFullYear()
const years = Array.from({ length: currentYear - 2009 + 1 }, (_, i) => currentYear - i)

// ---- 列表 ----
const list = ref<Paper[]>([])
const total = ref(0)
const page = ref(1)
const keyword = ref('')
const paperType = ref<number>(0)
const year = ref<number>(0)

async function load() {
  const res: any = await examApi.adminList({
    page: page.value,
    page_size: 20,
    subject_id: subjectStore.currentSubjectId,
    paper_type: paperType.value || undefined,
    year: year.value || undefined,
    keyword: keyword.value || undefined,
  })
  list.value = res.data.items || []
  total.value = res.data.total || 0
  // 当前页越界时自动回到第 1 页，避免停在一页空结果上
  if (!list.value.length && page.value > 1) {
    page.value = 1
    load()
  }
}

onMounted(load)
// 切换科目后强制刷新数据(整页 reload 是首选;此处 watch 是兜底,防止 reload 因任何原因被吞
// 时还停留在旧科目数据上)
watch(() => subjectStore.currentSubjectId, () => load())

function search() {
  page.value = 1
  load()
}

function reset() {
  keyword.value = ''
  paperType.value = 0
  year.value = 0
  page.value = 1
  load()
}

// ---- 新增 / 编辑 ----
const dialogVisible = ref(false)
const editing = ref<Paper | null>(null)
const saving = ref(false)
const form = ref({
  title: '',
  year: undefined as number | undefined, // 年份必须由用户选择，默认空则显示占位符而非 0
  batch: 0,
  paper_type: 1,
  total_score: 0,
  duration: 150,
  description: '',
})

function openCreate() {
  editing.value = null
  form.value = {
    title: '',
    year: undefined as number | undefined,
    batch: 0,
    paper_type: 1,
    total_score: 0,
    duration: 150,
    description: '',
  }
  dialogVisible.value = true
}

function openEdit(row: Paper) {
  editing.value = row
  form.value = {
    title: row.title,
    year: row.year,
    batch: row.batch || 0,
    paper_type: row.paper_type,
    total_score: row.total_score || 0,
    duration: row.duration || 150,
    description: row.description || '',
  }
  dialogVisible.value = true
}

async function save() {
  if (!form.value.title.trim()) {
    ElMessage.warning('请填写试卷标题')
    return
  }
  if (!form.value.year) {
    ElMessage.warning('请填写试卷年份')
    return
  }
  saving.value = true
  try {
    const payload = { ...form.value, subject_id: subjectStore.currentSubjectId }
    if (editing.value) {
      await examApi.adminUpdate(editing.value.id, payload)
    } else {
      await examApi.adminCreate(payload)
    }
    dialogVisible.value = false
    ElMessage.success('已保存')
    load()
  } finally {
    saving.value = false
  }
}

async function toggleStatus(row: Paper) {
  const status = row.status === 1 ? 0 : 1
  await examApi.adminUpdate(row.id, { status })
  ElMessage.success(status === 1 ? '已上线' : '已下线')
  load()
}

async function remove(row: Paper) {
  await ElMessageBox.confirm(
    `确定删除试卷「${row.title}」？该卷的题目关联与考试记录将一并删除，且无法恢复。`,
    '提示',
    { type: 'warning' },
  )
  await examApi.adminDelete(row.id)
  ElMessage.success('已删除')
  load()
}

// ---- 卷内题目管理 ----
const drawerVisible = ref(false)
const currentPaper = ref<Paper | null>(null)
const paperQuestions = ref<Question[]>([])
const qSaving = ref(false)

async function openQuestionManage(row: Paper) {
  currentPaper.value = row
  drawerVisible.value = true
  await loadPaperQuestions()
}

async function loadPaperQuestions() {
  const res: any = await examApi.adminQuestions(currentPaper.value!.id)
  paperQuestions.value = res.data || []
}

function removeQuestion(index: number) {
  paperQuestions.value.splice(index, 1)
}

function moveQuestion(index: number, offset: number) {
  const target = index + offset
  if (target < 0 || target >= paperQuestions.value.length) return
  const arr = paperQuestions.value
  ;[arr[index], arr[target]] = [arr[target], arr[index]]
}

async function saveQuestions() {
  qSaving.value = true
  try {
    await examApi.adminReplaceQuestions(
      currentPaper.value!.id,
      paperQuestions.value.map((q) => q.id),
    )
    ElMessage.success('题目编排已保存')
    load() // 刷新列表里的题目数
  } finally {
    qSaving.value = false
  }
}

// ---- 题库搜索（添加题目） ----
const qKeyword = ref('')
const qType = ref<number>(0)
const qYear = ref<number>(0)
const qList = ref<Question[]>([])
const qTotal = ref(0)
const qPage = ref(1)
const qLoading = ref(false)
const questionTableRef = ref()
const selectedQuestions = ref<Question[]>([])

async function searchQuestions() {
  qPage.value = 1
  await loadQuestions()
}

async function loadQuestions() {
  if (!currentPaper.value) return
  qLoading.value = true
  try {
    const res: any = await questionApi.list({
      page: qPage.value,
      page_size: 10,
      subject_id: subjectStore.currentSubjectId,
      question_type: qType.value || undefined,
      year: qYear.value || undefined,
      keyword: qKeyword.value || undefined,
    })
    qList.value = res.data.items || []
    qTotal.value = res.data.total || 0
  } finally {
    qLoading.value = false
  }
}

function onSelectionChange(rows: Question[]) {
  selectedQuestions.value = rows
}

function addSelected() {
  if (!selectedQuestions.value.length) {
    ElMessage.warning('请先勾选要加入的题目')
    return
  }
  const existing = new Set(paperQuestions.value.map((q) => q.id))
  const added = selectedQuestions.value.filter((q) => !existing.has(q.id))
  if (!added.length) {
    ElMessage.info('所选题目已在卷内')
    return
  }
  paperQuestions.value.push(...added)
  questionTableRef.value?.clearSelection()
  selectedQuestions.value = []
  ElMessage.success(`已加入 ${added.length} 道题`)
}
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-bold">
        试卷管理
        <el-tag size="small" class="ml-2">{{ subjectStore.currentSubject()?.name || '当前科目' }}</el-tag>
      </h2>
      <el-button type="primary" @click="openCreate">新增试卷</el-button>
    </div>

    <p class="mb-3 text-sm text-gray-400">
      仅展示 / 维护「{{ subjectStore.currentSubject()?.name }}」的真题卷，模拟卷由用户侧自动生成，不受影响。
    </p>

    <!-- 筛选栏 -->
    <div class="mb-4 flex flex-wrap items-center gap-3">
      <el-input
        v-model="keyword"
        placeholder="搜索试卷标题"
        clearable
        style="width: 220px"
        @keyup.enter="search"
        @clear="search"
      />
      <el-select v-model="paperType" placeholder="试卷类型" style="width: 160px" @change="search">
        <el-option label="类型：全部" :value="0" />
        <el-option v-for="(label, key) in paperTypeLabels" :key="key" :label="label" :value="Number(key)" />
      </el-select>
      <el-select v-model="year" placeholder="年份" style="width: 140px" @change="search">
        <el-option label="年份：全部" :value="0" />
        <el-option v-for="y in years" :key="y" :label="`${y} 年`" :value="y" />
      </el-select>
      <el-button @click="search">查询</el-button>
      <el-button @click="reset">重置</el-button>
    </div>

    <el-table :data="list" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="title" label="标题" min-width="260" show-overflow-tooltip />
      <el-table-column label="年份" width="90">
        <template #default="{ row }: { row: any }">{{ row.year }}</template>
      </el-table-column>
      <el-table-column label="批次" width="90">
        <template #default="{ row }: { row: any }">
          {{ batchLabels[row.batch] || '单次' }}
        </template>
      </el-table-column>
      <el-table-column label="类型" width="100">
        <template #default="{ row }: { row: any }">
          <el-tag size="small">{{ paperTypeLabels[row.paper_type] || row.paper_type }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="total_score" label="总分" width="80" />
      <el-table-column prop="duration" label="时长(分)" width="90" />
      <el-table-column prop="question_count" label="题目数" width="80" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }: { row: any }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">
            {{ row.status === 1 ? '已上线' : '已下线' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="300" fixed="right">
        <template #default="{ row }: { row: any }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="primary" plain @click="openQuestionManage(row)">题目</el-button>
          <el-button size="small" :type="row.status === 1 ? 'warning' : 'success'" plain @click="toggleStatus(row)">
            {{ row.status === 1 ? '下线' : '上线' }}
          </el-button>
          <el-button size="small" type="danger" plain @click="remove(row)">删除</el-button>
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

    <!-- 新增 / 编辑试卷 -->
    <el-dialog v-model="dialogVisible" :title="editing ? '编辑试卷' : '新增试卷'" width="560px">
      <el-form label-width="80px">
        <el-form-item label="标题">
          <el-input v-model="form.title" placeholder="如：2025年上半年系统架构设计师上午真题" maxlength="200" show-word-limit />
        </el-form-item>
        <el-form-item label="年份">
          <el-select v-model="form.year" placeholder="选择年份" style="width: 160px">
            <el-option v-for="y in years" :key="y" :label="`${y} 年`" :value="y" />
          </el-select>
        </el-form-item>
        <el-form-item label="批次">
          <el-select v-model="form.batch" style="width: 160px">
            <el-option label="单次" :value="0" />
            <el-option label="上半年" :value="1" />
            <el-option label="下半年" :value="2" />
          </el-select>
        </el-form-item>
        <el-form-item label="试卷类型">
          <el-select v-model="form.paper_type" style="width: 160px">
            <el-option v-for="(label, key) in paperTypeLabels" :key="key" :label="label" :value="Number(key)" />
          </el-select>
        </el-form-item>
        <el-form-item label="总分">
          <el-input-number v-model="form.total_score" :min="0" :max="1000" :precision="1" />
        </el-form-item>
        <el-form-item label="时长">
          <el-input-number v-model="form.duration" :min="1" :max="600" />
          <span class="ml-2 text-sm text-gray-400">分钟</span>
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="3" placeholder="试卷说明（可选）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <!-- 卷内题目管理 -->
    <el-drawer v-model="drawerVisible" :title="`题目编排：${currentPaper?.title || ''}`" size="68%">
      <div class="flex h-full flex-col gap-4">
        <!-- 当前卷内题目 -->
        <div>
          <div class="mb-2 flex items-center justify-between">
            <div class="text-sm font-semibold text-gray-600 dark:text-gray-300">
              卷内题目（{{ paperQuestions.length }} 道）
            </div>
            <el-button type="primary" :loading="qSaving" @click="saveQuestions">保存编排</el-button>
          </div>
          <el-table :data="paperQuestions" size="small" stripe>
            <el-table-column label="#" width="50">
              <template #default="{ $index }">{{ $index + 1 }}</template>
            </el-table-column>
            <el-table-column label="题型" width="80">
              <template #default="{ row }: { row: any }">
                <el-tag size="small">{{ questionTypeLabels[row.question_type] || row.question_type }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="year" label="年份" width="70" />
            <el-table-column prop="title" label="题干" min-width="240" show-overflow-tooltip />
            <el-table-column label="操作" width="180" fixed="right">
              <template #default="{ $index }">
                <el-button size="small" :disabled="$index === 0" @click="moveQuestion($index, -1)">上移</el-button>
                <el-button size="small" :disabled="$index === paperQuestions.length - 1" @click="moveQuestion($index, 1)">下移</el-button>
                <el-button size="small" type="danger" plain @click="removeQuestion($index)">移除</el-button>
              </template>
            </el-table-column>
            <template #empty>
              <div class="py-6 text-sm text-gray-400">卷内还没有题目，从下方题库中勾选加入</div>
            </template>
          </el-table>
        </div>

        <!-- 从题库添加 -->
        <el-divider content-position="left">从题库添加题目</el-divider>
        <div class="flex flex-wrap items-center gap-3">
          <el-input
            v-model="qKeyword"
            placeholder="搜索题干"
            clearable
            style="width: 200px"
            @keyup.enter="searchQuestions"
            @clear="searchQuestions"
          />
          <el-select v-model="qType" placeholder="题型" style="width: 130px" @change="searchQuestions">
            <el-option label="题型：全部" :value="0" />
            <el-option v-for="(label, key) in questionTypeLabels" :key="key" :label="label" :value="Number(key)" />
          </el-select>
          <el-select v-model="qYear" placeholder="年份" style="width: 130px" @change="searchQuestions">
            <el-option label="年份：全部" :value="0" />
            <el-option v-for="y in years" :key="y" :label="`${y} 年`" :value="y" />
          </el-select>
          <el-button type="primary" plain @click="searchQuestions">搜索题库</el-button>
        </div>

        <el-table
          ref="questionTableRef"
          v-loading="qLoading"
          :data="qList"
          size="small"
          stripe
          @selection-change="onSelectionChange"
        >
          <el-table-column type="selection" width="45" />
          <el-table-column prop="id" label="ID" width="70" />
          <el-table-column label="题型" width="80">
            <template #default="{ row }: { row: any }">
              <el-tag size="small">{{ questionTypeLabels[row.question_type] || row.question_type }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="year" label="年份" width="70" />
          <el-table-column prop="title" label="题干" min-width="240" show-overflow-tooltip />
        </el-table>

        <div class="flex items-center justify-between">
          <el-button type="primary" :disabled="!selectedQuestions.length" @click="addSelected">
            加入所选（{{ selectedQuestions.length }}）
          </el-button>
          <el-pagination
            v-model:current-page="qPage"
            :page-size="10"
            :total="qTotal"
            layout="prev, pager, next"
            @current-change="loadQuestions"
          />
        </div>
      </div>
    </el-drawer>
  </div>
</template>
