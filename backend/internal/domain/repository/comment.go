package repository

import (
	"time"

	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type CommentRepository interface {
	Create(c *model.QuestionComment) error
	FindByID(id uint64) (*model.QuestionComment, error)
	// ListTop 顶级评论(status=0)分页,按创建时间倒序
	ListTop(questionID uint64, page, pageSize int) ([]model.CommentWithUser, int64, error)
	// ListReplies 指定顶级评论下的所有回复(status=0),按创建时间正序
	ListReplies(questionID uint64, parentIDs []uint64) ([]model.CommentWithUser, error)
	// CountReplies 批量统计每条顶级评论的回复数(仅 status=0)
	CountReplies(parentIDs []uint64) (map[uint64]int64, error)
	// SoftDelete 软删自己的评论(status=0 → 1),返回影响行数(0=无权或不存在)
	SoftDelete(id, userID uint64) (int64, error)
	SetStatus(id uint64, status int8) error
	// AdminList 管理端列表;status=0 表示「全部(status IN (0,2))」,1/2 精确过滤
	AdminList(status int, keyword string, questionID uint64, page, pageSize int) ([]model.AdminCommentView, int64, error)
}

type CommentRepositoryImpl struct {
	db *gorm.DB
}

func NewCommentRepository(db *gorm.DB) CommentRepository {
	return &CommentRepositoryImpl{db: db}
}

func (r *CommentRepositoryImpl) Create(c *model.QuestionComment) error {
	return r.db.Create(c).Error
}

func (r *CommentRepositoryImpl) FindByID(id uint64) (*model.QuestionComment, error) {
	var c model.QuestionComment
	if err := r.db.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CommentRepositoryImpl) ListTop(questionID uint64, page, pageSize int) ([]model.CommentWithUser, int64, error) {
	var total int64
	if err := r.db.Model(&model.QuestionComment{}).
		Where("question_id = ? AND parent_id = 0 AND status = 0", questionID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.CommentWithUser
	err := r.db.Table("question_comments c").
		Select("c.*, u.nickname, u.avatar").
		Joins("LEFT JOIN users u ON u.id = c.user_id").
		Where("c.question_id = ? AND c.parent_id = 0 AND c.status = 0", questionID).
		Order("c.created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&rows).Error
	return rows, total, err
}

func (r *CommentRepositoryImpl) ListReplies(questionID uint64, parentIDs []uint64) ([]model.CommentWithUser, error) {
	if len(parentIDs) == 0 {
		return nil, nil
	}
	var rows []model.CommentWithUser
	err := r.db.Table("question_comments c").
		Select("c.*, u.nickname, u.avatar").
		Joins("LEFT JOIN users u ON u.id = c.user_id").
		Where("c.question_id = ? AND c.parent_id IN ? AND c.status = 0", questionID, parentIDs).
		Order("c.created_at ASC").
		Scan(&rows).Error
	return rows, err
}

func (r *CommentRepositoryImpl) CountReplies(parentIDs []uint64) (map[uint64]int64, error) {
	out := make(map[uint64]int64)
	if len(parentIDs) == 0 {
		return out, nil
	}
	type cntRow struct {
		ParentID uint64
		Cnt      int64
	}
	var rows []cntRow
	err := r.db.Table("question_comments").
		Select("parent_id, COUNT(*) AS cnt").
		Where("parent_id IN ? AND status = 0", parentIDs).
		Group("parent_id").
		Scan(&rows).Error
	for _, rr := range rows {
		out[rr.ParentID] = rr.Cnt
	}
	return out, err
}

func (r *CommentRepositoryImpl) SoftDelete(id, userID uint64) (int64, error) {
	res := r.db.Model(&model.QuestionComment{}).
		Where("id = ? AND user_id = ? AND status = 0", id, userID).
		Update("status", 1)
	return res.RowsAffected, res.Error
}

func (r *CommentRepositoryImpl) SetStatus(id uint64, status int8) error {
	return r.db.Model(&model.QuestionComment{}).Where("id = ?", id).Update("status", status).Error
}

func (r *CommentRepositoryImpl) AdminList(status int, keyword string, questionID uint64, page, pageSize int) ([]model.AdminCommentView, int64, error) {
	cond := func(q *gorm.DB) *gorm.DB {
		if status == 0 {
			q = q.Where("c.status IN (0, 2)")
		} else {
			q = q.Where("c.status = ?", status)
		}
		if questionID > 0 {
			q = q.Where("c.question_id = ?", questionID)
		}
		if keyword != "" {
			like := "%" + keyword + "%"
			q = q.Where("(c.content LIKE ? OR u.nickname LIKE ?)", like, like)
		}
		return q
	}
	// Count 用独立查询(带 join 的 select 里含 SUBSTRING,不能直接 Count)
	var total int64
	cb := r.db.Table("question_comments c").
		Joins("LEFT JOIN users u ON u.id = c.user_id").
		Joins("LEFT JOIN questions q ON q.id = c.question_id")
	if err := cond(cb).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.AdminCommentView
	q := r.db.Table("question_comments c").
		Select("c.*, u.nickname, u.avatar, SUBSTRING(q.content, 1, 60) AS question_snippet").
		Joins("LEFT JOIN users u ON u.id = c.user_id").
		Joins("LEFT JOIN questions q ON q.id = c.question_id")
	err := cond(q).
		Order("c.created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&rows).Error
	return rows, total, err
}

type CommentReportRepository interface {
	Create(r *model.CommentReport) error
	FindByID(id uint64) (*model.CommentReport, error)
	CountByUserAndComment(userID, commentID uint64) (int64, error)
	// List status=0 只出待处理;status=-1 出全部
	List(status int, page, pageSize int) ([]model.AdminCommentReportView, int64, error)
	SetStatus(id uint64, status int8, adminNote string) error
}

type CommentReportRepositoryImpl struct {
	db *gorm.DB
}

func NewCommentReportRepository(db *gorm.DB) CommentReportRepository {
	return &CommentReportRepositoryImpl{db: db}
}

func (r *CommentReportRepositoryImpl) Create(report *model.CommentReport) error {
	return r.db.Create(report).Error
}

func (r *CommentReportRepositoryImpl) FindByID(id uint64) (*model.CommentReport, error) {
	var rep model.CommentReport
	if err := r.db.First(&rep, id).Error; err != nil {
		return nil, err
	}
	return &rep, nil
}

func (r *CommentReportRepositoryImpl) CountByUserAndComment(userID, commentID uint64) (int64, error) {
	var n int64
	err := r.db.Model(&model.CommentReport{}).
		Where("user_id = ? AND comment_id = ?", userID, commentID).
		Count(&n).Error
	return n, err
}

func (r *CommentReportRepositoryImpl) List(status int, page, pageSize int) ([]model.AdminCommentReportView, int64, error) {
	base := r.db.Table("comment_reports cr").
		Joins("JOIN question_comments c ON c.id = cr.comment_id")
	if status >= 0 {
		base = base.Where("cr.status = ?", status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.AdminCommentReportView
	q := r.db.Table("comment_reports cr").
		Select("cr.*, c.content AS comment_content, au.nickname AS author_nickname, ru.nickname AS reporter_nickname").
		Joins("JOIN question_comments c ON c.id = cr.comment_id").
		Joins("LEFT JOIN users au ON au.id = c.user_id").
		Joins("LEFT JOIN users ru ON ru.id = cr.user_id")
	if status >= 0 {
		q = q.Where("cr.status = ?", status)
	}
	err := q.Order("cr.created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&rows).Error
	return rows, total, err
}

func (r *CommentReportRepositoryImpl) SetStatus(id uint64, status int8, adminNote string) error {
	now := time.Now()
	return r.db.Model(&model.CommentReport{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"status": status, "admin_note": adminNote, "handled_at": now}).Error
}

// ---- ExamSchedule ----
