package repository

import (
	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type ArticleRepository interface {
	Create(article *model.Article) error
	FindByID(id uint64) (*model.Article, error)
	Update(article *model.Article) error
	Delete(id uint64) error
	List(req ArticleListReq) ([]model.Article, int64, error)
}

type ArticleRepositoryImpl struct {
	db *gorm.DB
}

func NewArticleRepository(db *gorm.DB) ArticleRepository {
	return &ArticleRepositoryImpl{db: db}
}

func (r *ArticleRepositoryImpl) Create(article *model.Article) error {
	return r.db.Create(article).Error
}

func (r *ArticleRepositoryImpl) FindByID(id uint64) (*model.Article, error) {
	var article model.Article
	err := r.db.First(&article, id).Error
	return &article, err
}

func (r *ArticleRepositoryImpl) Update(article *model.Article) error {
	return r.db.Save(article).Error
}

func (r *ArticleRepositoryImpl) Delete(id uint64) error {
	return r.db.Delete(&model.Article{}, id).Error
}

func (r *ArticleRepositoryImpl) List(req ArticleListReq) ([]model.Article, int64, error) {
	var articles []model.Article
	var total int64
	query := r.db.Where("status = 1")
	if req.ArticleType > 0 {
		query = query.Where("article_type = ?", req.ArticleType)
	}
	if req.SubjectID > 0 {
		query = query.Where("subject_id = ?", req.SubjectID)
	}
	if req.Category != "" {
		query = query.Where("category = ?", req.Category)
	}
	if req.Keyword != "" {
		query = query.Where("title LIKE ?", "%"+req.Keyword+"%")
	}
	query.Model(&model.Article{}).Count(&total)
	err := query.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("created_at DESC").Find(&articles).Error
	return articles, total, err
}

// AdminPaperListReq 管理端试卷列表筛选条件。
// 与用户端 List 不同：不排除已下线的试卷（后台要能看全量再决定上/下线）。
type AdminPaperListReq struct {
	Page      int
	PageSize  int
	PaperType int8
	Year      int16
	Keyword   string
	SubjectID uint64
}
