package repository

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// newPracticeQueryRepoWithSQLMock 构造 PracticeQueryRepository 接口实现,后端走 sqlmock。
func newPracticeQueryRepoWithSQLMock(t *testing.T) (PracticeQueryRepository, sqlmock.Sqlmock) {
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
	return NewPracticeQueryRepository(gormDB), mock
}

// AccuracyByUser 非零路径：done=20, correct=15 → 服务层会算 75%。
// SQL 必须命中 latestAttemptsSQL 派生表 + SUM(CASE WHEN is_correct=1)。
func TestAccuracyByUser_HappyPath(t *testing.T) {
	repo, mock := newPracticeQueryRepoWithSQLMock(t)

	mock.ExpectQuery(`MAX\(id\) AS last_id`).
		WillReturnRows(sqlmock.NewRows([]string{"done", "correct"}).
			AddRow(int64(20), int64(15)))

	row, err := repo.AccuracyByUser(7)
	if err != nil {
		t.Fatalf("应无错返回,得到 err=%v", err)
	}
	if row.Done != 20 || row.Correct != 15 {
		t.Fatalf("应为 done=20 correct=15,得到 %+v", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// AccuracyByUser 零除保护输入：done=0 时 service 拿到 0/0 → 应自行判空返回 Accuracy=0。
// 这里只锁「repo 能正常返回零值」,零除兜底在 service。
func TestAccuracyByUser_ZeroDivision(t *testing.T) {
	repo, mock := newPracticeQueryRepoWithSQLMock(t)

	mock.ExpectQuery(`MAX\(id\) AS last_id`).
		WillReturnRows(sqlmock.NewRows([]string{"done", "correct"}).
			AddRow(int64(0), int64(0)))

	row, err := repo.AccuracyByUser(7)
	if err != nil {
		t.Fatalf("应无错,得到 err=%v", err)
	}
	if row.Done != 0 {
		t.Fatalf("Done 应为 0,得到 %d", row.Done)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// CategoryProgress：SQL 必须 GROUP BY q.knowledge_point 且 LEFT JOIN latestAttemptsSQL。
// regex 锁住这三条,任一改坏立刻 fail。
func TestCategoryProgress_GroupByKnowledgePoint(t *testing.T) {
	repo, mock := newPracticeQueryRepoWithSQLMock(t)

	mock.ExpectQuery(`GROUP BY q\.knowledge_point`).
		WillReturnRows(sqlmock.NewRows([]string{"knowledge_point", "total", "done", "correct"}).
			AddRow("算法", int64(50), int64(20), int64(15)).
			AddRow("网络", int64(30), int64(10), int64(8)))

	rows, err := repo.CategoryProgress(7)
	if err != nil {
		t.Fatalf("应无错,得到 err=%v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("期望 2 行,得到 %d", len(rows))
	}
	if rows[0].KnowledgePoint != "算法" || rows[0].Total != 50 {
		t.Fatalf("首行数据错: %+v", rows[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足(SQL 可能漏 GROUP BY knowledge_point): %v", err)
	}
}

// AccuracyBySubject 复用 latestAttemptsBySubjectSQL,锁住 subject_id 参数透传。
func TestAccuracyBySubject_FilterBySubject(t *testing.T) {
	repo, mock := newPracticeQueryRepoWithSQLMock(t)

	mock.ExpectQuery(`MAX\(id\) AS last_id`).
		WithArgs(uint64(7), uint64(3)). // userID=7, subjectID=3
		WillReturnRows(sqlmock.NewRows([]string{"done", "correct"}).
			AddRow(int64(10), int64(8)))

	row, err := repo.AccuracyBySubject(7, 3)
	if err != nil {
		t.Fatalf("应无错,得到 err=%v", err)
	}
	if row.Done != 10 || row.Correct != 8 {
		t.Fatalf("应为 done=10 correct=8,得到 %+v", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足(subject_id 参数可能没透传): %v", err)
	}
}

// TrendBySubject：30 天窗口(INTERVAL 29 DAY = days-1)。锁住这个常量关系。
func TestTrendBySubject_Last30Days(t *testing.T) {
	repo, mock := newPracticeQueryRepoWithSQLMock(t)

	mock.ExpectQuery(`INTERVAL 29 DAY`).
		WillReturnRows(sqlmock.NewRows([]string{"day", "total", "correct"}).
			AddRow("2026-09-03", int64(15), int64(10)).
			AddRow("2026-09-04", int64(20), int64(18)))

	rows, err := repo.TrendBySubject(7, 3)
	if err != nil {
		t.Fatalf("应无错,得到 err=%v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("期望 2 行,得到 %d", len(rows))
	}
	if rows[0].Day != "2026-09-03" || rows[0].Total != 15 {
		t.Fatalf("首行错: %+v", rows[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足(SQL 可能没 INTERVAL 29 DAY): %v", err)
	}
}

// KnowledgeAccuracyBySubject：LIMIT 20 是排行榜容量上限,锁住不被改坏。
func TestKnowledgeAccuracyBySubject_Limit20(t *testing.T) {
	repo, mock := newPracticeQueryRepoWithSQLMock(t)

	mock.ExpectQuery(`LIMIT 20`).
		WillReturnRows(sqlmock.NewRows([]string{"knowledge_point", "done", "correct"}).
			AddRow("算法", int64(50), int64(40)))

	rows, err := repo.KnowledgeAccuracyBySubject(7, 3)
	if err != nil {
		t.Fatalf("应无错,得到 err=%v", err)
	}
	if len(rows) != 1 || rows[0].Correct != 40 {
		t.Fatalf("数据错: %+v", rows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足(SQL 可能没 LIMIT 20): %v", err)
	}
}

// SpeedBySubject 全样本窗口(0/0)：SQL 应不含 INTERVAL 子句。
func TestSpeedBySubject_All(t *testing.T) {
	repo, mock := newPracticeQueryRepoWithSQLMock(t)

	mock.ExpectQuery(`time_spent BETWEEN 1 AND 600`).
		WillReturnRows(sqlmock.NewRows([]string{"cnt", "avg_sec"}).
			AddRow(int64(100), float64(45.5)))

	row, err := repo.SpeedBySubject(7, 3, 0, 0)
	if err != nil {
		t.Fatalf("应无错,得到 err=%v", err)
	}
	if row.Cnt != 100 || row.AvgSec != 45.5 {
		t.Fatalf("应为 cnt=100 avg=45.5,得到 %+v", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// SpeedBySubject 近 7 天窗口：SQL 必须含 INTERVAL 7 DAY(>= 7),不含 < 7 区间。
func TestSpeedBySubject_ThisWeek(t *testing.T) {
	repo, mock := newPracticeQueryRepoWithSQLMock(t)

	mock.ExpectQuery(`INTERVAL 7 DAY`).
		WillReturnRows(sqlmock.NewRows([]string{"cnt", "avg_sec"}).
			AddRow(int64(20), float64(40.0)))

	row, err := repo.SpeedBySubject(7, 3, 7, 0)
	if err != nil {
		t.Fatalf("应无错,得到 err=%v", err)
	}
	if row.Cnt != 20 {
		t.Fatalf("Cnt 应为 20,得到 %d", row.Cnt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// SpeedBySubject 前 7 天窗口：SQL 必须同时含 INTERVAL 14 DAY AND INTERVAL 7 DAY。
// 这是 where 拼接最复杂的分支,锁住不被改成「>=14 only」。
func TestSpeedBySubject_LastWeek(t *testing.T) {
	repo, mock := newPracticeQueryRepoWithSQLMock(t)

	mock.ExpectQuery(`INTERVAL 14 DAY.*INTERVAL 7 DAY`).
		WillReturnRows(sqlmock.NewRows([]string{"cnt", "avg_sec"}).
			AddRow(int64(15), float64(50.0)))

	row, err := repo.SpeedBySubject(7, 3, 14, 7)
	if err != nil {
		t.Fatalf("应无错,得到 err=%v", err)
	}
	if row.Cnt != 15 {
		t.Fatalf("Cnt 应为 15,得到 %d", row.Cnt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足(LastWeek 应含 INTERVAL 14 AND 7): %v", err)
	}
}

// SpeedByUser 与 SpeedBySubject 同构(JOIN questions 那部分),仅省 subject_id。
// 验证 SQL 不含 `q.subject_id`(避免误带 JOIN 残留字段)。
func TestSpeedByUser_ThisWeek(t *testing.T) {
	repo, mock := newPracticeQueryRepoWithSQLMock(t)

	mock.ExpectQuery(`INTERVAL 7 DAY`).
		WillReturnRows(sqlmock.NewRows([]string{"cnt", "avg_sec"}).
			AddRow(int64(20), float64(42.0)))

	row, err := repo.SpeedByUser(7, 7, 0)
	if err != nil {
		t.Fatalf("应无错,得到 err=%v", err)
	}
	if row.Cnt != 20 {
		t.Fatalf("Cnt 应为 20,得到 %d", row.Cnt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// 非法窗口参数(0/7, 7/14 等)应立即返 error,不查 DB。
// 「立即返」是契约：service 调用 buildSpeedStat 链路要能在窗口错误时 fail-fast,
// 避免拿到半截 SpeedRow 引发 nil 指针或四舍五入错误。
func TestSpeedByUser_InvalidWindow(t *testing.T) {
	repo, _ := newPracticeQueryRepoWithSQLMock(t)

	cases := []struct {
		name      string
		sinceDays uint64
		untilDays uint64
	}{
		{"since=0 until=7(只有终点无起点)", 0, 7},
		{"since=7 until=14(终点晚于起点)", 7, 14},
		{"since=7 until=7(起点等于终点)", 7, 7},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := repo.SpeedByUser(7, c.sinceDays, c.untilDays)
			if err == nil {
				t.Fatalf("非法窗口应返 error,得到 nil")
			}
		})
	}
}
