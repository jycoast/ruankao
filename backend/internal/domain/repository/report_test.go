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
