# 后台题目反馈列表显示科目 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让管理端「题目错误报告」列表的每条反馈显示其题目所属的科目名称。

**Architecture:** 后端在既有的 `LEFT JOIN users` / `LEFT JOIN questions` 之外再 `LEFT JOIN subjects`，把 `s.name` 作为 `subject_name` 随列表接口一并返回；前端在 `QuestionReportManage.vue` 的表格里加一列渲染它。`question_reports` 表没有 `subject_id`，科目只能经题目推导（`question_reports.question_id → questions.subject_id → subjects.id`）。**不动数据库结构。**

**Tech Stack:** Go 1.24（CI）/ 1.25.5（本机）、Gin、GORM、MySQL 8.0；Vue 3 + Element Plus + vue-tsc；测试用 `DATA-DOG/go-sqlmock`。

**Spec:** `docs/specs/2026-10-14-admin-report-subject-column-design.md`

## Global Constraints

- **不加 `subject_id` 列到 `question_reports`，不做任何数据库结构变更、不新增迁移文件。**
- **必须是 `LEFT JOIN`，不可以是 `INNER JOIN`。** 两个理由：`INNER` 会让「题目已被删」的反馈整条从列表消失；而且 `ListAll`/`ListByUserID` 里的 `base.Count(&total)`（`report.go:55`、`:75`）复用的是**带 JOIN 的同一个 base**，`INNER` 会让 `total` 偏小——被隐藏的行数在分页器上完全看不出来。
- **新增的 JOIN 不得改变行数。** 规则是挂在 `s.id = q.subject_id` 上，`subjects.id` 是 `BIGINT UNSIGNED` 主键，最多命中一行。不要改成挂一对多的表。
- **`ListAll` 与 `ListByUserID` 必须一起改。** 两者 Select 列表逐字相同（注释即写明 "identical query plus local user filter"），只改一处会让 `subject_name` 变成有时有值、有时为空的字段。
- **YAGNI，明确不做：** 不按科目筛选/排序；不改用户侧 `MyReports.vue`；不给 `FeedbackView`（意见反馈）加科目。
- 前端 Node 需 ≥22.12（`unplugin@3` 用了 `import.meta.dirname`）。前端的自动化闸门是 `vue-tsc` 类型检查，CI 的 Frontend job 只跑 `npm run build`，**不跑 `npm test`**。
- 提交信息用中文 + 约定式前缀（`feat:` / `test:` / `fix:`），与仓库既有风格一致。

---

### Task 1: 后端列表 JOIN 出科目名

**Files:**
- Create: `backend/internal/domain/repository/report_test.go`
- Modify: `backend/internal/domain/repository/notification.go:71-86`（`QuestionReportView`）
- Modify: `backend/internal/domain/repository/report.go:40-81`（`ListAll` 与 `ListByUserID`）

**Interfaces:**
- Consumes: 无（本任务是最底层）。
- Produces:
  - `QuestionReportView.SubjectName string`，json tag `subject_name`，gorm tag `column:subject_name`
  - `QuestionReportRepository.ListAll(page, pageSize, status int) ([]QuestionReportView, int64, error)` —— 签名不变，返回的每行多带 `SubjectName`
  - `QuestionReportRepository.ListByUserID(userID uint64, page, pageSize int) ([]QuestionReportView, int64, error)` —— 同上
  - 测试 helper `newQuestionReportRepoWithSQLMock(t *testing.T) (QuestionReportRepository, sqlmock.Sqlmock)`

> **为什么先写测试：** 这个查询目前**零测试覆盖**。写测试还有一个当下的理由——`TestQuestionReportListAll_NullSubjectNameScansEmpty` 会把设计稿边界情况 1 的后半句（NULL 落到非指针 `string` 上不会报错）从「我读过 GORM 源码」变成一条会红的断言。**如果那条测试红了，不要改测试**——那说明 NULL 确实扫不进去，得把 `SubjectName` 改成 `*string` 或 `sql.NullString`，回来告诉我。

- [ ] **Step 1: 写测试文件**

创建 `backend/internal/domain/repository/report_test.go`。helper 刻意与本包其它测试文件解耦（与 `wrong_question_test.go:15` 的写法同款，各自独立维护）：

```go
package repository

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// newQuestionReportRepoWithSQLMock 构造 QuestionReportRepository，后端走 sqlmock。
// 与其它 repo 测试的 helper 解耦，本文件独立维护。
func newQuestionReportRepoWithSQLMock(t *testing.T) (QuestionReportRepository, sqlmock.Sqlmock) {
	t.Helper()
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      conn,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return NewQuestionReportRepository(gormDB), mock
}

// 两条 SQL 的匹配片段。用 raw string 避免反斜杠双重转义。
// 说明：GORM 生成的 SQL 是单行的，`.*` 可以安全跨越中间的 JOIN。
// 注意 `.*` 两侧不要留空格 —— 那会要求两个空格，而 SQL 里通常只有一个
// （写成 `subject_name .* FROM` 永远匹配不上，因为实际只有 `subject_name FROM`）。
const (
	joinSubjects      = `LEFT JOIN subjects s ON s\.id = q\.subject_id`
	selectSubjectName = `s\.name AS subject_name`
)

const wantSubjectName = "系统架构设计师"

// 管理端列表（ListAll）：COUNT 与主查询都必须 JOIN subjects，
// 且主查询要 SELECT 出 s.name AS subject_name —— 少了任何一处这里就红。
func TestQuestionReportListAll_JoinsSubjectAndScansName(t *testing.T) {
	repo, mock := newQuestionReportRepoWithSQLMock(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM question_reports qr.*` + joinSubjects).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(int64(1)))
	mock.ExpectQuery(selectSubjectName + `.*FROM question_reports qr.*` + joinSubjects + `.*ORDER BY qr\.created_at DESC LIMIT [?]`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "subject_name"}).AddRow(1, wantSubjectName))

	list, total, err := repo.ListAll(1, 20, -1)
	if err != nil {
		t.Fatalf("ListAll 应无错返回，得到 %v", err)
	}
	if total != 1 {
		t.Fatalf("期望 total=1，实际 %d", total)
	}
	if len(list) != 1 {
		t.Fatalf("期望 1 行，实际 %d", len(list))
	}
	if list[0].SubjectName != wantSubjectName {
		t.Fatalf("期望 SubjectName=%q，实际 %q", wantSubjectName, list[0].SubjectName)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// 用户侧列表（ListByUserID）与 ListAll 共用同一份 Select/Join，
// 必须一起改 —— 只改 ListAll 时这条会红。
func TestQuestionReportListByUserID_JoinsSubjectAndScansName(t *testing.T) {
	repo, mock := newQuestionReportRepoWithSQLMock(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM question_reports qr.*` + joinSubjects + `.*WHERE qr\.user_id = [?]`).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(int64(1)))
	mock.ExpectQuery(selectSubjectName + `.*FROM question_reports qr.*` + joinSubjects + `.*WHERE qr\.user_id = [?] ORDER BY qr\.created_at DESC LIMIT [?]`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "subject_name"}).AddRow(1, wantSubjectName))

	list, total, err := repo.ListByUserID(42, 1, 20)
	if err != nil {
		t.Fatalf("ListByUserID 应无错返回，得到 %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("期望 total=1 / 1 行，实际 total=%d len=%d", total, len(list))
	}
	if list[0].SubjectName != wantSubjectName {
		t.Fatalf("期望 SubjectName=%q，实际 %q", wantSubjectName, list[0].SubjectName)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// 题目被删 → LEFT JOIN 出 NULL。NULL 扫进非指针 string 必须是零值而不是错误，
// 否则管理端整个列表接口会 500（而不是某一行显示「-」）。
// 这条红了请改 SubjectName 的类型，不要改断言。
func TestQuestionReportListAll_NullSubjectNameScansEmpty(t *testing.T) {
	repo, mock := newQuestionReportRepoWithSQLMock(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM question_reports qr.*` + joinSubjects).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(int64(1)))
	mock.ExpectQuery(selectSubjectName + `.*FROM question_reports qr.*` + joinSubjects + `.*ORDER BY qr\.created_at DESC LIMIT [?]`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "subject_name"}).AddRow(1, nil))

	list, _, err := repo.ListAll(1, 20, -1)
	if err != nil {
		t.Fatalf("subject_name 为 NULL 时不应报错，得到 %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("期望 1 行，实际 %d", len(list))
	}
	if list[0].SubjectName != "" {
		t.Fatalf("NULL 应落成空串，实际 %q", list[0].SubjectName)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}
```

- [ ] **Step 2: 跑测试，确认它红了**

每个步骤都从仓库根目录 `E:\github\ruankao` 开始。

```bash
cd backend && go test ./internal/domain/repository/ -run TestQuestionReport -v
```

Expected: 编译失败，`undefined: ... SubjectName` 之类。这是第一层红——测试引用了一个还不存在的字段。

- [ ] **Step 3: 加上 `SubjectName` 字段，让包先能编译**

修改 `backend/internal/domain/repository/notification.go` 的 `QuestionReportView`（:71-86）。**只加一个字段**，`QuestionPreview` 那行不动：

```go
	QuestionPreview string    `json:"question_preview" gorm:"column:question_preview"` // 兼容旧前端
	// 科目名经 questions.subject_id 关联出来；题目缺失时为 NULL → 零值空串
	SubjectName string `json:"subject_name" gorm:"column:subject_name"`
}
```

> **`SubjectName` 后面是单空格，不是补齐到对齐列。** 上面那行独立注释切断了对齐组，gofmt 因此把它单独算一组——写成 `SubjectName     string` 是不合 gofmt 的。这一点容易想当然（"新字段名比现有最长字段短，对齐列不变"），第一版计划就是这么写错的，实测 `gofmt -d` 会报出来。写完请按下面的命令自查。

- [ ] **Step 4: 再跑测试，这回红在 SQL 上**

```bash
cd backend && go test ./internal/domain/repository/ -run TestQuestionReport -v
```

Expected: 三个用例全部 **FAIL**，报错是 sqlmock 的 `Query: could not match actual sql ... with expected regexp ...`。**这一步是关键的证据**——它证明红的确实是「SQL 里没有 JOIN subjects」，而不是别的什么东西。

- [ ] **Step 5: 在 `ListAll` 里加上 JOIN 与 Select**

替换 `backend/internal/domain/repository/report.go:40-61` 的整个 `ListAll`：

```go
func (r *QuestionReportRepositoryImpl) ListAll(page, pageSize, status int) ([]QuestionReportView, int64, error) {
	var list []QuestionReportView
	var total int64

	// 科目须经题目推导（question_reports 自己没有 subject_id）。必须 LEFT：
	// INNER 会把「题目已被删」的反馈整条隐藏，还会让下面 Count 出的 total 偏小。
	base := r.db.Table("question_reports qr").
		Select("qr.id, qr.question_id, qr.user_id, qr.report_type, qr.description, qr.status, qr.admin_note, qr.created_at, " +
			"u.username, q.title AS question_title, q.year AS question_year, q.question_type AS question_type, " +
			"SUBSTR(q.content, 1, 300) AS question_content, SUBSTR(q.content, 1, 100) AS question_preview, " +
			"s.name AS subject_name").
		Joins("LEFT JOIN users u ON u.id = qr.user_id").
		Joins("LEFT JOIN questions q ON q.id = qr.question_id").
		Joins("LEFT JOIN subjects s ON s.id = q.subject_id")

	if status >= 0 {
		base = base.Where("qr.status = ?", status)
	}

	base.Count(&total)
	err := base.Order("qr.created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&list).Error

	return list, total, err
}
```

- [ ] **Step 6: 在 `ListByUserID` 里加同样的两处**

替换 `backend/internal/domain/repository/report.go:63-81` 的整个 `ListByUserID`：

```go
func (r *QuestionReportRepositoryImpl) ListByUserID(userID uint64, page, pageSize int) ([]QuestionReportView, int64, error) {
	var list []QuestionReportView
	var total int64

	// 与 ListAll 的 Select/Join 逐字一致，保持同一个 View 的字段处处可用：
	// 只改一处会让 subject_name 变成时有时无的字段。
	base := r.db.Table("question_reports qr").
		Select("qr.id, qr.question_id, qr.user_id, qr.report_type, qr.description, qr.status, qr.admin_note, qr.created_at, "+
			"u.username, q.title AS question_title, q.year AS question_year, q.question_type AS question_type, "+
			"SUBSTR(q.content, 1, 300) AS question_content, SUBSTR(q.content, 1, 100) AS question_preview, "+
			"s.name AS subject_name").
		Joins("LEFT JOIN users u ON u.id = qr.user_id").
		Joins("LEFT JOIN questions q ON q.id = qr.question_id").
		Joins("LEFT JOIN subjects s ON s.id = q.subject_id").
		Where("qr.user_id = ?", userID)

	base.Count(&total)
	err := base.Order("qr.created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&list).Error

	return list, total, err
}
```

- [ ] **Step 7: 跑测试，确认三个用例全绿**

```bash
cd backend && go test ./internal/domain/repository/ -run TestQuestionReport -v
```

Expected: 三个用例全部 **PASS**。

> 本步的期望值是**实测**的，不是推测：写计划时已在本地按 Step 2→4→5→6→7 的次序跑过一遍——只加字段未加 JOIN 时三条全红于 `could not match actual sql`，加上 JOIN 后三条全绿，`go test ./...` 全绿。若你跑出来不一致，说明代码与计划有偏离，先比对再继续。

- [ ] **Step 8: 跑整个后端测试套件，确认没有连带弄坏别的**

```bash
cd backend && go build ./... && go test ./...
```

Expected: 全部 PASS。特别留意 `dashboard_test.go`（只数 `question_reports` 行数）与 `notification_test.go`（`fakeReportRepo` 是接口实现）——按设计它们应当**不受影响**；如果这里红了，说明判断错了，回来重新分析。

顺带跑一遍 gofmt 自查。**注意本机工作区是 CRLF（仓库 `core.autocrlf=true`，git 里存的 blob 是 LF），直接 `gofmt -l` 会把每个文件都判为需格式化——那是行尾假象。** 要先归一化行尾再问 gofmt：

```bash
cd backend && for f in internal/domain/repository/report.go internal/domain/repository/notification.go internal/domain/repository/report_test.go; do echo "== $f"; tr -d '\r' < "$f" | gofmt -d; done
```

Expected: 三处都无输出。若有输出，照着把它改掉——**CI 没有 gofmt 闸门，这个偏差不会有人替你拦下来**，会静默进入 main。

- [ ] **Step 9: 提交**

```bash
cd backend && git add internal/domain/repository/report.go internal/domain/repository/notification.go internal/domain/repository/report_test.go
cd .. && git commit -m "feat: 后台反馈列表 JOIN 出科目名

question_reports 没有 subject_id，科目经题目推导
（question_id → questions.subject_id → subjects.id）。

用 LEFT JOIN 不是 INNER：INNER 会让「题目已被删」的反馈整条
从列表消失，而且 Count 复用的是带 JOIN 的同一个 base，
total 会跟着偏小 —— 被隐藏的行数在分页器上看不出来。

ListAll 与 ListByUserID 的 Select 逐字相同，一起改，
避免 subject_name 变成时有时无的字段。

该查询原先零测试覆盖，顺带用 sqlmock 补上。"
```

---

### Task 2: 前端新增「科目」列

**Files:**
- Modify: `frontend/src/types/index.ts:169-173`（`QuestionReport`）
- Modify: `frontend/src/views/admin/QuestionReportManage.vue:141-142`（表格列）

**Interfaces:**
- Consumes: Task 1 产出的后端字段 `subject_name`（`string`，题目缺失时后端给空串）。
- Produces: 无。本任务是叶子节点。

> **本任务的闸门是类型检查，不是单元测试 —— 这是有意的。** 这个改动是一行模板表达式（`row.subject_name || '-'`），仓库里没有组件测试设施（`frontend/src` 下只有一个 `utils/wrongBook.test.ts`，CI 的 Frontend job 也只跑 `npm run build`）。为了一个 `|| '-'` 去搭一套 Element Plus 组件挂载环境不划算。`vue-tsc` 会挡住类型/模板写错，剩下的是肉眼看一眼。

- [ ] **Step 1: 给前端类型加上 `subject_name`**

修改 `frontend/src/types/index.ts` 的 `QuestionReport`，把管理端附加字段那组扩成：

```ts
  // 管理端列表附带题目识别字段（标题/年份/题型/题干摘要/科目），便于直接定位并编辑
  question_title?: string
  question_year?: number
  question_type?: number
  question_content?: string
  // 题目的科目名，由后端经题目推导（questions.subject_id → subjects.name）；
  // 题目已被删时后端给空串，渲染成「-」
  subject_name?: string
```

- [ ] **Step 2: 插入「科目」列**

在 `frontend/src/views/admin/QuestionReportManage.vue` 里，把新列插在 `题目` 列（:128-141）之后、`题干摘要` 列（:142）之前。改动后的样子：

```vue
      </el-table-column>
      <el-table-column label="科目" width="120">
        <template #default="{ row }: { row: any }">
          <span class="text-sm">{{ row.subject_name || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="题干摘要" min-width="280">
```

注意 `</el-table-column>` 与 `<el-table-column label="题干摘要"` 之间**只**插这一个块，`题干摘要` 那一列本身不动。用固定 `width="120"`（科目是短值，与同表「问题类型 130」「状态 90」「提交用户 100」同类），不要用 `min-width`。

- [ ] **Step 3: 跑类型检查**

```bash
cd frontend && npx vue-tsc -b --force
```

Expected: 无输出、退出码 0。

- [ ] **Step 4: 跑一次与 CI 等价的完整构建**

CI 的 Frontend job 跑的就是这条命令，用同一序列验证可以提前暴露平台差异：

```bash
cd frontend && npm run build
```

Expected: 成功。若报 `VITE_API_BASE_URL` 缺失/为空，用 CI 的占位地址再跑一次：

```bash
cd frontend && VITE_API_BASE_URL=http://api.invalid/api npm run build
```

- [ ] **Step 5: 肉眼确认**

```bash
cd frontend && npm run dev
```

打开 `/admin/reports`（需管理员账号），确认：新列在「题目」与「题干摘要」之间；标题是「科目」；有科目的行显示科目名（如「系统架构设计师」）；**没有科目的行显示 `-` 而不是空白或 `undefined`**。这一步没有自动化替代——列的位置和空值表现只有看才知道。

- [ ] **Step 6: 提交**

```bash
cd frontend && git add src/types/index.ts src/views/admin/QuestionReportManage.vue
cd .. && git commit -m "feat: 后台反馈列表显示科目列

列插在「题目」与「题干摘要」之间，固定宽 120（短值列）。
值为空时显示「-」：题目被删时后端给的是空串。"
```

---

### Task 3: 推送并确认 CI 绿

**Files:** 无（本任务只验证，不改代码）。

**Interfaces:**
- Consumes: Task 1 与 Task 2 的提交。
- Produces: `main` 与 `origin/main` 同步，两个 CI job 绿。

> 这个仓库的 CI 接连红过四次，所以「跑绿」是完成度的一部分，不是走过场。**本机没有官方 `gh` CLI，未登录状态下拿不到步骤原始日志**，但注解区是服务端渲染的、可以免登录读；最终判据以分支徽章为准（job 页里那个 `octicon-x-circle-fill` 是页面骨架，会误判成红）。

- [ ] **Step 1: 确认工作区干净，且两个任务的提交都在**

```bash
git status --short && git log --oneline -4
```

Expected: `git status` 无输出（干净）。日志里能看到 Task 1 与 Task 2 的两个 `feat:` 提交，以及更早的 `docs:` 提交（设计稿 + 本实施计划）。

- [ ] **Step 2: 推送**

```bash
git push origin main
```

- [ ] **Step 3: 等 CI 跑完，并读最终判据（分支徽章）**

CI 大约要几分钟。**不要用前台 `sleep` 轮询**——Claude Code 的 Bash 工具会拒绝前台 sleep。用 Monitor 的 until-loop（或等一会儿再手动重跑本步）。

判据只用这一条，它不需要 run id：

```bash
curl -s "https://github.com/jycoast/ruankao/actions/workflows/ci.yml/badge.svg?branch=main" | grep -o '<title>[^<]*</title>'
```

Expected: `<title>CI - passing</title>`。

脚本还在跑时会看到 `<title>CI - (pending|running)</title>` 之类的中间态，那不代表失败；重跑本步直到出现 `passing` 或 `failing`。

**若看到 `failing`，不要只凭 run 页上的 `octicon-x-circle-fill` 判定哪个 job 红**——那个图标是页面骨架，会误判。走 Step 4 读注解，那里才有真正的错因。

- [ ] **Step 4: 若 CI 红（或想确认注解区干净），读注解**

未登录拿不到步骤原始日志，但注解区是服务端渲染的、可以免登录读。这条命令自己解析出最新 run id，不需要手工填任何东西：

```bash
RUN=$(curl -s https://github.com/jycoast/ruankao/actions | grep -o 'actions/runs/[0-9]*' | head -1 | grep -o '[0-9]*') && echo "run=$RUN" && curl -s "https://github.com/jycoast/ruankao/actions/runs/$RUN" | grep -o '<annotation-message>.\{0,400\}'
```

Expected: 无输出。上一个绿 run 的注解区就是空的（Node 20 弃用与 Ubuntu 26 迁移两条提示都已随 action 升级消失），所以「没有输出」是正常结果，不是命令没跑对——`run=` 那行能证明它确实取到了 run。

若出现含 `could not match actual sql` 或 `migrate 退出码` 字样的注解：说明后端 SQL 与测试期望的正则对不上。本机可直接复现，不必靠猜：

```bash
cd backend && go test ./internal/domain/repository/ -run TestQuestionReport -v
```

按 Task 1 Step 2-4 的两层红流程重新对齐实际 SQL。

- [ ] **Step 5: 确认本地与远端一致**

```bash
git status -sb && git log --oneline origin/main -1
```

Expected: `## main...origin/main`（无 ahead/behind），远端最新提交就是 Task 2 的那个。
