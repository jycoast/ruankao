<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { articleApi } from '@/api'
import { useSubjectStore } from '@/stores/subject'
import { useAI } from '@/composables/useAI'
import { essayExcerpt } from '@/utils/essay'
import type { Article } from '@/types'

const subjectStore = useSubjectStore()
const router = useRouter()
const { aiEnabled, ensureAIStatus } = useAI()
const articles = ref<Article[]>([])
const total = ref(0)
const page = ref(1)
const activeType = ref(0)

const filters = [
  { label: '全部', value: 0 },
  { label: '论文范文', value: 1 },
  { label: '写作模板', value: 2 },
  { label: '评分标准', value: 3 },
]

async function load() {
  const res: any = await articleApi.list({
    article_type: activeType.value || 0,
    page: page.value,
    page_size: 12,
    subject_id: subjectStore.currentSubjectId,
  })
  articles.value = res.data.items || []
  total.value = res.data.total || 0
}

onMounted(load)
onMounted(ensureAIStatus)
watch(() => subjectStore.currentSubjectId, () => {
  page.value = 1
  load()
})

function setType(v: number) {
  activeType.value = v
  page.value = 1
  load()
}

function typeLabel(t: number) {
  return { 1: '论文范文', 2: '写作模板', 3: '评分标准', 4: 'AI点评' }[t] || '其他'
}
function typeTag(t: number) {
  return ({ 1: 'success', 2: 'primary', 3: 'warning', 4: 'info' } as const)[t as 1] || 'info'
}
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-bold text-gray-900 dark:text-gray-100">论文范文库</h2>
      <div class="flex flex-wrap gap-2">
        <el-button v-if="aiEnabled" type="primary" plain @click="router.push('/essays/ai-generate')">
          <el-icon class="mr-1"><MagicStick /></el-icon>AI 生成范文
        </el-button>
        <el-button v-if="aiEnabled" plain @click="router.push('/essays/ai-grade')">
          <el-icon class="mr-1"><EditPen /></el-icon>AI 批阅
        </el-button>
        <el-button @click="router.push('/essays/my-submissions')">我的投稿</el-button>
        <el-button type="primary" @click="router.push('/essays/submit')">提交论文</el-button>
      </div>
    </div>

    <!-- 分段筛选 -->
    <div class="mb-5 flex flex-wrap gap-1 rounded-lg border border-slate-200 bg-white p-0.5 dark:border-slate-700 dark:bg-slate-800 w-fit">
      <button
        v-for="f in filters"
        :key="f.value"
        class="rounded-md px-3 py-1.5 text-sm font-medium transition"
        :class="activeType === f.value
          ? 'bg-blue-600 text-white shadow-sm'
          : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'"
        @click="setType(f.value)"
      >{{ f.label }}</button>
    </div>

    <!-- 卡片网格 -->
    <div v-if="articles.length" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4">
      <router-link
        v-for="a in articles"
        :key="a.id"
        :to="`/essays/${a.id}`"
        class="group flex flex-col rounded-xl border border-slate-200 bg-white p-4 shadow-sm transition hover:-translate-y-0.5 hover:border-blue-200 hover:shadow-md dark:border-slate-800 dark:bg-slate-900"
      >
        <div class="mb-2 flex flex-wrap items-center gap-2">
          <el-tag :type="typeTag(a.article_type)" size="small">{{ typeLabel(a.article_type) }}</el-tag>
          <span v-if="a.year" class="text-xs text-slate-400">{{ a.year }}年</span>
          <span
            v-if="a.score"
            class="ml-auto rounded-full bg-orange-50 px-2 py-0.5 text-xs font-medium text-orange-600 dark:bg-orange-500/15 dark:text-orange-300"
          >{{ a.score }} 分</span>
        </div>
        <h3 class="line-clamp-2 font-semibold text-slate-900 group-hover:text-blue-600 dark:text-white">{{ a.title }}</h3>
        <p class="mt-2 line-clamp-3 flex-1 text-sm text-slate-500 dark:text-slate-400">{{ essayExcerpt(a.content, 120) }}</p>
        <div v-if="a.author" class="mt-3 text-xs text-slate-400">作者：{{ a.author }}</div>
      </router-link>
    </div>
    <div v-else class="py-16 text-center text-sm text-slate-500">暂无内容</div>

    <div class="mt-6 flex justify-center">
      <el-pagination
        v-model:current-page="page"
        :page-size="12"
        :total="total"
        layout="prev, pager, next"
        @current-change="load"
      />
    </div>
  </div>
</template>
