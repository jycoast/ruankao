# 案例分析答题卡按大题分组 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `/exam/:id` 答题页与 `/exam/:id/review` 背题页的答题卡按大题分组显示，并标出大题条数。

**Architecture:** 分组完全在前端推断。库里没有显式大题结构，改用两个既有字段取并集：`title` 连续段，以及「正文第一个『问题』之前、去掉全部空白后的前 40 个码点」这一归一化前缀。两个信号互为补集——只用一个会分别漏切/误切。新增纯函数 `frontend/src/utils/questionGroup.ts` 承载规则，`AnswerCard.vue` 加一个可选 prop 渲染分组。**后端、数据库、接口一律不动。**

**Tech Stack:** Vue 3 + TypeScript + vue-tsc + Vite 6 + vitest 5（`environment: 'node'`，无组件测试能力）

## Global Constraints

- **归一化前缀（`stemKey`）的定义逐字固定**：正文以第一个「问题」切开后的**左半段** → 去掉**全部空白** → 取前 **40** 个**码点**。
- 去空白用 JS 的 `\s`。已实测本库 MySQL 8 的 ICU 正则 `[[:space:]]` 会把全角空格 U+3000、不换行空格 U+00A0 与 ASCII 空格一并剥掉，与 `\s` 等价，勿改用 `LIKE`（`utf8mb4_unicode_ci` 下口径失真）。
- 按**码点**切，不按 UTF-16 单元：`Array.from(s).slice(0, 40).join('')`，与 MySQL `LEFT(..., 40)` 的字符语义一致。
- **换组条件是并集**：`title` 与上一问不同 **或** `stemKey` 与上一问不同。
- 答题卡格子**保持全局问号**（`g.start + n`，即 1..total），与页面顶部「第 N / total 题」严格一致。
- 大题序号用「试题一/二/…」，**不得**使用「第 N 题」——那会与全局问号在界面上打架。1–10 用中文数字，超过 10 回退阿拉伯数字。
- **仅当存在某组 `count >= 2` 时**才渲染分组标题与「共 N 道大题」。形状 A 卷（一行一道大题）必须保持原样。
- **不传 `groups` prop 时**，`AnswerCard` 的渲染结果必须与改动前完全一致。
- 无后端改动、无数据库结构变更、无接口变更。
- 提交信息用中文，与仓库既有风格一致。

---

## 文件结构

| 文件 | 职责 | 动作 |
|---|---|---|
| `frontend/src/utils/questionGroup.ts` | 分组规则的唯一实现：归一化前缀、分组、大题序号文案 | 新建 |
| `frontend/src/utils/questionGroup.test.ts` | 用 7 份真实卷的夹具钉死分组结果 | 新建 |
| `frontend/src/components/AnswerCard.vue` | 加可选 `groups` prop 并渲染分组标题 | 修改 |
| `frontend/src/views/exam/ExamSession.vue` | 答题页：算分组并传给两处 `AnswerCard` | 修改 |
| `frontend/src/views/exam/ExamReview.vue` | 背题页：同上 | 修改 |
| `.github/workflows/ci.yml` | 前端 job 补 `npm test` | 修改 |

规则全部集中在 `questionGroup.ts` 一个文件里，两个页面只负责调用——这样将来规则要调，只有一个地方要改。

---

### Task 1: 分组规则与单测

**Files:**
- Create: `frontend/src/utils/questionGroup.ts`
- Test: `frontend/src/utils/questionGroup.test.ts`

**Interfaces:**
- Consumes: 无（本任务不依赖任何前序改动）
- Produces:
  - `export interface QuestionGroup { start: number; end: number; title: string; count: number }` —— `start`/`end` 为闭区间的**全局问下标**
  - `export function groupQuestions(questions: { title?: string; content?: string }[]): QuestionGroup[]`
  - `export function groupLabel(index: number): string` —— 传**组下标**（0 起），返回「试题一」这类文案

- [ ] **Step 1: 写失败的单测**

创建 `frontend/src/utils/questionGroup.test.ts`。夹具全部取自真实卷子（`paper_questions` 关联 `questions`，`paper_type=2`、`status=1`、`source=0`），按后端加载顺序（`backend/internal/domain/repository/paper.go:128` 的 `(questions.sort_order = 0) ASC, questions.sort_order ASC, pq.sort_order ASC, questions.id ASC`）排列。

`content` 已截到 80–140 字且空白压成单个空格。**这对 `stemKey` 是忠实的**：规则只读第一个「问题」之前的前 40 个非空白码点，截断点与空白数量都在该窗口之外，不影响取值。

```ts
import { describe, expect, it } from 'vitest'
import { groupLabel, groupQuestions } from '@/utils/questionGroup'

interface Row {
  title: string
  content: string
}

const sizes = (rows: Row[]) => groupQuestions(rows).map((g) => g.count)
const bounds = (rows: Row[]) => groupQuestions(rows).map((g) => `${g.start}-${g.end}`)

// —— 真实卷夹具（content 截断，见文件头说明）——

// paper 78：title 分成 5 段，但中间三段大题的正文都从「阅读以下关于数据库缓存架构的叙述」开头
const PAPER_78: Row[] = [
  { title: '系统架构设计与评估', content: '阅读以下关于软件架构设计与评估的叙述，在答题纸上回答问题1和问题2。 【说明】 某软件公司拟为一所大型写字楼开发一套访客登记系统，以提升访客管理的效率和安全性。' },
  { title: '系统架构设计与评估', content: '阅读以下关于软件架构设计与评估的叙述，在答题纸上回答问题1和问题2。 【说明】 某软件公司拟为一所大型写字楼开发一套访客登记系统，以提升访客管理的效率和安全性。' },
  { title: '数据库系统与缓存设计', content: '阅读以下关于数据库缓存架构的叙述，在答题纸上回答问题1-3。 某电商平台使用缓存来存储热点商品数据。旁路缓存模式是常见的一种缓存模式，其核心是将缓存模块设置为I' },
  { title: '数据库系统与缓存设计', content: '阅读以下关于数据库缓存架构的叙述，在答题纸上回答问题1-3。 某电商平台使用缓存来存储热点商品数据。旁路缓存模式是常见的一种缓存模式，其核心是将缓存模块设置为I' },
  { title: '数据库系统与缓存设计', content: '阅读以下关于数据库缓存架构的叙述，在答题纸上回答问题1-3。 某电商平台使用缓存来存储热点商品数据。旁路缓存模式是常见的一种缓存模式，其核心是将缓存模块设置为I' },
  { title: '嵌入式系统架构设计', content: '阅读以下关于数据库缓存架构的叙述，在答题纸上回答问题1-3。 【说明】 某智能科技公司承接了一项开发服务型机器人的重要项目，该机器人旨在应用于大型商场，为顾客提' },
  { title: '嵌入式系统架构设计', content: '阅读以下关于数据库缓存架构的叙述，在答题纸上回答问题1-3。 【说明】 某智能科技公司承接了一项开发服务型机器人的重要项目，该机器人旨在应用于大型商场，为顾客提' },
  { title: '嵌入式系统架构设计', content: '阅读以下关于数据库缓存架构的叙述，在答题纸上回答问题1-3。 【说明】 某智能科技公司承接了一项开发服务型机器人的重要项目，该机器人旨在应用于大型商场，为顾客提' },
  { title: 'Web应用设计', content: '阅读以下关于数据库缓存架构的叙述，在答题纸上回答问题1-3。 【说明】 某电商企业计划开发一款商品推荐系统，并接入微信小程序，以提升用户购物体验，增加商品销量。' },
  { title: 'Web应用设计', content: '阅读以下关于数据库缓存架构的叙述，在答题纸上回答问题1-3。 【说明】 某电商企业计划开发一款商品推荐系统，并接入微信小程序，以提升用户购物体验，增加商品销量。' },
  { title: 'Web应用设计', content: '阅读以下关于数据库缓存架构的叙述，在答题纸上回答问题1-3。 【说明】 某电商企业计划开发一款商品推荐系统，并接入微信小程序，以提升用户购物体验，增加商品销量。' },
  { title: '系统设计与建模', content: '阅读下列说明，在答题纸上回答问题1-3。 【说明】 随着科技的进步与医疗需求的增长，某医疗科技公司计划开发一套智能胰岛素注射系统，旨在为糖尿病患者提供更精准、便' },
  { title: '系统设计与建模', content: '阅读下列说明，在答题纸上回答问题1-3。 【说明】 随着科技的进步与医疗需求的增长，某医疗科技公司计划开发一套智能胰岛素注射系统，旨在为糖尿病患者提供更精准、便' },
  { title: '系统设计与建模', content: '阅读下列说明，在答题纸上回答问题1-3。 【说明】 随着科技的进步与医疗需求的增长，某医疗科技公司计划开发一套智能胰岛素注射系统，旨在为糖尿病患者提供更精准、便' },
]

// paper 81：前 6 行 title 全都是「系统架构设计与评估」，实际是「电动车充电」+「点菜系统」两道大题
const P81_A = '某市计划建设一套电动车充电管理系统，为城市中的集中式充电站、路侧分布式充电桩以及用户手机 APP 提供统一的接入与管理服务。 问题 1 系统需满足如下质量需求场景（a～n）： a) 当用户通过手机 APP 或现场终端发起 “开始充电” 操作后，系统必须在 3 秒内给出响应结果（成'
const P81_B = '某连锁餐饮集团建设了“云–端一体的点菜系统”。 终端侧（端）：部署在门店的收银机、服务员手持PAD、自助点餐机等，负责： 1）桌台管理与点菜下单； 2）本地订单暂存与支付； 3）与云端同步菜品、价格等基础数据。 云端系统（云）：部署在集团数据中心或公有云，负责： 1）菜品基础信息'
const P81_C = '某互联网电商平台采用 MySQL + Redis的缓存架构。核心接口为“查询商品详情”，日常峰值 QPS 达到几万。 系统目前使用 Cache-Aside 模式：先查 Redis 缓存，命中则直接返回；未命中时访问数据库，并把结果写回 Redis，设置过期时间（TTL）。 近期运'
const P81_D = '某科技公司希望在其嵌入式产品线上全面提升智能化能力，计划研发一套“嵌入式智能融合操作系统”。项目目标强调AI 与操作系统深度融合，以实现统一的智能调度、更高效的数据流动、边云协同以及更智能的用户交互体验。 但在项目需求讨论阶段，老王与老李对 “AI for OS（AI 支撑操作系'

const PAPER_81: Row[] = [
  { title: '系统架构设计与评估', content: P81_A },
  { title: '系统架构设计与评估', content: P81_A },
  { title: '系统架构设计与评估', content: P81_A },
  { title: '系统架构设计与评估', content: P81_B },
  { title: '系统架构设计与评估', content: P81_B },
  { title: '系统架构设计与评估', content: P81_B },
  { title: 'Web应用设计', content: P81_C },
  { title: 'Web应用设计', content: P81_C },
  { title: 'Web应用设计', content: P81_C },
  { title: '嵌入式系统架构设计', content: P81_D },
  { title: '嵌入式系统架构设计', content: P81_D },
  { title: '嵌入式系统架构设计', content: P81_D },
]

// paper 96：后 6 行 title 全都是「Web应用设计」，实际是「数据库设计」+「Web 系统架构设计」两道大题
const P96_A = '阅读以下关于软件架构评估的叙述，在答题纸上回答问题1和问题2。 【说明】 某单位为了建设健全的公路桥梁养护管理档案，拟开发一套公路桥梁在线管理系统。在系统的需求'
const P96_B = '阅读以下关于软件系统设计的叙述，在答题纸上回答问题1至问题3。 【说明】 某软件企业受该省教育部门委托建设高校数字化教育教学资源共享平台，实现以众筹众创的方式组'
const P96_C = '阅读以下关于机器人操作系统架构的描述，回答问题1至问题3。 【说明】 随着人工智能技术的发展，工业机器人已成为当前工业界的热点研究对象。某宇航设备公司为了扩大业'
const P96_D = '阅读以下关于数据库设计的叙述，在答题纸上回答问题 1至问题 3。 【说明】 某制造企业为拓展网上销售业务，委托某软件企业开发一套电子商务网站。初期仅解决基本的网'
const P96_E = '阅读以下关于 Web 系统架构设计的叙述，在答题纸上回答问题 1至问题 3。 【说明】 某电子商务企业因发展良好，客户量逐步增大，企业业务不断扩充，导致其原有的'

const PAPER_96: Row[] = [
  { title: '系统架构设计与评估', content: P96_A },
  { title: '系统架构设计与评估', content: P96_A },
  { title: 'Web应用设计', content: P96_B },
  { title: 'Web应用设计', content: P96_B },
  { title: 'Web应用设计', content: P96_B },
  { title: '嵌入式系统架构设计', content: P96_C },
  { title: '嵌入式系统架构设计', content: P96_C },
  { title: '嵌入式系统架构设计', content: P96_C },
  { title: 'Web应用设计', content: P96_D },
  { title: 'Web应用设计', content: P96_D },
  { title: 'Web应用设计', content: P96_D },
  { title: 'Web应用设计', content: P96_E },
  { title: 'Web应用设计', content: P96_E },
  { title: 'Web应用设计', content: P96_E },
]

// paper 104：前 3 行正文都以「阅读下列说明，回答」开头，靠 title（第5题/第4题/Web应用设计）拆开
const P104_1 = '阅读下列说明，回答问题1至问题3，将解答填入答题纸的对应栏内。 【说明】 某公司拟开发一个电商平台用户行为分析系统，该系统可支持对电商平台用户行为数据的采集和实'
const P104_2 = '阅读下列说明，回答问题1至问题3，将解答填入答题纸的对应栏内。 【说明】 某国际大型企业拟向企业开发一个在线社交网络服务(SNS)系统，让企业内部的员工能够通过'
const P104_3 = '阅读下列说明，回答问题1至问题2，将解答填入答题纸的对应栏内。 【说明】 说明:某软件公司为企业开发一套员工在线教育系统，支持员工利用业余时间开展专业技术培训，'
const P104_4 = '随着嵌入式计算资源快速提升，容器技术(Docker )发挥重要作用，某公司对原有平台升级，公司将平台升级任务交给了张工，张工经过分析、调研，提出在园嵌入式操作系'
const P104_5 = '某网作为某电视台在互联网上的大型门户入口，某一年成为某奥运会中国大陆地区的特权转播商，独家全程直播了某奥运会全部的赛事，积累了庞大稳定的用户群，这些用户在使用各'

const PAPER_104: Row[] = [
  { title: '第5题', content: P104_1 },
  { title: '第4题', content: P104_2 },
  { title: 'Web应用设计', content: P104_3 },
  { title: '嵌入式', content: P104_4 },
  { title: '系统架构设计与评估', content: P104_5 },
  { title: '系统架构设计与评估', content: P104_5 },
  { title: '系统架构设计与评估', content: P104_5 },
]

// paper 310：正文开头就带「### 问题N」，前三行的前缀必须切在「问题」之前才不会各自成组
const P310_A = '题干给出项目背景，还有一个顺序图和一个用例图（暂无，欢迎大家反馈补充） --- ### 问题 1 （6分）请写出 6 种需求获取技术。'
const P310_B = '题干给出 10 种需求描述，让你作答下面的题目。 --- ### 问题 1 （10）分别将10种需求描述通过代码编号填写至功能需求，非功能需求，和设计约束'
const P310_C = '题干给出五个关系，让你解答下面的问题 --- ### 问题 1 （8 分）请问这五种关系是否满足BCNF，如果不满足用什么方式满足。'
const P310_D = '题干暂无 --- ### 问题 1 （12 分）请说明http和websocket连接的异同。'
const P310_D2 = '题干暂无 --- ### 问题 2 '

const PAPER_310: Row[] = [
  { title: '系统分析与设计', content: P310_A },
  { title: '系统分析与设计', content: '题干给出项目背景，还有一个顺序图和一个用例图（暂无，欢迎大家反馈补充） --- ### 问题 2 （10分）根据用例图和顺序图，补充用例描述参与者前置条件和后置条件。' },
  { title: '系统分析与设计', content: '题干给出项目背景，还有一个顺序图和一个用例图（暂无，欢迎大家反馈补充） --- ### 问题 3 （9分）需求获取用例的三类对象分别是什么（面向对象）。' },
  { title: '系统分析与设计', content: P310_B },
  { title: '系统分析与设计', content: '题干给出 10 种需求描述，让你作答下面的题目。 --- ### 问题 2 （10 分）根据需求描述补充数据流图实体和数据流。并且解释一下判定表，然后找到题干里哪个需求会用到判定表。' },
  { title: '系统分析与设计', content: '题干给出 10 种需求描述，让你作答下面的题目。 --- ### 问题 3 （5 分）说明设计数据流图需要遵循的基本原则有哪些。' },
  { title: '数据库系统', content: P310_C },
  { title: '数据库系统', content: '题干给出五个关系，让你解答下面的问题 --- ### 问题 2 （10 分）根据数据表，补全 SQL 语句。再谈谈如何通过反规范化实现，然后再说反规范化的缺点以及解决的思路。' },
  { title: '数据库系统', content: '题干给出五个关系，让你解答下面的问题 --- ### 问题 3 （7分）针对于王工采用触发器来解决**关系表间的数据一致性问题**，采用哪两种触发器（**题干信息缺失，上下文不完整不好作答，可以直接回答触发器有哪几种类型**）; 以及对于触发器这种方式是否合理，其有什么缺陷; 如' },
  { title: 'Web开发', content: P310_D },
  { title: 'Web开发', content: P310_D2 },
]

// paper 311：后 6 行 title 全是「数据库系统」，靠「数据库设计」/「物联网（IoT）」前缀拆开
const P311_A = '阅读以下关于软件维护的叙述，在答题纸上回答问题1至问题3。 【说明】 某企业自主研发了超市自助结算系统具备以下主要功能： 自动计价：顾客在结算时，通过自助设备扫'
const P311_B = '阅读以下关于系统分析与设计的叙述，在答题纸上回答问题1至问题3。 【说明】 某高校需开发一套在线论文管理系统，功能包括： 学生用户通过界面登录、查看可选课题列表'
const P311_C = '阅读以下关于数据库设计的叙述，在答题纸上回答问题1至问题3。 【说明】 某公司为了鼓励员工考取技能证书，提升公司竞争力，准备设计一套技能需求水平管理系统。王工在'
const P311_D = '阅读以下关于物联网（IoT）的叙述，在答题纸上回答问题1至问题 3。 【说明】 在现代农业中，为了提高生产效率和作物质量，越来越多的农场开始利用物联网（IoT）'

const PAPER_311: Row[] = [
  { title: '软件需求工程', content: P311_A },
  { title: '软件需求工程', content: P311_A },
  { title: '系统分析与设计', content: P311_B },
  { title: '系统分析与设计', content: P311_B },
  { title: '系统分析与设计', content: P311_B },
  { title: '数据库系统', content: P311_C },
  { title: '数据库系统', content: P311_C },
  { title: '数据库系统', content: P311_C },
  { title: '数据库系统', content: P311_D },
  { title: '数据库系统', content: P311_D },
  { title: '数据库系统', content: P311_D },
]

// paper 312：前 6 行 title 全是「系统分析与设计」，实际是「CRM」+「电力风险管理系统」两道大题
const P312_A = '某客户关系管理系统（CRM）正在进行面向对象分析。系统涉及多个用户角色，其主要职责如下： **系统管理员**：管理系统基础数据，包括系统参数配置、操作日志查看、'
const P312_B = '某电力风险管理系统用于对电网运行中的各类风险信息进行统一管理。 系统功能包括: - 录入和维护各类风险信息，如工作风险、基建风险、电网运行风险等； - 支持风险'
const P312_C = '随着嵌入式系统向智能化、联网化发展，传统的单一固化式部署已无法满足灵活性和敏捷性需求。容器技术特别是Docker的引入，为嵌入式系统部署提供了新的解决方案。 -'
const P312_D = '某电商平台设计了如下两个表： **订单表（orders）**：包含字段为：`订单号（order_id）`、`用户编号（user_id）`、`商品编号（produ'
const P312_E = '某智慧城市运维平台采用典型 Web 三层架构。前端展示层用多种浏览器技术实现可视化；业务应用层由多个子系统组成；数据存储层同时引入关系型与非关系型数据库，并按数'

const PAPER_312: Row[] = [
  { title: '系统分析与设计', content: P312_A },
  { title: '系统分析与设计', content: P312_A },
  { title: '系统分析与设计', content: P312_A },
  { title: '系统分析与设计', content: P312_B },
  { title: '系统分析与设计', content: P312_B },
  { title: '系统分析与设计', content: P312_B },
  { title: '嵌入式系统设计', content: P312_C },
  { title: '嵌入式系统设计', content: P312_C },
  { title: '嵌入式系统设计', content: P312_C },
  { title: '数据库系统', content: P312_D },
  { title: '数据库系统', content: P312_D },
  { title: '数据库系统', content: P312_D },
  { title: 'Web开发', content: P312_E },
  { title: 'Web开发', content: P312_E },
  { title: 'Web开发', content: P312_E },
]

describe('groupQuestions —— 真实卷子', () => {
  it('paper 78：title 分 5 段，正文共用开头的三段靠 title 拆开', () => {
    expect(sizes(PAPER_78)).toEqual([2, 3, 3, 3, 3])
    expect(bounds(PAPER_78)).toEqual(['0-1', '2-4', '5-7', '8-10', '11-13'])
  })

  it('paper 81：两道同名「系统架构设计与评估」靠正文前缀拆开', () => {
    expect(sizes(PAPER_81)).toEqual([3, 3, 3, 3])
  })

  it('paper 96：三道同名「Web应用设计」里后两道靠正文前缀拆开', () => {
    expect(sizes(PAPER_96)).toEqual([2, 3, 3, 3, 3])
  })

  it('paper 104：前三行正文都以「阅读下列说明，回答」开头，靠 title 拆开', () => {
    expect(sizes(PAPER_104)).toEqual([1, 1, 1, 1, 3])
  })

  it('paper 310：正文自带「### 问题N」，前缀必须切在「问题」之前', () => {
    expect(sizes(PAPER_310)).toEqual([3, 3, 3, 2])
  })

  it('paper 311：两道同名「数据库系统」靠正文前缀拆开', () => {
    expect(sizes(PAPER_311)).toEqual([2, 3, 3, 3])
  })

  it('paper 312：两道同名「系统分析与设计」靠正文前缀拆开', () => {
    expect(sizes(PAPER_312)).toEqual([3, 3, 3, 3, 3])
  })
})

describe('groupQuestions —— 退化与边界', () => {
  it('形状 A：每行 title 都不同 → 每组恰好 1 格，组数等于行数', () => {
    const rows: Row[] = [
      { title: '第1题', content: '甲系统需求如下。' },
      { title: '第2题', content: '乙系统需求如下。' },
      { title: '第3题', content: '丙系统需求如下。' },
    ]
    expect(sizes(rows)).toEqual([1, 1, 1])
  })

  it('单行卷 → 1 组 1 格', () => {
    expect(sizes([{ title: '第1题', content: '某项目有10个活动。' }])).toEqual([1])
  })

  it('空数组 → 空分组', () => {
    expect(groupQuestions([])).toEqual([])
  })

  it('title / content 缺失不抛错', () => {
    const rows = [
      {} as Row,
      {} as Row,
      { title: '甲', content: undefined as unknown as string },
    ]
    expect(() => groupQuestions(rows)).not.toThrow()
    expect(sizes(rows)).toEqual([2, 1])
  })

  it('只有空白差异的两行归为同一组（换行 / 全角空格 / 不换行空格）', () => {
    const rows: Row[] = [
      { title: '甲', content: '某市建设充电系统。\n\n问题 1 说明需求。' },
      { title: '甲', content: '某市建设充电系统。　问题 1 说明需求。' },
      { title: '甲', content: '某市建设充电系统。  问题 2 说明需求。' },
    ]
    expect(sizes(rows)).toEqual([3])
  })

  it('前缀只取前 40 个码点：第 41 个码点起不同不影响分组', () => {
    const head = '甲'.repeat(40)
    const rows: Row[] = [
      { title: '甲', content: `${head}第一种场景描述。` },
      { title: '甲', content: `${head}第二种场景描述。` },
    ]
    expect(sizes(rows)).toEqual([2])
  })

  it('前缀第 40 个码点以内不同即分组（去掉空白后按码点计数）', () => {
    const rows: Row[] = [
      { title: '甲', content: `${'甲'.repeat(39)}乙更多内容。` },
      { title: '甲', content: `${'甲'.repeat(39)}丙更多内容。` },
    ]
    expect(sizes(rows)).toEqual([1, 1])
  })
})

describe('groupLabel', () => {
  it('1–10 用中文数字', () => {
    expect(groupLabel(0)).toBe('试题一')
    expect(groupLabel(9)).toBe('试题十')
  })
  it('超过 10 回退阿拉伯数字', () => {
    expect(groupLabel(10)).toBe('试题11')
    expect(groupLabel(15)).toBe('试题16')
  })
})
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd frontend && npx vitest run src/utils/questionGroup.test.ts`
Expected: FAIL —— 报 `Failed to resolve import "@/utils/questionGroup"`

- [ ] **Step 3: 写实现**

创建 `frontend/src/utils/questionGroup.ts`：

```ts
export interface QuestionGroup {
  // 闭区间，均为全局问下标（0 起）
  start: number
  end: number
  title: string
  count: number
}

// 归一化前缀长度：与后端校验所用 SQL 的 LEFT(..., 40) 一致
const STEM_LIMIT = 40

/**
 * 大题归属的归一化前缀：正文以第一个「问题」切开后的左半段，
 * 去掉全部空白后取前 40 个码点。
 *
 * 与校验所用 SQL 等价（已实测本库 MySQL 8 的 ICU 正则会把全角空格 U+3000、
 * 不换行空格 U+00A0 与 ASCII 空格一并剥掉，与 JS 的 \s 一致）：
 *   LEFT(REGEXP_REPLACE(SUBSTRING_INDEX(content, '问题', 1), '[[:space:]]+', ''), 40)
 * 用 Array.from 按码点切，与 MySQL LEFT 的字符语义一致（非 BMP 字符下才分叉）。
 */
function stemKey(content: string): string {
  return Array.from(content.split('问题')[0].replace(/\s+/g, ''))
    .slice(0, STEM_LIMIT)
    .join('')
}

/**
 * 把整卷题目按大题分组。
 *
 * 换组条件是两个信号的并集，缺一不可：
 *   - title 与上一问不同；或
 *   - 归一化前缀与上一问不同。
 * 只用 title 会漏切「相邻两道大题同名」的情况（title 是知识域标签，会重名）；
 * 只用前缀会误切「多道大题共用开头」的情况（比如都从「阅读以下关于数据库缓存架构的叙述」起）。
 */
export function groupQuestions(
  questions: { title?: string; content?: string }[],
): QuestionGroup[] {
  const groups: QuestionGroup[] = []
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
      last.count += 1
    }
  })
  return groups
}

const CN_NUMERALS = ['一', '二', '三', '四', '五', '六', '七', '八', '九', '十']

/** 大题序号文案。index 为组下标（0 起）；1–10 用中文数字，超过 10 回退阿拉伯数字。 */
export function groupLabel(index: number): string {
  return `试题${CN_NUMERALS[index] ?? String(index + 1)}`
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd frontend && npx vitest run src/utils/questionGroup.test.ts`
Expected: PASS —— 全部用例通过

- [ ] **Step 5: 跑全量单测确认没有打破既有用例**

Run: `cd frontend && npm test`
Expected: PASS —— 3 个测试文件全绿（改动前为 2 个文件 23 个用例）

- [ ] **Step 6: 提交**

```bash
git add frontend/src/utils/questionGroup.ts frontend/src/utils/questionGroup.test.ts
git commit -m "feat(frontend): 新增按大题分组的纯函数 questionGroup

答题卡要按大题分组，而库里没有显式大题结构。用 title 连续段与
「正文首个『问题』之前 40 个非空白码点」两个信号取并集：前者在
共用开头的大题上会误并，后者在重名大题上会漏切，缺一不可。

夹具取 7 份真实案例卷，覆盖四种失败模式。"
```

---

### Task 2: AnswerCard 渲染分组并接入两个整卷页面

**Files:**
- Modify: `frontend/src/components/AnswerCard.vue:1-46`（script）、`:55-61`（头部）、`:71-82`（格子）
- Modify: `frontend/src/views/exam/ExamSession.vue:25`（import）、`:128` 后（computed）、`:587` 与 `:600`（两处 prop）
- Modify: `frontend/src/views/exam/ExamReview.vue:18`（import）、`:49` 后（computed）、`:193` 与 `:205`（两处 prop）

**Interfaces:**
- Consumes: Task 1 的 `groupQuestions`、`groupLabel`、`QuestionGroup`
- Produces: `AnswerCard` 新增可选 prop `groups?: QuestionGroup[]`；两个页面内新增 `const questionGroups = computed(...)`

**关于本任务的验证方式（重要）：** 本仓库的 vitest 跑在 `environment: 'node'`，devDependencies 里**没有** `@vue/test-utils` 也没有 jsdom/happy-dom，所以本任务的自动化闸门只有 `vue-tsc` 类型检查与构建，渲染效果靠 Step 5 的人工浏览器核对。

> ⚠️ **开工前需确认的取舍：** 按本计划做完，新增的渲染代码**没有**自动化测试覆盖。补齐它要引入 `@vue/test-utils` + jsdom（或 happy-dom）两个新依赖和一整套新测试环境——**本设计把它排除在范围外，这是本计划新做的决定，spec 的「明确不做」一节并未覆盖这一条。** 执行前请确认接受；若不接受，本任务需要先单独做一件事：搭组件测试环境。

- [ ] **Step 1: 给 AnswerCard 加 prop 与分组渲染**

把 `frontend/src/components/AnswerCard.vue` 的 `<script setup>` 段（第 1–46 行）替换为：

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'
import { groupLabel, type QuestionGroup } from '@/utils/questionGroup'

interface Props {
  total: number
  currentIndex: number
  answeredMap: Record<number, boolean>
  // 判题结果（练习模式/已交卷）：true=正确 false=错误 null=已答未判(主观题)。
  // 不传则为普通作答模式，仅区分已答/未答。
  resultMap?: Record<number, boolean | null>
  // 大题分组（整卷场景才传，见 utils/questionGroup）。不传则按键位平铺，与旧版一致。
  groups?: QuestionGroup[]
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'jump', index: number): void
}>()

const collapsed = ref(false)
const hasResult = computed(() => !!props.resultMap)

const groupList = computed(() => props.groups ?? [])
// 只有真的存在「一道大题含多问」时才画分组标题：
// 形状 A 的卷子（一行一道大题）每组恰好 1 格，保持原来的平铺观感。
const showGroups = computed(() => groupList.value.some((g) => g.count >= 2))

function getBlockClass(index: number) {
  const base = 'w-8 h-8 rounded-md text-xs font-medium cursor-pointer transition-all flex items-center justify-center shrink-0'
  if (index === props.currentIndex) {
    return `${base} bg-blue-600 text-white shadow-md ring-2 ring-blue-300`
  }
  // 判题结果模式：正确=绿，错误=红，已答未判=琥珀，未作答=灰
  if (props.resultMap) {
    if (index in props.resultMap) {
      const r = props.resultMap[index]
      if (r === true) {
        return `${base} bg-green-100 text-green-700 hover:bg-green-200 dark:bg-green-900/40 dark:text-green-400`
      }
      if (r === false) {
        return `${base} bg-red-100 text-red-700 hover:bg-red-200 dark:bg-red-900/40 dark:text-red-400`
      }
      return `${base} bg-amber-100 text-amber-700 hover:bg-amber-200 dark:bg-amber-900/40 dark:text-amber-400`
    }
    return `${base} bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600`
  }
  // 普通作答模式：已答=绿，未答=灰
  if (props.answeredMap[index]) {
    return `${base} bg-green-100 text-green-700 hover:bg-green-200 dark:bg-green-900/40 dark:text-green-400`
  }
  return `${base} bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600`
}
</script>
```

- [ ] **Step 2: 头部补「共 N 道大题」**

把第 59–60 行的两个 `<span>` 之后补一个：

```vue
        <span class="font-medium text-gray-700 dark:text-gray-300">答题卡</span>
        <span class="text-sm text-gray-400">({{ Object.keys(answeredMap).length }}/{{ total }})</span>
        <span v-if="showGroups" class="text-sm text-gray-400">· 共 {{ groupList.length }} 道大题</span>
```

- [ ] **Step 3: 格子区按分组渲染**

把第 **71–82** 行（`<!-- Answer Grid -->` 注释行，到平铺网格那个 `</div>`）整体替换为（注意第 71 行就是注释本身，别从 72 行起替换，否则注释会留下两份）：

```vue
    <!-- Answer Grid -->
    <div v-show="!collapsed" class="px-4 pb-4">
      <!-- 整卷且存在多问大题时按大题分组；其余情况保持按键位平铺 -->
      <template v-if="showGroups">
        <div v-for="(g, gi) in groupList" :key="g.start" :class="gi > 0 ? 'mt-3' : ''">
          <div class="flex items-baseline justify-between gap-2 mb-1.5">
            <span
              class="text-xs font-medium text-gray-600 dark:text-gray-300 truncate"
              :title="g.title"
            >
              {{ groupLabel(gi) }}<template v-if="g.title"> · {{ g.title }}</template>
            </span>
            <span class="text-xs text-gray-400 shrink-0">{{ g.count }}问</span>
          </div>
          <div class="grid gap-2" style="grid-template-columns: repeat(auto-fill, minmax(2rem, 1fr));">
            <button
              v-for="n in g.count"
              :key="g.start + n - 1"
              :class="getBlockClass(g.start + n - 1)"
              @click="emit('jump', g.start + n - 1)"
            >
              {{ g.start + n }}
            </button>
          </div>
        </div>
      </template>
      <div v-else class="grid gap-2" style="grid-template-columns: repeat(auto-fill, minmax(2rem, 1fr));">
        <button
          v-for="(_, i) in total"
          :key="i"
          :class="getBlockClass(i)"
          @click="emit('jump', i)"
        >
          {{ i + 1 }}
        </button>
      </div>
```

（其后的 `<!-- Legend -->` 及两个图例块保持原样不动。）

- [ ] **Step 4: 接入答题页与背题页**

`frontend/src/views/exam/ExamSession.vue`：

1. 第 25 行的 blanks import 块之后新增一行：
   ```ts
   import { groupQuestions } from '@/utils/questionGroup'
   ```
2. 第 128 行 `const currentQuestion = computed(...)` 之后新增：
   ```ts
   // 答题卡按大题分组：案例分析卷一行往往是一道大题的一问（见 utils/questionGroup）
   const questionGroups = computed(() => groupQuestions(questions.value))
   ```
3. 两处 `AnswerCard` 各新增一行 `:groups`，位置是**紧跟 `:current-index="currentIndex"` 之后**，缩进与该行逐字一致。两处的缩进**不同**，别照抄同一个：

   `ExamSession.vue:589`（桌面，缩进 **12 空格**）之后：
   ```vue
               :groups="questionGroups"
   ```
   `ExamSession.vue:602`（移动，缩进 **10 空格**）之后：
   ```vue
             :groups="questionGroups"
   ```

`frontend/src/views/exam/ExamReview.vue`：

1. 第 18 行的 blanks import 块之后新增一行：
   ```ts
   import { groupQuestions } from '@/utils/questionGroup'
   ```
2. 第 49 行 `const currentQuestion = computed(...)` 之后新增：
   ```ts
   // 答题卡按大题分组：背题页与答题页读同一份卷子，分组口径必须一致
   const questionGroups = computed(() => groupQuestions(questions.value))
   ```
3. 两处 `AnswerCard` 各新增一行 `:groups`，同样紧跟 `:current-index="currentIndex"` 之后、缩进与该行一致：

   `ExamReview.vue:195`（桌面，缩进 **12 空格**）之后：
   ```vue
               :groups="questionGroups"
   ```
   `ExamReview.vue:207`（移动，缩进 **10 空格**）之后：
   ```vue
             :groups="questionGroups"
   ```

- [ ] **Step 5: 构建（类型检查）并人工验收**

Run: `cd frontend && npm run build`
Expected: PASS —— `vue-tsc -b` 无类型错误，vite 构建完成

然后人工在浏览器里核对（**本仓库无法自动化这一步**），`npm run dev` 后：

| 地址 | 预期 |
|---|---|
| `/exam/84?mode=practice` | 形状 A，5 行 5 道大题 → **分组标题不出现**，答题卡与改动前逐像素一致 |
| `/exam/78?mode=practice` | 14 格，出现 5 个分组标题：试题一 · 系统架构设计与评估（2问）/ 试题二 · 数据库系统与缓存设计（3问）/ 试题三 · 嵌入式系统架构设计（3问）/ 试题四 · Web应用设计（3问）/ 试题五 · 系统设计与建模（3问）；头部显示「· 共 5 道大题」 |
| `/exam/81?mode=practice` | 12 格，**4 个**分组，每个 3 问（两道同名「系统架构设计与评估」被拆开） |
| `/exam/78/review` | 与答题页同样的分组 |
| 任一有分组的卷子点第 6 格 | 右侧/顶部仍显示「第 6 / N 题」，高亮落在第 6 格 |

- [ ] **Step 6: 提交**

```bash
git add frontend/src/components/AnswerCard.vue frontend/src/views/exam/ExamSession.vue frontend/src/views/exam/ExamReview.vue
git commit -m "feat(frontend): 答题卡按大题分组显示

AnswerCard 新增可选 groups prop，仅当存在某组含 ≥2 格时才渲染分组标题，
形状 A 的卷子（一行一道大题）保持原样。格子仍是全局问号，大题走「试题N」，
两者不打架。接入答题页与背题页。"
```

---

### Task 3: CI 前端 job 补 npm test

**Files:**
- Modify: `.github/workflows/ci.yml:98-105`

**Interfaces:**
- Consumes: Task 1 的单测文件
- Produces: 无（纯 CI 配置）

**背景：** 前端 job 目前只跑 `npm ci` 与 `npm run build`，**不跑 `npm test`**（`package.json` 里 `test` = `vitest run` 是有的）。不加这一步，Task 1 的算法单测就只是一份本机可跑的文档，没有任何闸门会在合并前拦住它回归。

- [ ] **Step 1: 在 Install 与 Type-check + Build 之间插入 Test 步骤**

把 `.github/workflows/ci.yml` 中：

```yaml
      - name: Install
        working-directory: frontend
        run: npm ci
      # build = vue-tsc -b && vite build && prerender。
```

改为：

```yaml
      - name: Install
        working-directory: frontend
        run: npm ci
      # vitest 跑纯函数单测（environment: node，无组件测试）。
      # 放在 build 之前：单测失败比 vue-tsc/vite 报错定位快得多。
      - name: Test
        working-directory: frontend
        run: npm test
      # build = vue-tsc -b && vite build && prerender。
```

- [ ] **Step 2: 本地确认这条命令本身是绿的**

Run: `cd frontend && npm test`
Expected: PASS —— 3 个测试文件全绿。若不绿，说明 Task 1 引入了回归，先修 Task 1，**不要**为了放过而删掉这个步骤。

- [ ] **Step 3: 提交并观察 CI**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: 前端 job 补 npm test

vitest 早就配好了但 CI 一直没跑它。新加的分组算法单测若不上闸门，
就只是本机可跑的文档。步骤插在 Install 与 build 之间，失败定位更快。"
git push origin main
```

Expected: 推送后 GitHub Actions 的 `Frontend (type-check + build)` job 里出现 `Test` 步骤且为绿。

**判 CI 成败不要用分支徽章**——新 run 排队未跑完时徽章会吐上一次的 `passing`（陈旧绿）。用公开 REST API 比对 `head_sha` 是否等于本次推送的 sha，再看 `status` / `conclusion`：
`curl -s "https://api.github.com/repos/jycoast/ruankao/actions/runs?branch=main&per_page=3"`

---

## 计划自查

**Spec 覆盖：**

| Spec 章节 | 对应任务 |
|---|---|
| 决策 1 分组算法 | Task 1 Step 3（`stemKey` + `groupQuestions`） |
| 决策 2 验证结果 | Task 1 Step 1（7 份真实卷夹具把结论钉死） |
| 决策 3 放在哪 | Task 1（新建 `utils/questionGroup.ts`） |
| 决策 4 AnswerCard 展示 | Task 2 Step 1–3 |
| 决策 5 接入范围 | Task 2 Step 4 |
| 决策 6 CI 闸门 | Task 3 |
| 边界情况 1（字段缺失） | Task 1「title / content 缺失不抛错」 |
| 边界情况 2（单行卷） | Task 1「单行卷」 |
| 边界情况 3（形状 A） | Task 1「形状 A」+ Task 2 Step 5 的 `/exam/84` 验收 |
| 边界情况 4（分组不精确） | 无代码对策（spec 已声明为可接受），Task 1 的夹具是回归网 |
| 边界情况 5（大题数 > 10） | Task 1 `groupLabel` 用例 |

**顺序依赖：** Task 2 依赖 Task 1 导出的类型与函数；Task 3 依赖 Task 1 的单测文件存在。Task 1 必须先做。

**已知的验证缺口（不掩饰）：** Task 2 的渲染没有自动化测试——本仓库 vitest 是 `environment: 'node'` 且没有 `@vue/test-utils`，加组件测试等于引入新依赖与新测试环境，超出本设计范围。因此 Task 2 的渲染正确性依赖 Step 5 的人工浏览器核对，Reviewer 应以类型检查 + 代码阅读 + 该核对的记录为准。
