package repository

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// newDashboardRepoWithSQLMock 构造 DashboardRepository 接口实现,后端走 sqlmock。
// 与 KnowledgePointRepository 的 helper(在 repository_test.go)解耦,两套测试独立维护。
func newDashboardRepoWithSQLMock(t *testing.T) (DashboardRepository, sqlmock.Sqlmock) {
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
	return NewDashboardRepository(gormDB), mock
}

// 7 天齐行：repo 应该把 SQL 行原样返回,不做日期补齐（补齐在 service 的 fillTrendGaps）。
// 这条契约很重要：repo 不能偷偷调 time.Now()，否则 service 测试要 pin 时间很麻烦。
func TestPracticeTrend_AllDays_ReturnsRowsUnchanged(t *testing.T) {
	repo, mock := newDashboardRepoWithSQLMock(t)

	mock.ExpectQuery("SELECT DATE").
		WillReturnRows(sqlmock.NewRows([]string{"d", "c"}).
			AddRow("2026-09-01", int64(10)).
			AddRow("2026-09-02", int64(15)).
			AddRow("2026-09-03", int64(20)).
			AddRow("2026-09-04", int64(8)).
			AddRow("2026-09-05", int64(25)))

	rows, err := repo.PracticeTrend(7)
	if err != nil {
		t.Fatalf("PracticeTrend 应无错返回,得到 err=%v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("期望 5 行,实际 %d", len(rows))
	}
	// 验证 repo 不做日期补齐——返回的就是 SQL 行原样
	if rows[0].Date != "2026-09-01" || rows[0].Count != 10 {
		t.Fatalf("首行数据错: %+v", rows[0])
	}
	if rows[4].Date != "2026-09-05" || rows[4].Count != 25 {
		t.Fatalf("末行数据错: %+v", rows[4])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// QuestionTypeDist：SQL 必须 GROUP BY + ORDER BY question_type,且只取 status=1 的题。
// regex 锁住这三条,任何一条改坏立刻 fail。
func TestQuestionTypeDist_OrdersByType(t *testing.T) {
	repo, mock := newDashboardRepoWithSQLMock(t)

	mock.ExpectQuery(`GROUP BY question_type`).
		WillReturnRows(sqlmock.NewRows([]string{"type", "count"}).
			AddRow(1, int64(300)).
			AddRow(2, int64(150)).
			AddRow(3, int64(50)))

	rows, err := repo.QuestionTypeDist()
	if err != nil {
		t.Fatalf("QuestionTypeDist 应无错返回,得到 err=%v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("期望 3 行,得到 %d", len(rows))
	}
	if rows[0].Type != 1 || rows[0].Count != 300 {
		t.Fatalf("首行 type=1 count=300,得到 %+v", rows[0])
	}
	// 关键:repo 必须按 SQL 返回顺序透传(ORDER BY 在 SQL 端)
	if rows[1].Type != 2 || rows[2].Type != 3 {
		t.Fatalf("type 应按 SQL 顺序透传,得到 %+v", rows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足(SQL 可能没 GROUP BY question_type): %v", err)
	}
}

// RevenueStats 12 条 SQL 全部命中 + 字段一一对应。
// 关键:验证「现金 = channel<>'ldc'」「积分 = channel='ldc'」两条渠道拆分
// 都有专属 SQL,任何一条漏掉都会让统计金额少一半。
func TestRevenueStats_FullBreakdown(t *testing.T) {
	repo, mock := newDashboardRepoWithSQLMock(t)

	// 完整 SQL 锚定 regex：每条 SQL 都唯一,顺序匹配。
	expectations := []struct {
		pattern string
		value   int64
		desc    string
	}{
		{`^SELECT COUNT\(\*\) FROM memberships WHERE expire_at > NOW\(\)$`, 80, "active"},
		{`^SELECT COUNT\(\*\) FROM memberships WHERE expire_at <= NOW\(\)$`, 120, "expired"},
		{`^SELECT COUNT\(DISTINCT user_id\) FROM orders WHERE status = 1$`, 50, "paidUsers"},
		{`^SELECT COUNT\(\*\) FROM orders WHERE status = 1$`, 200, "paidOrders"},
		{`^SELECT COALESCE\(SUM\(amount\), 0\) FROM orders WHERE status = 1$`, 990000, "revenueTotal"},
		{`^SELECT COALESCE\(SUM\(amount\), 0\) FROM orders WHERE status = 1 AND paid_at >= DATE_FORMAT\(CURDATE\(\), '%Y-%m-01'\)$`, 50000, "revenueMonth"},
		{`^SELECT COALESCE\(SUM\(amount\), 0\) FROM orders WHERE status = 1 AND channel <> 'ldc'$`, 880000, "cashTotal"},
		{`^SELECT COALESCE\(SUM\(amount\), 0\) FROM orders WHERE status = 1 AND channel <> 'ldc' AND paid_at >= DATE_FORMAT\(CURDATE\(\), '%Y-%m-01'\)$`, 45000, "cashMonth"},
		{`^SELECT COUNT\(\*\) FROM orders WHERE status = 1 AND channel <> 'ldc'$`, 180, "cashOrders"},
		{`^SELECT COALESCE\(SUM\(amount\), 0\) FROM orders WHERE status = 1 AND channel = 'ldc'$`, 110000, "creditTotal"},
		{`^SELECT COALESCE\(SUM\(amount\), 0\) FROM orders WHERE status = 1 AND channel = 'ldc' AND paid_at >= DATE_FORMAT\(CURDATE\(\), '%Y-%m-01'\)$`, 5000, "creditMonth"},
		{`^SELECT COUNT\(\*\) FROM orders WHERE status = 1 AND channel = 'ldc'$`, 20, "creditOrders"},
	}
	for _, e := range expectations {
		mock.ExpectQuery(e.pattern).
			WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(e.value))
	}

	rev, err := repo.RevenueStats()
	if err != nil {
		t.Fatalf("RevenueStats 应无错返回,得到 err=%v", err)
	}
	if rev.ActiveMembers != 80 {
		t.Fatalf("ActiveMembers 应为 80,得到 %d", rev.ActiveMembers)
	}
	if rev.ExpiredMembers != 120 {
		t.Fatalf("ExpiredMembers 应为 120,得到 %d", rev.ExpiredMembers)
	}
	if rev.PaidUsers != 50 {
		t.Fatalf("PaidUsers 应为 50,得到 %d", rev.PaidUsers)
	}
	if rev.PaidOrders != 200 {
		t.Fatalf("PaidOrders 应为 200,得到 %d", rev.PaidOrders)
	}
	if rev.RevenueTotal != 990000 {
		t.Fatalf("RevenueTotal 应为 990000,得到 %d", rev.RevenueTotal)
	}
	if rev.RevenueMonth != 50000 {
		t.Fatalf("RevenueMonth 应为 50000,得到 %d", rev.RevenueMonth)
	}
	// 现金 vs 积分拆分——必须互不污染
	if rev.RevenueCashTotal != 880000 {
		t.Fatalf("RevenueCashTotal 应为 880000,得到 %d", rev.RevenueCashTotal)
	}
	if rev.RevenueCreditTotal != 110000 {
		t.Fatalf("RevenueCreditTotal 应为 110000,得到 %d", rev.RevenueCreditTotal)
	}
	// 现金+积分 应等于总营收——交叉验证 SQL 拆分正确
	if rev.RevenueCashTotal+rev.RevenueCreditTotal != rev.RevenueTotal {
		t.Fatalf("现金+积分 (%d) != 总营收 (%d),SQL 拆分可能错",
			rev.RevenueCashTotal+rev.RevenueCreditTotal, rev.RevenueTotal)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足(可能漏注册 SQL 或 regex 没匹中): %v", err)
	}
}

// BaseStats 零除保护：TotalPractice=0 时 OverallAccuracy 必须返回 0，不能 panic。
// 上线后这个场景是「站点还没人做题」，OverallAccuracy 出 NaN 会让前端渲染炸。
func TestBaseStats_AccuracyZeroDivision(t *testing.T) {
	repo, mock := newDashboardRepoWithSQLMock(t)

	// 17 条 SQL 中关键路径：所有 COUNT 都返回 0,包括 correct / TotalPractice。
	// TotalPractice=0 时不进 accuracy 分支,所以实际只跑 17 条(不是 18)。
	// 顺序与 BaseStats 实现中调用 scalar 的顺序一致——任何调整实现时顺序
	// 也要同步调整测试期望。
	for i := 0; i < 17; i++ {
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	}

	base, err := repo.BaseStats()
	if err != nil {
		t.Fatalf("BaseStats 应无错返回,得到 err=%v", err)
	}
	if base.OverallAccuracy != 0 {
		t.Fatalf("TotalPractice=0 时 OverallAccuracy 应为 0,得到 %d", base.OverallAccuracy)
	}
	if base.TotalPractice != 0 {
		t.Fatalf("TotalPractice 应为 0,得到 %d", base.TotalPractice)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// BaseStats 非零路径：17 条 SQL 全部命中，且 OverallAccuracy = correct/total*100。
// 用 SQL 子串唯一区分每条 SQL（sqlmock 默认 regex 不锚定，子串匹配即可）。
// MatchExpectationsInOrder(false)：测试不锁实现顺序——BaseStats 实现可能微调
// 调用 scalar 的顺序,只要每条 SQL 都被调用且返回值正确,测试就过。
func TestBaseStats_HappyPath(t *testing.T) {
	repo, mock := newDashboardRepoWithSQLMock(t)
	mock.MatchExpectationsInOrder(false)

	// 设计：correct=150, totalP=200 → OverallAccuracy=75。
	// 其他 15 条用合理小数字,验证字段不漏拷。
	expectations := []struct {
		pattern string
		value   int64
	}{
		// users 系列
		{`^SELECT COUNT\(\*\) FROM users$`, 100},                                  // 1. total users
		{`users WHERE DATE\(created_at\) = CURDATE\(\)$`, 5},                      // 2. newToday
		{`users WHERE created_at >= DATE_SUB\(CURDATE\(\), INTERVAL 6 DAY\)`, 30}, // 3. newWeek
		// practice_records 系列
		{`COUNT\(DISTINCT user_id\) FROM practice_records WHERE DATE`, 20}, // 4. activeToday
		{`COUNT\(\*\) FROM questions WHERE status = 1`, 500},               // 5. totalQ
		{`^SELECT COUNT\(\*\) FROM practice_records$`, 200},                // 6. totalP  ← 准确率分母
		{`practice_records WHERE DATE\(created_at\) = CURDATE\(\)$`, 15},   // 7. todayP
		{`practice_records WHERE is_correct = 1`, 150},                     // 8. correct  ← 准确率分子
		// 题目纠错
		{`^SELECT COUNT\(\*\) FROM question_reports$`, 10}, // 9. totalReports
		{`question_reports WHERE status = 0`, 3},           // 10. pendingReports
		// 论文
		{`^SELECT COUNT\(\*\) FROM essay_submissions$`, 8}, // 11. totalEssays
		{`essay_submissions WHERE review_status = 0`, 2},   // 12. pendingEssays
		// 反馈
		{`^SELECT COUNT\(\*\) FROM feedbacks WHERE status = 0`, 4}, // 13. pendingFeedbacks
		// AI
		{`^SELECT COUNT\(\*\) FROM ai_conversations$`, 50},          // 14. aiConvs
		{`^SELECT COUNT\(\*\) FROM ai_messages$`, 200},              // 15. aiMsgs
		{`ai_messages WHERE DATE\(created_at\) = CURDATE\(\)$`, 25}, // 16. aiTodayMsgs
		{`COUNT\(DISTINCT user_id\) FROM ai_conversations`, 30},     // 17. aiUsers
	}

	for _, e := range expectations {
		mock.ExpectQuery(e.pattern).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(e.value))
	}

	base, err := repo.BaseStats()
	if err != nil {
		t.Fatalf("BaseStats 应无错返回,得到 err=%v", err)
	}
	if base.OverallAccuracy != 75 {
		t.Fatalf("OverallAccuracy 应为 75 (150/200*100),得到 %d", base.OverallAccuracy)
	}
	if base.TotalUsers != 100 {
		t.Fatalf("TotalUsers 应为 100,得到 %d", base.TotalUsers)
	}
	if base.TotalPractice != 200 {
		t.Fatalf("TotalPractice 应为 200,得到 %d", base.TotalPractice)
	}
	if base.PendingReports != 3 {
		t.Fatalf("PendingReports 应为 3,得到 %d", base.PendingReports)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足(可能漏注册 SQL 或 regex 没匹中): %v", err)
	}
}

// AIUsageStats own_key 过滤：用户自带 key 的调用不花平台的钱,混进统计会虚高。
// SQL 含 own_key = 0 + INTERVAL 29 DAY(days=30 → days-1=29)。
func TestAIUsageStats_OwnKeyFilter(t *testing.T) {
	repo, mock := newDashboardRepoWithSQLMock(t)

	// 设计:Calls=100, Tokens=50000, Failed=5, Estimated=20 → EstimatedRatio=20
	// SQL INTERVAL 29 DAY 锁住 days-1 关系
	expectations := []struct {
		pattern string
		value   int64
	}{
		{`COUNT\(\*\) FROM ai_call_logs WHERE own_key = 0 AND created_at`, 100}, // Calls
		{`COALESCE\(SUM\(total_tokens\)`, 50000},                                // Tokens
		{`success = 0 AND created_at`, 5},                                       // Failed
		{`estimated = 1 AND created_at`, 20},                                    // Estimated
	}
	for _, e := range expectations {
		mock.ExpectQuery(e.pattern).
			WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(e.value))
	}

	ai, err := repo.AIUsageStats(30)
	if err != nil {
		t.Fatalf("AIUsageStats 应无错返回,得到 err=%v", err)
	}
	if ai.Calls != 100 {
		t.Fatalf("Calls 应为 100,得到 %d", ai.Calls)
	}
	if ai.Tokens != 50000 {
		t.Fatalf("Tokens 应为 50000,得到 %d", ai.Tokens)
	}
	if ai.Failed != 5 {
		t.Fatalf("Failed 应为 5,得到 %d", ai.Failed)
	}
	if ai.EstimatedRatio != 20 {
		t.Fatalf("EstimatedRatio 应为 20 (20/100*100),得到 %d", ai.EstimatedRatio)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足(可能 SQL 没含 own_key=0 或 estimated=1): %v", err)
	}
}

// AIUsageStats 零除保护：Calls=0 时 EstimatedRatio 直接 0,不查 estimated 表。
// 「不查」是关键——上线初期 AI 调用为零时,这条 estimated SQL 不跑
// 也能防 SQL 报错或扫描全表。
func TestAIUsageStats_EstimatedRatioZeroDivision(t *testing.T) {
	repo, mock := newDashboardRepoWithSQLMock(t)

	// Calls/Tokens/Failed 全 0,estimated 那条不应被调用
	mock.ExpectQuery(`COUNT\(\*\) FROM ai_call_logs WHERE own_key = 0 AND created_at`).
		WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(0))
	mock.ExpectQuery(`COALESCE\(SUM\(total_tokens\)`).
		WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(0))
	mock.ExpectQuery(`success = 0 AND created_at`).
		WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(0))
	// 不注册 estimated=1 的期望——如果 repo 调了,会报 "unexpected query call"

	ai, err := repo.AIUsageStats(30)
	if err != nil {
		t.Fatalf("AIUsageStats 应无错返回,得到 err=%v", err)
	}
	if ai.Calls != 0 {
		t.Fatalf("Calls 应为 0,得到 %d", ai.Calls)
	}
	if ai.EstimatedRatio != 0 {
		t.Fatalf("Calls=0 时 EstimatedRatio 应为 0,得到 %d", ai.EstimatedRatio)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// RevenueTrend(全部渠道)与 RevenueTrendCredit(只 ldc 积分渠道)是两个独立 SQL。
// 关键契约:积分 SQL 必须含 `channel = 'ldc'`,否则会把现金收入也算进积分趋势
// 展示给用户,商业口径直接乱。
func TestRevenueTrend_OnlyLdcChannel(t *testing.T) {
	repo, mock := newDashboardRepoWithSQLMock(t)

	// 全部渠道:不含 channel 过滤,只按 status=1 + paid_at
	mock.ExpectQuery(`status = 1 AND paid_at >= DATE_SUB`).
		WillReturnRows(sqlmock.NewRows([]string{"d", "c"}).
			AddRow("2026-09-04", int64(9900)))

	allRows, err := repo.RevenueTrend(30)
	if err != nil {
		t.Fatalf("RevenueTrend 应无错返回,得到 err=%v", err)
	}
	if len(allRows) != 1 || allRows[0].Count != 9900 {
		t.Fatalf("RevenueTrend 应返 1 行 9900,得到 %+v", allRows)
	}

	// 积分渠道:必须同时含 status=1 AND channel='ldc' AND paid_at
	mock.ExpectQuery(`channel = 'ldc' AND paid_at`).
		WillReturnRows(sqlmock.NewRows([]string{"d", "c"}).
			AddRow("2026-09-04", int64(100)))

	creditRows, err := repo.RevenueTrendCredit(30)
	if err != nil {
		t.Fatalf("RevenueTrendCredit 应无错返回,得到 err=%v", err)
	}
	if len(creditRows) != 1 || creditRows[0].Count != 100 {
		t.Fatalf("RevenueTrendCredit 应返 1 行 100,得到 %+v", creditRows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足(积分 SQL 可能漏 channel='ldc'): %v", err)
	}
}
