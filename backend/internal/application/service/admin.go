package service

import (
	"context"
	"errors"
	"time"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
	"software-exam/pkg/online"

	"golang.org/x/crypto/bcrypt"
)

type AdminService struct {
	userRepo       repository.UserRepository
	questionRepo   repository.QuestionRepository
	profileRepo    repository.UserProfileRepository
	membershipRepo repository.MembershipRepository
	dashboardRepo  repository.DashboardRepository
	userState      *UserStateService
}

func NewAdminService(
	userRepo repository.UserRepository,
	questionRepo repository.QuestionRepository,
	profileRepo repository.UserProfileRepository,
	membershipRepo repository.MembershipRepository,
	dashboardRepo repository.DashboardRepository,
	userState *UserStateService,
) *AdminService {
	return &AdminService{
		userRepo:       userRepo,
		questionRepo:   questionRepo,
		profileRepo:    profileRepo,
		membershipRepo: membershipRepo,
		dashboardRepo:  dashboardRepo,
		userState:      userState,
	}
}

// AdminUserItem 用户管理列表项：用户基础信息 + 会员状态。
// 会员口径与 MembershipService.Status 一致：只看 expire_at 是否晚于当前时间。
type AdminUserItem struct {
	model.User
	IsMember bool       `json:"is_member"` // 当前是否有效会员
	Plan     string     `json:"plan"`      // 会员套餐 key（month/year 等）
	ExpireAt *time.Time `json:"expire_at"` // 到期时间；非会员为 null
	DaysLeft int        `json:"days_left"` // 剩余天数；非会员/已到期为 0
}

func (s *AdminService) ListUsers(page, pageSize int, keyword string, role, status int8) ([]AdminUserItem, int64, error) {
	users, total, err := s.userRepo.List(repository.UserListReq{
		Page: page, PageSize: pageSize,
		Keyword: keyword, Role: role, Status: status,
	})
	if err != nil {
		return nil, 0, err
	}

	items := make([]AdminUserItem, 0, len(users))
	if len(users) == 0 {
		return items, total, nil
	}
	// 一次性批量查本页用户的会员记录，避免逐用户 N+1
	ids := make([]uint64, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	memberships, err := s.membershipRepo.FindByUserIDs(ids)
	if err != nil {
		return nil, 0, err
	}
	byUser := make(map[uint64]model.Membership, len(memberships))
	for _, m := range memberships {
		byUser[m.UserID] = m
	}

	now := time.Now()
	for _, u := range users {
		item := AdminUserItem{User: u}
		if m, ok := byUser[u.ID]; ok && m.ExpireAt.After(now) {
			item.IsMember = true
			item.Plan = m.Plan
			item.ExpireAt = &m.ExpireAt
			item.DaysLeft = int(m.ExpireAt.Sub(now).Hours()/24) + 1
		}
		items = append(items, item)
	}
	return items, total, nil
}

func (s *AdminService) UpdateUserStatus(userID uint64, status int8) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}
	user.Status = status
	if err := s.userRepo.Update(user); err != nil {
		return err
	}
	s.userState.Invalidate(userID)
	return nil
}

func (s *AdminService) UpdateUserRole(userID uint64, role int8) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}
	user.Role = role
	if err := s.userRepo.Update(user); err != nil {
		return err
	}
	s.userState.Invalidate(userID)
	return nil
}

func (s *AdminService) ResetUserPassword(userID uint64, newPassword string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hash)
	return s.userRepo.Update(user)
}

// OnlineUser 最近活跃的在线用户（管理后台「实时在线」卡片用）。
type OnlineUser struct {
	ID           uint64 `json:"id"`
	Username     string `json:"username"`
	Nickname     string `json:"nickname"`
	Avatar       string `json:"avatar"`
	LastActiveAt int64  `json:"last_active_at"` // 最近活跃的 unix 秒
	IsMember     bool   `json:"is_member"`      // 当前是否有效会员
}

// OnlineStats 在线统计快照。
type OnlineStats struct {
	Online int64        `json:"online"`
	Users  []OnlineUser `json:"users"`
	Window int          `json:"window"` // 在线窗口（秒），前端据此展示"近 N 分钟"
	Now    int64        `json:"now"`    // 服务器当前 unix 秒，前端配合 last_active_at 算相对时间，规避客户端时钟偏差
}

// GetOnline 在线用户数 + 最近活跃用户列表。
// 定义：统计窗口（5 分钟）内有请求的登录用户。Redis 不可用时
// online.Snapshot 返回 0（fail-open），这里照常返回成功。
func (s *AdminService) GetOnline() (*OnlineStats, error) {
	count, members, err := online.Snapshot(context.Background())
	if err != nil {
		return nil, err
	}
	stats := &OnlineStats{
		Online: count,
		Window: int(online.OnlineWindow.Seconds()),
		Now:    time.Now().Unix(),
	}
	if len(members) == 0 {
		return stats, nil
	}

	// 批量点查，避免逐用户 N+1；管理后台专用、30s 一次，量级可忽略
	ids := make([]uint64, 0, len(members))
	activeAt := make(map[uint64]int64, len(members))
	for _, m := range members {
		ids = append(ids, m.ID)
		activeAt[m.ID] = m.Score
	}

	users, err := s.userRepo.FindByIDs(ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint64]model.User, len(users))
	for _, u := range users {
		byID[u.ID] = u
	}

	// 会员口径与 ListUsers 一致：只看 expire_at 是否晚于当前时间。查询失败按非会员降级
	now := time.Now()
	memberIDs := make(map[uint64]bool)
	if mlist, err := s.membershipRepo.FindByUserIDs(ids); err == nil {
		for _, m := range mlist {
			if m.ExpireAt.After(now) {
				memberIDs[m.UserID] = true
			}
		}
	}

	// 保持 Snapshot 的活跃倒序
	stats.Users = make([]OnlineUser, 0, len(members))
	for _, m := range members {
		u, ok := byID[m.ID]
		if !ok {
			continue // 已删号等脏数据直接跳过
		}
		stats.Users = append(stats.Users, OnlineUser{
			ID:           u.ID,
			Username:     u.Username,
			Nickname:     u.Nickname,
			Avatar:       u.Avatar,
			LastActiveAt: activeAt[m.ID],
			IsMember:     memberIDs[m.ID],
		})
	}
	return stats, nil
}

type DashboardStats struct {
	TotalUsers       int64 `json:"total_users"`
	NewUsersToday    int64 `json:"new_users_today"`
	NewUsersWeek     int64 `json:"new_users_week"`
	ActiveUsersToday int64 `json:"active_users_today"`
	TotalQuestions   int64 `json:"total_questions"`
	TotalPractice    int64 `json:"total_practice"`
	TodayPractice    int64 `json:"today_practice"`
	OverallAccuracy  int   `json:"overall_accuracy"`
	TotalReports     int64 `json:"total_reports"`
	PendingReports   int64 `json:"pending_reports"`
	TotalEssays      int64 `json:"total_essays"`
	PendingEssays    int64 `json:"pending_essays"`
	PendingFeedbacks int64 `json:"pending_feedbacks"`
	AIConversations  int64 `json:"ai_conversations"`
	AIMessages       int64 `json:"ai_messages"`
	AITodayMessages  int64 `json:"ai_today_messages"`
	AIUsers          int64 `json:"ai_users"`
	// 商业化指标。金额一律以「分」返回，除以 100 的动作只在前端做一次，
	// 两侧各转一次迟早会有人漏掉或转两遍。
	ActiveMembers    int64          `json:"active_members"`
	ExpiredMembers   int64          `json:"expired_members"`
	PaidUsers        int64          `json:"paid_users"`
	PaidOrders       int64          `json:"paid_orders"`
	RevenueTotal     int64          `json:"revenue_total"`
	RevenueMonth     int64          `json:"revenue_month"`
	// 营收按渠道拆分：现金（yipay/yuetongyun）与 LDC 积分（ldc）是两条独立定价线、
	// 量纲不同（分 vs 0.01 积分），直接相加没有意义，必须分开展示。
	// 单位仍以「分」返回，除 100 只在各自前端展示时做一次。
	RevenueCashTotal    int64        `json:"revenue_cash_total"`
	RevenueCashMonth    int64        `json:"revenue_cash_month"`
	RevenueCashOrders   int64        `json:"revenue_cash_orders"`
	RevenueCreditTotal  int64        `json:"revenue_credit_total"`
	RevenueCreditMonth  int64        `json:"revenue_credit_month"`
	RevenueCreditOrders int64        `json:"revenue_credit_orders"`
	RevenueTrend        []repository.TrendPoint `json:"revenue_trend"`        // 全部渠道，近 30 天逐日（分）
	RevenueTrendCredit  []repository.TrendPoint `json:"revenue_trend_credit"` // 积分渠道，近 30 天逐日（0.01 积分），与 RevenueTrend 同日期序列
	AICalls30d          int64                    `json:"ai_calls_30d"`
	AITokens30d      int64                      `json:"ai_tokens_30d"`
	AIFailed30d      int64                      `json:"ai_failed_30d"`
	AIEstimatedRatio int                        `json:"ai_estimated_ratio"`
	PracticeTrend    []repository.TrendPoint   `json:"practice_trend"`
	TypeDist         []repository.TypeDistItem `json:"type_dist"`
}

func (s *AdminService) GetDashboard() (*DashboardStats, error) {
	// SQL 全部下沉到 DashboardRepository。service 只做编排 + 日期补齐：
	// 零除保护(OverallAccuracy/AIEstimatedRatio)已在 repo 内完成,日期补齐
	// 留在 service 是因为 time.Now() 是副作用,放 repo 会藏时间来源。
	base, err := s.dashboardRepo.BaseStats()
	if err != nil {
		return nil, err
	}
	rev, err := s.dashboardRepo.RevenueStats()
	if err != nil {
		return nil, err
	}
	ai, err := s.dashboardRepo.AIUsageStats(30)
	if err != nil {
		return nil, err
	}
	practiceRows, err := s.dashboardRepo.PracticeTrend(7)
	if err != nil {
		return nil, err
	}
	typeDist, err := s.dashboardRepo.QuestionTypeDist()
	if err != nil {
		return nil, err
	}
	revRows, err := s.dashboardRepo.RevenueTrend(30)
	if err != nil {
		return nil, err
	}
	creditRows, err := s.dashboardRepo.RevenueTrendCredit(30)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &DashboardStats{
		TotalUsers:       base.TotalUsers,
		NewUsersToday:    base.NewUsersToday,
		NewUsersWeek:     base.NewUsersWeek,
		ActiveUsersToday: base.ActiveUsersToday,
		TotalQuestions:   base.TotalQuestions,
		TotalPractice:    base.TotalPractice,
		TodayPractice:    base.TodayPractice,
		OverallAccuracy:  base.OverallAccuracy,
		TotalReports:     base.TotalReports,
		PendingReports:   base.PendingReports,
		TotalEssays:      base.TotalEssays,
		PendingEssays:    base.PendingEssays,
		PendingFeedbacks: base.PendingFeedbacks,
		AIConversations:  base.AIConversations,
		AIMessages:       base.AIMessages,
		AITodayMessages:  base.AITodayMessages,
		AIUsers:          base.AIUsers,

		ActiveMembers:       rev.ActiveMembers,
		ExpiredMembers:      rev.ExpiredMembers,
		PaidUsers:           rev.PaidUsers,
		PaidOrders:          rev.PaidOrders,
		RevenueTotal:        rev.RevenueTotal,
		RevenueMonth:        rev.RevenueMonth,
		RevenueCashTotal:    rev.RevenueCashTotal,
		RevenueCashMonth:    rev.RevenueCashMonth,
		RevenueCashOrders:   rev.RevenueCashOrders,
		RevenueCreditTotal:  rev.RevenueCreditTotal,
		RevenueCreditMonth:  rev.RevenueCreditMonth,
		RevenueCreditOrders: rev.RevenueCreditOrders,

		AICalls30d:       ai.Calls,
		AITokens30d:      ai.Tokens,
		AIFailed30d:      ai.Failed,
		AIEstimatedRatio: ai.EstimatedRatio,

		PracticeTrend:      fillTrendGaps(practiceRows, 7, now),
		RevenueTrend:       fillTrendGaps(revRows, 30, now),
		RevenueTrendCredit: fillTrendGaps(creditRows, 30, now),
		TypeDist:           typeDist,
	}, nil
}

// fillTrendGaps 把 SQL 返回的不连续日期补齐为 N 天连续 TrendPoint，缺失日期 count=0。
// 留在这里：repo 只翻译 SQL → 行，时间窗口/补齐是呈现决策；测试用传入 now 即可 pin。
func fillTrendGaps(rows []repository.TrendPoint, days int, now time.Time) []repository.TrendPoint {
	byDay := map[string]int64{}
	for _, r := range rows {
		if len(r.Date) >= 10 {
			byDay[r.Date[:10]] = r.Count
		}
	}
	out := make([]repository.TrendPoint, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		key := d.Format("2006-01-02")
		out = append(out, repository.TrendPoint{
			Date:  d.Format("01-02"),
			Count: byDay[key],
		})
	}
	return out
}
