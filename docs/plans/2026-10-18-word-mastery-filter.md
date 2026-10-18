# 英语单词按掌握状态筛选 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `/sprint/words` 单词页增加「全部 / 未掌握 / 已掌握」三态筛选，让用户能只背未掌握的单词。

**Architecture:** 过滤逻辑抽成纯函数 `frontend/src/utils/wordFilter.ts`（分类 × 掌握状态取交集、进度键读取与旧数据回退），配 vitest 单测；`WordReview.vue` 只负责 UI 与状态。词表模式实时过滤；卡片模式用「本遍卡组快照」，点「认识」不跳词，走完一遍或用「重新开始」时才按最新掌握状态重建。**后端、数据库、接口一律不动**——掌握状态本来就只存在浏览器 `localStorage`。

**Tech Stack:** Vue 3 + TypeScript（`strict: false`）+ vue-tsc + Vite 6 + Element Plus 2.9 + vitest 5（`environment: 'node'`，无组件测试能力）

## Global Constraints

- 无后端改动、无数据库结构变更、无接口变更；掌握状态继续存 `localStorage` 的 `sprint_words_mastered`（单词字符串数组）。
- 筛选维度固定为三态：`'all' | 'unmastered' | 'mastered'`，文案「全部 / 未掌握 / 已掌握」。
- 分类为 `'全部'` 表示不按分类过滤；分类与掌握状态**取交集**。
- 统计展示固定为「已掌握 X / Y」，**X、Y 都基于"只按分类过滤"的词集**，不随掌握筛选变化（否则「未掌握」下分子恒为 0）。
- 卡片模式使用**本遍卡组快照**：一遍之内点「认识」只写掌握状态、不从卡组移除；重建只发生在进入卡片模式、切换分类、切换掌握筛选、点「重新开始」时。
- 进度位置键格式固定为 `` `${category}::${mastery}` ``；`mastery = 'all'` 且新键缺失时回退旧键 `positions[category]`，其余筛选不回退。
- `localStorage` 键名不变：`sprint_words_mastered`、`sprint_words_progress`；后者新增 `mastery` 字段，非法值回落 `'all'`。
- 不改 `frontend/src/data/sprintWords.ts` 的词库内容。
- 提交信息用中文，与仓库既有风格一致。
- CI 的 frontend job 已跑 `npm test`（提交 `0c1a459`），新单测自动上闸门，无需改 CI。
- 本仓库 vitest 为 `environment: 'node'` 且无 `@vue/test-utils`/jsdom，**不引入组件测试环境**；组件行为靠类型检查 + 人工浏览器核对。

---

## 文件结构

| 文件 | 职责 | 动作 |
|---|---|---|
| `frontend/src/utils/wordFilter.ts` | 筛选规则与进度键读取的唯一实现 | 新建 |
| `frontend/src/utils/wordFilter.test.ts` | 钉死三态过滤、分类交集、旧键回退 | 新建 |
| `frontend/src/views/sprint/WordReview.vue` | 筛选控件、卡组快照、完成面板、空状态、进度持久化 | 修改 |

规则集中在纯函数里，组件只调用——将来筛选语义要调，只有一个地方要改。

---

### Task 1: 筛选纯函数与单测

**Files:**
- Create: `frontend/src/utils/wordFilter.ts`
- Test: `frontend/src/utils/wordFilter.test.ts`

**Interfaces:**
- Consumes: 无（不依赖任何前序改动）
- Produces:
  - `export type MasteryFilter = 'all' | 'unmastered' | 'mastered'`
  - `export function positionKey(category: string, mastery: MasteryFilter): string` —— 返回 `` `${category}::${mastery}` ``
  - `export function filterWords<T extends { word: string; category: string }>(words: readonly T[], opts: { category: string; mastery: MasteryFilter; mastered: ReadonlySet<string> }): T[]`
  - `export function pickSavedIndex(positions: Record<string, number>, category: string, mastery: MasteryFilter): number`

- [ ] **Step 1: 写失败的单测**

创建 `frontend/src/utils/wordFilter.test.ts`：

```ts
import { describe, expect, it } from 'vitest'
import { filterWords, pickSavedIndex, positionKey } from '@/utils/wordFilter'

interface W {
  word: string
  category: string
}

// 三种分类、5 个词，足够覆盖「分类 × 掌握状态」的所有组合
const WORDS: W[] = [
  { word: 'architecture', category: '架构设计' },
  { word: 'component', category: '架构设计' },
  { word: 'scalability', category: '质量属性' },
  { word: 'latency', category: '质量属性' },
  { word: 'agile', category: '软件工程' },
]

const masteredOf = (...words: string[]) => new Set(words)
const words = (list: W[]) => list.map((w) => w.word)

describe('filterWords —— 分类 × 掌握状态', () => {
  it("category = '全部' + mastery = 'all'：原样返回", () => {
    const out = filterWords(WORDS, { category: '全部', mastery: 'all', mastered: new Set() })
    expect(words(out)).toEqual(['architecture', 'component', 'scalability', 'latency', 'agile'])
  })

  it("category = '全部'：只按掌握状态过滤", () => {
    const out = filterWords(WORDS, {
      category: '全部',
      mastery: 'unmastered',
      mastered: masteredOf('component', 'latency'),
    })
    expect(words(out)).toEqual(['architecture', 'scalability', 'agile'])
  })

  it('具体分类 + all：等价于只按分类过滤', () => {
    const out = filterWords(WORDS, { category: '质量属性', mastery: 'all', mastered: new Set() })
    expect(words(out)).toEqual(['scalability', 'latency'])
  })

  it('分类与掌握状态取交集', () => {
    const out = filterWords(WORDS, {
      category: '架构设计',
      mastery: 'unmastered',
      mastered: masteredOf('architecture'),
    })
    expect(words(out)).toEqual(['component'])
  })

  it('mastered：只保留已掌握的词', () => {
    const out = filterWords(WORDS, {
      category: '全部',
      mastery: 'mastered',
      mastered: masteredOf('architecture', 'agile'),
    })
    expect(words(out)).toEqual(['architecture', 'agile'])
  })

  it('空词库 → 空数组', () => {
    expect(filterWords([], { category: '全部', mastery: 'mastered', mastered: masteredOf('architecture') })).toEqual([])
  })

  it('筛选结果为空 → 空数组，不抛错', () => {
    expect(
      words(filterWords(WORDS, { category: '架构设计', mastery: 'mastered', mastered: new Set() })),
    ).toEqual([])
    expect(
      words(filterWords(WORDS, { category: '不存在的分类', mastery: 'all', mastered: new Set() })),
    ).toEqual([])
  })

  it('保持入参顺序', () => {
    const out = filterWords(WORDS, { category: '全部', mastery: 'all', mastered: new Set() })
    expect(words(out)).toEqual(words(WORDS))
  })

  it('不修改入参', () => {
    const input = WORDS.map((w) => ({ ...w }))
    const mastered = masteredOf('architecture')
    filterWords(input, { category: '全部', mastery: 'unmastered', mastered })
    expect(input).toEqual(WORDS)
    expect([...mastered]).toEqual(['architecture'])
  })

  it('掌握集合含词库外的词：不影响结果', () => {
    const unmastered = filterWords(WORDS, {
      category: '全部',
      mastery: 'unmastered',
      mastered: masteredOf('obsolete'),
    })
    expect(words(unmastered)).toEqual(words(WORDS))
    const mastered = filterWords(WORDS, {
      category: '全部',
      mastery: 'mastered',
      mastered: masteredOf('obsolete'),
    })
    expect(mastered).toEqual([])
  })
})

describe('pickSavedIndex —— 进度位置读取', () => {
  it('新键优先', () => {
    const positions = { '架构设计::unmastered': 7, '架构设计::all': 3 }
    expect(pickSavedIndex(positions, '架构设计', 'unmastered')).toBe(7)
  })

  it('mastery = all 且新键缺失：回退旧版按分类存的键', () => {
    expect(pickSavedIndex({ 架构设计: 5 }, '架构设计', 'all')).toBe(5)
  })

  it('unmastered / mastered 且新键缺失：不回退旧键，返回 0', () => {
    expect(pickSavedIndex({ 架构设计: 5, '架构设计::all': 3 }, '架构设计', 'unmastered')).toBe(0)
    expect(pickSavedIndex({ 架构设计: 5 }, '架构设计', 'mastered')).toBe(0)
  })

  it('都缺失：返回 0', () => {
    expect(pickSavedIndex({}, '架构设计', 'all')).toBe(0)
  })

  it('新键值为 0 时仍以新键为准（不能当缺失处理）', () => {
    expect(pickSavedIndex({ '架构设计::all': 0, 架构设计: 9 }, '架构设计', 'all')).toBe(0)
  })
})

describe('positionKey', () => {
  it('与 pickSavedIndex 使用同一键格式', () => {
    expect(positionKey('架构设计', 'unmastered')).toBe('架构设计::unmastered')
    expect(pickSavedIndex({ [positionKey('架构设计', 'unmastered')]: 4 }, '架构设计', 'unmastered')).toBe(4)
  })
})
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd frontend && npx vitest run src/utils/wordFilter.test.ts`
Expected: FAIL —— `Failed to resolve import "@/utils/wordFilter"`

- [ ] **Step 3: 写实现**

创建 `frontend/src/utils/wordFilter.ts`：

```ts
export type MasteryFilter = 'all' | 'unmastered' | 'mastered'

/** 进度键：卡片位置按「分类::掌握筛选」分别记录。 */
export function positionKey(category: string, mastery: MasteryFilter): string {
  return `${category}::${mastery}`
}

/**
 * 单词筛选：分类与掌握状态取交集。
 * category 为「全部」表示不按分类过滤；mastery 为 all 时不过滤掌握状态。
 */
export function filterWords<T extends { word: string; category: string }>(
  words: readonly T[],
  opts: { category: string; mastery: MasteryFilter; mastered: ReadonlySet<string> },
): T[] {
  const { category, mastery, mastered } = opts
  return words.filter((w) => {
    if (category !== '全部' && w.category !== category) return false
    if (mastery === 'unmastered') return !mastered.has(w.word)
    if (mastery === 'mastered') return mastered.has(w.word)
    return true
  })
}

/**
 * 读取某筛选组合上次的卡片位置。
 * 新键优先；mastery = all 时回退旧版按分类存的键，老用户进度不丢。
 */
export function pickSavedIndex(
  positions: Record<string, number>,
  category: string,
  mastery: MasteryFilter,
): number {
  const key = positionKey(category, mastery)
  if (typeof positions[key] === 'number') return positions[key]
  if (mastery === 'all' && typeof positions[category] === 'number') return positions[category]
  return 0
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd frontend && npx vitest run src/utils/wordFilter.test.ts`
Expected: PASS —— 16 个用例全绿

- [ ] **Step 5: 跑全量单测确认没有打破既有用例**

Run: `cd frontend && npm test`
Expected: PASS —— 4 个测试文件、55 个用例（改动前基线为 3 个文件、39 个用例）

- [ ] **Step 6: 提交**

```bash
git add frontend/src/utils/wordFilter.ts frontend/src/utils/wordFilter.test.ts
git commit -m "feat(frontend): 新增单词筛选纯函数 wordFilter

按分类与掌握状态取交集，并提供进度键读取：新键优先，
mastery=all 时回退旧版按分类存的位置，老用户刷新后进度不丢。"
```

---

### Task 2: WordReview 接入三态筛选

**Files:**
- Modify: `frontend/src/views/sprint/WordReview.vue`（整体替换 `<script setup>` 与 `<template>`）

**Interfaces:**
- Consumes: Task 1 的 `filterWords`、`pickSavedIndex`、`positionKey`、`MasteryFilter`
- Produces: 无对外导出（页面组件）

**关于验证方式：** 本仓库 vitest 为 `environment: 'node'` 且没有组件测试依赖，本任务的自动化闸门只有 `vue-tsc` 类型检查；渲染与交互靠 Step 3 的人工浏览器核对。`npm run build` 会要求 `VITE_API_BASE_URL`（生产构建强校验），本地类型检查用 `npx vue-tsc -b` 即可。

- [ ] **Step 1: 整体替换 WordReview.vue**

把 `frontend/src/views/sprint/WordReview.vue` 的全文替换为：

```vue
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

// 按当前分类 × 掌握筛选重建卡组，并恢复该组合上次的位置（按新长度 clamp）
function rebuildDeck() {
  deck.value = pool.value.slice()
  const saved = pickSavedIndex(positions.value, category.value, mastery.value)
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

// 切换分类 / 掌握筛选：先记下旧组合的位置，再按新组合恢复
function onFilterChange(oldCategory: string, oldMastery: MasteryFilter) {
  savePosition(oldCategory, oldMastery)
  if (mode.value === 'card') rebuildDeck()
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
  rebuildDeck()
  index.value = 0
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
```

- [ ] **Step 2: 类型检查**

Run: `cd frontend && npx vue-tsc -b`
Expected: PASS —— 无类型错误（`npm run build` 需要生产 `VITE_API_BASE_URL`，本步只跑类型检查）

- [ ] **Step 3: 跑全量单测**

Run: `cd frontend && npm test`
Expected: PASS —— 4 个文件、55 个用例

- [ ] **Step 4: 人工浏览器核对**

`cd frontend && npm run dev` 后打开 `http://localhost:3000/sprint/words`（需登录；若本地无账号，用测试账号登录）。逐条核对：

| 场景 | 预期 |
|---|---|
| 默认（全部 + 卡片） | 与改动前一致；「已掌握 X / Y」中的 Y 为全词库数 |
| 选「未掌握」 | 卡组只剩未掌握词，计数第几位 / 卡组长度随之变化 |
| 卡片下点「认识」 | 立即显示释义，卡片**不跳**；统计数字 +1；点「下一个」才前进 |
| 走到卡组最后一张点「完成」 | 动作区出现「本组已过完一遍 🎉」+「上一个 / 重新开始」 |
| 点「重新开始」 | 卡组按最新掌握状态重建、从第 1 张开始；刚点「认识」的词不再出现 |
| 在「未掌握」下把所有词点「认识」后重新开始 | 显示「当前分类的单词都已掌握 🎉」 |
| 选「已掌握」 | 只出现已掌握词；点「不认识」后本遍仍在，点「重新开始」后消失 |
| 「已掌握」且该分类没有已掌握词 | 显示「当前分类还没有已掌握的单词」 |
| 词表 + 「未掌握」 | 点某行「标记掌握」，该行**立即消失**；切到「已掌握」能找到它 |
| 词表筛选后列表为空 | 显示与卡片模式一致的空状态文案 |
| 分类 × 筛选各自记忆位置 | 在 A 分类背到第 5 张，切到 B 分类再切回 A（筛选相同），仍停在第 5 张；切到「未掌握」再切回「全部」，各自位置独立 |
| 刷新页面 | 分类、掌握筛选、卡片/词表模式、**当前卡片位置**全部恢复（`next`/`prev` 已把索引写入进度） |
| 旧数据兼容 | 浏览器控制台先执行 `localStorage.setItem('sprint_words_progress', JSON.stringify({ positions: { '架构设计': 3 }, category: '架构设计', mode: 'card' }))` 再刷新：停在「架构设计 · 全部」第 4 张（旧键回退生效） |
| 统计口径 | 切到「未掌握」后「已掌握 X / Y」分子**不为 0**，且与「全部」下同一分类的数字一致 |

- [ ] **Step 5: 提交**

```bash
git add frontend/src/views/sprint/WordReview.vue
git commit -m "feat(frontend): 单词页增加全部/未掌握/已掌握筛选

词表模式实时过滤；卡片模式用本遍卡组快照，点「认识」不跳词，
走完一遍点「重新开始」才按最新掌握状态重建。进度位置按
「分类::筛选」分别记忆，mastery=all 回退旧键，老用户进度不丢。"
```

---

## 计划自查

**Spec 覆盖：**

| Spec 章节 | 对应任务 |
|---|---|
| 决策 1 过滤纯函数 + `pickSavedIndex` | Task 1 Step 3 |
| 决策 2 `categoryPool` / `pool` 两个 computed + 统计口径 | Task 2 Step 1（script） |
| 决策 3 卡片卡组快照 | Task 2 Step 1（`deck` / `rebuildDeck` / `know`） |
| 决策 4 完成面板 + 重新开始 | Task 2 Step 1（`finished` / `restart`）；`prev` 复位可回看上一张 |
| 决策 5 进度持久化（键格式、`mastery` 字段、旧键回退、clamp） | Task 1 `positionKey`/`pickSavedIndex` + Task 2 `loadProgress`/`savePosition`/`rebuildDeck` |
| 决策 6 筛选控件 | Task 2 Step 1（template `el-radio-group`） |
| 决策 7 空状态三文案 + 词表空状态 | Task 2 Step 1（`emptyText`） |
| 边界情况 1–5 | Task 2 Step 4 人工核对表 |
| 测试一节 | Task 1 Step 1 |
| 明确不做（不上云、不动词库、不重置） | 全计划未涉及 |

**类型一致性：** Task 1 产出的 `filterWords` / `pickSavedIndex` / `positionKey` / `MasteryFilter` 在 Task 2 的 import 与调用处签名逐字一致；进度键在 util 与组件中都只经 `positionKey` 生成，无第二处手拼。

**已知验证缺口（不掩饰）：** Task 2 的渲染与交互没有自动化测试——仓库 vitest 是 `environment: 'node'` 且无组件测试依赖，引入它们超出本设计范围。因此以 `vue-tsc` 类型检查 + Step 4 的人工核对记录为准。

**与 spec 的一处有意偏离（已并入 Task 2）：** 旧实现的 `next()`/`prev()` 只持久化 `positions` 对象、不写当前 `index`，导致同分类内刷新会丢卡片位置——这与现有注释声称的「刷新后从上次位置继续」不符。本计划在 `next`/`prev`/离开卡片模式时补 `savePosition()`。这是让 spec「进度按组合记忆、刷新可恢复」成立的必要修复，不影响其余行为。spec 未单列此条，执行时以本计划为准。

**顺序依赖：** Task 2 依赖 Task 1 的导出；Task 1 必须先做。

**CI：** frontend job 已跑 `npm test`，Task 1 的单测自动上闸门，本计划无需改 CI。
