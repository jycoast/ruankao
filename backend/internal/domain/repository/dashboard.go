package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// TrendPoint / TypeDistItem 共享 DTO：repo 用 gorm 标签 Scan SQL 行，service
// 直接嵌入 DashboardStats 出 JSON。加 gorm 标签后,SQL 列名 `d`/`c`/`type`/`count`
// 可直接 Scan 到对应字段。
type TrendPoint struct {
	Date  string `gorm:"column:d"     json:"date"`
	Count int64  `gorm:"column:c"     json:"count"`
}

type TypeDistItem struct {
	Type  int   `gorm:"column:type"  json:"type"`
	Count int64 `gorm:"column:count" json:"count"`
}

// BaseStats 一组基础计数（用户/题库/练习/题目纠错/论文/AI 会话等）。
// OverallAccuracy 在 repo 内含零除保护（TotalPractice=0 时为 0,不 panic）。
type BaseStats struct {
	TotalUsers       int64
	NewUsersToday    int64
	NewUsersWeek     int64
	ActiveUsersToday int64
	TotalQuestions   int64
	TotalPractice    int64
	TodayPractice    int64
	OverallAccuracy  int // 0–100
	TotalReports     int64
	PendingReports   int64
	TotalEssays      int64
	PendingEssays    int64
	PendingFeedbacks int64
	AIConversations  int64
	AIMessages       int64
	AITodayMessages  int64
	AIUsers          int64
}

// RevenueStats 会员 + 营收 + 按渠道拆分（现金 channel<>'ldc' / 积分 channel='ldc'）。
// 金额一律以「分」返回，量纲分离见 DashboardStats 注释。
type RevenueStats struct {
	ActiveMembers       int64
	ExpiredMembers      int64
	PaidUsers           int64
	PaidOrders          int64
	RevenueTotal        int64
	RevenueMonth        int64
	RevenueCashTotal    int64
	RevenueCashMonth    int64
	RevenueCashOrders   int64
	RevenueCreditTotal  int64
	RevenueCreditMonth  int64
	RevenueCreditOrders int64
}

// AIUsageStats AI 调用统计：调用数 / token / 失败 / 估算占比。
// EstimatedRatio 在 repo 内含零除保护（Calls=0 时为 0）。
type AIUsageStats struct {
	Calls          int64
	Tokens         int64
	Failed         int64
	EstimatedRatio int // 0–100
}

// DashboardRepository 后台仪表盘聚合查询。SQL 集中下沉到 repo，
// service 层只负责编排与日期补齐（fillTrendGaps）。
type DashboardRepository interface {
	BaseStats() (*BaseStats, error)
	PracticeTrend(days int) ([]TrendPoint, error)
	QuestionTypeDist() ([]TypeDistItem, error)
	RevenueStats() (*RevenueStats, error)
	RevenueTrend(days int) ([]TrendPoint, error)
	RevenueTrendCredit(days int) ([]TrendPoint, error)
	AIUsageStats(days int) (*AIUsageStats, error)
}

type DashboardRepositoryImpl struct {
	db *gorm.DB
}

func NewDashboardRepository(db *gorm.DB) DashboardRepository {
	return &DashboardRepositoryImpl{db: db}
}

// scalar 复用 admin.go:270-274 的小工具：单值 COUNT/SUM 走 Scan(&v)。
func (r *DashboardRepositoryImpl) scalar(sql string, args ...interface{}) int64 {
	var v int64
	r.db.Raw(sql, args...).Scan(&v)
	return v
}

func (r *DashboardRepositoryImpl) BaseStats() (*BaseStats, error) {
	s := r.scalar
	total := s(`SELECT COUNT(*) FROM users`)
	newToday := s(`SELECT COUNT(*) FROM users WHERE DATE(created_at) = CURDATE()`)
	newWeek := s(`SELECT COUNT(*) FROM users WHERE created_at >= DATE_SUB(CURDATE(), INTERVAL 6 DAY)`)
	activeToday := s(`SELECT COUNT(DISTINCT user_id) FROM practice_records WHERE DATE(created_at) = CURDATE()`)
	totalQ := s(`SELECT COUNT(*) FROM questions WHERE status = 1`)
	totalP := s(`SELECT COUNT(*) FROM practice_records`)
	todayP := s(`SELECT COUNT(*) FROM practice_records WHERE DATE(created_at) = CURDATE()`)
	correct := s(`SELECT COUNT(*) FROM practice_records WHERE is_correct = 1`)
	// 零除保护：TotalPractice=0 时直接 0,不让 division by zero 冒到 service 层
	accuracy := 0
	if totalP > 0 {
		accuracy = int(float64(correct) / float64(totalP) * 100.0)
	}
	totalReports := s(`SELECT COUNT(*) FROM question_reports`)
	pendingReports := s(`SELECT COUNT(*) FROM question_reports WHERE status = 0`)
	totalEssays := s(`SELECT COUNT(*) FROM essay_submissions`)
	pendingEssays := s(`SELECT COUNT(*) FROM essay_submissions WHERE review_status = 0`)
	pendingFeedbacks := s(`SELECT COUNT(*) FROM feedbacks WHERE status = 0`)
	aiConvs := s(`SELECT COUNT(*) FROM ai_conversations`)
	aiMsgs := s(`SELECT COUNT(*) FROM ai_messages`)
	aiTodayMsgs := s(`SELECT COUNT(*) FROM ai_messages WHERE DATE(created_at) = CURDATE()`)
	aiUsers := s(`SELECT COUNT(DISTINCT user_id) FROM ai_conversations`)

	return &BaseStats{
		TotalUsers:       total,
		NewUsersToday:    newToday,
		NewUsersWeek:     newWeek,
		ActiveUsersToday: activeToday,
		TotalQuestions:   totalQ,
		TotalPractice:    totalP,
		TodayPractice:    todayP,
		OverallAccuracy:  accuracy,
		TotalReports:     totalReports,
		PendingReports:   pendingReports,
		TotalEssays:      totalEssays,
		PendingEssays:    pendingEssays,
		PendingFeedbacks: pendingFeedbacks,
		AIConversations:  aiConvs,
		AIMessages:       aiMsgs,
		AITodayMessages:  aiTodayMsgs,
		AIUsers:          aiUsers,
	}, nil
}

func (r *DashboardRepositoryImpl) PracticeTrend(days int) ([]TrendPoint, error) {
	rows, err := r.dayRows(fmtDays(days, "practice_records", "created_at"))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *DashboardRepositoryImpl) QuestionTypeDist() ([]TypeDistItem, error) {
	var rows []TypeDistItem
	err := r.db.Raw(`SELECT question_type AS type, COUNT(*) AS count FROM questions WHERE status = 1 GROUP BY question_type ORDER BY question_type`).Scan(&rows).Error
	return rows, err
}

func (r *DashboardRepositoryImpl) RevenueStats() (*RevenueStats, error) {
	s := r.scalar
	// 会员有效性以 memberships.expire_at 为唯一判据，与 MembershipService.IsActive 同口径
	active := s(`SELECT COUNT(*) FROM memberships WHERE expire_at > NOW()`)
	expired := s(`SELECT COUNT(*) FROM memberships WHERE expire_at <= NOW()`)
	// 只认 status = 1（已支付），排除 0 待支付 / 2 已退款 / 3 已关闭
	paidUsers := s(`SELECT COUNT(DISTINCT user_id) FROM orders WHERE status = 1`)
	paidOrders := s(`SELECT COUNT(*) FROM orders WHERE status = 1`)
	revenueTotal := s(`SELECT COALESCE(SUM(amount), 0) FROM orders WHERE status = 1`)
	revenueMonth := s(`SELECT COALESCE(SUM(amount), 0) FROM orders WHERE status = 1 AND paid_at >= DATE_FORMAT(CURDATE(), '%Y-%m-01')`)
	// 现金 = 非 ldc 通道（yipay / yuetongyun），积分 = ldc。
	// 用「channel <> 'ldc'」而非枚举两个现金通道：后续新增现金渠道时无需改这里，
	// 凡是积分以外的都算现金；存量订单（064 前默认 ''/yipay）也归入现金。
	cashTotal := s(`SELECT COALESCE(SUM(amount), 0) FROM orders WHERE status = 1 AND channel <> 'ldc'`)
	cashMonth := s(`SELECT COALESCE(SUM(amount), 0) FROM orders WHERE status = 1 AND channel <> 'ldc' AND paid_at >= DATE_FORMAT(CURDATE(), '%Y-%m-01')`)
	cashOrders := s(`SELECT COUNT(*) FROM orders WHERE status = 1 AND channel <> 'ldc'`)
	creditTotal := s(`SELECT COALESCE(SUM(amount), 0) FROM orders WHERE status = 1 AND channel = 'ldc'`)
	creditMonth := s(`SELECT COALESCE(SUM(amount), 0) FROM orders WHERE status = 1 AND channel = 'ldc' AND paid_at >= DATE_FORMAT(CURDATE(), '%Y-%m-01')`)
	creditOrders := s(`SELECT COUNT(*) FROM orders WHERE status = 1 AND channel = 'ldc'`)
	return &RevenueStats{
		ActiveMembers:       active,
		ExpiredMembers:      expired,
		PaidUsers:           paidUsers,
		PaidOrders:          paidOrders,
		RevenueTotal:        revenueTotal,
		RevenueMonth:        revenueMonth,
		RevenueCashTotal:    cashTotal,
		RevenueCashMonth:    cashMonth,
		RevenueCashOrders:   cashOrders,
		RevenueCreditTotal:  creditTotal,
		RevenueCreditMonth:  creditMonth,
		RevenueCreditOrders: creditOrders,
	}, nil
}

func (r *DashboardRepositoryImpl) RevenueTrend(days int) ([]TrendPoint, error) {
	return r.dayRows(fmt.Sprintf(`
		SELECT DATE(paid_at) AS d, COALESCE(SUM(amount), 0) AS c
		FROM orders
		WHERE status = 1 AND paid_at >= DATE_SUB(CURDATE(), INTERVAL %d DAY)
		GROUP BY DATE(paid_at)
	`, days-1))
}

func (r *DashboardRepositoryImpl) RevenueTrendCredit(days int) ([]TrendPoint, error) {
	return r.dayRows(fmt.Sprintf(`
		SELECT DATE(paid_at) AS d, COALESCE(SUM(amount), 0) AS c
		FROM orders
		WHERE status = 1 AND channel = 'ldc' AND paid_at >= DATE_SUB(CURDATE(), INTERVAL %d DAY)
		GROUP BY DATE(paid_at)
	`, days-1))
}

func (r *DashboardRepositoryImpl) AIUsageStats(days int) (*AIUsageStats, error) {
	s := r.scalar
	// own_key = 0 —— 用户自带 key 的调用不花平台的钱，混进来会虚高。
	// 与 AICallLogRepositoryImpl.TopUsers 的过滤口径保持一致。
	since := days - 1
	calls := s(`SELECT COUNT(*) FROM ai_call_logs WHERE own_key = 0 AND created_at >= DATE_SUB(CURDATE(), INTERVAL ? DAY)`, since)
	tokens := s(`SELECT COALESCE(SUM(total_tokens), 0) FROM ai_call_logs WHERE own_key = 0 AND created_at >= DATE_SUB(CURDATE(), INTERVAL ? DAY)`, since)
	failed := s(`SELECT COUNT(*) FROM ai_call_logs WHERE own_key = 0 AND success = 0 AND created_at >= DATE_SUB(CURDATE(), INTERVAL ? DAY)`, since)
	// 估算占比要露出来：多数流式服务不返回 usage，这部分 token 是按字符估的，
	// 不标注会被当成精确计数混进成本口径。
	ratio := 0
	if calls > 0 {
		est := s(`SELECT COUNT(*) FROM ai_call_logs WHERE own_key = 0 AND estimated = 1 AND created_at >= DATE_SUB(CURDATE(), INTERVAL ? DAY)`, since)
		ratio = int(float64(est) / float64(calls) * 100.0)
	}
	return &AIUsageStats{
		Calls:          calls,
		Tokens:         tokens,
		Failed:         failed,
		EstimatedRatio: ratio,
	}, nil
}

// dayRows 把「按日 GROUP BY」的 SQL 跑成 []TrendPoint。仅做 SQL → 行映射,
// 日期补齐由 service 层 fillTrendGaps 负责（保持 time.Now() 透明可测）。
func (r *DashboardRepositoryImpl) dayRows(sql string) ([]TrendPoint, error) {
	var rows []TrendPoint
	err := r.db.Raw(sql).Scan(&rows).Error
	return rows, err
}

// fmtDays 构造 practice_records 按日聚合 SQL：近 days 天（含今天）。
// INTERVAL N DAY 中 N=days-1（curdate 自身算一天）。
func fmtDays(days int, table, timeCol string) string {
	return fmt.Sprintf(`
		SELECT DATE(%s) AS d, COUNT(*) AS c
		FROM %s
		WHERE %s >= DATE_SUB(CURDATE(), INTERVAL %d DAY)
		GROUP BY DATE(%s)
	`, timeCol, table, timeCol, days-1, timeCol)
}
