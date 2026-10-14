package repository

// 诊断报告（会员专属页 /my/diagnosis）的聚合查询。
//
// 聚合口径统一按「题面自带的知识大类」—— SUBSTRING_INDEX(knowledge_point,'>',1)
// 取 level-1 主类。刻意不 join knowledge_points 表按 parent 分组：该表有过两代
// 父节点（migration 040 的 9 章节 vs 048 的 8 大类）且 match_key 模糊覆盖不全，
// 按它归组会漏归/错归。题目自带的层级字符串自包含、数据驱动，正是薄弱点画像
// 想要的题目侧分类。

// WeaknessRow 薄弱点画像一行：某大类最近的客观题作答。
type WeaknessRow struct {
	Category string `gorm:"column:category"`
	Done     int64  `gorm:"column:done"`
	Correct  int64  `gorm:"column:correct"`
	Wrong    int64  `gorm:"column:wrong"`
}

// CategoryCountRow 某科目客观题按大类计数一行。
type CategoryCountRow struct {
	Category string `gorm:"column:category"`
	Total    int64  `gorm:"column:total"`
}

// WrongBookStats 错题本存量（按科目）。
type WrongBookStats struct {
	Total int64 `gorm:"column:total"`
	Due   int64 `gorm:"column:due"`
}

// WrongSampleRow AI 诊断用的错题样例一行（最近答错的客观题）。
type WrongSampleRow struct {
	QuestionID     uint64 `gorm:"column:question_id"`
	Title          string `gorm:"column:title"`
	KnowledgePoint string `gorm:"column:knowledge_point"`
	Explanation    string `gorm:"column:explanation"`
}

// CategoryAccuracyBySubject 每大类最近一次作答的正确/错数。
// 只统计做过客观题的大类（没做过的由 CategoryQuestionCountsBySubject 补总量）。
func (r *PracticeQueryRepositoryImpl) CategoryAccuracyBySubject(userID, subjectID uint64) ([]WeaknessRow, error) {
	var rows []WeaknessRow
	err := r.db.Raw(`
		SELECT SUBSTRING_INDEX(a.knowledge_point, '>', 1) AS category,
		       COUNT(*) as done,
		       SUM(CASE WHEN a.is_correct = 1 THEN 1 ELSE 0 END) as correct,
		       SUM(CASE WHEN a.is_correct = 0 THEN 1 ELSE 0 END) as wrong
		FROM (`+latestAttemptsBySubjectSQL+`) a
		WHERE a.knowledge_point IS NOT NULL AND a.knowledge_point != ''
		GROUP BY category
		ORDER BY wrong DESC, done DESC
	`, userID, subjectID).Scan(&rows).Error
	return rows, err
}

// CategoryQuestionCountsBySubject 每大类客观题总量（含未做过的大类，用于「未开始」画像）。
func (r *PracticeQueryRepositoryImpl) CategoryQuestionCountsBySubject(subjectID uint64) ([]CategoryCountRow, error) {
	var rows []CategoryCountRow
	err := r.db.Raw(`
		SELECT SUBSTRING_INDEX(q.knowledge_point, '>', 1) AS category,
		       COUNT(DISTINCT q.id) as total
		FROM questions q
		WHERE q.subject_id = ? AND q.status = 1 AND q.question_type IN (?)
		  AND q.knowledge_point IS NOT NULL AND q.knowledge_point != ''
		GROUP BY category
	`, subjectID, objectiveQuestionTypes()).Scan(&rows).Error
	return rows, err
}

// WrongBookStatsBySubject 错题本存量：总数 + 到期待复习数（口径对齐 FindDueReview）。
// 按科目过滤：诊断页是按当前科目出的画像。
func (r *PracticeQueryRepositoryImpl) WrongBookStatsBySubject(userID, subjectID uint64) (WrongBookStats, error) {
	var row WrongBookStats
	err := r.db.Raw(`
		SELECT COUNT(*) as total,
		       COALESCE(SUM(CASE WHEN wq.next_review_time IS NOT NULL AND wq.next_review_time <= NOW() THEN 1 ELSE 0 END), 0) as due
		FROM wrong_questions wq
		JOIN questions q ON q.id = wq.question_id
		WHERE wq.user_id = ? AND wq.status = 1 AND q.subject_id = ? AND q.status = 1
	`, userID, subjectID).Scan(&row).Error
	return row, err
}

// RecentWrongSamplesBySubject 最近答错的客观题样例，供 AI 诊断参考。
// limit 由 service 传入（一般 5 条）；标题/解析太长由 service 截断。
func (r *PracticeQueryRepositoryImpl) RecentWrongSamplesBySubject(userID, subjectID uint64, limit int) ([]WrongSampleRow, error) {
	var rows []WrongSampleRow
	err := r.db.Raw(`
		SELECT a.question_id, q.title, q.knowledge_point, q.explanation
		FROM (`+latestAttemptsBySubjectSQL+`) a
		JOIN questions q ON q.id = a.question_id
		WHERE a.is_correct = 0 AND a.knowledge_point IS NOT NULL AND a.knowledge_point != ''
		ORDER BY a.id DESC
		LIMIT ?
	`, userID, subjectID, limit).Scan(&rows).Error
	return rows, err
}
