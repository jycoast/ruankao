# 英语单词界面按掌握状态筛选

- 日期：2026-10-18
- 状态：设计已确认，待实施
- 影响面：**纯前端**；掌握状态本就存在浏览器 `localStorage`，**无数据库结构变更、无接口变更**

## 背景与目标

单词页 `/sprint/words` 目前只能按**分类**（架构设计、质量属性、软件工程……）筛选。用户背完一轮后，已掌握的单词仍混在卡组里，下一轮还得逐张翻过去点「下一个」——无法「只背未掌握的」。

目标：在单词页增加**全部 / 未掌握 / 已掌握**三态筛选，让用户能只背未掌握的词，也能回头复习已掌握的词。

## 现状

`frontend/src/views/sprint/WordReview.vue` 是唯一的单词页，词库为前端静态内置的 `frontend/src/data/sprintWords.ts`（`SPRINT_WORDS`）。掌握状态保存在 `localStorage` 的 `sprint_words_mastered`（单词字符串数组），页面另用 `sprint_words_progress` 记录每个分类的卡片位置与模式。

现有数据流：

```
SPRINT_WORDS ──按 category 过滤──> pool ──> 卡片模式按 index 取 current / 词表模式 v-for
```

`mastered` 只用于展示（计数、✓ 已掌握标记），不参与过滤。

## 决策

### 1. 过滤逻辑抽成纯函数

新建 `frontend/src/utils/wordFilter.ts`：

```ts
export type MasteryFilter = 'all' | 'unmastered' | 'mastered'

export function filterWords<T extends { word: string; category: string }>(
  words: readonly T[],
  opts: { category: string; mastery: MasteryFilter; mastered: ReadonlySet<string> },
): T[]
```

- 分类与掌握状态取交集；`category === '全部'` 表示不按分类过滤。
- 保持入参顺序，不修改入参。
- 与 `questionGroup.ts` / `wrongBook.ts` 同落位惯例，可直接单测。

同时导出位置回退的纯函数：

```ts
export function pickSavedIndex(
  positions: Record<string, number>,
  category: string,
  mastery: MasteryFilter,
): number
```

- 优先读 `` positions[`${category}::${mastery}`] ``。
- `mastery === 'all'` 读不到时回退旧的 `` positions[category] ``（兼容老数据）。
- 都读不到返回 `0`。

### 2. 组件里拆成两个 computed

| 名称 | 含义 | 用途 |
|---|---|---|
| `categoryPool` | 只按分类过滤 | 统计「已掌握 X / Y」的分母 |
| `pool` | 在 `categoryPool` 上再按掌握状态过滤 | 词表模式的实时数据源 |

统计展示保持 `已掌握 X / Y`：**X 与 Y 都基于 `categoryPool`**（X = `categoryPool` 中已掌握的数量，Y = `categoryPool.length`），不随掌握筛选变化。这样切到「未掌握」时分子不会恒为 0，数字始终有意义。

词表模式下点「标记掌握 / 取消掌握」，该行按 `pool` 实时消失或出现。

### 3. 卡片模式使用本遍卡组快照

卡片模式不在 `pool` 上实时取词，而是维护 `deck = ref<Word[]>([])`：

- 进入卡片模式，或卡片模式下分类 / 掌握筛选变化时，用 `filterWords` 重建一次快照。
- 一遍之内，点「认识」只写入掌握状态，**不从 `deck` 移除**，照常展开释义，点「下一个」继续——与现有节奏完全一致，不会跳词。
- 本遍走完后，重新进入或切换筛选时按最新掌握状态重建，已掌握的词自然不再出现。
- `current`、`index`、`pool.length` 的展示全部改用 `deck`。

### 4. 完成面板

现有逻辑走完一遍只停在最后一张并弹一条提示。新增 `finished = ref(false)`：

- `next()` 在最后一张时不再只弹 `ElMessage`，而是置 `finished = true`，页面显示完成面板：提示「本组已过完一遍」+ **「重新开始」** 按钮。
- `prev()` 把 `finished` 复位为 `false`，可以退回上一张继续看。
- 点「重新开始」：按最新掌握状态（和当前分类、筛选）重建 `deck`，`index` 归 0，`revealed` / `flipped` / `finished` 复位并持久化进度。
- 没有这个入口，在「未掌握」筛选下走完一遍就无处可去。

### 5. 进度持久化

- `positions` 的键由 `category` 改为 `` `${category}::${mastery}` ``，三种筛选各自记忆位置。
- 切换分类**和切换掌握筛选**时，都先把当前位置写入旧键，再按新键恢复索引——现有 `watch(category)` 的逻辑复制一份给 `mastery`。
- `sprint_words_progress` 增加 `mastery` 字段；读取时校验非法值回落 `'all'`，刷新后保持筛选状态。
- 恢复的位置一律按 `deck.length` clamp（现有 `applyCategoryIndex` 已做）。
- 旧数据兼容：`mastery === 'all'` 且新键不存在时读旧键 `positions[category]`，老用户进度不丢。

`loadProgress` 会恢复 `mode` 和 `category` 并触发 watch，现有的 `initialized` 标志继续用于跳过首次「存旧分类」逻辑。

### 6. 筛选控件

在分类下拉旁加 `el-radio-group + el-radio-button`（`OrderManage.vue` / `EssaySubmissionManage.vue` 同款），`size="small"`：

```
[ 分类 ▾ ]  ( 全部 | 未掌握 | 已掌握 )   已掌握 12 / 45
```

三态文案：`全部`、`未掌握`、`已掌握`。

### 7. 空状态

| 场景 | 文案 |
|---|---|
| 卡片 / 词表，`mastery = unmastered` 且为空 | 当前分类的单词都已掌握 🎉 |
| 卡片 / 词表，`mastery = mastered` 且为空 | 当前分类还没有已掌握的单词 |
| 卡片 / 词表，`mastery = all` 且为空 | 该分类暂无单词（沿用现有文案） |

词表模式目前没有空状态，补一个与卡片模式一致的提示。

## 边界情况

1. **「已掌握」筛选下点「不认识」** —— 该词本遍仍留在卡组（快照），下一遍重建时消失，符合预期。
2. **词表模式标记后行立即消失** —— 由实时 `pool` 自然产生；若列表因此变空，走上面的空状态。
3. **掌握集合含词库外的词** —— 例如词库删词后残留的旧记录，过滤结果不受影响，无需清理 `localStorage`。
4. **旧位置键** —— 只有 `mastery = all` 回退旧键；`unmastered` / `mastered` 没有旧数据，从 0 开始。
5. **分类数据变化** —— 现有 `categories` 兜底校验保留，失效分类回落「全部」。

## 测试

新建 `frontend/src/utils/wordFilter.test.ts`（vitest，风格同 `questionGroup.test.ts`）。

`filterWords`：

| 用例 | 断言 |
|---|---|
| `mastery = all` + 具体分类 | 等于按分类过滤的结果 |
| `mastery = unmastered` | 排除 `mastered` 中的词 |
| `mastery = mastered` | 只保留 `mastered` 中的词 |
| `category = '全部'` | 不按分类过滤，仅按掌握状态过滤 |
| 分类 + 掌握状态交集 | 两个条件同时生效 |
| 空词库 / 筛选结果为空 | 返回 `[]`，不抛错 |
| 顺序 | 输出顺序与入参一致 |
| 入参 | 不被修改 |
| 掌握集合含词库外的词 | 结果不受影响 |

`pickSavedIndex`：

| 用例 | 断言 |
|---|---|
| 新键存在 | 返回新键值 |
| `mastery = all` 且新键缺失、旧键存在 | 返回旧键值 |
| `unmastered` / `mastered` 且新键缺失 | 返回 0（不回退旧键） |
| 都缺失 | 返回 0 |

组件层不引入新测试框架（项目目前没有组件测试），行为通过纯函数测试 + 手工验证覆盖。

## 明确不做

- **不把掌握状态上云**：不动后端与数据库，不做多端同步。
- 不改 `sprintWords.ts` 词库内容。
- 不改 `/sprint/words` 之外的路由或页面。
- 不改「认识 / 不认识」的判词语义（仍只写入掌握状态）。
- 不新增全量「重置掌握记录」功能。
