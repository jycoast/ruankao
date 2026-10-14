package service

import (
	"math"
	"time"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

type PracticeService struct {
	questionRepo      repository.QuestionRepository
	wrongQuestionRepo repository.WrongQuestionRepository
	practiceRepo      repository.PracticeRecordRepository
	profileRepo       repository.UserProfileRepository
	subjectStatRepo   repository.UserSubjectStatRepository
	sessionRepo       repository.PracticeSessionRepository
	practiceQueryRepo repository.PracticeQueryRepository
}

func NewPracticeService(
	questionRepo repository.QuestionRepository,
	wrongQuestionRepo repository.WrongQuestionRepository,
	practiceRepo repository.PracticeRecordRepository,
	profileRepo repository.UserProfileRepository,
	subjectStatRepo repository.UserSubjectStatRepository,
	sessionRepo repository.PracticeSessionRepository,
	practiceQueryRepo repository.PracticeQueryRepository,
) *PracticeService {
	return &PracticeService{
		questionRepo:      questionRepo,
		wrongQuestionRepo: wrongQuestionRepo,
		practiceRepo:      practiceRepo,
		profileRepo:       profileRepo,
		subjectStatRepo:   subjectStatRepo,
		sessionRepo:       sessionRepo,
		practiceQueryRepo: practiceQueryRepo,
	}
}

type AnswerResult struct {
	IsCorrect   bool   `json:"is_correct"`
	Objective   bool   `json:"objective"` // false 表示主观题，前端应显示"参考答案"而非对错
	Answer      string `json:"answer"`
	Explanation string `json:"explanation"`
	Analysis    string `json:"analysis"`
}

func isObjective(qType int8) bool {
	// 只统计客观题(单选 1、多选 2)。案例/论文(3/4)系统判不了对错,
	// 一律不判分、不进错题本。与 repository 内 objectiveQuestionTypes 同源。
	return qType == 1 || qType == 2
}

func (s *PracticeService) SubmitAnswer(userID, questionID uint64, userAnswer string, practiceType int8, timeSpent int) (*AnswerResult, error) {
	question, err := s.questionRepo.FindByID(questionID)
	if err != nil {
		return nil, err
	}

	correctAnswer := buildCorrectAnswer(question.Options)

	// 案例/论文没有标准选项，字符串比对必然不等。
	// 原实现把它们一律判错，既拉低正确率又把每道大题塞进错题本。
	objective := isObjective(question.QuestionType)
	isCorrect := objective && userAnswer == correctAnswer

	record := &model.PracticeRecord{
		UserID:       userID,
		QuestionID:   questionID,
		PracticeType: practiceType,
		UserAnswer:   userAnswer,
		IsCorrect:    boolToInt8(isCorrect),
		TimeSpent:    timeSpent,
	}
	s.practiceRepo.Create(record)

	s.updateProfileStats(userID, isCorrect, timeSpent)
	s.updateSubjectStats(userID, question.SubjectID, isCorrect, timeSpent)

	// 主观题不进错题本、不参与间隔重复：系统判不了对错，
	// 排进复习队列只会让错题本被大题淹没。
	if objective {
		if isCorrect {
			s.handleCorrectAnswer(userID, questionID)
		} else {
			s.recordWrongQuestion(userID, questionID, userAnswer)
		}
	}

	return &AnswerResult{
		IsCorrect:   isCorrect,
		Objective:   objective,
		Answer:      correctAnswer,
		Explanation: question.Explanation,
		Analysis:    question.Analysis,
	}, nil
}

// RecordAnswer 落一条作答到学习流水/错题本（不判分，isCorrect 由调用方算好）。
// 考试交卷逐题调用：让考试模式的做题进学习报告、错题进错题本，与练习模式看齐。
// timeSpent 考试场景无法逐题拆分，传 0；做题速度统计（1..600 秒过滤）不会受污染。
func (s *PracticeService) RecordAnswer(userID, questionID, subjectID, paperID uint64, userAnswer string, isCorrect bool, timeSpent int, practiceType int8) {
	if s.practiceRepo == nil {
		return
	}
	s.practiceRepo.Create(&model.PracticeRecord{
		UserID:       userID,
		QuestionID:   questionID,
		PracticeType: practiceType,
		UserAnswer:   userAnswer,
		IsCorrect:    boolToInt8(isCorrect),
		TimeSpent:    timeSpent,
		PaperID:      paperID,
	})
	s.updateProfileStats(userID, isCorrect, timeSpent)
	s.updateSubjectStats(userID, subjectID, isCorrect, timeSpent)
	if isCorrect {
		s.handleCorrectAnswer(userID, questionID)
	} else {
		s.recordWrongQuestion(userID, questionID, userAnswer)
	}
}

// ---------------- 间隔重复（SM-2 简化版） ----------------

const (
	minEaseFactor        = 1.3
	maxEaseFactor        = 2.8
	defaultEaseFactor    = 2.5
	maxIntervalDays      = 30
	graduateCorrectCount = 3 // 连续答对多少次算掌握
)

// nextInterval 计算下一次复习间隔（天）。
//
// 原实现是 int(float32(cur) * ef) 直接截断：EF 衰减到下限 1.3 后
// 1*1.3=1.3 被截回 1，间隔永远卡在 1 天，错题天天重复出现。
// 改为四舍五入并强制至少增长 1 天，保证间隔单调递增。
func nextInterval(cur int, ef float32) int {
	if cur < 1 {
		return 1
	}
	if cur == 1 {
		return 6 // SM-2 规定第二次复习固定 6 天
	}
	next := int(math.Round(float64(cur) * float64(ef)))
	if next <= cur {
		next = cur + 1
	}
	if next > maxIntervalDays {
		next = maxIntervalDays
	}
	return next
}

// adjustEase 调整难度系数。
// 原实现只有答错时的 *= 0.8，EF 单向衰减到下限后再也回不来，
// 用户后来把题做熟了系统也感知不到。答对时须回调。
func adjustEase(ef float32, correct bool) float32 {
	if ef <= 0 {
		ef = defaultEaseFactor
	}
	if correct {
		ef += 0.1
	} else {
		ef -= 0.2
	}
	if ef < minEaseFactor {
		ef = minEaseFactor
	}
	if ef > maxEaseFactor {
		ef = maxEaseFactor
	}
	return ef
}

// applyReviewResult 按一次作答结果更新错题的复习计划。
// 答错一律重置为次日复习：间隔重复的前提是「记不住就从头来」。
func applyReviewResult(wq *model.WrongQuestion, correct bool) {
	wq.EaseFactor = adjustEase(wq.EaseFactor, correct)
	if correct {
		wq.CorrectCount++
		if wq.CorrectCount >= graduateCorrectCount {
			wq.Status = 2 // 已掌握，移出复习队列
			return
		}
		wq.IntervalDays = nextInterval(wq.IntervalDays, wq.EaseFactor)
	} else {
		wq.WrongCount++
		wq.CorrectCount = 0
		wq.IntervalDays = 1
		wq.Status = 1
	}
	next := time.Now().Add(time.Duration(wq.IntervalDays) * 24 * time.Hour)
	wq.NextReviewTime = &next
}

func (s *PracticeService) recordWrongQuestion(userID, questionID uint64, answer string) {
	wq, err := s.wrongQuestionRepo.FindByUserAndQuestion(userID, questionID)
	if err != nil {
		nextReview := time.Now().Add(24 * time.Hour)
		wq = &model.WrongQuestion{
			UserID:          userID,
			QuestionID:      questionID,
			WrongCount:      1,
			LastWrongAnswer: answer,
			NextReviewTime:  &nextReview,
			IntervalDays:    1,
			EaseFactor:      defaultEaseFactor,
			Status:          1,
		}
		s.wrongQuestionRepo.Create(wq)
		return
	}
	wq.LastWrongAnswer = answer
	applyReviewResult(wq, false)
	s.wrongQuestionRepo.Update(wq)
}

func (s *PracticeService) handleCorrectAnswer(userID, questionID uint64) {
	wq, err := s.wrongQuestionRepo.FindByUserAndQuestion(userID, questionID)
	if err != nil {
		return
	}
	applyReviewResult(wq, true)
	s.wrongQuestionRepo.Update(wq)
}

func (s *PracticeService) GetWrongQuestions(userID uint64, status int8, page, pageSize int) ([]model.WrongQuestion, int64, error) {
	return s.wrongQuestionRepo.FindByUserID(userID, status, page, pageSize)
}

func (s *PracticeService) DeleteWrongQuestion(userID, questionID uint64) error {
	return s.wrongQuestionRepo.Delete(userID, questionID)
}

func (s *PracticeService) GetDueReview(userID uint64, page, pageSize int) ([]model.WrongQuestion, int64, error) {
	return s.wrongQuestionRepo.FindDueReview(userID, page, pageSize)
}

func (s *PracticeService) Review(userID, questionID uint64, isCorrect bool) error {
	wq, err := s.wrongQuestionRepo.FindByUserAndQuestion(userID, questionID)
	if err != nil {
		return err
	}
	applyReviewResult(wq, isCorrect)
	return s.wrongQuestionRepo.Update(wq)
}

func (s *PracticeService) GetHistory(userID uint64, page, pageSize int) ([]model.PracticeRecord, int64, error) {
	return s.practiceRepo.FindByUserID(userID, 0, page, pageSize)
}

// ---------------- 统计口径 ----------------
//
// 全站统一按两条规则计算正确率，避免各页面各算各的：
//  1. 一道题只按**最近一次**作答计入，反复刷同一题不会把正确率刷虚高，
//     也保证 correct ≤ done 恒成立（原实现 done 去重、correct 不去重，
//     进度条能超过 100%）。
//  2. 只统计客观题。案例/论文（question_type 3/4）系统判不了对错，
//     计入分母只会凭空拉低用户的正确率（见 SubmitAnswer 的 objective 分支）。
//
// SQL 与题型常量(单选 1、多选 2)已下沉到 repository.PracticeQueryRepository,
// service 只做结果转换与「去重 / 只统计客观题」的编排,不再接触原始 SQL。
//
// AccuracyStat 按统一口径算出的正确率，供首页与学习报告共用。
type AccuracyStat struct {
	Done     int `json:"distinct_done"`    // 做过的客观题数（去重）
	Correct  int `json:"distinct_correct"` // 其中最近一次答对的数量
	Accuracy int `json:"accuracy"`         // 百分比
}

// GetAccuracy 口径见 PracticeQueryRepository.AccuracyByUser 的 SQL。
// 首页与学习报告都走这里,避免出现「首页 82% / 报告 67%」这种
// 同一用户看到两个数的情况。零除保护在 service(Done==0 时 Accuracy=0)。
func (s *PracticeService) GetAccuracy(userID uint64) AccuracyStat {
	acc, _ := s.practiceQueryRepo.AccuracyByUser(userID)
	stat := AccuracyStat{Done: int(acc.Done), Correct: int(acc.Correct)}
	if acc.Done > 0 {
		stat.Accuracy = int(float64(acc.Correct) / float64(acc.Done) * 100.0)
	}
	return stat
}

// CategoryProgressItem 分类刷题进度一行：知识点题量/已做/答对。
type CategoryProgressItem struct {
	KnowledgePoint string `json:"knowledge_point"`
	Total          int64  `json:"total"`
	Done           int64  `json:"done"`
	Correct        int64  `json:"correct"`
}

func (s *PracticeService) GetCategoryProgress(userID uint64) ([]CategoryProgressItem, error) {
	rows, err := s.practiceQueryRepo.CategoryProgress(userID)
	if err != nil {
		return nil, err
	}

	result := make([]CategoryProgressItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, CategoryProgressItem{
			KnowledgePoint: row.KnowledgePoint,
			Total:          row.Total,
			Done:           row.Done,
			Correct:        row.Correct,
		})
	}
	return result, nil
}

// TrendItem 学习趋势一行：近30日每日 {date, total, correct}。
type TrendItem struct {
	Date    string `json:"date"`
	Total   int64  `json:"total"`
	Correct int64  `json:"correct"`
}

// KnowledgeItem 知识点正确率排行一行 {knowledge_point, done, correct, accuracy}。
type KnowledgeItem struct {
	KnowledgePoint string `json:"knowledge_point"`
	Done           int64  `json:"done"`
	Correct        int64  `json:"correct"`
	Accuracy       int    `json:"accuracy"`
}

// StudyReport 学习数据报告聚合结果。
type StudyReport struct {
	TotalPractice   int             `json:"total_practice"`
	TotalCorrect    int             `json:"total_correct"`
	TotalTime       int             `json:"total_time"`
	StreakDays      int             `json:"streak_days"`
	DistinctDone    int             `json:"distinct_done"`    // 做过的客观题数（去重）
	DistinctCorrect int             `json:"distinct_correct"` // 其中最近一次答对的数量
	Accuracy        int             `json:"accuracy"`         // 正确率 %，口径见 latestAttemptsSQL
	Trend           []TrendItem     `json:"trend"`            // 近30日每日 {date, total, correct}
	Knowledge       []KnowledgeItem `json:"knowledge"`        // 知识点正确率 {knowledge_point, done, correct, accuracy}
	Speed           *SpeedStat      `json:"speed"`            // 做题速度，样本不足时为 null
}

// SpeedStat 做题速度：累计平均与本周/上周对比。
type SpeedStat struct {
	AvgSeconds    int  `json:"avg_seconds"`     // 累计平均每题秒数
	ThisWeekAvg   int  `json:"this_week_avg"`   // 近7天平均每题秒数
	LastWeekAvg   int  `json:"last_week_avg"`   // 前7天平均每题秒数
	ThisWeekCount int  `json:"this_week_count"` // 近7天题量
	FasterPercent int  `json:"faster_percent"`  // 比上周快的百分比，负数表示变慢
	HasComparison bool `json:"has_comparison"`  // 上周样本足够才可比
}

// GetReport 生成学习数据报告：累计概览 + 近30日趋势 + 知识点正确率排行。
// GetReport 学习数据报告（按当前科目过滤）。
// 累计做题/时长/连续天数来自 user_subject_stats（按科目累加）；
// 趋势/知识点正确率/做题速度基于 practice_records join questions.subject_id 现算。
func (s *PracticeService) GetReport(userID, subjectID uint64) (*StudyReport, error) {
	report := &StudyReport{}

	// 累计概览：按科目累加。科目行不存在则全 0（该科目尚未练习）。
	if stat, err := s.subjectStatRepo.FindOne(userID, subjectID); err == nil && stat != nil {
		report.TotalPractice = stat.TotalPractice
		report.TotalCorrect = stat.TotalCorrect
		report.TotalTime = stat.TotalTime
		report.StreakDays = stat.StreakDays
	}
	acc := s.getAccuracyBySubject(userID, subjectID)
	report.DistinctDone = acc.Done
	report.DistinctCorrect = acc.Correct
	report.Accuracy = acc.Accuracy

	// 近30日每日答题量 + 正确数（仅当前科目）
	trendRows, err := s.practiceQueryRepo.TrendBySubject(userID, subjectID)
	if err != nil {
		return nil, err
	}
	report.Trend = []TrendItem{}
	for _, r := range trendRows {
		report.Trend = append(report.Trend, TrendItem{
			Date:    r.Day,
			Total:   r.Total,
			Correct: r.Correct,
		})
	}

	// 知识点正确率排行（每题只算最近一次，仅当前科目）
	kpRows, err := s.practiceQueryRepo.KnowledgeAccuracyBySubject(userID, subjectID)
	if err != nil {
		return nil, err
	}
	report.Knowledge = []KnowledgeItem{}
	for _, r := range kpRows {
		kacc := 0
		if r.Done > 0 {
			kacc = int(float64(r.Correct) / float64(r.Done) * 100.0)
		}
		report.Knowledge = append(report.Knowledge, KnowledgeItem{
			KnowledgePoint: r.KnowledgePoint,
			Done:           r.Done,
			Correct:        r.Correct,
			Accuracy:       kacc,
		})
	}

	report.Speed = s.buildSpeedStatBySubject(userID, subjectID)

	return report, nil
}

// latestAttemptsBySubjectSQL 与 latestAttemptsSQL 同构,但额外按科目过滤。
// 已迁到 repository.PracticeQueryRepository 内,service 不再接触。

// getAccuracyBySubject 按科目算正确率：每题只算最近一次客观题尝试。
func (s *PracticeService) getAccuracyBySubject(userID, subjectID uint64) AccuracyStat {
	acc, _ := s.practiceQueryRepo.AccuracyBySubject(userID, subjectID)
	stat := AccuracyStat{Done: int(acc.Done), Correct: int(acc.Correct)}
	if acc.Done > 0 {
		stat.Accuracy = int(float64(acc.Correct) / float64(acc.Done) * 100.0)
	}
	return stat
}

// buildSpeedStatBySubject 做题速度（按科目）。
func (s *PracticeService) buildSpeedStatBySubject(userID, subjectID uint64) *SpeedStat {
	// 窗口语义:(0,0)=全部,(7,0)=近 7 天,(14,7)=前 7 天
	all := mustSpeed(s.practiceQueryRepo.SpeedBySubject(userID, subjectID, 0, 0))
	if all.Cnt < 5 {
		return nil
	}
	thisWeek := mustSpeed(s.practiceQueryRepo.SpeedBySubject(userID, subjectID, 7, 0))
	lastWeek := mustSpeed(s.practiceQueryRepo.SpeedBySubject(userID, subjectID, 14, 7))

	stat := &SpeedStat{
		AvgSeconds:    int(all.AvgSec + 0.5),
		ThisWeekAvg:   int(thisWeek.AvgSec + 0.5),
		LastWeekAvg:   int(lastWeek.AvgSec + 0.5),
		ThisWeekCount: int(thisWeek.Cnt),
	}
	if thisWeek.Cnt >= 5 && lastWeek.Cnt >= 5 && lastWeek.AvgSec > 0 {
		stat.HasComparison = true
		stat.FasterPercent = int((lastWeek.AvgSec - thisWeek.AvgSec) / lastWeek.AvgSec * 100.0)
	}
	return stat
}

// mustSpeed 吞掉窗口参数错误：窗口由本文件硬编码调用(0/0、7/0、14/7)，
// 不可能非法；真出错时返回空行，buildSpeedStatBySubject 会按 Cnt<5 的路径
// 当"无样本"处理。
func mustSpeed(r repository.SpeedRow, err error) repository.SpeedRow {
	if err != nil {
		return repository.SpeedRow{}
	}
	return r
}

func (s *PracticeService) updateProfileStats(userID uint64, isCorrect bool, timeSpent int) {
	profile, err := s.profileRepo.FindByUserID(userID)
	if err != nil {
		profile = &model.UserProfile{
			UserID:        userID,
			TotalPractice: 0,
			TotalCorrect:  0,
			TotalTime:     0,
			StreakDays:    0,
		}
		if createErr := s.profileRepo.Create(profile); createErr != nil {
			profile, _ = s.profileRepo.FindByUserID(userID)
			if profile == nil {
				return
			}
		}
	}
	profile.TotalPractice++
	if isCorrect {
		profile.TotalCorrect++
	}
	profile.TotalTime += timeSpent
	now := time.Now()
	if profile.LastPracticeAt != nil {
		last := *profile.LastPracticeAt
		lastDay := time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, time.Local)
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
		daysDiff := int(today.Sub(lastDay).Hours() / 24)
		if daysDiff == 1 {
			profile.StreakDays++
		} else if daysDiff > 1 {
			profile.StreakDays = 1
		}
	} else {
		profile.StreakDays = 1
	}
	profile.LastPracticeAt = &now
	s.profileRepo.Update(profile)
}

// updateSubjectStats 按科目维度累加：每个 (user, subject) 一行，读不到则建。
// streak_days 从最近一次该科目练习日起算。
func (s *PracticeService) updateSubjectStats(userID, subjectID uint64, isCorrect bool, timeSpent int) {
	if s.subjectStatRepo == nil || subjectID == 0 {
		return
	}
	stat, err := s.subjectStatRepo.FindOne(userID, subjectID)
	if err != nil {
		stat = &model.UserSubjectStat{
			UserID:        userID,
			SubjectID:     subjectID,
		}
		if e := s.subjectStatRepo.Create(stat); e != nil {
			stat, _ = s.subjectStatRepo.FindOne(userID, subjectID)
			if stat == nil {
				return
			}
		}
	}
	stat.TotalPractice++
	if isCorrect {
		stat.TotalCorrect++
	}
	stat.TotalTime += timeSpent
	now := time.Now()
	if stat.LastPracticeAt != nil {
		last := *stat.LastPracticeAt
		lastDay := time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, time.Local)
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
		daysDiff := int(today.Sub(lastDay).Hours() / 24)
		if daysDiff == 1 {
			stat.StreakDays++
		} else if daysDiff > 1 {
			stat.StreakDays = 1
		}
	} else {
		stat.StreakDays = 1
	}
	stat.LastPracticeAt = &now
	_ = s.subjectStatRepo.Save(stat)
}

func boolToInt8(b bool) int8 {
	if b {
		return 1
	}
	return 0
}
