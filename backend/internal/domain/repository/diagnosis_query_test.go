package repository

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// 诊断报告的 repo 聚合查询。口径要点：SUBSTRING_INDEX 取 level-1 大类
// （刻意不 join knowledge_points 按 parent 分组），错题存量 join questions
// 按科目过滤。这里锁 SQL 形态 + 行字段映射 + 参数透传。

// CategoryAccuracyBySubject：外层必须按 SUBSTRING_INDEX 分组、参数透传 userID+subjectID。
// regex 锁住 SUBSTRING_INDEX 片段,SQL 改回按原 knowledge_point 分组立刻失败。
func TestCategoryAccuracyBySubject_GroupByLevel1Category(t *testing.T) {
	repo, mock := newPracticeQueryRepoWithSQLMock(t)

	mock.ExpectQuery(`SUBSTRING_INDEX\(a\.knowledge_point, '>', 1\)`).
		WithArgs(uint64(7), uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"category", "done", "correct", "wrong"}).
			AddRow("操作系统", int64(20), int64(8), int64(12)).
			AddRow("计算机网络", int64(10), int64(9), int64(1)))

	rows, err := repo.CategoryAccuracyBySubject(7, 1)
	if err != nil {
		t.Fatalf("应无错返回,得到 err=%v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("期望 2 行,得到 %d", len(rows))
	}
	if rows[0].Category != "操作系统" || rows[0].Done != 20 || rows[0].Correct != 8 || rows[0].Wrong != 12 {
		t.Fatalf("首行数据错: %+v", rows[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足(可能没按 SUBSTRING_INDEX 分组): %v", err)
	}
}

// CategoryAccuracyBySubject 无数据：返回空切片而非 nil,service 侧据此判 has_data。
func TestCategoryAccuracyBySubject_Empty(t *testing.T) {
	repo, mock := newPracticeQueryRepoWithSQLMock(t)

	mock.ExpectQuery(`SUBSTRING_INDEX\(a\.knowledge_point`).
		WithArgs(uint64(7), uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"category", "done", "correct", "wrong"}))

	rows, err := repo.CategoryAccuracyBySubject(7, 1)
	if err != nil {
		t.Fatalf("应无错返回,得到 err=%v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("期望空集,得到 %d 行", len(rows))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// CategoryQuestionCountsBySubject：题目总量也按 level-1 大类,锁 SUBSTRING_INDEX(q...)。
func TestCategoryQuestionCountsBySubject_GroupByLevel1(t *testing.T) {
	repo, mock := newPracticeQueryRepoWithSQLMock(t)

	mock.ExpectQuery(`SUBSTRING_INDEX\(q\.knowledge_point, '>', 1\)`).
		WillReturnRows(sqlmock.NewRows([]string{"category", "total"}).
			AddRow("操作系统", int64(30)).
			AddRow("架构", int64(40)))

	rows, err := repo.CategoryQuestionCountsBySubject(1)
	if err != nil {
		t.Fatalf("应无错返回,得到 err=%v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("期望 2 行,得到 %d", len(rows))
	}
	if rows[0].Category != "操作系统" || rows[0].Total != 30 {
		t.Fatalf("首行数据错: %+v", rows[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// WrongBookStatsBySubject：待复习口径 = status=1 且 next_review_time <= NOW(),
// 与 FindDueReview 口径一致;join questions 按科目过滤。
func TestWrongBookStatsBySubject_CountsAndDue(t *testing.T) {
	repo, mock := newPracticeQueryRepoWithSQLMock(t)

	mock.ExpectQuery(`next_review_time IS NOT NULL AND wq\.next_review_time <= NOW\(\)`).
		WithArgs(uint64(7), uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"total", "due"}).
			AddRow(int64(13), int64(5)))

	row, err := repo.WrongBookStatsBySubject(7, 1)
	if err != nil {
		t.Fatalf("应无错返回,得到 err=%v", err)
	}
	if row.Total != 13 || row.Due != 5 {
		t.Fatalf("应为 total=13 due=5,得到 %+v", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足(口径可能跑偏): %v", err)
	}
}

// RecentWrongSamplesBySubject：只取最近答错的客观题,带 LIMIT 绑定。
func TestRecentWrongSamplesBySubject_Limit(t *testing.T) {
	repo, mock := newPracticeQueryRepoWithSQLMock(t)

	mock.ExpectQuery(`ORDER BY a\.id DESC LIMIT \?`).
		WithArgs(uint64(7), uint64(1), 5).
		WillReturnRows(sqlmock.NewRows([]string{"question_id", "title", "knowledge_point", "explanation"}).
			AddRow(uint64(100), "题干A", "操作系统>进程管理", "解析A"))

	rows, err := repo.RecentWrongSamplesBySubject(7, 1, 5)
	if err != nil {
		t.Fatalf("应无错返回,得到 err=%v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("期望 1 行,得到 %d", len(rows))
	}
	if rows[0].QuestionID != 100 || rows[0].Title != "题干A" ||
		rows[0].KnowledgePoint != "操作系统>进程管理" || rows[0].Explanation != "解析A" {
		t.Fatalf("行字段错: %+v", rows[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足(LIMIT 可能没绑定): %v", err)
	}
}
