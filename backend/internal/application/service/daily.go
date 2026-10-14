package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

type DailyService struct {
	checkinRepo      repository.DailyCheckinRepository
	questionRepo     repository.QuestionRepository
	profileRepo      repository.UserProfileRepository
	dailySessionRepo repository.DailySessionRepository
}

func NewDailyService(
	checkinRepo repository.DailyCheckinRepository,
	questionRepo repository.QuestionRepository,
	profileRepo repository.UserProfileRepository,
	dailySessionRepo repository.DailySessionRepository,
) *DailyService {
	return &DailyService{
		checkinRepo:      checkinRepo,
		questionRepo:     questionRepo,
		profileRepo:      profileRepo,
		dailySessionRepo: dailySessionRepo,
	}
}

// DailyQuestionCount 每日一练题量
const DailyQuestionCount = 15

// DailySessionData 每日一练断点草稿的对外形态。
type DailySessionData struct {
	CurrentIndex int                    `json:"current_index"`
	CurrentBlank int                    `json:"current_blank"`
	Answers      map[string]string      `json:"answers"`
	ShowResult   bool                   `json:"show_result"`
	Result       interface{}            `json:"result"`
	CorrectCount int                    `json:"correct_count"`
}

type DailyTodayResult struct {
	Date          string           `json:"date"`
	CheckedIn     bool             `json:"checked_in"`
	QuestionCount int              `json:"question_count"`
	CorrectCount  int              `json:"correct_count"`
	StreakDays    int              `json:"streak_days"`
	Questions     []model.Question `json:"questions"`
	Calendar      []string         `json:"calendar"` // 本月已签到的日期列表 YYYY-MM-DD
	Session       *DailySessionData `json:"session"` // 有会话时非 null，无会话时 null
}

func localToday() string {
	return time.Now().Format("2006-01-02")
}

// dailySeed 由「日期+用户+科目」派生每日一练的固定随机种子：
// 同一天同一用户同一科目必然抽到同一组题（换设备/刷新都不变），
// 次日 / 换账号 / 换科目自动换一批。
func dailySeed(date string, userID, subjectID uint64) uint32 {
	h := fnv.New32a()
	fmt.Fprintf(h, "%s:%d:%d", date, userID, subjectID)
	return h.Sum32()
}

// GetToday 返回今日一练：题目、是否已签到、连续天数、本月签到日历、当日进度。
func (s *DailyService) GetToday(userID, subjectID uint64) (*DailyTodayResult, error) {
	today := localToday()

	res := &DailyTodayResult{Date: today}

	// 今日签到状态
	if c, err := s.checkinRepo.FindByUserAndDate(userID, today); err == nil && c != nil {
		res.CheckedIn = true
		res.QuestionCount = c.QuestionCount
		res.CorrectCount = c.CorrectCount
	}

	// 连续天数（复用 profile.StreakDays）
	if p, err := s.profileRepo.FindByUserID(userID); err == nil && p != nil {
		res.StreakDays = p.StreakDays
	}

	// 本月签到日历
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).Format("2006-01-02")
	monthEnd := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.Local).Format("2006-01-02")
	if list, err := s.checkinRepo.ListByUserInRange(userID, monthStart, monthEnd); err == nil {
		for _, c := range list {
			res.Calendar = append(res.Calendar, c.CheckinDate)
		}
	}

	// 今日题目：选择题(question_type=1)按「日期+用户+科目」固定种子抽 15 题。
	// 种子确定性让同一天同一账号在任何设备拿到的都是同一组题；
	// 未签到才返回题目；已签到则前端展示"今日已完成"。
	if !res.CheckedIn {
		seed := dailySeed(today, userID, subjectID)
		questions, err := s.questionRepo.GetSeededRandomQuestions(1, DailyQuestionCount, "", subjectID, seed)
		if err != nil {
			return nil, err
		}
		res.Questions = questions

		// 读当日会话（跨设备进度同步）
		if sess, err := s.dailySessionRepo.FindOne(userID, today, subjectID); err == nil && sess != nil {
			var data DailySessionData
			if json.Unmarshal([]byte(sess.Data), &data) == nil {
				res.Session = &data
			}
		}
	}

	return res, nil
}

// SaveSession 保存每日一练进度草稿。date=localToday()，upsert 落库，可覆盖。
func (s *DailyService) SaveSession(userID, subjectID uint64, payload *DailySessionData) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.dailySessionRepo.Save(&model.DailySession{
		UserID:    userID,
		Date:      localToday(),
		SubjectID: subjectID,
		Data:      string(data),
	})
}

// Checkin 完成今日一练后签到，记录本次答题/正确数。幂等：重复调用只更新计数。
// 签到成功后删当日该科目会话，避免前端多发一次 DELETE。
func (s *DailyService) Checkin(userID, subjectID uint64, questionCount, correctCount int) error {
	if questionCount <= 0 {
		return errors.New("请先完成今日一练")
	}
	today := localToday()
	if err := s.checkinRepo.Upsert(&model.DailyCheckin{
		UserID:        userID,
		CheckinDate:   today,
		QuestionCount: questionCount,
		CorrectCount:  correctCount,
	}); err != nil {
		return err
	}
	// 签到成功后删当日该科目会话
	_ = s.dailySessionRepo.Delete(userID, today, subjectID)
	return nil
}