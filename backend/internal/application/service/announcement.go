package service

import (
	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

type AnnouncementService struct {
	repo repository.AnnouncementRepository
}

func NewAnnouncementService(repo repository.AnnouncementRepository) *AnnouncementService {
	return &AnnouncementService{repo: repo}
}

func (s *AnnouncementService) ListAll(page, pageSize int) ([]model.Announcement, int64, error) {
	return s.repo.ListAll(page, pageSize)
}

func (s *AnnouncementService) ListUnread(userID uint64) ([]model.Announcement, error) {
	return s.repo.ListUnread(userID)
}

func (s *AnnouncementService) MarkRead(userID, announcementID uint64) error {
	return s.repo.MarkRead(userID, announcementID)
}

func (s *AnnouncementService) Create(title, content string, createdBy uint64) (uint64, error) {
	a := &model.Announcement{
		Title:     title,
		Content:   content,
		CreatedBy: createdBy,
		Status:    1,
	}
	if err := s.repo.Create(a); err != nil {
		return 0, err
	}
	return a.ID, nil
}

func (s *AnnouncementService) Update(id uint64, title, content string, status int8) error {
	a, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if title != "" {
		a.Title = title
	}
	if content != "" {
		a.Content = content
	}
	a.Status = status
	return s.repo.Update(a)
}

func (s *AnnouncementService) Delete(id uint64) error {
	return s.repo.Delete(id)
}
