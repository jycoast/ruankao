package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// newWrongQuestionRepoWithSQLMock 构造 WrongQuestionRepository，后端走 sqlmock。
// 与其它 repo 测试的 helper 解耦，本文件独立维护。
func newWrongQuestionRepoWithSQLMock(t *testing.T) (WrongQuestionRepository, sqlmock.Sqlmock) {
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
	return NewWrongQuestionRepository(gormDB), mock
}

// FindByUserID 分页：先 COUNT 再 LIMIT/OFFSET，且排序带 id DESC tie-breaker
// （created_at 相同的数据翻页才不会重复/丢行）。
// status>0 时 WHERE 必须带 status 过滤。
func TestWrongQuestionFindByUserID_PaginatesWithStatusFilter(t *testing.T) {
	repo, mock := newWrongQuestionRepoWithSQLMock(t)

	// 1. COUNT：过滤条件与 Find 一致，pg 前执行
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `wrong_questions` WHERE user_id = [?] AND status = [?]").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(int64(8)))
	// 2. 主查询：翻页 + 稳定排序
	mock.ExpectQuery("FROM `wrong_questions` WHERE user_id = [?] AND status = [?] ORDER BY created_at DESC, id DESC LIMIT [?] OFFSET [?]").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "question_id", "status"}).
			AddRow(1, 42, 100, 1).AddRow(2, 42, 101, 1))
	// 3. Preload Question
	mock.ExpectQuery("FROM `questions` WHERE `questions`.`id` IN ").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title"}).AddRow(100, "题A").AddRow(101, "题B"))
	// 4. Preload Question.Options（外层 Order id ASC，GORM 大写）
	mock.ExpectQuery("FROM `options` WHERE `options`.`question_id` IN .* ORDER BY id ASC").
		WillReturnRows(sqlmock.NewRows([]string{"id", "question_id"}).AddRow(1, 100).AddRow(2, 101))

	wqs, total, err := repo.FindByUserID(42, 1, 2, 5)
	if err != nil {
		t.Fatalf("FindByUserID 应无错返回，得到 %v", err)
	}
	if total != 8 {
		t.Fatalf("期望 total=8，实际 %d", total)
	}
	if len(wqs) != 2 {
		t.Fatalf("期望 2 行，实际 %d", len(wqs))
	}
	if wqs[0].Question == nil || wqs[0].Question.Title != "题A" {
		t.Fatalf("Preload Question 缺失或错误: %+v", wqs[0].Question)
	}
	if len(wqs[0].Question.Options) != 1 || wqs[0].Question.Options[0].QuestionID != 100 {
		t.Fatalf("Preload Options 缺失或错误: %+v", wqs[0].Question.Options)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// status=0（全部错题）时不允许带 status 过滤。
func TestWrongQuestionFindByUserID_NoStatusFilter(t *testing.T) {
	repo, mock := newWrongQuestionRepoWithSQLMock(t)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `wrong_questions` WHERE user_id = [?]").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(int64(0)))
	mock.ExpectQuery("FROM `wrong_questions` WHERE user_id = [?] ORDER BY created_at DESC, id DESC LIMIT [?]").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	// 空结果 → 无 Preload 查询

	wqs, total, err := repo.FindByUserID(42, 0, 1, 20)
	if err != nil {
		t.Fatalf("FindByUserID 应无错返回，得到 %v", err)
	}
	if total != 0 || len(wqs) != 0 {
		t.Fatalf("期望空结果，实际 total=%d len=%d", total, len(wqs))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// Count 失败必须冒泡成错误，而不是静默返回 total=0 —— 否则前端拿空列表
// 会误以为「没有错题」。
func TestWrongQuestionFindByUserID_PropagatesCountError(t *testing.T) {
	repo, mock := newWrongQuestionRepoWithSQLMock(t)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `wrong_questions` WHERE user_id = [?] AND status = [?]").
		WillReturnError(errors.New("count failed"))

	_, _, err := repo.FindByUserID(42, 1, 1, 20)
	// 必须是 count 自己的错误冒泡出来——不能靠"Find 意外执行撞上 sqlmock"这种副作用返回错误
	if err == nil || err.Error() != "count failed" {
		t.Fatalf("Count 失败应原样返回其错误，得到 %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// FindDueReview：WHERE 带到期条件，排序必须是 next_review_time ASC, id ASC
// （原实现无排序，翻页会乱序/重复）。page=1 时不带 OFFSET。
func TestWrongQuestionFindDueReview_PaginatedWithStableOrder(t *testing.T) {
	repo, mock := newWrongQuestionRepoWithSQLMock(t)
	due := time.Now()

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `wrong_questions` WHERE user_id = [?] AND status = 1 AND next_review_time <= NOW").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(int64(3)))
	mock.ExpectQuery("FROM `wrong_questions` WHERE user_id = [?] AND status = 1 AND next_review_time <= NOW\\(\\) ORDER BY next_review_time ASC, id ASC LIMIT [?]").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "question_id", "next_review_time"}).
			AddRow(1, 42, 100, due))
	// 单个 id 时 GORM 用 `id = ?` 而不是 IN (...) —— 正则需兼容两种形态
	mock.ExpectQuery("FROM `questions` WHERE `questions`.`id` (IN|=) ").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title"}).AddRow(100, "到期题"))
	mock.ExpectQuery("FROM `options` WHERE `options`.`question_id` (IN|=) ").
		WillReturnRows(sqlmock.NewRows([]string{"id", "question_id"}))

	wqs, total, err := repo.FindDueReview(42, 1, 10)
	if err != nil {
		t.Fatalf("FindDueReview 应无错返回，得到 %v", err)
	}
	if total != 3 || len(wqs) != 1 {
		t.Fatalf("期望 total=3 / 1 行，实际 total=%d len=%d", total, len(wqs))
	}
	if wqs[0].Question == nil || wqs[0].Question.Title != "到期题" {
		t.Fatalf("Preload Question 缺失: %+v", wqs[0].Question)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}