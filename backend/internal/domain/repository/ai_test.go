package repository

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// newMockDB 用 sqlmock 构造只连假驱动的 GORM DB。
// 与 repository_test.go 一致走 mysql dialect，保证生成的 SQL 与生产同源。
func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      conn,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return db, mock
}

// 别人的会话 ID：一条消息都不许删。
// 原实现先无条件按 conversation_id 删消息，任何登录用户拿别人的会话 ID
// 就能清空对方聊天记录。这里断言只有会话行的带权属删除被执行。
func TestDeleteConversation_DoesNotTouchOtherUsersMessages(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewAIConversationRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `ai_conversations`").
		WithArgs(uint64(9), uint64(3)).
		WillReturnResult(sqlmock.NewResult(0, 0)) // 不属于 user 3
	mock.ExpectCommit()
	// 若实现仍去删 ai_messages，sqlmock 会因「未预期的 SQL」令 Delete 返回错误。

	if err := repo.Delete(9, 3); err != nil {
		t.Fatalf("非本人删除应静默成功，得到 %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// ---- 免费试用额度 ----

// 未达上限：条件 UPDATE 命中一行即占用成功。
func TestTryIncrement_BumpsWhenUnderLimit(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewAITrialUsageRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `ai_trial_usages` SET `used_count`=").
		WithArgs(uint64(7), 3).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	ok, err := repo.TryIncrement(7, 3)
	if err != nil || !ok {
		t.Fatalf("未达上限应占用成功，得到 ok=%v err=%v", ok, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// 额度已满：UPDATE 命中 0 行，COUNT 确认行存在后拒绝，不能插行。
func TestTryIncrement_RejectsAtLimit(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewAITrialUsageRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `ai_trial_usages` SET `used_count`=").
		WithArgs(uint64(7), 3).
		WillReturnResult(sqlmock.NewResult(0, 0)) // 条件不满足
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT count").
		WithArgs(uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1)) // 行存在 = 额度已满

	ok, err := repo.TryIncrement(7, 3)
	if err != nil || ok {
		t.Fatalf("额度已满应拒绝，得到 ok=%v err=%v", ok, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// 首次使用：行不存在时插入 UsedCount=1，直接占掉第一次。
func TestTryIncrement_FirstUseCreatesRow(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewAITrialUsageRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `ai_trial_usages` SET `used_count`=").
		WithArgs(uint64(7), 3).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT count").
		WithArgs(uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0)) // 首次使用
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `ai_trial_usages`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	ok, err := repo.TryIncrement(7, 3)
	if err != nil || !ok {
		t.Fatalf("首次使用应占用成功，得到 ok=%v err=%v", ok, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// 两个请求同时首次使用：一个 INSERT 成功，另一个撞唯一索引后重试条件自增。
func TestTryIncrement_RetriesAfterConcurrentInsert(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewAITrialUsageRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `ai_trial_usages` SET `used_count`=").
		WithArgs(uint64(7), 3).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT count").
		WithArgs(uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `ai_trial_usages`").
		WillReturnError(errors.New("Duplicate entry"))
	mock.ExpectRollback()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `ai_trial_usages` SET `used_count`=").
		WithArgs(uint64(7), 3).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	ok, err := repo.TryIncrement(7, 3)
	if err != nil || !ok {
		t.Fatalf("并发插入后应重试成功，得到 ok=%v err=%v", ok, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// 归还额度不能把 used_count 减成负数。
func TestDecrement_OnlyWhenPositive(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewAITrialUsageRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `ai_trial_usages` SET `used_count`=").
		WithArgs(uint64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.Decrement(7); err != nil {
		t.Fatalf("Decrement: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// 本人会话：会话行删掉后才清消息。
func TestDeleteConversation_CleansMessagesOnlyWhenOwned(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewAIConversationRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `ai_conversations`").
		WithArgs(uint64(9), uint64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `ai_messages`").
		WithArgs(uint64(9)).
		WillReturnResult(sqlmock.NewResult(0, 4))
	mock.ExpectCommit()

	if err := repo.Delete(9, 3); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}
