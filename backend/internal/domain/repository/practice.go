package repository

import (
	"software-exam/internal/domain/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PracticeRecordRepository interface {
	Create(record *model.PracticeRecord) error
	FindByUserID(userID uint64, practiceType int8, page, pageSize int) ([]model.PracticeRecord, int64, error)
}

type PracticeRecordRepositoryImpl struct {
	db *gorm.DB
}

func NewPracticeRecordRepository(db *gorm.DB) PracticeRecordRepository {
	return &PracticeRecordRepositoryImpl{db: db}
}

func (r *PracticeRecordRepositoryImpl) Create(record *model.PracticeRecord) error {
	return r.db.Create(record).Error
}

func (r *PracticeRecordRepositoryImpl) FindByUserID(userID uint64, practiceType int8, page, pageSize int) ([]model.PracticeRecord, int64, error) {
	var records []model.PracticeRecord
	var total int64
	query := r.db.Where("user_id = ?", userID)
	if practiceType > 0 {
		query = query.Where("practice_type = ?", practiceType)
	}
	query.Model(&model.PracticeRecord{}).Count(&total)
	err := query.Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&records).Error
	return records, total, err
}

// PracticeSessionRepository 分类刷题断点草稿。作用域是 (用户, 科目, 题型, 知识点)，
// 同一作用域只留一份：重新开始就整条覆盖，不留历史。

type PracticeSessionRepository interface {
	FindOne(userID, subjectID uint64, questionType int8, knowledgePoint string) (*model.PracticeSession, error)
	Save(session *model.PracticeSession) error
	Delete(userID, subjectID uint64, questionType int8, knowledgePoint string) error
}

type PracticeSessionRepositoryImpl struct {
	db *gorm.DB
}

func NewPracticeSessionRepository(db *gorm.DB) PracticeSessionRepository {
	return &PracticeSessionRepositoryImpl{db: db}
}

func (r *PracticeSessionRepositoryImpl) scope(userID, subjectID uint64, questionType int8, knowledgePoint string) *gorm.DB {
	return r.db.Where("user_id = ? AND subject_id = ? AND question_type = ? AND knowledge_point = ?",
		userID, subjectID, questionType, knowledgePoint)
}

func (r *PracticeSessionRepositoryImpl) FindOne(userID, subjectID uint64, questionType int8, knowledgePoint string) (*model.PracticeSession, error) {
	var s model.PracticeSession
	if err := r.scope(userID, subjectID, questionType, knowledgePoint).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// Save 落草稿。作答过程中每题都会调一次，走唯一索引 upsert，
// 避免「先查后插」在两个标签页同时刷题时撞唯一键。
func (r *PracticeSessionRepositoryImpl) Save(session *model.PracticeSession) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"}, {Name: "subject_id"}, {Name: "question_type"}, {Name: "knowledge_point"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"question_ids", "answers", "updated_at"}),
	}).Create(session).Error
}

func (r *PracticeSessionRepositoryImpl) Delete(userID, subjectID uint64, questionType int8, knowledgePoint string) error {
	return r.scope(userID, subjectID, questionType, knowledgePoint).Delete(&model.PracticeSession{}).Error
}

// PaperPracticeSessionRepository 真题练习模式断点草稿。作用域是 (用户, 试卷, 模式)，
// 同一作用域只留一份：重新开始就整条覆盖，不留历史。

type PaperPracticeSessionRepository interface {
	FindOne(userID, paperID uint64, mode string) (*model.PaperPracticeSession, error)
	Save(session *model.PaperPracticeSession) error
	Delete(userID, paperID uint64, mode string) error
}

type PaperPracticeSessionRepositoryImpl struct {
	db *gorm.DB
}

func NewPaperPracticeSessionRepository(db *gorm.DB) PaperPracticeSessionRepository {
	return &PaperPracticeSessionRepositoryImpl{db: db}
}

func (r *PaperPracticeSessionRepositoryImpl) scope(userID, paperID uint64, mode string) *gorm.DB {
	return r.db.Where("user_id = ? AND paper_id = ? AND mode = ?", userID, paperID, mode)
}

func (r *PaperPracticeSessionRepositoryImpl) FindOne(userID, paperID uint64, mode string) (*model.PaperPracticeSession, error) {
	var s model.PaperPracticeSession
	if err := r.scope(userID, paperID, mode).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// Save 落草稿。作答过程中每题都会调一次，走唯一索引 upsert，
// 避免「先查后插」在两个标签页同时练时撞唯一键。
func (r *PaperPracticeSessionRepositoryImpl) Save(session *model.PaperPracticeSession) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"}, {Name: "paper_id"}, {Name: "mode"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"data", "updated_at"}),
	}).Create(session).Error
}

func (r *PaperPracticeSessionRepositoryImpl) Delete(userID, paperID uint64, mode string) error {
	return r.scope(userID, paperID, mode).Delete(&model.PaperPracticeSession{}).Error
}

// ReorderItem 知识点排序条目：拖拽排序时前端整树序列化批量提交。
type ReorderItem struct {
	ID        uint64 `json:"id"`
	ParentID  uint64 `json:"parent_id"`
	SortOrder int    `json:"sort_order"`
}
