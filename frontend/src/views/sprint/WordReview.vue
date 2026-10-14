<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { SPRINT_WORDS } from '@/data/sprintWords'

const MASTERED_KEY = 'sprint_words_mastered'

const mode = ref<'list' | 'card'>('card')
const category = ref('全部')
const mastered = ref<Set<string>>(new Set())

const categories = computed(() => ['全部', ...Array.from(new Set(SPRINT_WORDS.map((w) => w.category)))])

const pool = computed(() =>
  category.value === '全部' ? SPRINT_WORDS : SPRINT_WORDS.filter((w) => w.category === category.value),
)

// —— 背诵进度持久化：卡片位置按分类记录，刷新/切换分类后从上次位置继续 ——
const PROGRESS_KEY = 'sprint_words_progress'
const positions = ref<Record<string, number>>({})
const index = ref(0)
const flipped = ref(false)
const revealed = ref(false) // 点击"认识/不认识"后展示释义与例句，需再点"下一个"才前进
// loadProgress 在 mounted 里恢复 category/mode 会触发 watch，用标志跳过首次"存旧分类"逻辑
let initialized = false

function persistProgress() {
  try {
    localStorage.setItem(
      PROGRESS_KEY,
      JSON.stringify({ positions: positions.value, category: category.value, mode: mode.value }),
    )
  } catch {}
}
function loadProgress() {
  try {
    const raw = localStorage.getItem(PROGRESS_KEY)
    if (raw) {
      const saved = JSON.parse(raw) as {
        positions?: Record<string, number>
        category?: string
        mode?: 'list' | 'card'
      }
      positions.value = saved.positions ?? {}
      // 数据变更后旧分类/模式可能失效，用现役列表兜底校验
      if (saved.mode === 'list' || saved.mode === 'card') mode.value = saved.mode
      if (saved.category && categories.value.includes(saved.category)) category.value = saved.category
    }
  } catch {}
}
function applyCategoryIndex() {
  index.value = Math.min(Math.max(positions.value[category.value] ?? 0, 0), Math.max(pool.value.length - 1, 0))
  flipped.value = false
  revealed.value = false
}

function loadMastered() {
  try {
    const raw = localStorage.getItem(MASTERED_KEY)
    mastered.value = new Set(raw ? (JSON.parse(raw) as string[]) : [])
  } catch {}
}
function persist() {
  localStorage.setItem(MASTERED_KEY, JSON.stringify([...mastered.value]))
}

onMounted(() => {
  loadMastered()
  loadProgress()
  applyCategoryIndex()
  initialized = true
})

// 切换分类：先记下刚离开分类的位置，再定位到新分类上次的位置
watch(category, (_val, old) => {
  if (!initialized) return
  positions.value[old] = index.value
  applyCategoryIndex()
  persistProgress()
})

// 切到卡片模式时恢复该分类的位置；切到词表时顺手保存进度
watch(mode, (m) => {
  if (!initialized) return
  if (m === 'card') applyCategoryIndex()
  else persistProgress()
})

const masteredCount = computed(() => pool.value.filter((w) => mastered.value.has(w.word)).length)

function toggleMastered(word: string) {
  if (mastered.value.has(word)) mastered.value.delete(word)
  else mastered.value.add(word)
  mastered.value = new Set(mastered.value)
  persist()
}

const current = computed(() => pool.value[index.value] || null)

// 点击"认识/不认识"：记录掌握状态并展开释义（不直接跳到下一个）
function know(isKnown: boolean) {
  const w = current.value
  if (!w) return
  if (isKnown) mastered.value.add(w.word)
  else mastered.value.delete(w.word)
  mastered.value = new Set(mastered.value)
  persist()
  revealed.value = true
  flipped.value = true
}
function next() {
  flipped.value = false
  revealed.value = false
  if (index.value < pool.value.length - 1) {
    index.value++
    persistProgress()
  } else ElMessage.success('本组已过完一遍！')
}
function prev() {
  flipped.value = false
  revealed.value = false
  if (index.value > 0) {
    index.value--
    persistProgress()
  }
}
</script>

<template>
  <div class="mx-auto max-w-3xl">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-bold text-gray-900 dark:text-gray-100">英语单词</h2>
      <div class="flex items-center gap-1 rounded-lg border border-slate-200 bg-white p-0.5 dark:border-slate-700 dark:bg-slate-800">
        <button class="rounded-md px-3 py-1 text-sm font-medium transition" :class="mode === 'card' ? 'bg-blue-600 text-white' : 'text-slate-500 dark:text-slate-400'" @click="mode = 'card'">卡片背诵</button>
        <button class="rounded-md px-3 py-1 text-sm font-medium transition" :class="mode === 'list' ? 'bg-blue-600 text-white' : 'text-slate-500 dark:text-slate-400'" @click="mode = 'list'">词表</button>
      </div>
    </div>

    <div class="mb-4 flex flex-wrap items-center gap-2">
      <el-select v-model="category" size="small" style="width: 150px">
        <el-option v-for="c in categories" :key="c" :label="c" :value="c" />
      </el-select>
      <span class="text-sm text-slate-500 dark:text-slate-400">已掌握 {{ masteredCount }} / {{ pool.length }}</span>
    </div>

    <!-- 卡片背诵 -->
    <div v-if="mode === 'card'">
      <div v-if="current">
        <button
          class="flex min-h-[220px] w-full flex-col items-center justify-center gap-3 rounded-2xl border border-slate-200 bg-white p-8 text-center shadow-sm transition hover:shadow-md dark:border-slate-800 dark:bg-slate-900"
          @click="flipped = !flipped"
        >
          <div class="text-2xl font-bold text-slate-900 dark:text-white">{{ current.word }}</div>
          <div v-if="current.phonetic" class="font-mono text-sm text-slate-500 dark:text-slate-400">{{ current.phonetic }}</div>
          <template v-if="flipped">
            <div class="text-lg text-blue-600 dark:text-blue-400">{{ current.meaning }}</div>
            <div v-if="current.example" class="mt-1 max-w-lg space-y-1">
              <div class="text-sm italic text-slate-600 dark:text-slate-300">{{ current.example }}</div>
              <div v-if="current.exampleZh" class="text-xs text-slate-400 dark:text-slate-500">{{ current.exampleZh }}</div>
            </div>
          </template>
          <div v-else class="text-sm text-slate-400">点击卡片显示释义</div>
          <el-tag size="small" type="info" class="mt-1">{{ current.category }}</el-tag>
          <span v-if="mastered.has(current.word)" class="text-xs text-emerald-500">✓ 已掌握</span>
        </button>

        <div class="mt-5 flex items-center justify-center gap-3">
          <el-button size="large" @click="prev" :disabled="index === 0">上一个</el-button>
          <template v-if="!revealed">
            <el-button size="large" type="danger" plain @click="know(false)">不认识</el-button>
            <el-button size="large" type="success" @click="know(true)">认识</el-button>
          </template>
          <el-button v-else size="large" type="primary" @click="next">
            {{ index < pool.length - 1 ? '下一个' : '完成' }}
          </el-button>
        </div>
        <div class="mt-3 text-center text-sm text-slate-400">{{ index + 1 }} / {{ pool.length }}</div>
      </div>
      <div v-else class="py-12 text-center text-slate-500">该分类暂无单词</div>
    </div>

    <!-- 词表 -->
    <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <div
        v-for="w in pool"
        :key="w.word"
        class="flex items-center justify-between gap-3 border-b border-slate-100 px-4 py-3 last:border-0 dark:border-slate-800"
      >
        <div class="min-w-0">
          <div class="font-medium text-slate-800 dark:text-slate-100">{{ w.word }}</div>
          <div v-if="w.phonetic" class="font-mono text-sm text-slate-500 dark:text-slate-400">{{ w.phonetic }}</div>
          <div class="text-sm text-slate-500 dark:text-slate-400">{{ w.meaning }}</div>
          <div v-if="w.example" class="mt-1 text-xs italic text-slate-400 dark:text-slate-500">{{ w.example }}</div>
        </div>
        <button
          class="shrink-0 rounded-lg border px-2.5 py-1 text-xs font-medium transition"
          :class="mastered.has(w.word)
            ? 'border-emerald-300 bg-emerald-50 text-emerald-600 dark:border-emerald-500/40 dark:bg-emerald-500/15 dark:text-emerald-300'
            : 'border-slate-200 text-slate-500 hover:border-emerald-300 hover:text-emerald-600 dark:border-slate-700 dark:text-slate-400'"
          @click="toggleMastered(w.word)"
        >
          {{ mastered.has(w.word) ? '✓ 已掌握' : '标记掌握' }}
        </button>
      </div>
    </div>
  </div>
</template>
