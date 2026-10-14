package repository

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newQuestionRepoWithSQLMock(t *testing.T) (QuestionRepository, sqlmock.Sqlmock) {
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
	return NewQuestionRepository(gormDB), mock
}

// GetCategories(subjectID>0)：subject 过滤必须参数化（WHERE subject_id = ?，
// 值随参数走而非拼进 SQL），结果直接 Scan 成 Category（不再二次转 map）。
// sqlmock 默认 regexp matcher 非锚定，`?` 用 `\?` 转义成字面量断言。
func TestQuestionGetCategories_ParameterizedSubjectFilter(t *testing.T) {
	repo, mock := newQuestionRepoWithSQLMock(t)

	mock.ExpectQuery("status = 1 AND subject_id = \\? GROUP BY").
		WillReturnRows(sqlmock.NewRows([]string{"main_cat", "sub_cat", "question_type", "cnt"}).
			AddRow("综合知识", "综合知识>算法", 1, 10).
			AddRow("案例分析", "案例分析>系统设计", 3, 5))

	cats, err := repo.GetCategories(3)
	if err != nil {
		t.Fatalf("GetCategories 应无错返回，得到 %v", err)
	}
	if len(cats) != 2 {
		t.Fatalf("期望 2 行，实际 %d", len(cats))
	}
	if cats[0].MainCat != "综合知识" || cats[0].SubCat != "综合知识>算法" || cats[0].Count != 10 {
		t.Fatalf("首行字段错: %+v", cats[0])
	}
	if cats[1].QuestionType != 3 || cats[1].Count != 5 {
		t.Fatalf("次行字段错: %+v", cats[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// GetCategories(subjectID=0)：不加 subject 过滤，也不带过滤参数。
func TestQuestionGetCategories_NoSubjectFilter(t *testing.T) {
	repo, mock := newQuestionRepoWithSQLMock(t)

	mock.ExpectQuery("GROUP BY main_cat, sub_cat, question_type ORDER BY main_cat").
		WillReturnRows(sqlmock.NewRows([]string{"main_cat", "sub_cat", "question_type", "cnt"}).
			AddRow("综合知识", "综合知识>基础", 1, 3))

	cats, err := repo.GetCategories(0)
	if err != nil {
		t.Fatalf("GetCategories 应无错返回，得到 %v", err)
	}
	if len(cats) != 1 || cats[0].Count != 3 || cats[0].QuestionType != 1 {
		t.Fatalf("单行内容错: %+v", cats)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// GetCategories 零行时必须返回非 nil 空 slice（JSON `[]`），不能是 nil（JSON `null`）。
// 该接口原实现先扫进临时行再构造 map slice，空库恒为 `[]`；P5b 改成 Raw().Scan 直扫
// 类型化 slice 后，若直接扫进声明的 nil slice，零行时 GORM 不做分配，序列化会变 `null`，
// 前端 `for (const cat of res.data)` 直接崩。前置初始化成空 slice 保证契约不变。
func TestQuestionGetCategories_ZeroRowsReturnsEmptySlice(t *testing.T) {
	repo, mock := newQuestionRepoWithSQLMock(t)

	mock.ExpectQuery("status = 1 GROUP BY main_cat").
		WillReturnRows(sqlmock.NewRows([]string{"main_cat", "sub_cat", "question_type", "cnt"}))

	cats, err := repo.GetCategories(0)
	if err != nil {
		t.Fatalf("GetCategories 应无错返回，得到 %v", err)
	}
	if cats == nil {
		t.Fatalf("零行返回 nil slice（JSON 序列化为 null），期望非 nil 空 slice")
	}
	if len(cats) != 0 {
		t.Fatalf("期望空结果，实际 %d 行", len(cats))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// 案例(3)/论文(4) 按设计没有选项，GetRandomQuestions 不能再要求 EXISTS options，
// 否则分类刷题和模拟卷的案例/论文整类抽 0 题（「该知识点暂无题目」）。
// 选项过滤只对选择题(1/2)保留。
// 注意：sqlmock 默认 regexp matcher 非锚定，用「条件后直接跟 ORDER BY」的紧致正则
// ——一旦字段间插入 `AND EXISTS (…)`，正则失配、Expect 不满足，从而断言"无 EXISTS"。
func TestQuestionGetRandomQuestions_CaseAndEssaySkipOptionsFilter(t *testing.T) {
	repo, mock := newQuestionRepoWithSQLMock(t)

	// qType=3/4：单条件查询不带外层括号，主查询必须「status = 1 ORDER BY RAND()」，
	// 中间不得插入 EXISTS（若插入会被 GORM 包成 `(…) AND EXISTS (…)`，此处正则失配）
	caseRegex := "FROM `questions` WHERE question_type = \\? AND status = 1 ORDER BY RAND\\(\\) LIMIT \\?"
	// 选择题(1)：双条件被 GORM 包括号，必须带 EXISTS options 过滤
	choiceRegex := "FROM `questions` WHERE \\(question_type = \\? AND status = 1\\) AND EXISTS \\(SELECT 1 FROM options o WHERE o.question_id = questions.id\\) ORDER BY RAND\\(\\) LIMIT \\?"

	for _, qType := range []int8{3, 4, 1} {
		expected := caseRegex
		if qType == 1 {
			expected = choiceRegex
		}
		mock.ExpectQuery(expected).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(qType)))
		// 选项行的 question_id 必须对应题干的 id，否则 GORM Preload 关联不回来报 "failed to assign association"
		mock.ExpectQuery("FROM `options` WHERE `options`.`question_id` (IN \\(\\?\\)|= \\?) ORDER BY id ASC").
			WillReturnRows(sqlmock.NewRows([]string{"id", "question_id"}).AddRow(2, int64(qType)))
	}

	for _, qType := range []int8{3, 4, 1} {
		qs, err := repo.GetRandomQuestions(qType, 1, "", 0)
		if err != nil {
			t.Fatalf("qType=%d 应无错返回: %v", qType, err)
		}
		if len(qs) != 1 || qs[0].ID != uint64(qType) {
			t.Fatalf("qType=%d 行内容错: %+v", qType, qs)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足（案例/论文仍带 EXISTS options 或顺序不符）: %v", err)
	}
}

// GetRandomByTypeYear（模拟卷生成）同样不能对案例/论文套 EXISTS options。
func TestQuestionGetRandomByTypeYear_CaseSkipsOptionsFilter(t *testing.T) {
	repo, mock := newQuestionRepoWithSQLMock(t)

	// 紧致正则：status = 1 后直接 ORDER BY，不得夹带 EXISTS
	mock.ExpectQuery("FROM `questions` WHERE question_type = \\? AND status = 1 ORDER BY RAND\\(\\) LIMIT \\?").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectQuery("FROM `options` WHERE `options`.`question_id` (IN \\(\\?\\)|= \\?) ORDER BY id ASC").
		WillReturnRows(sqlmock.NewRows([]string{"id", "question_id"}).AddRow(2, 3))

	qs, err := repo.GetRandomByTypeYear(3, 1, 0, 0)
	if err != nil {
		t.Fatalf("应无错返回: %v", err)
	}
	if len(qs) != 1 || qs[0].ID != 3 {
		t.Fatalf("行内容错: %+v", qs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足（案例仍带 EXISTS options）: %v", err)
	}
}

// GetSeededRandomQuestions（每日一练）的过滤条件与 GetRandomQuestions 保持一致：
// 案例/论文不过滤（当前仅选择题在用，锁住契约防未来误用）。
func TestQuestionGetSeededRandomQuestions_CaseSkipsOptionsFilter(t *testing.T) {
	repo, mock := newQuestionRepoWithSQLMock(t)

	// 紧致正则：status = 1 后直接 ORDER BY RAND(?)，不得夹带 EXISTS
	mock.ExpectQuery("FROM `questions` WHERE question_type = \\? AND status = 1 ORDER BY RAND\\(\\?\\) LIMIT \\?").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectQuery("FROM `options` WHERE `options`.`question_id` (IN \\(\\?\\)|= \\?) ORDER BY id ASC").
		WillReturnRows(sqlmock.NewRows([]string{"id", "question_id"}).AddRow(2, 3))

	qs, err := repo.GetSeededRandomQuestions(3, 1, "", 0, 42)
	if err != nil {
		t.Fatalf("应无错返回: %v", err)
	}
	if len(qs) != 1 || qs[0].ID != 3 {
		t.Fatalf("行内容错: %+v", qs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足（案例仍带 EXISTS options）: %v", err)
	}
}

// CountQuestions 的过滤条件必须与 GetRandomQuestions 的候选集完全一致：
// 选择题带 EXISTS options（无选项的题不算数），案例/论文不带。否则「继续刷题」
// 判定草稿是否过期时（草稿题数 vs 全量题数）要么放过残缺草稿、要么误伤正常草稿。
func TestQuestionCountQuestions_MirrorsDrawFilters(t *testing.T) {
	repo, mock := newQuestionRepoWithSQLMock(t)
	// 注意：sqlmock 的 Rows 单向一次性，两处期望各建各的，不能共用一个
	mock.ExpectQuery("FROM `questions` WHERE \\(question_type = \\? AND status = 1\\) AND EXISTS \\(SELECT 1 FROM options o WHERE o.question_id = questions.id\\)").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(37))
	mock.ExpectQuery("FROM `questions` WHERE question_type = \\? AND status = 1").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(37))

	n1, err := repo.CountQuestions(1, "", 0)
	if err != nil {
		t.Fatalf("选择题计数应无错: %v", err)
	}
	if n1 != 37 {
		t.Fatalf("选择题计数 = %d, 期望 37", n1)
	}
	n3, err := repo.CountQuestions(3, "", 0)
	if err != nil {
		t.Fatalf("案例计数应无错: %v", err)
	}
	if n3 != 37 {
		t.Fatalf("案例计数 = %d, 期望 37", n3)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// GetSeededRandomQuestions：排序键 RAND(seed) 必须参数化（RAND(?)），
// 种子值不再拼进 SQL 字符串。查询本身携带 Options Preload，需把后续关联查询一并预期。
func TestQuestionGetSeededRandomQuestions_RANDPlaceholder(t *testing.T) {
	repo, mock := newQuestionRepoWithSQLMock(t)

	mock.ExpectQuery("FROM `questions` WHERE \\(question_type = \\? AND status = 1\\).*ORDER BY RAND\\(\\?\\) LIMIT \\?").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery("FROM `options` WHERE `options`.`question_id` (IN \\(\\?\\)|= \\?) ORDER BY id ASC").
		WillReturnRows(sqlmock.NewRows([]string{"id", "question_id"}).AddRow(2, 1))

	qs, err := repo.GetSeededRandomQuestions(1, 10, "", 0, 42)
	if err != nil {
		t.Fatalf("应无错返回，得到 %v", err)
	}
	if len(qs) != 1 || qs[0].ID != 1 {
		t.Fatalf("行内容错: %+v", qs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}