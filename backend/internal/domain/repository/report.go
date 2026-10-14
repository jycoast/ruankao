package repository

import (
	"time"

	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type QuestionReportRepository interface {
	Create(r *model.QuestionReport) error
	FindByID(id uint64) (*model.QuestionReport, error)
	ListAll(page, pageSize, status int) ([]QuestionReportView, int64, error)
	ListByUserID(userID uint64, page, pageSize int) ([]QuestionReportView, int64, error)
	UpdateStatus(id uint64, status int8, adminNote string) error
	CountPending() (int64, error)
}

type QuestionReportRepositoryImpl struct {
	db *gorm.DB
}

func NewQuestionReportRepository(db *gorm.DB) QuestionReportRepository {
	return &QuestionReportRepositoryImpl{db: db}
}

func (r *QuestionReportRepositoryImpl) Create(rep *model.QuestionReport) error {
	return r.db.Create(rep).Error
}

func (r *QuestionReportRepositoryImpl) FindByID(id uint64) (*model.QuestionReport, error) {
	var rep model.QuestionReport
	if err := r.db.First(&rep, id).Error; err != nil {
		return nil, err
	}
	return &rep, nil
}

func (r *QuestionReportRepositoryImpl) ListAll(page, pageSize, status int) ([]QuestionReportView, int64, error) {
	var list []QuestionReportView
	var total int64

	// 科目须经题目推导（question_reports 自己没有 subject_id）。必须 LEFT：
	// INNER 会把「题目已被删」的反馈整条隐藏，还会让下面 Count 出的 total 偏小。
	base := r.db.Table("question_reports qr").
		Select("qr.id, qr.question_id, qr.user_id, qr.report_type, qr.description, qr.status, qr.admin_note, qr.created_at, " +
			"u.username, q.title AS question_title, q.year AS question_year, q.question_type AS question_type, " +
			"SUBSTR(q.content, 1, 300) AS question_content, SUBSTR(q.content, 1, 100) AS question_preview, " +
			"s.name AS subject_name").
		Joins("LEFT JOIN users u ON u.id = qr.user_id").
		Joins("LEFT JOIN questions q ON q.id = qr.question_id").
		Joins("LEFT JOIN subjects s ON s.id = q.subject_id")

	if status >= 0 {
		base = base.Where("qr.status = ?", status)
	}

	base.Count(&total)
	err := base.Order("qr.created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&list).Error

	return list, total, err
}

func (r *QuestionReportRepositoryImpl) ListByUserID(userID uint64, page, pageSize int) ([]QuestionReportView, int64, error) {
	var list []QuestionReportView
	var total int64

	// 与 ListAll 的 Select/Join 逐字一致，保持同一个 View 的字段处处可用：
	// 只改一处会让 subject_name 变成时有时无的字段。
	base := r.db.Table("question_reports qr").
		Select("qr.id, qr.question_id, qr.user_id, qr.report_type, qr.description, qr.status, qr.admin_note, qr.created_at, "+
			"u.username, q.title AS question_title, q.year AS question_year, q.question_type AS question_type, "+
			"SUBSTR(q.content, 1, 300) AS question_content, SUBSTR(q.content, 1, 100) AS question_preview, "+
			"s.name AS subject_name").
		Joins("LEFT JOIN users u ON u.id = qr.user_id").
		Joins("LEFT JOIN questions q ON q.id = qr.question_id").
		Joins("LEFT JOIN subjects s ON s.id = q.subject_id").
		Where("qr.user_id = ?", userID)

	base.Count(&total)
	err := base.Order("qr.created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&list).Error

	return list, total, err
}

func (r *QuestionReportRepositoryImpl) UpdateStatus(id uint64, status int8, adminNote string) error {
	return r.db.Model(&model.QuestionReport{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": status, "admin_note": adminNote}).Error
}

func (r *QuestionReportRepositoryImpl) CountPending() (int64, error) {
	var count int64
	err := r.db.Model(&model.QuestionReport{}).Where("status = 0").Count(&count).Error
	return count, err
}

// ---- Feedback ----

// FeedbackView 用于管理端列表，JOIN 了用户名；内容纠错类反馈附带题目识别字段，
// 管理员直接看到是哪个题目，并可从反馈页跳转编辑。
type FeedbackView struct {
	ID              uint64    `json:"id" gorm:"column:id"`
	UserID          uint64    `json:"user_id" gorm:"column:user_id"`
	Username        string    `json:"username" gorm:"column:username"`
	FeedbackType    int8      `json:"feedback_type" gorm:"column:feedback_type"`
	QuestionID      uint64    `json:"question_id" gorm:"column:question_id"`
	Content         string    `json:"content" gorm:"column:content"`
	Contact         string    `json:"contact" gorm:"column:contact"`
	Status          int8      `json:"status" gorm:"column:status"`
	AdminNote       string    `json:"admin_note" gorm:"column:admin_note"`
	CreatedAt       time.Time `json:"created_at" gorm:"column:created_at"`
	QuestionTitle   string    `json:"question_title" gorm:"column:question_title"`
	QuestionYear    int16     `json:"question_year" gorm:"column:question_year"`
	QuestionType    int8      `json:"question_type" gorm:"column:question_type"`
	QuestionContent string    `json:"question_content" gorm:"column:question_content"`
}

// FeedbackListReq 管理端反馈列表筛选条件（仿 OrderListReq）。
type FeedbackListReq struct {
	Page     int
	PageSize int
	Status   int    // -1=全部 0=待处理 1=已处理 2=已忽略
	Type     int8   // -1=全部 1=功能建议 2=问题反馈 3=内容纠错 4=其他
	Keyword  string // 模糊匹配反馈内容/用户名/联系方式
}

type FeedbackRepository interface {
	Create(f *model.Feedback) error
	FindByID(id uint64) (*model.Feedback, error)
	ListAll(req FeedbackListReq) ([]FeedbackView, int64, error)
	ListByUserID(userID uint64, page, pageSize int) ([]FeedbackView, int64, error)
	UpdateStatus(id uint64, status int8, adminNote string) error
	CountPending() (int64, error)
}

type FeedbackRepositoryImpl struct {
	db *gorm.DB
}

func NewFeedbackRepository(db *gorm.DB) FeedbackRepository {
	return &FeedbackRepositoryImpl{db: db}
}

func (r *FeedbackRepositoryImpl) Create(f *model.Feedback) error {
	return r.db.Create(f).Error
}

func (r *FeedbackRepositoryImpl) FindByID(id uint64) (*model.Feedback, error) {
	var f model.Feedback
	if err := r.db.First(&f, id).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *FeedbackRepositoryImpl) ListAll(req FeedbackListReq) ([]FeedbackView, int64, error) {
	var list []FeedbackView
	var total int64

	base := r.db.Table("feedbacks fb").
		Select("fb.id, fb.user_id, fb.feedback_type, fb.question_id, fb.content, fb.contact, fb.status, fb.admin_note, fb.created_at, " +
			"u.username, q.title AS question_title, q.year AS question_year, q.question_type AS question_type, " +
			"SUBSTR(q.content, 1, 300) AS question_content").
		Joins("LEFT JOIN users u ON u.id = fb.user_id").
		Joins("LEFT JOIN questions q ON q.id = fb.question_id")

	if req.Status >= 0 {
		base = base.Where("fb.status = ?", req.Status)
	}
	if req.Type >= 0 {
		base = base.Where("fb.feedback_type = ?", req.Type)
	}
	if req.Keyword != "" {
		kw := "%" + req.Keyword + "%"
		base = base.Where("(fb.content LIKE ? OR u.username LIKE ? OR fb.contact LIKE ?)", kw, kw, kw)
	}

	base.Count(&total)
	err := base.Order("fb.created_at DESC").
		Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Scan(&list).Error

	return list, total, err
}

func (r *FeedbackRepositoryImpl) ListByUserID(userID uint64, page, pageSize int) ([]FeedbackView, int64, error) {
	var list []FeedbackView
	var total int64

	base := r.db.Table("feedbacks fb").
		Select("fb.id, fb.user_id, fb.feedback_type, fb.question_id, fb.content, fb.contact, fb.status, fb.admin_note, fb.created_at, "+
			"u.username, q.title AS question_title, q.year AS question_year, q.question_type AS question_type, "+
			"SUBSTR(q.content, 1, 300) AS question_content").
		Joins("LEFT JOIN users u ON u.id = fb.user_id").
		Joins("LEFT JOIN questions q ON q.id = fb.question_id").
		Where("fb.user_id = ?", userID)

	base.Count(&total)
	err := base.Order("fb.created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&list).Error

	return list, total, err
}

func (r *FeedbackRepositoryImpl) UpdateStatus(id uint64, status int8, adminNote string) error {
	return r.db.Model(&model.Feedback{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": status, "admin_note": adminNote}).Error
}

func (r *FeedbackRepositoryImpl) CountPending() (int64, error) {
	var count int64
	err := r.db.Model(&model.Feedback{}).Where("status = 0").Count(&count).Error
	return count, err
}

// ---- PasswordReset ----

// PasswordResetView 管理端列表，JOIN 出当前用户名与邮箱。
// 邮箱要显示：申请人多半是拿邮箱来找回的，管理员得能对上。
type PasswordResetView struct {
	ID        uint64    `json:"id" gorm:"column:id"`
	UserID    uint64    `json:"user_id" gorm:"column:user_id"`
	Username  string    `json:"username" gorm:"column:username"`
	Email     string    `json:"email" gorm:"column:email"`
	Account   string    `json:"account" gorm:"column:account"`
	Contact   string    `json:"contact" gorm:"column:contact"`
	Reason    string    `json:"reason" gorm:"column:reason"`
	ClientIP  string    `json:"client_ip" gorm:"column:client_ip"`
	Status    int8      `json:"status" gorm:"column:status"`
	AdminNote string    `json:"admin_note" gorm:"column:admin_note"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
}
