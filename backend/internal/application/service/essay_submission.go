package service

import (
	"strconv"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

type EssaySubmissionService struct {
	repo        repository.EssaySubmissionRepository
	articleRepo repository.ArticleRepository
	notifSvc    *NotificationService
}

func NewEssaySubmissionService(repo repository.EssaySubmissionRepository, articleRepo repository.ArticleRepository, notifSvc *NotificationService) *EssaySubmissionService {
	return &EssaySubmissionService{repo: repo, articleRepo: articleRepo, notifSvc: notifSvc}
}

func (s *EssaySubmissionService) Submit(userID uint64, title, content, category string, year int16, score float32) (uint64, error) {
	sub := &model.EssaySubmission{
		UserID:   userID,
		Title:    title,
		Content:  content,
		Category: category,
		Year:     year,
		Score:    score,
	}
	if err := s.repo.Create(sub); err != nil {
		return 0, err
	}
	return sub.ID, nil
}

func (s *EssaySubmissionService) MySubmissions(userID uint64, page, pageSize int) ([]model.EssaySubmission, int64, error) {
	return s.repo.ListByUserID(userID, page, pageSize)
}

func (s *EssaySubmissionService) ListAll(page, pageSize, status int) ([]repository.EssaySubmissionView, int64, error) {
	return s.repo.ListAll(page, pageSize, status)
}

func (s *EssaySubmissionService) GetByID(id uint64) (*model.EssaySubmission, error) {
	return s.repo.FindByID(id)
}

// Approve 审核通过：写入 articles 表成为范文，更新投稿状态
func (s *EssaySubmissionService) Approve(id uint64, note string, articleType int8) error {
	sub, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	if articleType == 0 {
		articleType = 1 // 默认：论文范文
	}
	wasPending := sub.ReviewStatus == 0

	article := &model.Article{
		Title:       sub.Title,
		Content:     sub.Content,
		ArticleType: articleType,
		Category:    sub.Category,
		Year:        sub.Year,
		Score:       sub.Score,
		Status:      1,
	}
	if err := s.articleRepo.Create(article); err != nil {
		return err
	}

	sub.ReviewStatus = 1
	sub.ReviewerNote = note
	sub.ArticleID = article.ID
	if err := s.repo.Update(sub); err != nil {
		return err
	}
	if wasPending {
		content := "你投稿的《" + sub.Title + "》已通过审核，现已发布为范文。"
		if note != "" {
			content += "\n\n审核意见：" + note
		}
		s.notifSvc.Notify(sub.UserID, "投稿已采纳", content, BizEssay, sub.ID, "/essays/"+strconv.FormatUint(article.ID, 10))
	}
	return nil
}

// Reject 审核拒绝
func (s *EssaySubmissionService) Reject(id uint64, note string) error {
	sub, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	wasPending := sub.ReviewStatus == 0
	sub.ReviewStatus = 2
	sub.ReviewerNote = note
	if err := s.repo.Update(sub); err != nil {
		return err
	}
	if wasPending {
		content := "你投稿的《" + sub.Title + "》未通过审核。"
		if note != "" {
			content += "\n\n审核意见：" + note
		}
		s.notifSvc.Notify(sub.UserID, "投稿未通过", content, BizEssay, sub.ID, "/essays/my-submissions")
	}
	return nil
}

func (s *EssaySubmissionService) CountPending() (int64, error) {
	return s.repo.CountPending()
}
