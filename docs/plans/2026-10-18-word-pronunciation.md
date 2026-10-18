# 英语单词读音播放 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在单词页的卡片与词表里，每个单词旁加一个喇叭按钮，点击用浏览器语音合成朗读该单词。

**Architecture:** 选音色逻辑抽成可单测的纯函数 `pickEnglishVoice`；用户交互封装在 `useSpeech` composable 里；UI 是一个只接收 `text` 的 `SpeakerButton` 内联 SVG 组件，接进 `WordReview.vue` 的卡片模式和词表模式。

**Tech Stack:** Vue 3 + TypeScript + Vite 6 + Element Plus 2.14 + vitest 5（`environment: 'node'`）+ Web Speech API

## Global Constraints

- **纯前端**：无接口变更、无数据库变更、无新依赖。
- 发音来源只用浏览器 `window.speechSynthesis`，**不引入在线词典音频、不打包 mp3**。
- **不做自动朗读、不做全局静音开关**；只读单词本身，不读例句/释义/音标。
- 词库 `frontend/src/data/sprintWords.ts` 不改。
- 不改 `/sprint/words` 之外的路由或页面。
- 单测运行命令：`npm test`（工作目录 `frontend/`）；类型检查：`npx vue-tsc -b`。
- 中文注释，风格与 `wordFilter.ts` / `usePwaUpdate.ts` 一致。

---

## File Structure

| 文件 | 责任 |
|---|---|
| `frontend/src/utils/speech.ts`（新建） | 从音色列表里挑英语音色，纯函数、无副作用 |
| `frontend/src/utils/speech.test.ts`（新建） | `pickEnglishVoice` 的单测 |
| `frontend/src/composables/useSpeech.ts`（新建） | Web Speech API 薄封装：支持检测 + 朗读，处理连点取消 |
| `frontend/src/components/SpeakerButton.vue`（新建） | 喇叭按钮，props 只有 `text`，不支持时渲染空 |
| `frontend/src/views/sprint/WordReview.vue`（修改） | 卡片模式与词表模式接入喇叭；卡片外层由 `button` 改为可键盘操作的 `div` |

---

### Task 1: 英语音色选择纯函数 + 单测

**Files:**
- Create: `frontend/src/utils/speech.ts`
- Test: `frontend/src/utils/speech.test.ts`

**Interfaces:**
- Consumes: 无
- Produces: `pickEnglishVoice(voices: readonly SpeechSynthesisVoice[]): SpeechSynthesisVoice | null`

- [ ] **Step 1: Write the failing test**

Create `frontend/src/utils/speech.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { pickEnglishVoice } from '@/utils/speech'

// node 环境没有 SpeechSynthesisVoice 构造器，用最小对象冒充即可
const voice = (lang: string) => ({ lang, name: lang }) as SpeechSynthesisVoice

describe('pickEnglishVoice', () => {
  it('有 en-US 时优先选 en-US（即使排在其他英语音色后面）', () => {
    const voices = [voice('zh-CN'), voice('en-GB'), voice('en-US')]
    expect(pickEnglishVoice(voices)?.lang).toBe('en-US')
  })

  it('没有 en-US 时回退到任意 en-* 音色', () => {
    const voices = [voice('zh-CN'), voice('en-GB')]
    expect(pickEnglishVoice(voices)?.lang).toBe('en-GB')
  })

  it('只有非英语音色时返回 null', () => {
    expect(pickEnglishVoice([voice('zh-CN'), voice('ja-JP')])).toBeNull()
  })

  it('空数组返回 null', () => {
    expect(pickEnglishVoice([])).toBeNull()
  })

  it('lang 大小写不敏感', () => {
    expect(pickEnglishVoice([voice('EN-us')])?.lang).toBe('EN-us')
    expect(pickEnglishVoice([voice('en-US'), voice('EN-gb')])?.lang).toBe('en-US')
  })

  it('不修改入参', () => {
    const voices = [voice('en-GB'), voice('en-US')]
    const before = voices.map((v) => v.lang)
    pickEnglishVoice(voices)
    expect(voices.map((v) => v.lang)).toEqual(before)
    expect(voices).toHaveLength(2)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run（工作目录 `frontend/`）:

```bash
npx vitest run src/utils/speech.test.ts
```

Expected: FAIL —— 无法解析 `@/utils/speech`（模块不存在）。

- [ ] **Step 3: Write minimal implementation**

Create `frontend/src/utils/speech.ts`:

```ts
// 从浏览器提供的音色列表中挑一个英语音色：优先美音，其次任意英语音色。
// 抽成纯函数便于在 node 环境下单测，不触碰 window。
export function pickEnglishVoice(
  voices: readonly SpeechSynthesisVoice[],
): SpeechSynthesisVoice | null {
  const langOf = (v: SpeechSynthesisVoice) => v.lang.toLowerCase()
  return (
    voices.find((v) => langOf(v) === 'en-us') ??
    voices.find((v) => langOf(v).startsWith('en')) ??
    null
  )
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
npx vitest run src/utils/speech.test.ts
```

Expected: PASS —— 6 个用例全绿。

- [ ] **Step 5: Commit**

```bash
git add frontend/src/utils/speech.ts frontend/src/utils/speech.test.ts
git commit -m "feat(frontend): 新增英语音色选择纯函数 speech"
```

---

### Task 2: useSpeech + SpeakerButton + 单词页接入

**Files:**
- Create: `frontend/src/composables/useSpeech.ts`
- Create: `frontend/src/components/SpeakerButton.vue`
- Modify: `frontend/src/views/sprint/WordReview.vue`（卡片模式约 238–257 行；词表模式约 290–295 行）

**Interfaces:**
- Consumes: `pickEnglishVoice` from `@/utils/speech`
- Produces:
  - `useSpeech(): { isSupported: boolean; speak(text: string): void }`
  - `<SpeakerButton :text="string" />` —— 不支持 Web Speech 的浏览器渲染为空

- [ ] **Step 1: 新建 useSpeech composable**

Create `frontend/src/composables/useSpeech.ts`:

```ts
// Web Speech API 薄封装：只负责朗读给定文本，不自动播放、不排队。
// 不支持的环境（老浏览器/隐私模式）由 isSupported 让调用方隐藏入口。
import { pickEnglishVoice } from '@/utils/speech'

export function useSpeech() {
  const isSupported = typeof window !== 'undefined' && 'speechSynthesis' in window

  function speak(text: string): void {
    if (!isSupported || !text) return
    // 连点喇叭时不排队：先停掉正在播的，再读最后一次点击的词
    window.speechSynthesis.cancel()
    const utterance = new SpeechSynthesisUtterance(text)
    utterance.lang = 'en-US'
    utterance.rate = 0.9 // 略慢，便于跟读
    // 每次现取而不是缓存：音色列表在浏览器里是异步加载的，首次可能拿到空数组；
    // 拿不到英语音色也不影响，只设 lang 时浏览器会按语言选默认音色。
    const voice = pickEnglishVoice(window.speechSynthesis.getVoices())
    if (voice) utterance.voice = voice
    window.speechSynthesis.speak(utterance)
  }

  return { isSupported, speak }
}
```

- [ ] **Step 2: 新建 SpeakerButton 组件**

Create `frontend/src/components/SpeakerButton.vue`:

```vue
<script setup lang="ts">
// 喇叭按钮：点击朗读传入的文本。不支持 Web Speech 的浏览器不渲染。
// 用内联 SVG 而不是 @element-plus/icons-vue —— 图标库里没有喇叭形状。
import { useSpeech } from '@/composables/useSpeech'

defineProps<{ text: string }>()

const { isSupported, speak } = useSpeech()
</script>

<template>
  <button
    v-if="isSupported"
    type="button"
    class="inline-flex shrink-0 items-center justify-center rounded-full p-1 text-slate-400 transition hover:bg-slate-100 hover:text-blue-600 dark:hover:bg-slate-800 dark:hover:text-blue-400"
    aria-label="播放读音"
    title="播放读音"
    @click.stop="speak(text)"
  >
    <svg
      class="h-4 w-4"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <path d="M11 5 6 9H2v6h4l5 4V5z" />
      <path d="M15.54 8.46a5 5 0 0 1 0 7.07" />
      <path d="M19.07 4.93a10 10 0 0 1 0 14.14" />
    </svg>
  </button>
</template>
```

- [ ] **Step 3: 卡片模式接入喇叭，并把外层 button 改为 div**

`frontend/src/views/sprint/WordReview.vue` 的 script setup 顶部补一行显式引入（避免依赖 `components.d.ts` 的自动生成）：

```ts
import SpeakerButton from '@/components/SpeakerButton.vue'
```

把卡片模板里这一段：

```vue
        <button
          class="flex min-h-[220px] w-full flex-col items-center justify-center gap-3 rounded-2xl border border-slate-200 bg-white p-8 text-center shadow-sm transition hover:shadow-md dark:border-slate-800 dark:bg-slate-900"
          @click="flipped = !flipped"
        >
          <div class="text-2xl font-bold text-slate-900 dark:text-white">{{ current.word }}</div>
```

改为：

```vue
        <!-- 外层用 div 而不是 button：喇叭是真正的 <button>，button 嵌套 button 是非法 DOM -->
        <div
          role="button"
          tabindex="0"
          class="flex min-h-[220px] w-full cursor-pointer flex-col items-center justify-center gap-3 rounded-2xl border border-slate-200 bg-white p-8 text-center shadow-sm transition hover:shadow-md dark:border-slate-800 dark:bg-slate-900"
          @click="flipped = !flipped"
          @keydown.enter="flipped = !flipped"
          @keydown.space.prevent="flipped = !flipped"
        >
          <div class="flex items-center justify-center gap-2">
            <span class="text-2xl font-bold text-slate-900 dark:text-white">{{ current.word }}</span>
            <SpeakerButton :text="current.word" />
          </div>
```

再把该卡片结尾的 `</button>` 改为 `</div>`。**只改卡片这一处**：它的前一行是 `<span v-if="mastered.has(current.word)" ...>✓ 已掌握</span>`，改完后这一段应为：

```vue
          <span v-if="mastered.has(current.word)" class="text-xs text-emerald-500">✓ 已掌握</span>
        </div>
```

文件里另外三处 `</button>`（顶部两个模式切换按钮、词表里的「标记掌握」按钮）不要动；`el-button` 的结束标签是 `</el-button>`，也不受影响。

- [ ] **Step 4: 词表模式接入喇叭**

把词表中的：

```vue
            <div class="font-medium text-slate-800 dark:text-slate-100">{{ w.word }}</div>
```

改为：

```vue
            <div class="flex items-center gap-2">
              <span class="font-medium text-slate-800 dark:text-slate-100">{{ w.word }}</span>
              <SpeakerButton :text="w.word" />
            </div>
```

- [ ] **Step 5: 类型检查与全量测试**

Run（工作目录 `frontend/`）:

```bash
npx vue-tsc -b
npm test
```

Expected: 两者均退出码 0；测试 5 个文件、61 个用例全绿（原 55 + 新增 6）。

- [ ] **Step 6: 浏览器手测**

Run:

```bash
npm run dev
```

打开 `http://localhost:3000/sprint/words`，逐项确认：

1. 卡片模式单词右侧出现喇叭；点击喇叭只出声、**不翻卡**。
2. 点击卡片其他地方仍正常翻面；键盘 Tab 聚焦卡片后按 Enter / Space 也能翻面。
3. 词表模式每个单词右侧有喇叭；点击只出声，不触发「标记掌握」。
4. 连续快速点两个不同单词的喇叭，只听到最后一次的读音。
5. 卡片翻到背面时喇叭仍在单词旁，行为不变。

- [ ] **Step 7: Commit**

```bash
git add frontend/src/composables/useSpeech.ts frontend/src/components/SpeakerButton.vue frontend/src/views/sprint/WordReview.vue
git commit -m "feat(frontend): 单词页增加喇叭按钮，点击朗读读音"
```

---

## Self-Review

**1. Spec coverage**

| Spec 要求 | 对应任务 |
|---|---|
| 用 Web Speech API，零依赖 | Task 2 `useSpeech.ts` |
| `pickEnglishVoice` 优先 en-US、回退 en-*、无则 null | Task 1 |
| `speak`：cancel → lang/rate → 现取音色 | Task 2 Step 1 |
| `SpeakerButton`：props `text`、不支持时渲染 null、`@click.stop` | Task 2 Step 2 |
| 卡片外层 button 改 div + 键盘可达 | Task 2 Step 3 |
| 卡片与词表都接入 | Task 2 Step 3、4 |
| 单测 6 个用例 | Task 1 Step 1 |
| 不改词库、不改其他页面、不做自动朗读/静音开关 | Global Constraints 约束 |

**2. Placeholder scan**：无 TBD/TODO；所有代码步骤都有完整代码；验收步骤有明确命令与预期。

**3. Type consistency**：`pickEnglishVoice(voices: readonly SpeechSynthesisVoice[]): SpeechSynthesisVoice | null` 在 Task 1 定义、Task 2 的 `useSpeech` 调用一致；`SpeakerButton` 的 prop 名 `text` 在组件定义、卡片接入、词表接入三处一致；`useSpeech` 返回 `{ isSupported, speak }` 在组件里解构一致。
