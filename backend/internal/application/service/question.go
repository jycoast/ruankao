package service

import (
	"strings"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

type QuestionService struct {
	questionRepo repository.QuestionRepository
}

func NewQuestionService(questionRepo repository.QuestionRepository) *QuestionService {
	return &QuestionService{questionRepo: questionRepo}
}

func (s *QuestionService) List(req repository.QuestionListReq) ([]model.Question, int64, error) {
	return s.questionRepo.List(req)
}

func (s *QuestionService) GetByID(id uint64) (*model.Question, error) {
	return s.questionRepo.FindByID(id)
}

func (s *QuestionService) GetRandom(qType int8, count int, knowledgePoint string, subjectID uint64) ([]model.Question, error) {
	return s.questionRepo.GetRandomQuestions(qType, count, knowledgePoint, subjectID)
}

func (s *QuestionService) GetCategories(subjectID uint64) ([]repository.Category, error) {
	return s.questionRepo.GetCategories(subjectID)
}

type QuestionOptionRequest struct {
	Label     string `json:"label"`
	Content   string `json:"content"`
	IsCorrect int8   `json:"is_correct"`
}

type QuestionRequest struct {
	Title          string                `json:"title"`
	Content        string                `json:"content"`
	QuestionType   int8                  `json:"question_type"`
	Difficulty     int8                  `json:"difficulty"`
	CategoryID     uint64                `json:"category_id"`
	SubjectID      uint64                `json:"subject_id"`
	KnowledgePoint string                `json:"knowledge_point"`
	Year           int16                 `json:"year"`
	PaperSection   string                `json:"paper_section"`
	Explanation    string                `json:"explanation"`
	Analysis       string                `json:"analysis"`
	Score          float32               `json:"score"`
	Options        []QuestionOptionRequest `json:"options"`
}

// toOptions 把请求里的选项转成模型；缺失标号按 A/B/C… 顺序补齐。
func toOptions(reqs []QuestionOptionRequest) []model.Option {
	opts := make([]model.Option, 0, len(reqs))
	for i, o := range reqs {
		label := strings.TrimSpace(o.Label)
		if label == "" {
			label = string(rune('A' + i))
		}
		opts = append(opts, model.Option{
			Label:     label,
			Content:   o.Content,
			IsCorrect: o.IsCorrect,
			SortOrder: i,
		})
	}
	return opts
}

func (s *QuestionService) Create(q *QuestionRequest) (uint64, error) {
	subjectID := q.SubjectID
	if subjectID == 0 {
		subjectID = 1
	}
	question := &model.Question{
		Title:          q.Title,
		Content:        q.Content,
		QuestionType:   q.QuestionType,
		Difficulty:     q.Difficulty,
		CategoryID:     q.CategoryID,
		SubjectID:      subjectID,
		KnowledgePoint: q.KnowledgePoint,
		Year:           q.Year,
		PaperSection:   q.PaperSection,
		Explanation:    q.Explanation,
		Analysis:       q.Analysis,
		Score:          q.Score,
		Status:         1,
	}
	if err := s.questionRepo.Create(question); err != nil {
		return 0, err
	}
	if len(q.Options) > 0 {
		if err := s.questionRepo.ReplaceOptions(question.ID, toOptions(q.Options)); err != nil {
			return question.ID, err
		}
	}
	return question.ID, nil
}

func (s *QuestionService) Update(id uint64, q *QuestionRequest) error {
	question, err := s.questionRepo.FindByID(id)
	if err != nil {
		return err
	}
	if q.Title != "" {
		question.Title = q.Title
	}
	if q.Content != "" {
		question.Content = q.Content
	}
	if q.QuestionType > 0 {
		question.QuestionType = q.QuestionType
	}
	if q.Difficulty > 0 {
		question.Difficulty = q.Difficulty
	}
	if q.Explanation != "" {
		question.Explanation = q.Explanation
	}
	if q.Analysis != "" {
		question.Analysis = q.Analysis
	}
	if q.Score > 0 {
		question.Score = q.Score
	}
	if q.SubjectID > 0 {
		question.SubjectID = q.SubjectID
	}
	if q.KnowledgePoint != "" {
		question.KnowledgePoint = q.KnowledgePoint
	}
	if q.Year > 0 {
		question.Year = q.Year
	}
	if q.PaperSection != "" {
		question.PaperSection = q.PaperSection
	}
	if q.CategoryID > 0 {
		question.CategoryID = q.CategoryID
	}
	if err := s.questionRepo.Update(question); err != nil {
		return err
	}
	// nil 表示本次不改选项；空数组才是「清空选项」。
	// 爬来的题正确答案标错时，这里是唯一的纠正入口。
	if q.Options != nil {
		return s.questionRepo.ReplaceOptions(id, toOptions(q.Options))
	}
	return nil
}

func (s *QuestionService) Delete(id uint64) error {
	return s.questionRepo.Delete(id)
}

// UpdateSort 按管理员拖拽后的顺序（ids 即新顺序）批量重排 sort_order。
func (s *QuestionService) UpdateSort(ids []uint64) error {
	return s.questionRepo.UpdateSortOrders(ids)
}
