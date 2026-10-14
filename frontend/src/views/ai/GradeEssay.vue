<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { aiApi } from '@/api'

const router = useRouter()

interface Dim { name: string; score: number; full: number; comment: string }
interface Grade { total: number; dimensions: Dim[]; summary: string; suggestions: string[] }

const content = ref('')
const loading = ref(false)
const result = ref<Grade | null>(null)

const scoreColor = computed(() => {
  const t = result.value?.total || 0
  if (t >= 45) return 'text-emerald-600'
  if (t >= 30) return 'text-blue-600'
  return 'text-orange-600'
})

async function grade() {
  if (content.value.trim().length < 50) {
    ElMessage.warning('论文内容过短（至少 50 字）')
    return
  }
  loading.value = true
  result.value = null
  try {
    const res: any = await aiApi.gradeEssay(content.value.trim())
    result.value = res.data
  } catch {
    // 拦截器已提示
  } finally {
    loading.value = false
  }
}

function barColor(pct: number) {
  if (pct >= 80) return 'bg-emerald-500'
  if (pct >= 50) return 'bg-blue-500'
  return 'bg-orange-400'
}
</script>

<template>
  <div class="mx-auto max-w-3xl">
    <button class="mb-3 flex items-center gap-1 text-sm text-slate-500 transition hover:text-orange-600 dark:text-slate-400" @click="router.push('/essays')">
      <el-icon><ArrowLeft /></el-icon> 返回论文库
    </button>
    <h2 class="mb-1 text-xl font-bold text-gray-900 dark:text-gray-100">AI 批阅论文</h2>
    <p class="mb-4 text-sm text-slate-500 dark:text-slate-400">粘贴你的论文，AI 按维度打分并给出改进建议（满分 75）</p>

    <div class="mb-6 rounded-xl border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <textarea
        v-model="content"
        rows="10"
        placeholder="在此粘贴论文全文…"
        class="w-full resize-y rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm focus:border-orange-400 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
      />
      <div class="mt-3 flex items-center justify-between">
        <span class="text-xs text-slate-400">{{ content.length }} 字</span>
        <button class="rounded-lg bg-orange-500 px-5 py-2 text-sm font-medium text-white transition hover:bg-orange-600 disabled:opacity-50" :disabled="loading" @click="grade">
          {{ loading ? '批阅中…' : '开始批阅' }}
        </button>
      </div>
    </div>

    <div v-if="result" class="space-y-5">
      <!-- 总分 -->
      <div class="flex items-center gap-5 rounded-xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div class="text-center">
          <div class="text-4xl font-bold" :class="scoreColor">{{ result.total }}</div>
          <div class="text-xs text-slate-400">/ 75 分</div>
        </div>
        <p class="min-w-0 flex-1 text-sm leading-relaxed text-slate-600 dark:text-slate-300">{{ result.summary }}</p>
      </div>

      <!-- 维度 -->
      <div class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <h3 class="mb-4 font-semibold text-slate-900 dark:text-white">维度评分</h3>
        <div class="space-y-4">
          <div v-for="d in result.dimensions" :key="d.name">
            <div class="flex items-center justify-between text-sm">
              <span class="font-medium text-slate-700 dark:text-slate-200">{{ d.name }}</span>
              <span class="text-slate-500">{{ d.score }} / {{ d.full }}</span>
            </div>
            <div class="mt-1.5 h-2 overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800">
              <div class="h-full rounded-full transition-all" :class="barColor(Math.round((d.score / d.full) * 100))" :style="{ width: Math.round((d.score / d.full) * 100) + '%' }" />
            </div>
            <p v-if="d.comment" class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ d.comment }}</p>
          </div>
        </div>
      </div>

      <!-- 建议 -->
      <div v-if="result.suggestions?.length" class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <h3 class="mb-3 font-semibold text-slate-900 dark:text-white">改进建议</h3>
        <ul class="space-y-2">
          <li v-for="(s, i) in result.suggestions" :key="i" class="flex items-start gap-2 text-sm text-slate-600 dark:text-slate-300">
            <span class="mt-0.5 shrink-0 text-orange-500">•</span><span>{{ s }}</span>
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>
