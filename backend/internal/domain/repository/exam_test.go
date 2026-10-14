package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newExamRepoWithSQLMock(t *testing.T) (ExamRecordRepository, sqlmock.Sqlmock) {
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
	return NewExamRecordRepository(gormDB), mock
}

// FindByUserID 分页：COUNT + LIMIT/OFFSET，排序 created_at DESC + id DESC tie-breaker。
func TestExamRecordFindByUserID_Paginated(t *testing.T) {
	repo, mock := newExamRepoWithSQLMock(t)
	started := time.Now()

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `exam_records` WHERE user_id = [?]").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(int64(5)))
	// page=1 → offset=0，GORM 省略 OFFSET 子句，只有 LIMIT
	mock.ExpectQuery("FROM `exam_records` WHERE user_id = [?] ORDER BY created_at DESC, id DESC LIMIT [?]").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "paper_id", "score", "status", "started_at"}).
			AddRow(7, 42, 100, 88.5, 1, started).AddRow(6, 42, 99, 66, 1, started))

	records, total, err := repo.FindByUserID(42, 1, 10)
	if err != nil {
		t.Fatalf("FindByUserID 应无错返回，得到 %v", err)
	}
	if total != 5 {
		t.Fatalf("期望 total=5，实际 %d", total)
	}
	if len(records) != 2 || records[0].ID != 7 || records[1].ID != 6 {
		t.Fatalf("分页行内容错: %+v", records)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// Count 失败必须冒泡成错误，而不是静默返回 total=0。
func TestExamRecordFindByUserID_PropagatesCountError(t *testing.T) {
	repo, mock := newExamRepoWithSQLMock(t)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `exam_records` WHERE user_id = [?]").
		WillReturnError(errors.New("count failed"))

	_, _, err := repo.FindByUserID(42, 1, 10)
	// 必须是 count 自己的错误冒泡出来——不能靠"Find 意外执行撞上 sqlmock"这种副作用返回错误
	if err == nil || err.Error() != "count failed" {
		t.Fatalf("Count 失败应原样返回其错误，得到 %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}