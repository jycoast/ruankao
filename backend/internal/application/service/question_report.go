package service

import (
	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

type QuestionReportService struct {
	repo     repository.QuestionReportRepository
	notifSvc *NotificationService
}

func NewQuestionReportService(repo repository.QuestionReportRepository, notifSvc *NotificationService) *QuestionReportService {
	return &QuestionReportService{repo: repo, notifSvc: notifSvc}
}

func (s *QuestionReportService) Submit(questionID, userID uint64, reportType int8, description string) error {
	r := &model.QuestionReport{
		QuestionID:  questionID,
		UserID:      userID,
		ReportType:  reportType,
		Description: description,
		Status:      0,
	}
	return s.repo.Create(r)
}

func (s *QuestionReportService) ListAll(page, pageSize, status int) ([]repository.QuestionReportView, int64, error) {
	return s.repo.ListAll(page, pageSize, status)
}

func (s *QuestionReportService) MyReports(userID uint64, page, pageSize int) ([]repository.QuestionReportView, int64, error) {
	return s.repo.ListByUserID(userID, page, pageSize)
}

func (s *QuestionReportService) UpdateStatus(id uint64, status int8, adminNote string) error {
	// 先读原记录：一是要拿 user_id 才能发通知，二是状态没变就不该重复打扰用户
	// —— 管理员改个备注再保存一次，用户不该收到第二条「已处理」。
	before, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateStatus(id, status, adminNote); err != nil {
		return err
	}
	if before.Status != status && status != 0 {
		title, content := reportNotifyText(status, adminNote)
		s.notifSvc.Notify(before.UserID, title, content, BizReport, id, "/my/reports")
	}
	return nil
}

func reportNotifyText(status int8, adminNote string) (string, string) {
	title := "你反馈的题目问题已修正"
	content := "感谢你的纠错，题目内容已更新。"
	if status == 2 {
		title = "你反馈的题目经核对无误"
		content = "感谢你的关注，我们核对后确认该题目没有问题。"
	}
	if adminNote != "" {
		content += "\n\n管理员回复：" + adminNote
	}
	return title, content
}

func (s *QuestionReportService) CountPending() (int64, error) {
	return s.repo.CountPending()
}
