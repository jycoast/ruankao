# 案例分析答题卡按大题分组

- 日期：2026-10-14
- 状态：设计已确认，待实施
- 影响面：**纯前端**读取既有字段；**无数据库结构变更、无接口变更**（另含一处 CI 配置）

## 背景与目标

答题页 `/exam/:id` 的答题卡把整卷的格子平铺成一排编号。对案例分析卷，一个格子往往不是一道题，而是某道大题的**一问**：以系统架构设计师的案例卷为例，答题卡上出现的是 12–15 个格子，实际是 4–5 道大题（每道 2–3 问）。用户既看不出「总共有几道题目」，也看不出「这几个格子属于哪道大题」。

目标：在该答题卡上按大题分组显示，并标出大题的条数。

## 现状

### 答题卡

`frontend/src/components/AnswerCard.vue`（123 行）只接收 `total` / `currentIndex` / `answeredMap` / `resultMap`，**对题目内容一无所知**，靠 `v-for="(_, i) in total"` 平铺渲染。

它有 6 个消费方，但只有两个是**整卷**场景，都经 `examApi.getPaper()` 拿到带 `title`、`content` 的完整题目：

| 消费方 | 场景 | 有 `questions[]`（含 title/content） |
|---|---|---|
| `views/exam/ExamSession.vue`（答题页） | 整卷 | 有 |
| `views/exam/ExamReview.vue`（真题背题） | 整卷 | 有 |
| `views/question/PracticeByCategory.vue` | 分类练习 | 单题流，无大题结构 |
| `views/question/ReviewMode.vue` | 复习 | 同上 |
| `views/question/WrongQuestions.vue` | 错题本 | 同上 |

### 数据里没有显式的大题结构

`paper_questions` 关联表**没有任何**大题字段：没有 `parent_id`，也没有 `paper_section` 表（只有一个同名字符串列，且全卷同值）。`paper_questions.sort_order` 在多数卷上恒为 0，`questions.sort_order` 亦然——都不是分组信号。

关联表里两种排布并存：

|  | 形状 A | 形状 B |
|---|---|---|
| 一行等于 | 一道大题 | 某道大题的一问 |
| 小问位置 | 写在正文里（问题1/问题2/…） | 每行各一问 |
| 共享背景 | —— | 整段背景**逐行重复**，`title` 也重复 |
| 分布 | subject 2/4/8/10，共 136 份 | subject 1（19 份里 15 份）、subject 3 |

**形状 A 下「一行就是一道大题」**，分组退化为恒等——用户看到的「14 个格子 vs 5 道大题」只发生在形状 B。

### 两个可用信号，都已经在接口返回里

`getPaper` 返回的 `PaperWithQuestions.questions[]` 每一项都带 `title` 与 `content`（`backend/internal/application/service/exam.go` 的 `GetPaperWithQuestions`），因此**不需要动后端、也不需要动库**。这两个信号单独用都不行，互为补集：

- **`title`**：形状 B 下同一大题的连续行 `title` 相同。但它是**知识域标签**，相邻两道大题经常重名——paper 81 的两道大题都叫「系统架构设计与评估」，paper 96 有三道都叫「Web应用设计」，paper 310/311/312 同理。单用会**少算**大题。
- **内容前缀**：取正文第一个「问题」之前的、去掉全部空白后的前 40 字。在 paper 78/104 上它会把三段不同的大题**过度合并**（三者共用「阅读以下关于数据库缓存架构的叙述，在答题纸上回答」这个开头），但在 81/96/310/311/312 上恰好能把重名段拆开。单用会**多算**。

## 决策

### 1. 分组算法：两个信号取并集

**换组条件：`title` 与上一行不同，或归一化前缀与上一行不同。**

```ts
const STEM_LIMIT = 40

// 与校验所用 SQL 等价：
//   LEFT(REGEXP_REPLACE(SUBSTRING_INDEX(content, '问题', 1), '[[:space:]]+', ''), 40)
function stemKey(content: string): string {
  return Array.from(content.split('问题')[0].replace(/\s+/g, ''))
    .slice(0, STEM_LIMIT)
    .join('')
}

export function groupQuestions(questions: { title?: string; content?: string }[]) {
  const groups: { start: number; end: number; title: string; count: number }[] = []
  questions.forEach((q, i) => {
    const prev = questions[i - 1]
    const isNew =
      !prev ||
      (q.title ?? '') !== (prev.title ?? '') ||
      stemKey(q.content ?? '') !== stemKey(prev.content ?? '')
    if (isNew) {
      groups.push({ start: i, end: i, title: q.title ?? '', count: 1 })
    } else {
      const last = groups[groups.length - 1]
      last.end = i
      last.count++
    }
  })
  return groups
}
```

两个移植细节必须照做，否则实现与验证会对不上：

- **去空白要与 MySQL 对齐。** 已实测本库 MySQL 8 的 ICU 正则会把全角空格 U+3000、不换行空格 U+00A0 与 ASCII 空格一并剥掉，与 JS 的 `\s` 等价，可直接用 `\s`。（`LIKE` 不行——`utf8mb4_unicode_ci` 下 `LIKE '%　%'` 会匹配普通空格，实测口径失真。）
- **取前 40 个码点，而非 40 个 UTF-16 单元。** 用 `Array.from(...)` 与 MySQL `LEFT(..., 40)` 的字符语义一致；非 BMP 字符（emoji 等）下二者才会分叉。

### 2. 验证结果

在后端真实加载顺序下（`backend/internal/domain/repository/paper.go:128` 的 `(questions.sort_order = 0) ASC, questions.sort_order ASC, pq.sort_order ASC, questions.id ASC`）复算：

| 卷 | 行数 | 仅 `title` | 仅前缀 | **并集** | 真实大题数 |
|---|---|---|---|---|---|
| 78 | 14 | 5 ✓ | 3 ✗ | **5** | 5 |
| 81 | 12 | 3 ✗ | 4 ✓ | **4** | 4 |
| 96 | 14 | 4 ✗ | 5 ✓ | **5** | 5 |
| 104 | 7 | 5 ✓ | 3 ✗ | **5** | 5 |
| 310 | 11 | 3 ✗ | 4 ✓ | **4** | 4 |
| 311 | 11 | 3 ✗ | 4 ✓ | **4** | 4 |
| 312 | 15 | 4 ✗ | 5 ✓ | **5** | 5 |
| subject 1 其余 15 份 | 5–15 | 5 ✓ | 5 ✓ | **5** | 5 |
| subject 3 其余 11 份 | 5 | 5 ✓ | 5 ✓ | **5** | 5 |

- **subject 1 + subject 3 共 33 份（全部形状 B 卷）：33/33 命中真实大题数。**
- subject 2/4/8/10 共 136 份：并集从不比 `title` 多切一组，退化为「一行一道大题」，**无回归**。
- 全库 169 份里只有 paper 84、104 两份的行序与 `pq.id` 序不同，两份都在上表已验证集合内且结论不变。
- **不存在**「多行被整体并成 1 组」的卷子（`comb = 1` 且行数 > 1 的记录为 0 条）。

### 3. 放在哪

新建纯函数 `frontend/src/utils/questionGroup.ts`，导出上面的 `groupQuestions`。无依赖、无副作用，可直接单测——与 `frontend/src/utils/wrongBook.test.ts` 的既有落位惯例一致。**前端算，后端与数据库都不动。**

### 4. AnswerCard 的展示

`AnswerCard.vue` 新增**可选** prop `groups?: QuestionGroup[]`：

- **不传 `groups`，渲染结果与今天逐像素一致**。其余 4 个消费方零影响。
- 传了 `groups`，**且存在某个组的 `count >= 2`** 时才渲染分组。这条让形状 A 卷（一行一道大题）保持原样——paper 84 不会平白多出 5 行标题；也顺带说明，下面的组标题样式只需应对形状 B 那种短标签。
- 分组渲染：每组前插入一行组标题，格子在组内正常排列：

```
答题卡 (3/14) · 共 5 道大题
────────────────────────────────
试题一 · 系统架构设计与评估          2问
  [1] [2]
试题二 · 数据库系统与缓存设计        3问
  [3] [4] [5]
```

- **格子保持全局小问序号**（1..total），与页面顶部「第 4 / 14 题」、跳转下标严格一致；大题序号走「试题一/二/…」，**不占用「第 N 题」这个说法**，避免两套编号在界面上打架。
- 组标题右侧显示该组的问数（`count`）。
- 卡片头部在传了 `groups` 且渲染分组时追加「· 共 N 道大题」，直接回答「总共有几道题目」。
- 组标题的 `title` 加 CSS 截断 + `title` 属性兜底（形状 B 的标签很短，正常不会触发）。

### 5. 接入范围

| 文件 | 改动 |
|---|---|
| `views/exam/ExamSession.vue` | 引入 `groupQuestions`，对 `questions` 求分组，桌面（:587）与移动（:600）两处 `AnswerCard` 传 `:groups` |
| `views/exam/ExamReview.vue` | 同上，桌面（:193）与移动（:205）两处 |

两个页面都走同一份 `getPaper` 数据、同一个 util、同一个 prop，读同一份卷子时体验一致。

### 6. CI 闸门

`.github/workflows/ci.yml` 的 `frontend` job 目前只跑 `npm ci` 与 `npm run build`，**不跑 `npm test`**（`package.json` 里 `test` = `vitest run` 是有的）。新写的算法单测若不加这一步，就只是一份本机可跑的文档，没有任何闸门会在合并前拦住它回归。

在该 job 的 `Install` 与 `Type-check + Build` 之间加一步：

```yaml
      - name: Test
        working-directory: frontend
        run: npm test
```

已实测本地 `npm test` 全绿（2 个文件、23 个用例），加闸门不会让 CI 立刻变红。

## 边界情况

1. **`title` 或 `content` 缺失/为空** —— `?? ''` 兜底，退化为按另一信号分组；两者都空则该行自成一组长。真实数据里 169 份案例卷 `title` 无一为空。
2. **单行卷**（只有 1 道大题）—— 得到 1 组、`count = 1`，不渲染分组，表现为原样。
3. **形状 A 卷** —— 每行 `title` 不同，每组恰好 1 格，不渲染分组。此时页面上「14 个格子」本来就等于「14 道大题」，标题无意义。
4. **分组不精确的卷子** —— 已知并集规则在 33 份形状 B 卷上全部正确，但这是**推断**而非库里的事实。若将来导入的卷子出现「相邻两道大题 `title` 与正文前 40 字都相同」，会被并成一组、少算一道。此时卡片上的格子编号与跳转仍然完全正确，错的只是分组标题的行数。
5. **大题数超过 10** —— 实际观测最大值是 6。`试题N` 的序号 1–10 用中文数字，超过 10 回退到阿拉伯数字，避免为不可能的分支引入中文数字换算表。

## 测试

新建 `frontend/src/utils/questionGroup.test.ts`（vitest，与 `wrongBook.test.ts` 同落位），夹具取**真实卷子**的 `title` 序列与 `content` 片段，断言组数与每组问数：

| 用例 | 断言 |
|---|---|
| paper 78 | 5 组，问数 2/3/3/3/3 |
| paper 81 | 4 组（两道同名「系统架构设计与评估」被前缀拆开） |
| paper 96 | 5 组（三道同名「Web应用设计」被前缀拆开） |
| paper 104 | 5 组（前缀过度合并段被 `title` 拆开） |
| paper 310 / 311 | 4 组 |
| paper 312 | 5 组 |
| 形状 A（每行 `title` 不同） | 每组 1 格，组数 = 行数 |
| 单行卷 | 1 组 |
| `title`/`content` 缺失 | 不抛错，退化为可预期的分组 |

夹具里 `content` 截断到约 120 字即可，**这是安全的**：规则只读第一个「问题」之前的前 40 个非空白码点，而截断点远在该位置之后，截断不改变 `stemKey` 的取值。

## 明确不做

- 不改答题页顶部的「第 N / 14 题」与「已答 x/14」——用户已明确本次只做答题卡分组。
- **不落库、不加字段、不改导入器**：先验算法已在全部形状 B 卷上验证通过，无需 schema 变更。
- 不改其余 4 个 `AnswerCard` 消费方。
- 不做形状 A 卷正文内部小问（问题1/问题2/问题3）的拆分显示——那需要解析正文，本设计不碰。
