package repository

import (
	"strings"

	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type QuestionNoteRepository interface {
	Upsert(note *model.QuestionNote) error
	FindByUserAndQuestion(userID, questionID uint64) (*model.QuestionNote, error)
	Delete(userID, questionID uint64) error
	ListByUser(userID uint64, page, pageSize int, keyword string) ([]model.NoteWithQuestion, int64, error)
	FindWithQuestion(userID, questionID uint64) (*model.NoteWithQuestion, error)
}

type QuestionNoteRepositoryImpl struct {
	db *gorm.DB
}

func NewQuestionNoteRepository(db *gorm.DB) QuestionNoteRepository {
	return &QuestionNoteRepositoryImpl{db: db}
}

func (r *QuestionNoteRepositoryImpl) Upsert(note *model.QuestionNote) error {
	existing := &model.QuestionNote{}
	err := r.db.Where("user_id = ? AND question_id = ?", note.UserID, note.QuestionID).First(existing).Error
	if err != nil {
		// not found, create
		return r.db.Create(note).Error
	}
	// found, update content
	existing.Content = note.Content
	return r.db.Save(existing).Error
}

func (r *QuestionNoteRepositoryImpl) FindByUserAndQuestion(userID, questionID uint64) (*model.QuestionNote, error) {
	var note model.QuestionNote
	err := r.db.Where("user_id = ? AND question_id = ?", userID, questionID).First(&note).Error
	return &note, err
}

func (r *QuestionNoteRepositoryImpl) Delete(userID, questionID uint64) error {
	return r.db.Where("user_id = ? AND question_id = ?", userID, questionID).Delete(&model.QuestionNote{}).Error
}

// noteListBase 组装「我的笔记」查询基座（含关键词过滤），供分页查询与去重计数共用。
func (r *QuestionNoteRepositoryImpl) noteListBase(userID uint64, keyword string) *gorm.DB {
	base := r.db.Table("question_notes as n").
		Select(`n.id, n.question_id, n.content, n.updated_at,
			q.title as question_title,
			LEFT(q.content, 80) as question_stem,
			q.question_type as question_type,
			q.knowledge_point as knowledge_point,
			MAX(p.title) as paper_title`).
		Joins("left join questions q on q.id = n.question_id").
		Joins("left join paper_questions pq on pq.question_id = n.question_id").
		Joins("left join papers p on p.id = pq.paper_id").
		Where("n.user_id = ?", userID)
	if kw := strings.TrimSpace(keyword); kw != "" {
		like := "%" + kw + "%"
		base = base.Where("(n.content LIKE ? OR q.title LIKE ? OR p.title LIKE ?)", like, like, like)
	}
	return base
}

func (r *QuestionNoteRepositoryImpl) ListByUser(userID uint64, page, pageSize int, keyword string) ([]model.NoteWithQuestion, int64, error) {
	var list []model.NoteWithQuestion
	var total int64

	base := r.noteListBase(userID, keyword)

	// 题目可挂在多套卷下（LEFT JOIN 会放大行数），按 n.id 去重计数。
	// Session 克隆一份语句，避免 Distinct 污染下面的主查询。
	if err := base.Session(&gorm.Session{}).Distinct("n.id").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 必须有 id 兜底：updated_at 同秒的多条笔记在 LIMIT/OFFSET 下顺序由引擎自由决定，
	// 翻页会重复或漏行。列表改成无限滚动后是追加累积的，重复行会以两张一样的卡片
	// 同时留在屏幕上，用户直接看得见。
	err := base.Group("n.id").
		Order("n.updated_at desc, n.id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&list).Error
	return list, total, err
}

// FindWithQuestion 取用户在某道题上的笔记（含套卷名等归属信息）。
// 没有笔记时返回 gorm.ErrRecordNotFound —— 调用方靠它区分「不是我的笔记」并回 404。
// 这个 404 是接口的安全边界：详情接口只有走到这里拿到笔记，才会去读题目表。
//
// 注意这里不能用 First/Take：基座是 join + group 的聚合查询，Scan 不会替我们
// 把空结果翻成 ErrRecordNotFound，所以要自己判主键零值。
func (r *QuestionNoteRepositoryImpl) FindWithQuestion(userID, questionID uint64) (*model.NoteWithQuestion, error) {
	var row model.NoteWithQuestion
	err := r.noteListBase(userID, "").
		Where("n.question_id = ?", questionID).
		Group("n.id").
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

// ---- AI Config ----
