package service

import (
	"log"
	"time"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

// 通知业务类型，对应 notifications.biz_type
const (
	BizReport        = "report"
	BizFeedback      = "feedback"
	BizEssay         = "essay"
	BizComment       = "comment"
	BizCommentReport = "comment_report"
)

type NotificationService struct {
	repo repository.NotificationRepository
}

func NewNotificationService(repo repository.NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

// Notify 发一条站内信。错误一律吞掉只打日志 —— 通知是旁路，
// 管理员点了「已处理」不能因为通知写失败就让整个操作报错回滚。
func (s *NotificationService) Notify(userID uint64, title, content, bizType string, bizID uint64, link string) {
	if s == nil || s.repo == nil || userID == 0 {
		return
	}
	n := &model.Notification{
		UserID:    userID,
		Type:      1,
		Title:     title,
		Content:   content,
		BizType:   bizType,
		BizID:     bizID,
		Link:      link,
		CreatedAt: time.Now(),
	}
	if err := s.repo.Create(n); err != nil {
		log.Printf("notify: create failed user=%d biz=%s/%d: %v", userID, bizType, bizID, err)
	}
}

func (s *NotificationService) MyList(userID uint64, page, pageSize int, onlyUnread bool) ([]model.Notification, int64, error) {
	return s.repo.ListByUser(userID, page, pageSize, onlyUnread)
}

func (s *NotificationService) UnreadCount(userID uint64) (int64, error) {
	return s.repo.CountUnread(userID)
}

func (s *NotificationService) MarkRead(id, userID uint64) error {
	return s.repo.MarkRead(id, userID)
}

func (s *NotificationService) MarkAllRead(userID uint64) error {
	return s.repo.MarkAllRead(userID)
}
