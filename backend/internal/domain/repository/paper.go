package repository

import (
	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type PaperRepository interface {
	Create(paper *model.Paper) error
	CreateWithQuestions(paper *model.Paper, questionIDs []uint64) error
	FindByID(id uint64) (*model.Paper, error)
	Update(paper *model.Paper) error
	Delete(id uint64) error
	DeleteMockByUser(userID uint64) error
	List(paperType int8, page, pageSize int, subjectID uint64) ([]model.Paper, int64, error)
	FindQuestionsByPaperID(paperID uint64) ([]model.Question, error)
	// ---- 管理端 ----
	ListAdmin(req AdminPaperListReq) ([]model.Paper, int64, error)
	CountQuestionsByPaperIDs(paperIDs []uint64) (map[uint64]int64, error)
	ReplaceQuestions(paperID uint64, questionIDs []uint64) error
	DeleteFull(paperID uint64) error
}

type PaperRepositoryImpl struct {
	db *gorm.DB
}

func NewPaperRepository(db *gorm.DB) PaperRepository {
	return &PaperRepositoryImpl{db: db}
}

func (r *PaperRepositoryImpl) Create(paper *model.Paper) error {
	return r.db.Create(paper).Error
}

// CreateWithQuestions 在一个事务中创建试卷并按顺序写入 paper_questions 关联，
// 用于生成模拟卷：先落库拿到 PaperID，再批量写入题目关联。
func (r *PaperRepositoryImpl) CreateWithQuestions(paper *model.Paper, questionIDs []uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(paper).Error; err != nil {
			return err
		}
		if len(questionIDs) == 0 {
			return nil
		}
		links := make([]model.PaperQuestion, 0, len(questionIDs))
		for i, qid := range questionIDs {
			links = append(links, model.PaperQuestion{
				PaperID:    paper.ID,
				QuestionID: qid,
				SortOrder:  i,
			})
		}
		return tx.Create(&links).Error
	})
}

func (r *PaperRepositoryImpl) FindByID(id uint64) (*model.Paper, error) {
	var paper model.Paper
	err := r.db.First(&paper, id).Error
	return &paper, err
}

func (r *PaperRepositoryImpl) Update(paper *model.Paper) error {
	return r.db.Save(paper).Error
}

func (r *PaperRepositoryImpl) Delete(id uint64) error {
	return r.db.Delete(&model.Paper{}, id).Error
}

// DeleteMockByUser 删除某用户之前生成的所有模拟卷及其题目关联，
// 每次生成新模拟卷前调用，避免模拟卷无限累积。
func (r *PaperRepositoryImpl) DeleteMockByUser(userID uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var ids []uint64
		if err := tx.Model(&model.Paper{}).
			Where("source = 1 AND created_by = ?", userID).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if err := tx.Where("paper_id IN ?", ids).Delete(&model.PaperQuestion{}).Error; err != nil {
			return err
		}
		return tx.Where("id IN ?", ids).Delete(&model.Paper{}).Error
	})
}

func (r *PaperRepositoryImpl) List(paperType int8, page, pageSize int, subjectID uint64) ([]model.Paper, int64, error) {
	var papers []model.Paper
	var total int64
	query := r.db.Where("status = 1 AND source = 0") // 仅真题，排除用户生成的模拟卷
	if paperType > 0 {
		query = query.Where("paper_type = ?", paperType)
	}
	if subjectID > 0 {
		query = query.Where("subject_id = ?", subjectID)
	}
	query.Model(&model.Paper{}).Count(&total)
	err := query.Offset((page - 1) * pageSize).Limit(pageSize).Order("year DESC").Find(&papers).Error
	return papers, total, err
}

func (r *PaperRepositoryImpl) FindQuestionsByPaperID(paperID uint64) ([]model.Question, error) {
	var pqs []model.PaperQuestion
	err := r.db.Where("paper_id = ?", paperID).Order("sort_order ASC").Find(&pqs).Error
	if err != nil {
		return nil, err
	}
	var ids []uint64
	for _, pq := range pqs {
		ids = append(ids, pq.QuestionID)
	}
	if len(ids) == 0 {
		return []model.Question{}, nil
	}
	// 卷内题目排序与题库列表口径一致：后台拖拽过的题（questions.sort_order>0）按拖拽顺序置前，
	// 未拖拽过的题保持卷内编排顺序（paper_questions.sort_order）。
	// 不能只靠 WHERE id IN 查询——MySQL 的 IN 无 ORDER BY 时按主键升序返回，卷内顺序会丢。
	var questions []model.Question
	err = r.db.Model(&model.Question{}).
		Joins("JOIN paper_questions pq ON pq.question_id = questions.id AND pq.paper_id = ?", paperID).
		Where("questions.id IN ?", ids).
		Order("(questions.sort_order = 0) ASC, questions.sort_order ASC, pq.sort_order ASC, questions.id ASC").
		Preload("Options", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		Find(&questions).Error
	return questions, err
}

// ListAdmin 管理端试卷列表：只含真题（source=0），但不过滤 status，
// 后台需要看到已下线的试卷才能重新上线。不查 PaperQuestion 关联数，
// 题目数由 CountQuestionsByPaperIDs 单独批量补。
func (r *PaperRepositoryImpl) ListAdmin(req AdminPaperListReq) ([]model.Paper, int64, error) {
	var papers []model.Paper
	var total int64
	query := r.db.Where("source = 0")
	if req.PaperType > 0 {
		query = query.Where("paper_type = ?", req.PaperType)
	}
	if req.Year > 0 {
		query = query.Where("year = ?", req.Year)
	}
	if req.SubjectID > 0 {
		query = query.Where("subject_id = ?", req.SubjectID)
	}
	if req.Keyword != "" {
		query = query.Where("title LIKE ?", "%"+req.Keyword+"%")
	}
	query.Model(&model.Paper{}).Count(&total)
	err := query.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Order("year DESC, id DESC").Find(&papers).Error
	return papers, total, err
}

// CountQuestionsByPaperIDs 一次查出多套试卷的题目数，避免列表里逐卷 Count。
func (r *PaperRepositoryImpl) CountQuestionsByPaperIDs(paperIDs []uint64) (map[uint64]int64, error) {
	counts := make(map[uint64]int64)
	if len(paperIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		PaperID uint64
		Cnt     int64
	}
	err := r.db.Model(&model.PaperQuestion{}).
		Select("paper_id, COUNT(*) AS cnt").
		Where("paper_id IN ?", paperIDs).
		Group("paper_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.PaperID] = row.Cnt
	}
	return counts, nil
}

// ReplaceQuestions 事务内整删整插卷内题目关联，顺序由调用方传入的切片保证。
// 与 CreateWithQuestions 同款写法，用于管理端「保存题目编排」。
func (r *PaperRepositoryImpl) ReplaceQuestions(paperID uint64, questionIDs []uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("paper_id = ?", paperID).Delete(&model.PaperQuestion{}).Error; err != nil {
			return err
		}
		if len(questionIDs) == 0 {
			return nil
		}
		links := make([]model.PaperQuestion, 0, len(questionIDs))
		for i, qid := range questionIDs {
			links = append(links, model.PaperQuestion{
				PaperID:    paperID,
				QuestionID: qid,
				SortOrder:  i,
			})
		}
		return tx.Create(&links).Error
	})
}

// DeleteFull 删除试卷及其题目关联、考试记录。
// exam_records 一并清理：卷子都没了，历史成绩对用户也无意义，留着只会是孤儿数据。
func (r *PaperRepositoryImpl) DeleteFull(paperID uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("paper_id = ?", paperID).Delete(&model.PaperQuestion{}).Error; err != nil {
			return err
		}
		if err := tx.Where("paper_id = ?", paperID).Delete(&model.ExamRecord{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Paper{}, paperID).Error
	})
}
