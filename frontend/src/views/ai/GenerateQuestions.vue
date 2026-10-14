<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { aiApi, questionApi, knowledgeApi } from '@/api'
import { useUserStore } from '@/stores/user'
import { useSubjectStore } from '@/stores/subject'

const router = useRouter()
const userStore = useUserStore()
const subjectStore = useSubjectStore()
const isAdmin = () => userStore.user?.role === 1

interface GenOpt { label: string; content: string; is_correct: boolean }
interface GenQ { title: string; options: GenOpt[]; answer: string; explanation: string; knowledge_point: string; _saved?: boolean }

const knowledgePoint = ref('')
const count = ref(3)
const loading = ref(false)
const questions = ref<GenQ[]>([])
const revealed = ref<Record<number, boolean>>({})

// ---- 常用知识点：取当前科目的知识点列表，一键选填 ----
type KnowledgePoint = { id: number; name: string }
const knowledgePoints = ref<KnowledgePoint[]>([])
const kpLoading = ref(false)
const kpKeyword = ref('')

const filteredKps = computed(() => {
  const kw = kpKeyword.value.trim().toLowerCase()
  if (!kw) return knowledgePoints.value
  return knowledgePoints.value.filter(kp => kp.name.toLowerCase().includes(kw))
})

async function loadKnowledgePoints() {
  kpLoading.value = true
  try {
    const items = await knowledgeApi.listAll(subjectStore.currentSubjectId)
    knowledgePoints.value = items.map((kp: any) => ({ id: kp.id, name: kp.name }))
  } catch {
    knowledgePoints.value = []
  } finally {
    kpLoading.value = false
  }
}

function selectKp(name: string) {
  knowledgePoint.value = name
  generate()
}

onMounted(loadKnowledgePoints)

async function generate() {
  if (!knowledgePoint.value.trim()) {
    ElMessage.warning('请填写知识点')
    return
  }
  loading.value = true
  questions.value = []
  revealed.value = {}
  try {
    const res: any = await aiApi.generateQuestions({ knowledge_point: knowledgePoint.value.trim(), count: count.value })
    questions.value = res.data.questions || []
    if (!questions.value.length) ElMessage.info('未生成题目，请调整知识点重试')
  } catch {
    // 拦截器已提示
  } finally {
    loading.value = false
  }
}

async function saveToBank(q: GenQ, idx: number) {
  try {
    await questionApi.create({
      title: q.title,
      content: '',
      question_type: 1,
      knowledge_point: q.knowledge_point || knowledgePoint.value,
      explanation: q.explanation,
      options: q.options.map((o, i) => ({
        label: o.label || String.fromCharCode(65 + i),
        content: o.content,
        is_correct: o.is_correct ? 1 : 0,
      })),
    })
    q._saved = true
    ElMessage.success('已存入题库')
  } catch {
    // 拦截器已提示
  }
  void idx
}
</script>

<template>
  <div class="mx-auto max-w-3xl">
    <button class="mb-3 flex items-center gap-1 text-sm text-slate-500 transition hover:text-blue-600 dark:text-slate-400" @click="router.push('/sprint')">
      <el-icon><ArrowLeft /></el-icon> 返回冲刺
    </button>
    <h2 class="mb-1 text-xl font-bold text-gray-900 dark:text-gray-100">AI 出题</h2>
    <p class="mb-4 text-sm text-slate-500 dark:text-slate-400">按知识点生成单项选择练习题</p>

    <div class="mb-6 rounded-xl border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <div class="flex flex-wrap items-end gap-3">
        <div class="min-w-0 flex-1">
          <label class="mb-1 block text-sm text-slate-600 dark:text-slate-300">知识点</label>
          <input v-model="knowledgePoint" placeholder="如：数据结构、软件工程、操作系统、设计模式…" class="w-full rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm focus:border-blue-400 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100" @keyup.enter="generate" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-slate-600 dark:text-slate-300">数量</label>
          <el-input-number v-model="count" :min="1" :max="10" size="default" />
        </div>
        <button class="rounded-lg bg-blue-600 px-5 py-2 text-sm font-medium text-white transition hover:bg-blue-700 disabled:opacity-50" :disabled="loading" @click="generate">
          {{ loading ? '生成中…' : '生成题目' }}
        </button>
      </div>

      <!-- 常用知识点：从题库知识点一键选择 -->
      <div class="mt-4 border-t border-slate-100 pt-3 dark:border-slate-800">
        <div class="mb-2 flex items-center justify-between gap-2">
          <span class="text-sm text-slate-600 dark:text-slate-300">常用知识点</span>
          <input
            v-model="kpKeyword"
            placeholder="搜索知识点…"
            class="w-40 rounded-lg border border-slate-200 bg-slate-50 px-2 py-1 text-sm focus:border-blue-400 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          />
        </div>
        <div v-loading="kpLoading" class="flex flex-wrap gap-2">
          <button
            v-for="kp in filteredKps"
            :key="kp.id"
            class="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-sm text-slate-700 transition hover:border-blue-400 hover:bg-blue-50 hover:text-blue-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:border-blue-500 dark:hover:bg-blue-500/15 dark:hover:text-blue-300"
            @click="selectKp(kp.name)"
          >
            {{ kp.name }}
          </button>
          <span v-if="!kpLoading && filteredKps.length === 0" class="text-sm text-slate-400 dark:text-slate-500">
            {{ knowledgePoints.length === 0 ? '暂无常用知识点' : '无匹配知识点' }}
          </span>
        </div>
      </div>
    </div>

    <div v-loading="loading" class="space-y-4">
      <div v-for="(q, i) in questions" :key="i" class="rounded-xl border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div class="mb-3 flex items-start gap-2">
          <span class="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-blue-50 text-xs font-medium text-blue-600 dark:bg-blue-500/15 dark:text-blue-300">{{ i + 1 }}</span>
          <div class="font-medium text-slate-900 dark:text-white">{{ q.title }}</div>
        </div>
        <div class="space-y-2 pl-8">
          <div
            v-for="(o, oi) in q.options"
            :key="oi"
            class="rounded-lg border px-3 py-2 text-sm"
            :class="revealed[i] && o.is_correct
              ? 'border-emerald-400 bg-emerald-50 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
              : 'border-slate-200 text-slate-700 dark:border-slate-700 dark:text-slate-200'"
          >
            <span class="font-medium">{{ o.label || String.fromCharCode(65 + oi) }}.</span> {{ o.content }}
          </div>
        </div>
        <div class="mt-3 flex flex-wrap items-center gap-3 pl-8">
          <button class="text-sm font-medium text-blue-600 hover:underline dark:text-blue-400" @click="revealed[i] = !revealed[i]">
            {{ revealed[i] ? '隐藏答案' : '显示答案解析' }}
          </button>
          <button v-if="isAdmin()" class="text-sm font-medium text-emerald-600 hover:underline disabled:opacity-50 dark:text-emerald-400" :disabled="q._saved" @click="saveToBank(q, i)">
            {{ q._saved ? '✓ 已入库' : '存入题库' }}
          </button>
        </div>
        <div v-if="revealed[i]" class="mt-2 rounded-lg bg-slate-50 p-3 pl-8 text-sm text-slate-600 dark:bg-slate-800 dark:text-slate-300">
          <div class="font-medium text-emerald-600 dark:text-emerald-400">答案：{{ q.answer }}</div>
          <div class="mt-1 whitespace-pre-wrap">{{ q.explanation }}</div>
        </div>
      </div>
    </div>
  </div>
</template>
