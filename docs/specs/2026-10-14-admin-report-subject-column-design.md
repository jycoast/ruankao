# 后台题目反馈列表显示科目

- 日期：2026-10-14
- 状态：设计已确认，待实施
- 影响面：后端只读查询 + 管理端列表展示；**无数据库结构变更**

## 背景与目标

管理端「题目错误报告」（`/admin/reports`）列表目前能看出题目类型、年份、题干摘要，但**看不出这道题属于哪个科目**。

该列表是全站聚合的——一条反馈可能来自任意科目——所以管理员判断该转给谁处理时，恰恰缺了最关键的维度。

目标：在该列表中，为每条反馈显示其题目的**科目名称**。

## 现状

`backend/internal/domain/repository/report.go:44-49` 的 `ListAll` 已经是「后端 JOIN 出关联实体名称」的写法：

```go
base := r.db.Table("question_reports qr").
    Select("qr.id, qr.question_id, ... " +
        "u.username, q.title AS question_title, q.year AS question_year, " +
        "q.question_type AS question_type, SUBSTR(q.content, 1, 300) AS question_content, ...").
    Joins("LEFT JOIN users u ON u.id = qr.user_id").
    Joins("LEFT JOIN questions q ON q.id = qr.question_id")
```

它已 JOIN `users`（出 `username`）与 `questions`（出 `question_*`），**未 JOIN `subjects`**。全仓库检索 `JOIN subjects` / `subject_name` 零命中，本文是该模式在科目上的第一次应用。

`question_reports` 表本身**没有** `subject_id`（见 `backend/migrations/000_baseline_schema.sql:469-482`），科目只能经题目推导：

```
question_reports.question_id → questions.subject_id → subjects.id → subjects.name
```

另有一套既有惯例是「科目作为页面级上下文」：`frontend/src/stores/subject.ts` 的 `subjectStore` 加载科目字典，页面按其 `currentSubjectId` 过滤、表头显示当前科目（如 `QuestionManage.vue:38,173`）。**本文不适用**——本列表是全局的，handler `ListAll` 只接受 `status`（`backend/internal/api/handler/question_report.go:57`），且它根本不是科目作用域页面。

## 决策

后端 JOIN 出科目名，随列表接口一并返回；前端新增独立列渲染。

### 数据流

```
GET /api/admin/reports?status=&page=
  → QuestionReportRepo.ListAll()
      Table("question_reports qr")
      LEFT JOIN users     u ON u.id = qr.user_id       -- 既有
      LEFT JOIN questions q ON q.id = qr.question_id   -- 既有
      LEFT JOIN subjects  s ON s.id = q.subject_id     -- 新增：s.name AS subject_name
  → []repository.QuestionReportView（新增 SubjectName 字段）
  → QuestionReportManage.vue 新增一列，渲染 row.subject_name
```

### 改动清单

后端：

| 文件 | 改动 |
|---|---|
| `internal/domain/repository/report.go` `ListAll`（:44-49） | `Joins` 加一行、`Select` 加一段 |
| `internal/domain/repository/report.go` `ListByUserID`（:63-81） | 同上 |
| `internal/domain/repository/notification.go` `QuestionReportView`（:71-86） | 新增 `SubjectName string`，tag 与邻字段同款：`json:"subject_name" gorm:"column:subject_name"` |

`ListByUserID` 一并改：它与 `ListAll` 的 Select 列表逐字相同（代码注释即写明 "identical query plus WHERE"）。只改一处会让 `subject_name` 变成**有时有值、有时为空**的字段，是个会绊人的坑；而用户侧不渲染它，多返回一个字段无害。

前端：

| 文件 | 改动 |
|---|---|
| `src/types/index.ts` `QuestionReport`（:158-174） | 新增可选字段 `subject_name` |
| `src/views/admin/QuestionReportManage.vue` 表格（:126-176） | 在「题目」列之后、「题干摘要」之前插入「科目」列 |

新列用固定 `width="120"`——「科目」是短值，与同表的「问题类型 130」「状态 90」「提交用户 100」同属一类；不像「题干摘要」那样用 `min-width` 让长文本吃掉剩余宽度。值为空时显示 `-`。

## 边界情况

1. **题目被删，或 `subject_id` 指向不存在的科目** → JOIN 出 NULL，该条反馈照常列出，科目列显示 `-`。

   这里必须用 `LEFT JOIN` 而非 `INNER JOIN`：后者会让这类反馈**整条从列表消失**，管理端丢数据远比显示不全严重。

   另外，NULL 不会把接口打崩。`QuestionReportView` 里 LEFT JOIN 出来的字段全是非指针类型（`QuestionTitle string`、`QuestionYear int16`、`QuestionType int8`，:81-83），NULL 落到它们身上是**零值**而非错误——既有代码每天就在走这条路径。`SubjectName string` 沿用同一写法即可，行为一致。测试用例 3 会把这条路径钉住。
2. **分页总数不受影响**。`base.Count(&total)`（:55 / :75）复用的是**带 JOIN 的同一个 base**，总数是在 JOIN 之后的结果集上数的——所以这条新增 JOIN 必须保证不改变行数：`subjects.id` 是主键，`ON s.id = q.subject_id` 最多命中一行，行数不变，`total` 与列表仍对得上。这也正是此处不能改成 `INNER JOIN` 的第二个理由（`total` 会**少**）、更不能挂一张一对多的表（`total` 会**多**，末页变空）。
3. **不给 `question_reports` 加冗余 `subject_id` 列**：那是数据重复，题目改科目后必然漂移；科目统一经题目推导。
4. **科目在后台被改名** → 列表下次查询即为新名（JOIN 实时取值，不经任何缓存）。

## 测试

该查询目前**零测试覆盖**。在 `internal/domain/repository` 包按 `wrong_question_test.go` 的 sqlmock 写法补用例，断言三点：

1. 生成的 SQL 含对 `subjects` 的 JOIN——以后有人删掉这行会立刻变红；
2. 结果行里的 `subject_name` 能正确扫进 `QuestionReportView.SubjectName`；
3. **`subject_name` 为 NULL 的行扫出来是空串、不报错**——把边界情况 1 的后半句变成一条会红的断言。sqlmock 在 NULL 列上返回的 `driver.Value` 与 go-sql-driver 一致，走的是同一条扫描路径。

既有测试不受影响：`dashboard_test.go:215` 只数 `question_reports` 的行数；`notification_test.go` 的 `fakeReportRepo` 是接口实现，加字段不破坏它。

## 明确不做

- 不按科目筛选/排序（本次未要求）。
- 不改用户侧「我的纠错」（`MyReports.vue`）的显示。
- 不给「意见反馈」列表（`FeedbackView`）加科目。
