package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// PracticeQueryRepository 练习域聚合查询。
//
// 设计动机：practice.go 原本 6 处 type assertion + 5 处 Raw 直接拿
// QuestionRepositoryImpl.DB() 拼 SQL,SQL 模板 latestAttemptsSQL 和
// latestAttemptsBySubjectSQL 散在 service 包,跨函数复用难追踪。
// 这里把所有"客观题正确率 / 知识点聚合 / 趋势 / 做题速度"7 个聚合查询
// 集中到本接口,SQL 模板和题型常量(1=单选, 2=多选)迁到 repo,
// service 层只做结果转换与时间窗口编排。
//
// SQL 模板拆解说明：
//   - latestAttemptsSQL:某用户每题最近一次作答的派生表(全局)
//   - latestAttemptsBySubjectSQL:同构但按 subject_id 过滤
//     这俩被多个聚合查询(Accuracy / CategoryProgress / KnowledgeAccuracy)复用,
//     抽到 repo 后由 repo 内部组合 SQL,service 不再接触原始 SQL。
//
// sinceDays / untilDays 窗口语义(用于 SpeedBy*):
//   - (0, 0)         全部样本(无 where)
//   - (7, 0)         近 7 天(>= 7)
//   - (14, 7)        前 7 天(>= 14 AND < 7)
type PracticeQueryRepository interface {
	AccuracyByUser(userID uint64) (AccuracyRow, error)
	CategoryProgress(userID uint64) ([]ProgressRow, error)
	AccuracyBySubject(userID, subjectID uint64) (AccuracyRow, error)
	TrendBySubject(userID, subjectID uint64) ([]TrendRow, error)
	KnowledgeAccuracyBySubject(userID, subjectID uint64) ([]KpRow, error)
	SpeedBySubject(userID, subjectID, sinceDays, untilDays uint64) (SpeedRow, error)
	SpeedByUser(userID uint64, sinceDays, untilDays uint64) (SpeedRow, error)
	// ---- 诊断报告（会员专属页 /my/diagnosis） ----
	// 薄弱点画像 / 错题本存量 / AI 样例，SQL 实现见 diagnosis_query.go。
	CategoryAccuracyBySubject(userID, subjectID uint64) ([]WeaknessRow, error)
	CategoryQuestionCountsBySubject(subjectID uint64) ([]CategoryCountRow, error)
	WrongBookStatsBySubject(userID, subjectID uint64) (WrongBookStats, error)
	RecentWrongSamplesBySubject(userID, subjectID uint64, limit int) ([]WrongSampleRow, error)
}

// AccuracyRow 正确率统计一行:做过的客观题数 + 其中答对的数。
// Accuracy = Correct / Done * 100,零除时为 0(由 service 决定)。
type AccuracyRow struct {
	Done    int64 `gorm:"column:done"`
	Correct int64 `gorm:"column:correct"`
}

// ProgressRow 按知识点聚合的进度:题量 / 已做 / 答对。
type ProgressRow struct {
	KnowledgePoint string `gorm:"column:knowledge_point"`
	Total          int64  `gorm:"column:total"`
	Done           int64  `gorm:"column:done"`
	Correct        int64  `gorm:"column:correct"`
}

// TrendRow 学习趋势一行:日期 + 总答题数 + 正确数。
type TrendRow struct {
	Day     string `gorm:"column:day"`
	Total   int64  `gorm:"column:total"`
	Correct int64  `gorm:"column:correct"`
}

// KpRow 知识点正确率一行。
type KpRow struct {
	KnowledgePoint string `gorm:"column:knowledge_point"`
	Done           int64  `gorm:"column:done"`
	Correct        int64  `gorm:"column:correct"`
}

// SpeedRow 做题速度一行:样本数 + 平均用时秒数。
// AvgSec 保留为 float64(由 service 决定是否四舍五入到 int)。
type SpeedRow struct {
	Cnt    int64   `gorm:"column:cnt"`
	AvgSec float64 `gorm:"column:avg_sec"`
}

type PracticeQueryRepositoryImpl struct {
	db *gorm.DB
}

func NewPracticeQueryRepository(db *gorm.DB) PracticeQueryRepository {
	return &PracticeQueryRepositoryImpl{db: db}
}

// objectiveQuestionTypes 参与正确率统计的题型(单选、多选)。
// 与 service 包原本的 objectiveQuestionTypes 同源,迁到这里集中管理。
func objectiveQuestionTypes() []int8 {
	return []int8{1, 2}
}

// latestAttemptsSQL 是「用户每题最近一次作答」的派生表,? 为 user_id。
// 被 AccuracyByUser / CategoryProgress 复用。
const latestAttemptsSQL = `
	SELECT pr.id, pr.question_id, pr.is_correct, pr.created_at
	FROM practice_records pr
	JOIN (
		SELECT question_id, MAX(id) AS last_id
		FROM practice_records WHERE user_id = ?
		GROUP BY question_id
	) latest ON latest.last_id = pr.id
	JOIN questions q ON q.id = pr.question_id
	WHERE q.question_type IN (1, 2)
`

// latestAttemptsBySubjectSQL 与 latestAttemptsSQL 同构,但额外按科目过滤。
// 参数顺序:? 为 user_id,? 为 subject_id。
const latestAttemptsBySubjectSQL = `
	SELECT pr.id, pr.question_id, pr.is_correct, pr.created_at, q.knowledge_point
	FROM practice_records pr
	JOIN questions q ON q.id = pr.question_id
	JOIN (
		SELECT question_id, MAX(id) AS last_id
		FROM practice_records WHERE user_id = ?
		GROUP BY question_id
	) latest ON latest.last_id = pr.id
	WHERE q.question_type IN (1, 2) AND q.subject_id = ?
`

func (r *PracticeQueryRepositoryImpl) AccuracyByUser(userID uint64) (AccuracyRow, error) {
	var row AccuracyRow
	err := r.db.Raw(`SELECT COUNT(*) as done, SUM(CASE WHEN a.is_correct = 1 THEN 1 ELSE 0 END) as correct
		FROM (`+latestAttemptsSQL+`) a`, userID).Scan(&row).Error
	return row, err
}

func (r *PracticeQueryRepositoryImpl) CategoryProgress(userID uint64) ([]ProgressRow, error) {
	var rows []ProgressRow
	err := r.db.Raw(`
		SELECT
			q.knowledge_point,
			COUNT(DISTINCT q.id) as total,
			COUNT(a.question_id) as done,
			SUM(CASE WHEN a.is_correct = 1 THEN 1 ELSE 0 END) as correct
		FROM questions q
		LEFT JOIN (`+latestAttemptsSQL+`) a ON a.question_id = q.id
		WHERE q.knowledge_point IS NOT NULL AND q.knowledge_point != ''
		  AND q.status = 1 AND q.question_type IN (?)
		GROUP BY q.knowledge_point
		ORDER BY q.knowledge_point
	`, userID, objectiveQuestionTypes()).Scan(&rows).Error
	return rows, err
}

func (r *PracticeQueryRepositoryImpl) AccuracyBySubject(userID, subjectID uint64) (AccuracyRow, error) {
	var row AccuracyRow
	err := r.db.Raw(`SELECT COUNT(*) as done, SUM(CASE WHEN a.is_correct = 1 THEN 1 ELSE 0 END) as correct
		FROM (`+latestAttemptsBySubjectSQL+`) a`, userID, subjectID).Scan(&row).Error
	return row, err
}

func (r *PracticeQueryRepositoryImpl) TrendBySubject(userID, subjectID uint64) ([]TrendRow, error) {
	var rows []TrendRow
	err := r.db.Raw(`
		SELECT DATE(pr.created_at) as day,
		       COUNT(*) as total,
		       SUM(CASE WHEN pr.is_correct = 1 THEN 1 ELSE 0 END) as correct
		FROM practice_records pr
		JOIN questions q ON q.id = pr.question_id
		WHERE pr.user_id = ? AND q.subject_id = ? AND pr.created_at >= DATE_SUB(CURDATE(), INTERVAL 29 DAY)
		GROUP BY DATE(pr.created_at)
		ORDER BY day ASC
	`, userID, subjectID).Scan(&rows).Error
	return rows, err
}

func (r *PracticeQueryRepositoryImpl) KnowledgeAccuracyBySubject(userID, subjectID uint64) ([]KpRow, error) {
	var rows []KpRow
	err := r.db.Raw(`
		SELECT a.knowledge_point,
		       COUNT(*) as done,
		       SUM(CASE WHEN a.is_correct = 1 THEN 1 ELSE 0 END) as correct
		FROM (`+latestAttemptsBySubjectSQL+`) a
		WHERE a.knowledge_point IS NOT NULL AND a.knowledge_point != ''
		GROUP BY a.knowledge_point
		ORDER BY done DESC
		LIMIT 20
	`, userID, subjectID).Scan(&rows).Error
	return rows, err
}

// speedWindowSQL 根据 sinceDays/untilDays 拼 WHERE 子句:
//   - 0/0        → 空字符串(全部)
//   - 7/0        → >= 7(近 7 天)
//   - 14/7       → >= 14 AND < 7(前 7 天)
//     其他组合 SQL 注入:非法值拒绝(>=0 && > sinceDays)。
func speedWindowSQL(sinceDays, untilDays uint64) (string, error) {
	switch {
	case sinceDays == 0 && untilDays == 0:
		return "", nil
	case sinceDays > 0 && untilDays == 0:
		return fmt.Sprintf("AND created_at >= DATE_SUB(NOW(), INTERVAL %d DAY)", sinceDays), nil
	case sinceDays > untilDays && untilDays > 0:
		return fmt.Sprintf("AND created_at >= DATE_SUB(NOW(), INTERVAL %d DAY) AND created_at < DATE_SUB(NOW(), INTERVAL %d DAY)", sinceDays, untilDays), nil
	default:
		return "", fmt.Errorf("invalid window: sinceDays=%d untilDays=%d", sinceDays, untilDays)
	}
}

func (r *PracticeQueryRepositoryImpl) SpeedBySubject(userID, subjectID, sinceDays, untilDays uint64) (SpeedRow, error) {
	where, err := speedWindowSQL(sinceDays, untilDays)
	if err != nil {
		return SpeedRow{}, err
	}
	var row SpeedRow
	err = r.db.Raw(`SELECT COUNT(*) as cnt, COALESCE(AVG(pr.time_spent), 0) as avg_sec
		FROM practice_records pr JOIN questions q ON q.id = pr.question_id
		WHERE pr.user_id = ? AND q.subject_id = ? AND pr.time_spent BETWEEN 1 AND 600 `+where,
		userID, subjectID).Scan(&row).Error
	return row, err
}

func (r *PracticeQueryRepositoryImpl) SpeedByUser(userID uint64, sinceDays, untilDays uint64) (SpeedRow, error) {
	where, err := speedWindowSQL(sinceDays, untilDays)
	if err != nil {
		return SpeedRow{}, err
	}
	var row SpeedRow
	err = r.db.Raw(`SELECT COUNT(*) as cnt, COALESCE(AVG(time_spent), 0) as avg_sec
		FROM practice_records
		WHERE user_id = ? AND time_spent BETWEEN 1 AND 600 `+where,
		userID).Scan(&row).Error
	return row, err
}
