package service

import (
	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

type ArticleService struct {
	articleRepo repository.ArticleRepository
}

func NewArticleService(articleRepo repository.ArticleRepository) *ArticleService {
	return &ArticleService{articleRepo: articleRepo}
}

func (s *ArticleService) List(req repository.ArticleListReq) ([]model.Article, int64, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}
	return s.articleRepo.List(req)
}

func (s *ArticleService) GetByID(id uint64) (*model.Article, error) {
	return s.articleRepo.FindByID(id)
}

func (s *ArticleService) Create(title, content string, articleType int8, category string, year int16, score float32, author, tags string, subjectID uint64) (uint64, error) {
	if subjectID == 0 {
		subjectID = 1
	}
	article := &model.Article{
		Title:       title,
		Content:     content,
		ArticleType: articleType,
		Category:    category,
		Year:        year,
		Score:       score,
		Author:      author,
		Tags:        tags,
		SubjectID:   subjectID,
		Status:      1,
	}
	if err := s.articleRepo.Create(article); err != nil {
		return 0, err
	}
	return article.ID, nil
}

func (s *ArticleService) Update(id uint64, title, content string, articleType int8, category string, year int16, score float32, author, tags string, subjectID uint64) error {
	article, err := s.articleRepo.FindByID(id)
	if err != nil {
		return err
	}
	if subjectID > 0 {
		article.SubjectID = subjectID
	}
	if title != "" {
		article.Title = title
	}
	if content != "" {
		article.Content = content
	}
	if articleType > 0 {
		article.ArticleType = articleType
	}
	article.Category = category
	if year > 0 {
		article.Year = year
	}
	if score > 0 {
		article.Score = score
	}
	article.Author = author
	article.Tags = tags
	return s.articleRepo.Update(article)
}

func (s *ArticleService) Delete(id uint64) error {
	return s.articleRepo.Delete(id)
}
