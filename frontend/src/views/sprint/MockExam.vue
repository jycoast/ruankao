<script setup lang="ts">
import { reactive, ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { examApi } from '@/api'
import { useSubjectStore } from '@/stores/subject'

const router = useRouter()
const subjectStore = useSubjectStore()

// 各题型抽题数量（上限与后端一致：单选/多选 50、案例 10、论文 5）
const counts = reactive({ single: 20, multiple: 5, caseCount: 2, essay: 0 })

// 年份范围：近 N 年，0=不限
const years = ref(10)
const yearOptions = [
  { label: '近 5 年', value: 5 },
  { label: '近 10 年', value: 10 },
  { label: '近 15 年', value: 15 },
  { label: '不限年份', value: 0 },
]

// 模式：exam=模拟考试(限时交卷出分) practice=练习(逐题即时判题)
const mode = ref<'exam' | 'practice'>('exam')

const typeFields = [
  { field: 'single', label: '单选题', max: 50, hint: '综合知识客观题' },
  { field: 'multiple', label: '多选题', max: 50, hint: '综合知识多选' },
  { field: 'caseCount', label: '案例题', max: 10, hint: '案例分析简答' },
  { field: 'essay', label: '论文题', max: 5, hint: '论文写作' },
] as const

const totalCount = computed(() => counts.single + counts.multiple + counts.caseCount + counts.essay)
const tooMany = computed(() => totalCount.value > 100)
const valid = computed(() => totalCount.value > 0 && !tooMany.value)

const generating = ref(false)

async function generate() {
  if (!valid.value) return
  generating.value = true
  try {
    const res: any = await examApi.generateMock({
      subject_id: subjectStore.currentSubjectId,
      years: years.value,
      single: counts.single,
      multiple: counts.multiple,
      case: counts.caseCount,
      essay: counts.essay,
    })
    const paperId = res.data?.paper_id
    if (!paperId) {
      ElMessage.error('生成失败，请稍后重试')
      return
    }
    // 复用现有考试/练习会话：模拟考试限时交卷，练习模式逐题判题
    router.push(mode.value === 'practice' ? `/exam/${paperId}?mode=practice` : `/exam/${paperId}`)
  } finally {
    generating.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-3xl">
    <!-- 头图 -->
    <section class="relative mb-6 overflow-hidden rounded-2xl bg-gradient-to-br from-blue-600 to-indigo-600 p-6 text-white shadow-sm">
      <div class="relative z-10">
        <div class="text-lg font-semibold">📝 生成模拟卷</div>
        <p class="mt-1.5 max-w-xl text-sm text-blue-50">
          从历年真题中随机抽取各题型组卷，模拟真实考试环境，检验冲刺成果。
        </p>
      </div>
      <el-icon class="pointer-events-none absolute -right-4 -bottom-4 text-white/10" :size="140"><Tickets /></el-icon>
    </section>

    <div class="space-y-5 rounded-xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:p-6">
      <!-- 年份范围 -->
      <div>
        <div class="mb-2 text-sm font-medium text-slate-700 dark:text-slate-200">抽题范围</div>
        <el-radio-group v-model="years" size="default">
          <el-radio-button v-for="o in yearOptions" :key="o.value" :value="o.value">{{ o.label }}</el-radio-button>
        </el-radio-group>
      </div>

      <!-- 各题型数量 -->
      <div>
        <div class="mb-2 text-sm font-medium text-slate-700 dark:text-slate-200">题型与数量</div>
        <div class="grid gap-3 sm:grid-cols-2">
          <div
            v-for="f in typeFields"
            :key="f.label"
            class="flex items-center justify-between gap-3 rounded-lg border border-slate-200 px-3 py-2.5 dark:border-slate-700"
          >
            <div class="min-w-0">
              <div class="text-sm font-medium text-slate-800 dark:text-slate-100">{{ f.label }}</div>
              <div class="text-xs text-slate-400">{{ f.hint }} · 上限 {{ f.max }}</div>
            </div>
            <el-input-number v-model="counts[f.field]" :min="0" :max="f.max" size="small" controls-position="right" style="width: 110px" />
          </div>
        </div>
        <p v-if="tooMany" class="mt-2 text-xs text-red-500">题量合计 {{ totalCount }} 题，已超过单卷上限 100 题，请减少。</p>
        <p v-else class="mt-2 text-xs text-slate-400">合计 {{ totalCount }} 题。若某题型题库不足，会按实际可用数量抽取。</p>
      </div>

      <!-- 模式 -->
      <div>
        <div class="mb-2 text-sm font-medium text-slate-700 dark:text-slate-200">答题模式</div>
        <el-radio-group v-model="mode" size="default">
          <el-radio-button value="exam">模拟考试（限时 · 交卷出分）</el-radio-button>
          <el-radio-button value="practice">练习模式（逐题看解析）</el-radio-button>
        </el-radio-group>
      </div>

      <div class="flex items-center justify-end gap-3 border-t border-slate-100 pt-4 dark:border-slate-800">
        <el-button @click="router.push('/sprint')">返回</el-button>
        <el-button type="primary" :loading="generating" :disabled="!valid" @click="generate">
          生成并开始
        </el-button>
      </div>
    </div>

    <p class="mt-4 text-center text-xs text-slate-400">
      每次生成会替换你上一份模拟卷，历史成绩记录仍会保留在学习报告中。
    </p>
  </div>
</template>
