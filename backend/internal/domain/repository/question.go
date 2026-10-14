package repository

import (
	"software-exam/internal/domain/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type QuestionRepository interface {
	Create(question *model.Question) error
	FindByID(id uint64) (*model.Question, error)
	Update(question *model.Question) error
	ReplaceOptions(questionID uint64, options []model.Option) error
	Delete(id uint64) error
	List(req QuestionListReq) ([]model.Question, int64, error)
	FindByIDs(ids []uint64) ([]model.Question, error)
	GetRandomQuestions(qType int8, count int, knowledgePoint string, subjectID uint64) ([]model.Question, error)
	// GetSeededRandomQuestions 按固定种子随机抽题：同一种子必然得到同一组题。
	// 每日一练用它保证「同一天同一用户换设备不换题」；种子由日期+用户+科目哈希而来。
	GetSeededRandomQuestions(qType int8, count int, knowledgePoint string, subjectID uint64, seed uint32) ([]model.Question, error)
	GetRandomByTypeYear(qType int8, count int, subjectID uint64, minYear int16) ([]model.Question, error)
	// CountQuestions 统计当前候选集全量题数，过滤条件与 GetRandomQuestions 完全一致
	// （含选择题的 EXISTS options）。供「继续刷题」判定存量草稿是否过期（旧版 20 道上限
	// 留下的残缺草稿题数必然少于候选集，恢复出来答题卡还是残缺的）。
	CountQuestions(qType int8, knowledgePoint string, subjectID uint64) (int64, error)
	UpdateSortOrders(ids []uint64) error
	GetCategories(subjectID uint64) ([]Category, error)
}

type QuestionListReq struct {
	Page         int
	PageSize     int
	QuestionType int8
	CategoryID   uint64
	Year         int16
	Difficulty   int8
	Keyword      string
	SubjectID    uint64
}

type QuestionRepositoryImpl struct {
	db *gorm.DB
}

func NewQuestionRepository(db *gorm.DB) QuestionRepository {
	return &QuestionRepositoryImpl{db: db}
}

func (r *QuestionRepositoryImpl) Create(question *model.Question) error {
	return r.db.Create(question).Error
}

func (r *QuestionRepositoryImpl) FindByID(id uint64) (*model.Question, error) {
	var q model.Question
	err := r.db.Preload("Options", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).First(&q, id).Error
	return &q, err
}

func (r *QuestionRepositoryImpl) Update(question *model.Question) error {
	return r.db.Save(question).Error
}

// ReplaceOptions 用给定选项整体替换题目的选项。
// 整删整插而非逐条 diff：选项数量可增可减、标号可重排，
// 逐条比对的复杂度远高于收益，而选项表本身每题不过三五行。
// 必须在事务内：删完插失败会让题目变成没有选项的空壳。
func (r *QuestionRepositoryImpl) ReplaceOptions(questionID uint64, options []model.Option) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("question_id = ?", questionID).Delete(&model.Option{}).Error; err != nil {
			return err
		}
		if len(options) == 0 {
			return nil
		}
		for i := range options {
			options[i].ID = 0
			options[i].QuestionID = questionID
		}
		return tx.Create(&options).Error
	})
}

func (r *QuestionRepositoryImpl) Delete(id uint64) error {
	return r.db.Delete(&model.Question{}, id).Error
}

func (r *QuestionRepositoryImpl) List(req QuestionListReq) ([]model.Question, int64, error) {
	var questions []model.Question
	var total int64
	query := r.db.Model(&model.Question{}).Where("status = 1")

	if req.SubjectID > 0 {
		query = query.Where("subject_id = ?", req.SubjectID)
	}
	if req.QuestionType > 0 {
		query = query.Where("question_type = ?", req.QuestionType)
	}
	if req.CategoryID > 0 {
		query = query.Where("category_id = ?", req.CategoryID)
	}
	if req.Year > 0 {
		query = query.Where("year = ?", req.Year)
	}
	if req.Difficulty > 0 {
		query = query.Where("difficulty = ?", req.Difficulty)
	}
	if req.Keyword != "" {
		// 模糊匹配标题 + 题干正文（content）。用户反馈只复制题干片段时，仅搜 title
		// 会找不到——题干真正文字都在 content 里，两者都要兜住。
		kw := "%" + req.Keyword + "%"
		query = query.Where("title LIKE ? OR content LIKE ?", kw, kw)
	}

	query.Count(&total)
	// 手动排序的题（sort_order>0）按权重升序排前；未排序的（sort_order=0）排最后，内部仍按 id DESC
	err := query.Preload("Options", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("(sort_order = 0) ASC, sort_order ASC, id DESC").Find(&questions).Error
	return questions, total, err
}

// UpdateSortOrders 按传入顺序把题目 sort_order 重排为 1..N（0 保留给未手动排序的题，查询时排最后）。
func (r *QuestionRepositoryImpl) UpdateSortOrders(ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			if err := tx.Model(&model.Question{}).Where("id = ?", id).Update("sort_order", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *QuestionRepositoryImpl) FindByIDs(ids []uint64) ([]model.Question, error) {
	var questions []model.Question
	err := r.db.Preload("Options", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).Where("id IN ?", ids).Find(&questions).Error
	return questions, err
}

// GetRandomQuestions 按题型随机抽题。count<=0 表示不设上限（分类刷题一次覆盖知识点全部题目，
// 答题卡题数须与列表显示的全量题数一致）；候选集与过滤条件与 GetRandomByTypeYear 一致。
func (r *QuestionRepositoryImpl) GetRandomQuestions(qType int8, count int, knowledgePoint string, subjectID uint64) ([]model.Question, error) {
	var questions []model.Question
	query := r.db.Where("question_type = ? AND status = 1", qType)
	if knowledgePoint != "" {
		query = query.Where("knowledge_point = ?", knowledgePoint)
	}
	if subjectID > 0 {
		query = query.Where("subject_id = ?", subjectID)
	}
	// 只抽有选项的题：批量导入偶发漏掉选项，抽中会显示"无选项"——但仅对选择题(1/2)生效。
	// 案例(3)/论文(4) 按设计无选项，套用本过滤会让分类刷题的案例/论文整类抽 0 题。
	if isChoiceType(qType) {
		query = query.Where("EXISTS (SELECT 1 FROM options o WHERE o.question_id = questions.id)")
	}
	err := query.Order("RAND()").
		Limit(count).
		Preload("Options", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		Find(&questions).Error
	return questions, err
}

// CountQuestions 统计某知识点的候选集题数：与 GetRandomQuestions 同一套过滤
// （题型+status、可选知识点/subject、选择题另加 EXISTS options），不排序不限制。
// 注意过滤必须与抽题保持一致 —— 选择题漏掉 EXISTS 会把无选项的题算进全量，
// 「继续刷题」拿它对比草稿题数时会误判。
func (r *QuestionRepositoryImpl) CountQuestions(qType int8, knowledgePoint string, subjectID uint64) (int64, error) {
	var n int64
	query := r.db.Model(&model.Question{}).Where("question_type = ? AND status = 1", qType)
	if knowledgePoint != "" {
		query = query.Where("knowledge_point = ?", knowledgePoint)
	}
	if subjectID > 0 {
		query = query.Where("subject_id = ?", subjectID)
	}
	if isChoiceType(qType) {
		query = query.Where("EXISTS (SELECT 1 FROM options o WHERE o.question_id = questions.id)")
	}
	err := query.Count(&n).Error
	return n, err
}

// GetSeededRandomQuestions 按固定种子随机抽题（MySQL RAND(N) 对同一行集是确定性的）。
// 与 GetRandomQuestions 的唯一区别是排序键换成 RAND(seed)，候选集/过滤条件完全一致。
// seed 是日期+用户+科目的哈希（uint32），可安全内嵌 SQL，无注入风险。
func (r *QuestionRepositoryImpl) GetSeededRandomQuestions(qType int8, count int, knowledgePoint string, subjectID uint64, seed uint32) ([]model.Question, error) {
	var questions []model.Question
	query := r.db.Where("question_type = ? AND status = 1", qType)
	if knowledgePoint != "" {
		query = query.Where("knowledge_point = ?", knowledgePoint)
	}
	if subjectID > 0 {
		query = query.Where("subject_id = ?", subjectID)
	}
	// 只抽有选项的题：批量导入偶发漏掉选项，抽中会显示"无选项"——但仅对选择题(1/2)生效。
	// 案例(3)/论文(4) 按设计无选项，套用本过滤会让每日一练的案例/论文整类抽空。
	if isChoiceType(qType) {
		query = query.Where("EXISTS (SELECT 1 FROM options o WHERE o.question_id = questions.id)")
	}
	// seed 由日期+用户+科目哈希而来，经参数占位符传入，不拼进 SQL 字符串。
	// Order() 不接收裸 gorm.Expr，需用 clause.OrderBy.Expression 承载带参表达式。
	err := query.Order(clause.OrderBy{
		Expression: clause.Expr{SQL: "RAND(?)", Vars: []interface{}{seed}},
	}).Limit(count).Preload("Options", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).Find(&questions).Error
	return questions, err
}

// GetRandomByTypeYear 按题型从近若干年真题中随机抽题，用于生成模拟卷。
// minYear<=0 时不限年份；只抽真题（source 只作用于试卷，题目本身无来源之分）。
func (r *QuestionRepositoryImpl) GetRandomByTypeYear(qType int8, count int, subjectID uint64, minYear int16) ([]model.Question, error) {
	var questions []model.Question
	if count <= 0 {
		return questions, nil
	}
	query := r.db.Where("question_type = ? AND status = 1", qType)
	if subjectID > 0 {
		query = query.Where("subject_id = ?", subjectID)
	}
	if minYear > 0 {
		query = query.Where("year >= ?", minYear)
	}
	// 只抽有选项的题：批量导入偶发漏掉选项，抽中会显示"无选项"——但仅对选择题(1/2)生效。
	// 案例(3)/论文(4) 按设计无选项，套用本过滤会让模拟卷的案例/论文整类抽 0 题。
	if isChoiceType(qType) {
		query = query.Where("EXISTS (SELECT 1 FROM options o WHERE o.question_id = questions.id)")
	}
	err := query.Order("RAND()").Limit(count).Preload("Options", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).Find(&questions).Error
	return questions, err
}

// isChoiceType 是否选择题（单选1 / 多选2）。案例(3)/论文(4) 无选项，
// 抽题与筛选选项存在性时必须按题型区分，否则整类抽空。
func isChoiceType(qType int8) bool {
	return qType == 1 || qType == 2
}

// Category 分类刷题的知识点归类一行：SUBSTRING_INDEX 切 main_cat，count 为归类下的题目数。
// gorm column 标签映射原始 SQL 列（SUBSTRING_INDEX 别名 main_cat/sub_cat、COUNT(*) 别名 cnt），
// json 标签保持前端契约（count 而非 cnt）。
type Category struct {
	MainCat      string `gorm:"column:main_cat" json:"main_cat"`
	SubCat       string `gorm:"column:sub_cat" json:"sub_cat"`
	QuestionType int8   `gorm:"column:question_type" json:"question_type"`
	Count        int64  `gorm:"column:cnt" json:"count"`
}

func (r *QuestionRepositoryImpl) GetCategories(subjectID uint64) ([]Category, error) {
	// 初始化为空 slice 而非声明 nil：零行时 Scan 不做分配，nil slice 序列化成 JSON null，
	// 破坏「空分类列表返回 []」的契约（前端 for..of 会崩）。初始化后零行保持 `[]`。
	categories := []Category{}

	query := `
		SELECT
			SUBSTRING_INDEX(knowledge_point, '>', 1) as main_cat,
			knowledge_point as sub_cat,
			question_type,
			COUNT(*) as cnt
		FROM questions
		WHERE knowledge_point IS NOT NULL AND knowledge_point != '' AND status = 1
	`

	if subjectID > 0 {
		query += " AND subject_id = ?"
	}

	query += ` GROUP BY main_cat, sub_cat, question_type ORDER BY main_cat, sub_cat, question_type`

	// subjectID=0 时 SQL 无占位符，不能传冗余参数（database/sql 会报参数个数不匹配）
	var err error
	if subjectID > 0 {
		err = r.db.Raw(query, subjectID).Scan(&categories).Error
	} else {
		err = r.db.Raw(query).Scan(&categories).Error
	}
	return categories, err
}
