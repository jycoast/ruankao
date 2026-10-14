package repository

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newFavoriteRepoWithSQLMock(t *testing.T) (FavoriteRepository, sqlmock.Sqlmock) {
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
	return NewFavoriteRepository(gormDB), mock
}

// FindByUserID 分页：COUNT + LIMIT/OFFSET，排序 created_at DESC + id DESC tie-breaker。
// 按 target_type 过滤参数必须落到 WHERE。
func TestFavoriteFindByUserID_Paginated(t *testing.T) {
	repo, mock := newFavoriteRepoWithSQLMock(t)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `favorites` WHERE user_id = [?] AND target_type = [?]").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(int64(12)))
	mock.ExpectQuery("FROM `favorites` WHERE user_id = [?] AND target_type = [?] ORDER BY created_at DESC, id DESC LIMIT [?] OFFSET [?]").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "target_type", "target_id"}).
			AddRow(9, 42, 2, 301).AddRow(8, 42, 2, 300))

	favs, total, err := repo.FindByUserID(42, 2, 2, 10)
	if err != nil {
		t.Fatalf("FindByUserID 应无错返回，得到 %v", err)
	}
	if total != 12 {
		t.Fatalf("期望 total=12，实际 %d", total)
	}
	if len(favs) != 2 || favs[0].TargetID != 301 || favs[1].TargetID != 300 {
		t.Fatalf("分页行内容错: %+v", favs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// Count 失败必须冒泡成错误，而不是静默返回 total=0。
func TestFavoriteFindByUserID_PropagatesCountError(t *testing.T) {
	repo, mock := newFavoriteRepoWithSQLMock(t)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `favorites` WHERE user_id = [?] AND target_type = [?]").
		WillReturnError(errors.New("count failed"))

	_, _, err := repo.FindByUserID(42, 2, 1, 10)
	// 必须是 count 自己的错误冒泡出来——不能靠"Find 意外执行撞上 sqlmock"这种副作用返回错误
	if err == nil || err.Error() != "count failed" {
		t.Fatalf("Count 失败应原样返回其错误，得到 %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}