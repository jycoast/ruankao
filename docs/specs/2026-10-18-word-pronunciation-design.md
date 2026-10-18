# 英语单词页增加读音播放

- 日期：2026-10-18
- 状态：设计已确认，待实施
- 影响面：**纯前端**；**无接口变更、无数据变更、无新依赖**

## 背景与目标

单词页 `/sprint/words` 的卡片与词表现在只能看，不能听。用户需要点一下喇叭就能听到单词读音，便于跟读和记忆。

目标：在卡片模式和词表中，每个单词旁提供一个小喇叭按钮，点击用浏览器语音合成朗读该单词。

## 现状

- `frontend/src/views/sprint/WordReview.vue` 是唯一的单词页。
- 词库 `frontend/src/data/sprintWords.ts` 的 `Word` 已有 `word` / `phonetic` / `meaning` 等字段，**无需新增字段**。
- 项目无任何音频相关代码与依赖。
- `@element-plus/icons-vue` 中没有喇叭类图标（只有 `Microphone` / `Headset` / `Mute`），因此不新增图标依赖，用一个内联 SVG 组件。
- 图标需在 `frontend/src/icons.ts` 登记才可运行时可用；本方案的 SVG 直接写在组件模板里，不走图标注册。

## 决策

### 1. 发音来源：浏览器语音合成（Web Speech API）

使用 `window.speechSynthesis` + `SpeechSynthesisUtterance`：

- 零依赖、离线可用，契合 PWA 壳离线能力。
- 音色由设备/系统决定，可能与真人词典发音有差异，这是已接受的取舍。
- 不引入在线词典音频：避免外部服务依赖、跨域问题、断网不可用，以及多词短语（如 `design pattern`）无音频的问题。

### 2. `src/utils/speech.ts`——可单测的纯函数

```ts
/** 优先 en-US，其次任意 en-*，都没有返回 null（交给浏览器按 lang 兜底） */
export function pickEnglishVoice(
  voices: readonly SpeechSynthesisVoice[],
): SpeechSynthesisVoice | null
```

- 遍历顺序：先找 `lang.toLowerCase() === 'en-us'`，再找 `startsWith('en')`。
- 入参为 `readonly`，不修改。

选音色逻辑放纯函数，与 `wordFilter.ts` / `questionGroup.ts` 落位惯例一致，且 vitest `environment: 'node'` 下可直接测（不触碰 `window`）。

### 3. `src/composables/useSpeech.ts`——运行时薄封装

```ts
export function useSpeech() {
  const isSupported = typeof window !== 'undefined' && 'speechSynthesis' in window
  function speak(text: string): void
  return { isSupported, speak }
}
```

`speak` 行为：

1. `isSupported` 为假或 `text` 为空 → 直接返回。
2. `speechSynthesis.cancel()` 先取消正在播放/排队的语音——连点喇叭不会叠音。
3. 新建 `SpeechSynthesisUtterance(text)`，设 `lang = 'en-US'`、`rate = 0.9`（略慢，便于跟读）。
4. **每次调用时**现取 `speechSynthesis.getVoices()` 并用 `pickEnglishVoice` 选音色。不缓存：音色列表在浏览器中是异步加载的，首次可能拿到空数组；拿到空数组也不影响——只设 `lang` 时浏览器会按语言选默认音色。
5. `speechSynthesis.speak(u)`。

不做自动朗读、不做静音开关、不读例句——只读单词本身。

### 4. `src/components/SpeakerButton.vue`——喇叭按钮

- props：`text: string`（要朗读的内容）。
- 内部调用 `useSpeech()`；`isSupported` 为假时渲染 `null`。
- 渲染为真正的 `<button type="button">`，内联 SVG 喇叭图形，`aria-label="播放读音"`。
- 点击处理：`@click.stop="speak(text)"`——防止冒泡触发外层翻卡。

### 5. 接入 WordReview

| 位置 | 改法 |
|---|---|
| 卡片模式 | 卡片外层目前是 `<button>`；在里面再放 `<button>` 是非法嵌套（浏览器会纠正 DOM）。把外层改为 `<div role="button" tabindex="0">`，保留点击翻卡，并补 `@keydown.enter` / `@keydown.space.prevent` 处理；喇叭紧跟在单词右侧 |
| 词表模式 | 单词行内、`word` 文字右侧放 `<SpeakerButton :text="w.word" />`，不影响右侧「标记掌握」按钮布局 |

排版示意：

```
卡片：  architecture 🔊
        /ˌɑːrkɪˈtektʃər/
词表：  architecture 🔊
        /ˈɑːrkɪtektʃər/   — 中译
```

### 6. 边界情况

| 场景 | 处理 |
|---|---|
| 浏览器不支持 Web Speech API | 不渲染喇叭，页面其余功能不变 |
| 连点喇叭 | 每次先 `cancel()`，只读最后一次点击的词 |
| 无音频设备 / 系统静音 | 点击无反应但不报错（API 本身不抛错） |
| 系统无英语音色 | 只设 `lang='en-US'`，交给浏览器兜底 |
| 卡片外层由 `button` 改为 `div` | 需保留键盘可达性（`tabindex` + Enter/Space），并确保 `min-h-[220px]`、`w-full` 等样式不变 |
| 预渲染页 | `/sprint/words` 不在预渲染清单内；`useSpeech` 对 `window` 做过存在性判断，非浏览器环境下 `isSupported` 为 false |

## 测试

新建 `frontend/src/utils/speech.test.ts`（vitest，风格同 `wordFilter.test.ts`）：

| 用例 | 断言 |
|---|---|
| 含 `en-US` | 返回该音色 |
| 无 `en-US`、有 `en-GB` | 返回 `en-GB` |
| 只有非英语音色（如 `zh-CN`） | 返回 `null` |
| 空数组 | 返回 `null` |
| `lang` 大小写混合（如 `EN-us`） | 仍能匹配 |
| 入参 | 不被修改 |

组件与播放行为不做自动化测试（项目无组件测试环境、无 jsdom），靠 `vue-tsc` + 浏览器手测覆盖。

## 明确不做

- 不自动朗读、不加全局静音开关。
- 不读例句、释义或音标。
- 不引入在线词典音频、不打包本地 mp3。
- 不改词库 `sprintWords.ts`，不改 `/sprint/words` 之外的路由或页面。
