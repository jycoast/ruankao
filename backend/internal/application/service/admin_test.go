package service

import (
	"errors"
	"testing"
	"time"

	"software-exam/internal/domain/repository"
)

// stubDashboardRepo 7 个方法的实现都返回预设值/错误。
// GetDashboard 是编排 + 补齐,所有 SQL 行为已经在 repo 层 sqlmock 验证过,
// service 层只需锁住「编排 + 补齐 + 字段拷贝 + 错误传播」四件事。
type stubDashboardRepo struct {
	base               *repository.BaseStats
	practiceRows       []repository.TrendPoint
	typeDist           []repository.TypeDistItem
	revenue            *repository.RevenueStats
	revenueRows        []repository.TrendPoint
	creditRows         []repository.TrendPoint
	ai                 *repository.AIUsageStats
	baseErr            error
	practiceErr        error
	typeDistErr        error
	revenueErr         error
	revenueTrendErr    error
	creditTrendErr     error
	aiErr              error
	gotPracticeDays    int
	gotRevenueDays     int
	gotCreditDays      int
	gotAIDays          int
}

func (s *stubDashboardRepo) BaseStats() (*repository.BaseStats, error) {
	return s.base, s.baseErr
}
func (s *stubDashboardRepo) PracticeTrend(days int) ([]repository.TrendPoint, error) {
	s.gotPracticeDays = days
	return s.practiceRows, s.practiceErr
}
func (s *stubDashboardRepo) QuestionTypeDist() ([]repository.TypeDistItem, error) {
	return s.typeDist, s.typeDistErr
}
func (s *stubDashboardRepo) RevenueStats() (*repository.RevenueStats, error) {
	return s.revenue, s.revenueErr
}
func (s *stubDashboardRepo) RevenueTrend(days int) ([]repository.TrendPoint, error) {
	s.gotRevenueDays = days
	return s.revenueRows, s.revenueTrendErr
}
func (s *stubDashboardRepo) RevenueTrendCredit(days int) ([]repository.TrendPoint, error) {
	s.gotCreditDays = days
	return s.creditRows, s.creditTrendErr
}
func (s *stubDashboardRepo) AIUsageStats(days int) (*repository.AIUsageStats, error) {
	s.gotAIDays = days
	return s.ai, s.aiErr
}

func newAdminServiceWithStubRepo(stub *stubDashboardRepo) *AdminService {
	return NewAdminService(nil, nil, nil, nil, stub, nil)
}

// 补齐输出依赖 service 内部的 time.Now()(实时钟,非 UTC)。
// 为保证任意运行日期都能稳定断言,stub 数据不写死具体日期,
// 而是按「今天」「6 天前」从 time.Now() 推导——与 service 同一时间基准。
var todayStr = time.Now().Format("2006-01-02")
var sixDaysAgoStr = time.Now().AddDate(0, 0, -6).Format("2006-01-02")

// GetDashboard 补齐:stub repo 返回不连续 3 行 → service 应输出连续 7 个 TrendPoint,
// 缺失日期 count=0。这是 service 层最关键的语义——repo 不补齐,service 必须补。
// 这里偷用 stubPracticeRows 但实际填的是给定天数,验证 fillTrendGaps 接 7 入参
// 后输出 7 个点;repo 实际返 3 行就够测 service 补齐逻辑。
func TestGetDashboard_FillsMissingDays(t *testing.T) {
	stub := &stubDashboardRepo{
		base: &repository.BaseStats{},
		revenue: &repository.RevenueStats{},
		ai:     &repository.AIUsageStats{},
		// 模拟 repo 返 2 行不连续日期,service 必须补齐到 7。
		// fillTrendGaps 从 i=days-1(=6) 倒序循环到 i=0(=today):
		//   i=6 → today-6, i=0 → today
		// 所以只有 i=6 和 i=0 有数据,中间 5 个全 0
		practiceRows: []repository.TrendPoint{
			{Date: sixDaysAgoStr, Count: 5}, // i=6
			{Date: todayStr, Count: 12},     // i=0 今天
		},
		revenueRows: []repository.TrendPoint{
			{Date: todayStr, Count: 1000}, // 仅今天
		},
		creditRows: []repository.TrendPoint{
			{Date: todayStr, Count: 50},
		},
	}
	svc := newAdminServiceWithStubRepo(stub)

	got, err := svc.GetDashboard()
	if err != nil {
		t.Fatalf("GetDashboard 应无错返回,得到 err=%v", err)
	}
	// PracticeTrend 应拿到 7 天参数(repo 层 sqlmock 已验过参数透传)
	if stub.gotPracticeDays != 7 {
		t.Fatalf("PracticeTrend(days=7),实际 %d", stub.gotPracticeDays)
	}
	if stub.gotRevenueDays != 30 {
		t.Fatalf("RevenueTrend(days=30),实际 %d", stub.gotRevenueDays)
	}
	if stub.gotCreditDays != 30 {
		t.Fatalf("RevenueTrendCredit(days=30),实际 %d", stub.gotCreditDays)
	}
	if stub.gotAIDays != 30 {
		t.Fatalf("AIUsageStats(days=30),实际 %d", stub.gotAIDays)
	}

	// 关键:service 应补齐到 7 个点(9/5 今天起往前 6 天 + 今天)
	if len(got.PracticeTrend) != 7 {
		t.Fatalf("PracticeTrend 应补齐到 7 个点,得到 %d", len(got.PracticeTrend))
	}
	// fillTrendGaps 从 i=days-1(=6) 倒序 append:out[0]=i=6 (6 天前),out[6]=i=0 (今天)
	if got.PracticeTrend[0].Count != 5 {
		t.Fatalf("PracticeTrend[0] 6 天前应是 5,得到 %+v", got.PracticeTrend[0])
	}
	if got.PracticeTrend[6].Count != 12 {
		t.Fatalf("PracticeTrend[6] 今天应是 12,得到 %+v", got.PracticeTrend[6])
	}
	for i := 1; i <= 5; i++ {
		if got.PracticeTrend[i].Count != 0 {
			t.Fatalf("PracticeTrend[%d] 缺失日期应 count=0,得到 %+v", i, got.PracticeTrend[i])
		}
	}
	if len(got.RevenueTrend) != 30 {
		t.Fatalf("RevenueTrend 应补齐到 30 个点,得到 %d", len(got.RevenueTrend))
	}
	// RevenueTrend 同样:out[29] 是今天(today=9/5),i=29 是 now-29
	// stub 只填 today 一行,所以 out[29]=1000,前面 29 个全 0
	if got.RevenueTrend[29].Count != 1000 {
		t.Fatalf("RevenueTrend[29] 今天应是 1000,得到 %+v", got.RevenueTrend[29])
	}
	if len(got.RevenueTrendCredit) != 30 {
		t.Fatalf("RevenueTrendCredit 应补齐到 30 个点,得到 %d", len(got.RevenueTrendCredit))
	}
}

// 空 repo 返回零值:所有字段除切片外应为零,切片应补齐但全 0。
// 防「忘拷字段」回归——任何一个 DTO 字段没从 repo 拷到 DashboardStats 都暴露。
func TestGetDashboard_EmptyRepo_ReturnsZeros(t *testing.T) {
	stub := &stubDashboardRepo{
		base:    &repository.BaseStats{},
		revenue: &repository.RevenueStats{},
		ai:      &repository.AIUsageStats{},
	}
	svc := newAdminServiceWithStubRepo(stub)

	got, err := svc.GetDashboard()
	if err != nil {
		t.Fatalf("GetDashboard 应无错返回,得到 err=%v", err)
	}
	if got.TotalUsers != 0 || got.TotalPractice != 0 || got.OverallAccuracy != 0 {
		t.Fatalf("BaseStats 字段未全 0: %+v", got)
	}
	if got.ActiveMembers != 0 || got.RevenueTotal != 0 || got.RevenueCashTotal != 0 || got.RevenueCreditTotal != 0 {
		t.Fatalf("RevenueStats 字段未全 0: %+v", got)
	}
	if got.AICalls30d != 0 || got.AIEstimatedRatio != 0 {
		t.Fatalf("AIUsageStats 字段未全 0: %+v", got)
	}
	// 切片应补齐为长度,但每点 count=0
	if len(got.PracticeTrend) != 7 {
		t.Fatalf("PracticeTrend 应补齐到 7,得到 %d", len(got.PracticeTrend))
	}
	if len(got.RevenueTrend) != 30 {
		t.Fatalf("RevenueTrend 应补齐到 30,得到 %d", len(got.RevenueTrend))
	}
	if got.PracticeTrend[0].Count != 0 {
		t.Fatalf("PracticeTrend[0] 应 0,得到 %+v", got.PracticeTrend[0])
	}
}

// 错误传播:repo 任一处返回 err,service 应立即返回,不留半填 DashboardStats。
// 这是上游契约——admin handler 看到非 nil err 就能立刻返 500。
func TestGetDashboard_PropagatesRepoError(t *testing.T) {
	cases := []struct {
		name string
		stub *stubDashboardRepo
	}{
		{
			"BaseStats 出错",
			&stubDashboardRepo{baseErr: errors.New("db down"), revenue: &repository.RevenueStats{}, ai: &repository.AIUsageStats{}},
		},
		{
			"RevenueStats 出错",
			&stubDashboardRepo{base: &repository.BaseStats{}, revenueErr: errors.New("db down"), ai: &repository.AIUsageStats{}},
		},
		{
			"AIUsageStats 出错",
			&stubDashboardRepo{base: &repository.BaseStats{}, revenue: &repository.RevenueStats{}, aiErr: errors.New("db down")},
		},
		{
			"PracticeTrend 出错",
			&stubDashboardRepo{base: &repository.BaseStats{}, revenue: &repository.RevenueStats{}, ai: &repository.AIUsageStats{}, practiceErr: errors.New("db down")},
		},
		{
			"RevenueTrendCredit 出错",
			&stubDashboardRepo{base: &repository.BaseStats{}, revenue: &repository.RevenueStats{}, ai: &repository.AIUsageStats{}, creditTrendErr: errors.New("db down")},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := newAdminServiceWithStubRepo(c.stub)
			got, err := svc.GetDashboard()
			if err == nil {
				t.Fatalf("应传播 repo err,得到 nil")
			}
			if got != nil {
				t.Fatalf("err 时应返 nil DashboardStats,得到 %+v", got)
			}
		})
	}
}

// 一次性返完整数据,验证 38 字段一一对应。
// 防「忘拷字段」回归(比如新增 DTO 字段忘了搬到 DashboardStats)。
func TestGetDashboard_AssemblesAllSubdomains(t *testing.T) {
	stub := &stubDashboardRepo{
		base: &repository.BaseStats{
			TotalUsers: 100, NewUsersToday: 5, NewUsersWeek: 30, ActiveUsersToday: 20,
			TotalQuestions: 500, TotalPractice: 200, TodayPractice: 15, OverallAccuracy: 75,
			TotalReports: 10, PendingReports: 3,
			TotalEssays: 8, PendingEssays: 2,
			PendingFeedbacks: 4,
			AIConversations: 50, AIMessages: 200, AITodayMessages: 25, AIUsers: 30,
		},
		revenue: &repository.RevenueStats{
			ActiveMembers: 80, ExpiredMembers: 120,
			PaidUsers: 50, PaidOrders: 200,
			RevenueTotal: 990000, RevenueMonth: 50000,
			RevenueCashTotal: 880000, RevenueCashMonth: 45000, RevenueCashOrders: 180,
			RevenueCreditTotal: 110000, RevenueCreditMonth: 5000, RevenueCreditOrders: 20,
		},
		ai: &repository.AIUsageStats{
			Calls: 100, Tokens: 50000, Failed: 5, EstimatedRatio: 20,
		},
		practiceRows: []repository.TrendPoint{},
		revenueRows:  []repository.TrendPoint{},
		creditRows:   []repository.TrendPoint{},
		typeDist: []repository.TypeDistItem{
			{Type: 1, Count: 300},
		},
	}
	svc := newAdminServiceWithStubRepo(stub)
	got, err := svc.GetDashboard()
	if err != nil {
		t.Fatalf("应无错,得到 err=%v", err)
	}

	// BaseStats 17 字段一一拷
	if got.TotalUsers != 100 || got.NewUsersToday != 5 || got.OverallAccuracy != 75 {
		t.Fatalf("BaseStats 字段漏拷: %+v", got)
	}
	// RevenueStats 12 字段一一拷
	if got.ActiveMembers != 80 || got.RevenueCashTotal != 880000 || got.RevenueCreditOrders != 20 {
		t.Fatalf("RevenueStats 字段漏拷: %+v", got)
	}
	// AIUsageStats 4 字段一一拷
	if got.AICalls30d != 100 || got.AITokens30d != 50000 || got.AIFailed30d != 5 || got.AIEstimatedRatio != 20 {
		t.Fatalf("AIUsageStats 字段漏拷: %+v", got)
	}
	// TypeDist 直传(不补齐,SQL 端 ORDER BY 排好)
	if len(got.TypeDist) != 1 || got.TypeDist[0].Type != 1 {
		t.Fatalf("TypeDist 漏拷: %+v", got.TypeDist)
	}
}
