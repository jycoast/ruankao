<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { SPRINT_WORDS, type Word } from '@/data/sprintWords'
import { filterWords, pickSavedIndex, positionKey, type MasteryFilter } from '@/utils/wordFilter'

const MASTERED_KEY = 'sprint_words_mastered'

const mode = ref<'list' | 'card'>('card')
const category = ref('全部')
const mastery = ref<MasteryFilter>('all')
const mastered = ref<Set<string>>(new Set())

const categories = computed(() => ['全部', ...Array.from(new Set(SPRINT_WORDS.map((w) => w.category)))])

// 只按分类过滤：统计口径的分母是当前分类总数、分子是其中已掌握数，
// 不随「全部/未掌握/已掌握」变化，否则切到「未掌握」时分子恒为 0。
const categoryPool = computed(() =>
  filterWords(SPRINT_WORDS, { category: category.value, mastery: 'all', mastered: mastered.value }),
)

// 词表模式的实时数据源：分类 × 掌握状态
const pool = computed(() =>
  filterWords(SPRINT_WORDS, { category: category.value, mastery: mastery.value, mastered: mastered.value }),
)

// 卡片模式的本遍卡组快照：一遍之内点「认识」不缩水，
// 走完一遍或切换筛选时才按最新掌握状态重建。
const deck = ref<Word[]>([])

// —— 背诵进度持久化：卡片位置按「分类::掌握筛选」记录 ——
const PROGRESS_KEY = 'sprint_words_progress'
const positions = ref<Record<string, number>>({})
const index = ref(0)
const flipped = ref(false)
const revealed = ref(false) // 点击"认识/不认识"后展示释义与例句，需再点"下一个"才前进
const finished = ref(false) // 一遍走完：动作区换成完成提示与「重新开始」
// loadProgress 在 mounted 里恢复筛选会触发两处 watch，用标志跳过首次"存旧筛选"。
// 这两个 watch 必须 flush: 'sync'——默认的 pre-flush 回调排在 mounted 之后才执行，
// 那时 initialized 已置 true，标志就形同虚设，会把刚恢复的位置写进错误的键。
let initialized = false

function persistProgress() {
  try {
    localStorage.setItem(
      PROGRESS_KEY,
      JSON.stringify({
        positions: positions.value,
        category: category.value,
        mode: mode.value,
        mastery: mastery.value,
      }),
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
        mastery?: MasteryFilter
      }
      positions.value = saved.positions ?? {}
      // 数据变更后旧分类/模式/筛选可能失效，用现役列表兜底校验
      if (saved.mode === 'list' || saved.mode === 'card') mode.value = saved.mode
      if (saved.category && categories.value.includes(saved.category)) category.value = saved.category
      if (saved.mastery === 'all' || saved.mastery === 'unmastered' || saved.mastery === 'mastered') {
        mastery.value = saved.mastery
      }
    }
  } catch {}
}

// 记下某个筛选组合下的卡片位置。切筛选时旧值必须显式传入——
// watch 回调里 ref 已经变成新值了。
function savePosition(cat = category.value, m = mastery.value) {
  positions.value[positionKey(cat, m)] = index.value
}

// 按当前分类 × 掌握筛选重建卡组，并恢复该组合上次的位置（按新长度 clamp）。
// restart 传入 0 直接从头开始，省掉一次位置查询。
function rebuildDeck(forcedIndex?: number) {
  deck.value = pool.value.slice()
  const saved = forcedIndex ?? pickSavedIndex(positions.value, category.value, mastery.value)
  index.value = Math.min(Math.max(saved, 0), Math.max(deck.value.length - 1, 0))
}

function resetCardState() {
  flipped.value = false
  revealed.value = false
  finished.value = false
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
  rebuildDeck()
  initialized = true
})

// 切换分类 / 掌握筛选：仅在卡片模式下记下旧组合的位置并按新组合恢复。
// 词表模式下 index 并未指向新组合，此时写位置会把陈旧索引记到别的键下；
// 卡组等切回卡片模式时再按当时的筛选重建。
function onFilterChange(oldCategory: string, oldMastery: MasteryFilter) {
  if (mode.value === 'card') {
    savePosition(oldCategory, oldMastery)
    rebuildDeck()
  }
  resetCardState()
  persistProgress()
}

watch(
  category,
  (_val, old) => {
    if (!initialized) return
    onFilterChange(old, mastery.value)
  },
  { flush: 'sync' },
)

watch(
  mastery,
  (_val, old) => {
    if (!initialized) return
    onFilterChange(category.value, old)
  },
  { flush: 'sync' },
)

// 切到卡片模式时恢复该筛选组合的位置；切到词表时先把当前位置写进进度
watch(mode, (m) => {
  if (!initialized) return
  if (m === 'card') {
    rebuildDeck()
    resetCardState()
  } else {
    savePosition()
  }
  persistProgress()
})

const masteredCount = computed(() => categoryPool.value.filter((w) => mastered.value.has(w.word)).length)

function toggleMastered(word: string) {
  if (mastered.value.has(word)) mastered.value.delete(word)
  else mastered.value.add(word)
  mastered.value = new Set(mastered.value)
  persist()
}

const current = computed(() => deck.value[index.value] || null)

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
  if (index.value < deck.value.length - 1) {
    index.value++
  } else {
    finished.value = true
  }
  // 旧实现只持久化 positions 对象、不写当前 index，刷新会丢位置；
  // 这里先 savePosition 再存，刷新后能接着上一张背。
  savePosition()
  persistProgress()
}
function prev() {
  flipped.value = false
  revealed.value = false
  finished.value = false
  if (index.value > 0) {
    index.value--
    savePosition()
    persistProgress()
  }
}

// 一遍走完：按最新掌握状态重建卡组从头再来（「未掌握」下已掌握的词就此退出）
function restart() {
  rebuildDeck(0)
  resetCardState()
  savePosition()
  persistProgress()
}

const emptyText = computed(() => {
  if (mastery.value === 'unmastered') return '当前分类的单词都已掌握 🎉'
  if (mastery.value === 'mastered') return '当前分类还没有已掌握的单词'
  return '该分类暂无单词'
})
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
      <el-radio-group v-model="mastery" size="small">
        <el-radio-button value="all">全部</el-radio-button>
        <el-radio-button value="unmastered">未掌握</el-radio-button>
        <el-radio-button value="mastered">已掌握</el-radio-button>
      </el-radio-group>
      <span class="text-sm text-slate-500 dark:text-slate-400">已掌握 {{ masteredCount }} / {{ categoryPool.length }}</span>
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

        <!-- 一遍走完：动作区换成完成提示与「重新开始」 -->
        <div v-if="finished" class="mt-5 flex flex-col items-center gap-3 text-center">
          <div class="text-sm font-medium text-emerald-600 dark:text-emerald-400">本组已过完一遍 🎉</div>
          <div class="flex items-center justify-center gap-3">
            <el-button size="large" @click="prev" :disabled="index === 0">上一个</el-button>
            <el-button size="large" type="primary" @click="restart">重新开始</el-button>
          </div>
        </div>
        <div v-else class="mt-5 flex items-center justify-center gap-3">
          <el-button size="large" @click="prev" :disabled="index === 0">上一个</el-button>
          <template v-if="!revealed">
            <el-button size="large" type="danger" plain @click="know(false)">不认识</el-button>
            <el-button size="large" type="success" @click="know(true)">认识</el-button>
          </template>
          <el-button v-else size="large" type="primary" @click="next">
            {{ index < deck.length - 1 ? '下一个' : '完成' }}
          </el-button>
        </div>
        <div class="mt-3 text-center text-sm text-slate-400">{{ index + 1 }} / {{ deck.length }}</div>
      </div>
      <div v-else class="py-12 text-center text-slate-500">{{ emptyText }}</div>
    </div>

    <!-- 词表 -->
    <div v-else>
      <div v-if="pool.length" class="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
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
      <div v-else class="rounded-xl border border-slate-200 bg-white py-12 text-center text-slate-500 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        {{ emptyText }}
      </div>
    </div>
  </div>
</template>
