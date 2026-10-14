<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { questionApi } from '@/api'
import { useSubjectStore } from '@/stores/subject'
import type { Question } from '@/types'

const router = useRouter()
const subjectStore = useSubjectStore()
const keyword = ref('')
const searching = ref(false)
const searched = ref(false)
const questions = ref<Question[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20

const selectedType = ref(0) // 0=all, 1=选择, 3=案例, 4=论文
const selectedYear = ref(0)
const selectedDifficulty = ref(0)

const typeOptions = [
  { label: '全部', value: 0 },
  { label: '选择题', value: 1 },
  { label: '案例分析', value: 3 },
  { label: '论文', value: 4 },
]

const difficultyOptions = [
  { label: '不限', value: 0 },
  { label: '简单', value: 1 },
  { label: '中等', value: 2 },
  { label: '困难', value: 3 },
]

const CURRENT_YEAR = new Date().getFullYear()
const yearOptions = [0, ...Array.from({ length: 15 }, (_, i) => CURRENT_YEAR - i)]

const hasFilter = computed(
  () => !!keyword.value.trim() || selectedType.value > 0 || selectedYear.value > 0 || selectedDifficulty.value > 0,
)

async function search(toPage = 1) {
  if (!hasFilter.value) return
  page.value = toPage
  searching.value = true
  searched.value = true
  try {
    const res: any = await questionApi.list({
      page: page.value,
      page_size: pageSize,
      keyword: keyword.value.trim() || undefined,
      question_type: selectedType.value || undefined,
      year: selectedYear.value || undefined,
      difficulty: selectedDifficulty.value || undefined,
      subject_id: subjectStore.currentSubjectId || undefined,
    })
    questions.value = res.data.items || []
    total.value = res.data.total || 0
  } catch {
    questions.value = []
    total.value = 0
  }
  searching.value = false
}

// 筛选条件变化后，若已搜过则立即重查（回到第一页）
function onFilterChange() {
  if (searched.value || hasFilter.value) search(1)
}

function resetFilters() {
  keyword.value = ''
  selectedType.value = 0
  selectedYear.value = 0
  selectedDifficulty.value = 0
  questions.value = []
  total.value = 0
  searched.value = false
}

function viewQuestion(id: number) {
  router.push(`/questions/${id}`)
}

function typeLabel(t: number) {
  return { 1: '选择题', 3: '案例分析', 4: '论文' }[t] || '未知'
}

function typeColor(t: number) {
  return { 1: 'bg-blue-100 text-blue-700', 3: 'bg-green-100 text-green-700', 4: 'bg-purple-100 text-purple-700' }[t] || 'bg-gray-100 text-gray-700'
}
</script>

<template>
  <div class="max-w-6xl mx-auto">
    <!-- Header -->
    <div class="mb-6">
      <h1 class="text-2xl sm:text-3xl font-bold text-gray-900 dark:text-gray-100 mb-2">题库搜索</h1>
      <p class="text-sm sm:text-base text-gray-500 dark:text-gray-400">输入关键词搜索题目，覆盖综合知识、案例分析、论文全部模块</p>
    </div>

    <!-- Search Box -->
    <div class="bg-white dark:bg-gray-800 rounded-2xl shadow-lg border border-gray-100 dark:border-gray-700 p-4 sm:p-6 mb-6">
      <div class="flex gap-2 sm:gap-3">
        <input
          v-model="keyword"
          @keyup.enter="search(1)"
          type="text"
          placeholder="输入关键词，如：架构设计、设计模式..."
          class="min-w-0 flex-1 px-4 py-3 rounded-xl border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent text-base"
        />
        <button
          @click="search(1)"
          :disabled="!hasFilter || searching"
          class="shrink-0 px-5 sm:px-8 py-3 bg-gradient-to-r from-blue-600 to-purple-600 text-white rounded-xl font-medium hover:from-blue-700 hover:to-purple-700 transition-all shadow-lg hover:shadow-xl disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <span v-if="searching" class="flex items-center gap-2">
            <svg class="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            搜索中
          </span>
          <span v-else>搜索</span>
        </button>
      </div>

      <!-- Type Filter -->
      <div class="flex flex-wrap gap-2 mt-4">
        <button
          v-for="t in typeOptions"
          :key="t.value"
          @click="selectedType = t.value; onFilterChange()"
          class="px-4 py-1.5 rounded-lg text-sm font-medium transition-all"
          :class="selectedType === t.value
            ? 'bg-blue-600 text-white shadow'
            : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'"
        >
          {{ t.label }}
        </button>
      </div>

      <!-- Year / Difficulty -->
      <div class="flex flex-wrap items-center gap-3 mt-3">
        <select
          v-model.number="selectedYear"
          @change="onFilterChange()"
          class="px-3 py-1.5 rounded-lg border border-gray-200 bg-gray-50 text-sm text-gray-700 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200"
        >
          <option v-for="y in yearOptions" :key="y" :value="y">{{ y === 0 ? '全部年份' : y + '年' }}</option>
        </select>
        <select
          v-model.number="selectedDifficulty"
          @change="onFilterChange()"
          class="px-3 py-1.5 rounded-lg border border-gray-200 bg-gray-50 text-sm text-gray-700 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200"
        >
          <option v-for="d in difficultyOptions" :key="d.value" :value="d.value">{{ d.label }}</option>
        </select>
        <button
          v-if="hasFilter"
          @click="resetFilters"
          class="text-sm text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
        >
          清空筛选
        </button>
      </div>
    </div>

    <!-- Results -->
    <div v-if="searched">
      <!-- Result Count -->
      <div class="flex items-center justify-between mb-4">
        <div class="text-sm text-gray-500 dark:text-gray-400">
          找到 <span class="font-medium text-gray-700 dark:text-gray-300">{{ total }}</span> 个结果
          <span v-if="selectedType > 0" class="ml-2">
            （已筛选：{{ typeLabel(selectedType) }}）
          </span>
        </div>
      </div>

      <!-- Loading -->
      <div v-if="searching" class="text-center py-12">
        <div class="animate-spin inline-block w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full mb-4"></div>
        <div class="text-gray-500">搜索中...</div>
      </div>

      <!-- No Results -->
      <div v-else-if="questions.length === 0" class="text-center py-12">
        <div class="text-5xl mb-4">🔍</div>
        <div class="text-gray-500 text-lg">未找到相关题目</div>
        <div class="text-gray-400 text-sm mt-2">请尝试其他关键词或放宽筛选条件</div>
      </div>

      <!-- Results List -->
      <div v-else class="space-y-3">
        <div
          v-for="q in questions"
          :key="q.id"
          @click="viewQuestion(q.id)"
          class="bg-white dark:bg-gray-800 rounded-xl p-5 shadow-sm border border-gray-100 dark:border-gray-700 hover:shadow-lg hover:border-blue-200 dark:hover:border-blue-700 transition-all cursor-pointer"
        >
          <div class="flex flex-wrap items-center gap-2 mb-3">
            <span
              class="px-2.5 py-1 rounded-md text-xs font-medium"
              :class="typeColor(q.question_type)"
            >
              {{ typeLabel(q.question_type) }}
            </span>
            <span v-if="q.year" class="text-xs text-gray-400">{{ q.year }}年</span>
            <span v-if="q.knowledge_point" class="text-xs text-gray-400 sm:ml-auto">{{ q.knowledge_point }}</span>
          </div>
          <h3 class="font-medium text-gray-800 dark:text-gray-200 line-clamp-2">{{ q.title }}</h3>
          <div v-if="q.content" class="text-sm text-gray-500 dark:text-gray-400 mt-2 line-clamp-2">
            {{ q.content.replace(/<[^>]+>/g, '').replace(/\[图片\]/g, '[图片]').substring(0, 150) }}...
          </div>
          <div class="flex items-center gap-2 mt-3">
            <span class="text-xs text-blue-500 font-medium">查看详情 →</span>
          </div>
        </div>
      </div>

      <div v-if="!searching && total > pageSize" class="mt-6 flex justify-center">
        <el-pagination
          :current-page="page"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next"
          background
          @current-change="search"
        />
      </div>
    </div>

    <!-- Empty State (before search) -->
    <div v-else class="text-center py-16">
      <div class="text-6xl mb-4">📚</div>
      <div class="text-gray-500 text-lg mb-2">搜索题目</div>
      <div class="text-gray-400 text-sm">输入关键词开始搜索，支持题目内容、知识点等</div>
      <div class="flex flex-wrap gap-2 justify-center mt-6">
        <button 
          v-for="tag in ['架构设计', '设计模式', '数据库', '网络安全', '质量管理', 'UML']" 
          :key="tag"
          @click="keyword = tag; search()"
          class="px-4 py-2 bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded-full text-sm hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
        >
          {{ tag }}
        </button>
      </div>
    </div>
  </div>
</template>
