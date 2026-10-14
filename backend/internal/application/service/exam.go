package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

type ExamService struct {
	paperRepo    repository.PaperRepository
	questionRepo repository.QuestionRepository
	examRepo     repository.ExamRecordRepository
	// practiceSvc 用于交卷后把每道已作答的题记入学习流水/错题本。
	// 可为 nil（测试单测 ExamService 时），nil 时跳过记录，不影响判分主流程。
	practiceSvc *PracticeService
	// paperSessionRepo 存真题练习模式的断点草稿（可覆盖，不影响统计）。
	paperSessionRepo repository.PaperPracticeSessionRepository
}

func NewExamService(
	paperRepo repository.PaperRepository,
	questionRepo repository.QuestionRepository,
	examRepo repository.ExamRecordRepository,
	practiceSvc *PracticeService,
	paperSessionRepo repository.PaperPracticeSessionRepository,
) *ExamService {
	return &ExamService{
		paperRepo:        paperRepo,
		questionRepo:     questionRepo,
		examRepo:         examRepo,
		practiceSvc:      practiceSvc,
		paperSessionRepo: paperSessionRepo,
	}
}

func (s *ExamService) ListPapers(paperType int8, page, pageSize int, subjectID uint64) ([]model.Paper, int64, error) {
	return s.paperRepo.List(paperType, page, pageSize, subjectID)
}

type PaperWithQuestions struct {
	model.Paper
	Questions []model.Question `json:"questions"`
}

func (s *ExamService) GetPaperWithQuestions(id uint64) (*PaperWithQuestions, error) {
	paper, err := s.paperRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	questions, err := s.paperRepo.FindQuestionsByPaperID(id)
	if err != nil {
		return nil, err
	}
	return &PaperWithQuestions{
		Paper:    *paper,
		Questions: questions,
	}, nil
}

// StartExam 开始考试。同一用户同一套卷子若已有未交卷的记录则直接复用，
// 不再新建 —— 否则考试中途刷新浏览器就会多出一条永远不会被交卷的僵尸记录。
func (s *ExamService) StartExam(userID, paperID uint64) (*model.ExamRecord, error) {
	if _, err := s.paperRepo.FindByID(paperID); err != nil {
		return nil, err
	}

	if existing, err := s.examRepo.FindInProgress(userID, paperID); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}

	questions, err := s.paperRepo.FindQuestionsByPaperID(paperID)
	if err != nil {
		return nil, err
	}

	record := &model.ExamRecord{
		UserID:         userID,
		PaperID:        paperID,
		TotalQuestions: len(questions),
		StartedAt:      time.Now(),
		Status:         0,
	}
	if err := s.examRepo.Create(record); err != nil {
		return nil, err
	}
	return record, nil
}

type ExamAnswer struct {
	QuestionID uint64 `json:"question_id"`
	Answer     string `json:"answer"`
}

type ExamResult struct {
	ExamRecordID uint64  `json:"exam_record_id"`
	Score        float32 `json:"score"`
	TotalCount   int     `json:"total_count"`
	CorrectCount int     `json:"correct_count"`
	TimeSpent    int     `json:"time_spent"`
}

// examPracticeType 交卷后记入 practice_records 的练习来源。
// 练习来源枚举见 001_init.sql：6=模拟考试（真题卷与模拟卷共用此项）。
const examPracticeType int8 = 6

// SubmitExam 交卷判分。
// 分母取试卷上「可自动判分的题目」总数（即有选项的客观题），未作答按错处理，
// 避免只提交已答题目时出现「答 1 题得 100 分」；论文/案例等无选项的主观题无法机判，不计入分母。
// 同一场考试只能交卷一次，由 FinishOnce 的条件更新保证。
func (s *ExamService) SubmitExam(userID, examRecordID uint64, answers []ExamAnswer) (*ExamResult, error) {
	record, err := s.examRepo.FindByID(examRecordID)
	if err != nil {
		return nil, errors.New("考试记录不存在")
	}
	if record.UserID != userID {
		return nil, errors.New("无权提交该考试")
	}
	if record.Status != 0 {
		return nil, errors.New("该场考试已交卷，不能重复提交")
	}

	questions, err := s.paperRepo.FindQuestionsByPaperID(record.PaperID)
	if err != nil {
		return nil, err
	}

	// 只认试卷上的题，忽略请求里夹带的其它题号
	submitted := make(map[uint64]string, len(answers))
	for _, a := range answers {
		submitted[a.QuestionID] = a.Answer
	}

	// 第一遍：只判分、收集每题结果，不动数据库。
	// 判分口径与之前一致（选项 label 直接拼接比对），未作答按错处理。
	type gradedItem struct {
		questionID uint64
		subjectID  uint64
		userAnswer string
		answered   bool
		isCorrect  bool
	}
	gradable, correctCount := 0, 0
	var graded []gradedItem
	for _, q := range questions {
		correctAnswer := ""
		for _, opt := range q.Options {
			if opt.IsCorrect == 1 {
				correctAnswer += opt.Label
			}
		}
		if correctAnswer == "" {
			continue // 主观题或未标注答案的题，跳过不计分
		}
		userAnswer := submitted[q.ID]
		answered := userAnswer != ""
		isCorrect := answered && userAnswer == correctAnswer
		graded = append(graded, gradedItem{
			questionID: q.ID,
			subjectID:  q.SubjectID,
			userAnswer: userAnswer,
			answered:   answered,
			isCorrect:  isCorrect,
		})
		gradable++
		if isCorrect {
			correctCount++
		}
	}

	record.CorrectCount = correctCount
	record.TotalQuestions = gradable
	record.Score = 0
	if gradable > 0 {
		record.Score = float32(correctCount) / float32(gradable) * 100
	}
	now := time.Now()
	record.FinishedAt = &now
	record.TimeSpent = int(now.Sub(record.StartedAt).Seconds())

	ok, err := s.examRepo.FinishOnce(record)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("该场考试已交卷，不能重复提交")
	}

	// 第二遍：交卷落库成功后再逐题记入学习流水/错题本（best-effort）。
	// 只记「已作答」的题：未作答的题按错计分，但不应把整卷跳过的题灌进错题本。
	// 这样考试模式的做题也会进入学习报告，与练习模式共用一套 practice_records 数据。
	if s.practiceSvc != nil {
		for _, item := range graded {
			if !item.answered {
				continue
			}
			s.practiceSvc.RecordAnswer(userID, item.questionID, item.subjectID, record.PaperID,
				item.userAnswer, item.isCorrect, 0, examPracticeType)
		}
	}

	return &ExamResult{
		ExamRecordID: examRecordID,
		Score:        record.Score,
		TotalCount:   record.TotalQuestions,
		CorrectCount: correctCount,
		TimeSpent:    record.TimeSpent,
	}, nil
}

func (s *ExamService) GetExamRecords(userID uint64, page, pageSize int) ([]model.ExamRecord, int64, error) {
	return s.examRepo.FindByUserID(userID, page, pageSize)
}

// MockSpec 定义模拟卷各题型抽题数量（键为 question_type：1单选 2多选 3案例 4论文）
type MockSpec map[int8]int

// 各题型默认预估用时（分钟/题），用于估算模拟卷时长
var mockMinutesPerType = map[int8]int{1: 2, 2: 3, 3: 12, 4: 40}

// 题型显示名，用于组装模拟卷标题
var mockTypeName = map[int8]string{1: "单选", 2: "多选", 3: "案例", 4: "论文"}

// GenerateMockPaper 从近 years 年的真题中按题型随机抽题，生成一份模拟卷并落库，
// 返回新试卷 ID。生成前会清除该用户此前的模拟卷，避免累积。
// years<=0 表示不限年份。
func (s *ExamService) GenerateMockPaper(userID, subjectID uint64, spec MockSpec, years int) (uint64, error) {
	// 计算年份下限：近 years 年（含今年）
	var minYear int16
	if years > 0 {
		minYear = int16(time.Now().Year() - years + 1)
	}

	// 按题型顺序抽题，保持 单选→多选→案例→论文 的自然排布
	var picked []model.Question
	for _, qType := range []int8{1, 2, 3, 4} {
		count := spec[qType]
		if count <= 0 {
			continue
		}
		qs, err := s.questionRepo.GetRandomByTypeYear(qType, count, subjectID, minYear)
		if err != nil {
			return 0, err
		}
		picked = append(picked, qs...)
	}
	if len(picked) == 0 {
		return 0, errors.New("题库中没有符合条件的题目，请调整年份范围或减少题量")
	}

	// 组装标题（如「模拟卷 · 单选20 案例2」）、时长与总分
	var titleParts []string
	var totalScore float32
	duration := 0
	typeCount := map[int8]int{}
	for _, q := range picked {
		typeCount[q.QuestionType]++
		totalScore += q.Score
	}
	for _, qType := range []int8{1, 2, 3, 4} {
		if n := typeCount[qType]; n > 0 {
			titleParts = append(titleParts, fmt.Sprintf("%s%d", mockTypeName[qType], n))
			duration += n * mockMinutesPerType[qType]
		}
	}
	if duration < 15 {
		duration = 15
	}
	if totalScore <= 0 {
		totalScore = float32(len(picked)) // 兜底：真题未标注分值时按每题 1 分
	}

	// 先清理旧模拟卷，再创建新卷
	if err := s.paperRepo.DeleteMockByUser(userID); err != nil {
		return 0, err
	}

	paper := &model.Paper{
		SubjectID:   subjectID,
		Title:       "模拟卷 · " + strings.Join(titleParts, " "),
		Year:        int16(time.Now().Year()),
		Batch:       0,
		PaperType:   1,
		TotalScore:  totalScore,
		Duration:    duration,
		Description: "系统随机抽取历年真题生成的模拟卷",
		Status:      1,
		Source:      1,
		CreatedBy:   userID,
	}
	ids := make([]uint64, 0, len(picked))
	for _, q := range picked {
		ids = append(ids, q.ID)
	}
	if err := s.paperRepo.CreateWithQuestions(paper, ids); err != nil {
		return 0, err
	}
	return paper.ID, nil
}

// ---- 管理端：试卷管理 ----

// PaperRequest 管理端创建/更新试卷的请求体。
// Status 用指针：0=下线也是合法值，「未传则不改」与「显式置 0」要靠 nil 区分。
type PaperRequest struct {
	SubjectID   uint64  `json:"subject_id"`
	Title       string  `json:"title"`
	Year        int16   `json:"year"`
	Batch       int8    `json:"batch"`
	PaperType   int8    `json:"paper_type"`
	TotalScore  float32 `json:"total_score"`
	Duration    int     `json:"duration"`
	Description string  `json:"description"`
	Status      *int8   `json:"status"`
}

func (s *ExamService) ListAdminPapers(req repository.AdminPaperListReq) ([]model.Paper, int64, error) {
	papers, total, err := s.paperRepo.ListAdmin(req)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]uint64, 0, len(papers))
	for _, p := range papers {
		ids = append(ids, p.ID)
	}
	counts, err := s.paperRepo.CountQuestionsByPaperIDs(ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range papers {
		papers[i].QuestionCount = counts[papers[i].ID]
	}
	return papers, total, nil
}

// CreatePaper 管理端新建真题卷。后台创建的试卷一律 source=0（真题），
// 模拟卷只由用户侧生成。
func (s *ExamService) CreatePaper(adminID uint64, req *PaperRequest) (uint64, error) {
	if strings.TrimSpace(req.Title) == "" {
		return 0, errors.New("试卷标题不能为空")
	}
	if req.Year <= 0 {
		return 0, errors.New("请填写试卷年份")
	}
	if req.PaperType < 1 || req.PaperType > 4 {
		return 0, errors.New("请选择正确的试卷类型")
	}
	status := int8(1)
	if req.Status != nil {
		status = *req.Status
	}
	subjectID := req.SubjectID
	if subjectID == 0 {
		subjectID = 1
	}
	duration := req.Duration
	if duration <= 0 {
		duration = 150
	}
	paper := &model.Paper{
		SubjectID:   subjectID,
		Title:       strings.TrimSpace(req.Title),
		Year:        req.Year,
		Batch:       req.Batch,
		PaperType:   req.PaperType,
		TotalScore:  req.TotalScore,
		Duration:    duration,
		Description: req.Description,
		Status:      status,
		Source:      0,
		CreatedBy:   adminID,
	}
	if err := s.paperRepo.Create(paper); err != nil {
		return 0, err
	}
	return paper.ID, nil
}

// UpdatePaper 部分更新：非零值才覆盖。Status 用指针单独处理，允许置回 0（下线）。
func (s *ExamService) UpdatePaper(id uint64, req *PaperRequest) error {
	paper, err := s.paperRepo.FindByID(id)
	if err != nil {
		return errors.New("试卷不存在")
	}
	if strings.TrimSpace(req.Title) != "" {
		paper.Title = strings.TrimSpace(req.Title)
	}
	if req.Year > 0 {
		paper.Year = req.Year
	}
	if req.Batch > 0 {
		paper.Batch = req.Batch
	}
	if req.PaperType > 0 {
		if req.PaperType < 1 || req.PaperType > 4 {
			return errors.New("请选择正确的试卷类型")
		}
		paper.PaperType = req.PaperType
	}
	if req.TotalScore > 0 {
		paper.TotalScore = req.TotalScore
	}
	if req.Duration > 0 {
		paper.Duration = req.Duration
	}
	if req.Description != "" {
		paper.Description = req.Description
	}
	if req.SubjectID > 0 {
		paper.SubjectID = req.SubjectID
	}
	if req.Status != nil {
		paper.Status = *req.Status
	}
	return s.paperRepo.Update(paper)
}

// DeletePaper 删除整卷（含题目关联与考试记录）。
func (s *ExamService) DeletePaper(id uint64) error {
	if _, err := s.paperRepo.FindByID(id); err != nil {
		return errors.New("试卷不存在")
	}
	return s.paperRepo.DeleteFull(id)
}

func (s *ExamService) GetPaperQuestions(id uint64) ([]model.Question, error) {
	if _, err := s.paperRepo.FindByID(id); err != nil {
		return nil, errors.New("试卷不存在")
	}
	return s.paperRepo.FindQuestionsByPaperID(id)
}

// ReplacePaperQuestions 保存卷内题目编排：去重保序后整删整插。
// 落库前校验题目都存在，避免管理员引用了已删除的题。
func (s *ExamService) ReplacePaperQuestions(id uint64, questionIDs []uint64) error {
	if _, err := s.paperRepo.FindByID(id); err != nil {
		return errors.New("试卷不存在")
	}
	// 去重保序：前端多选可能重复勾选，同一题在一份卷里只应出现一次
	seen := make(map[uint64]bool, len(questionIDs))
	unique := make([]uint64, 0, len(questionIDs))
	for _, qid := range questionIDs {
		if qid == 0 || seen[qid] {
			continue
		}
		seen[qid] = true
		unique = append(unique, qid)
	}
	if len(unique) > 0 {
		questions, err := s.questionRepo.FindByIDs(unique)
		if err != nil {
			return err
		}
		if len(questions) != len(unique) {
			return errors.New("部分题目不存在，请刷新题库后重试")
		}
	}
	return s.paperRepo.ReplaceQuestions(id, unique)
}

// ---- 真题练习模式断点草稿 ----

// PaperSessionResult 一道题的判题结果（主观题 is_correct 为 null，用 *bool 保真）。
type PaperSessionResult struct {
	IsCorrect     *bool  `json:"is_correct"`
	CorrectAnswer string `json:"correctAnswer"`
}

// PaperSessionData 一份真题练习草稿的对外形态，与前端 ExamCache 一致（去掉 exam 才用的 timer）。
type PaperSessionData struct {
	Answers         map[string]map[string]string `json:"answers"`
	PracticeResults map[string]PaperSessionResult `json:"practiceResults"`
	CurrentIndex    int                           `json:"currentIndex"`
}

// GetPracticeSessionState 读练习草稿。无会话、非 practice 模式或草稿损坏返回 (nil, false)。
func (s *ExamService) GetPracticeSessionState(userID, paperID uint64, mode string) (*PaperSessionData, bool) {
	if mode != "practice" {
		return nil, false
	}
	sess, err := s.paperSessionRepo.FindOne(userID, paperID, mode)
	if err != nil {
		return nil, false
	}
	var data PaperSessionData
	if err := json.Unmarshal([]byte(sess.Data), &data); err != nil {
		return nil, false
	}
	return &data, true
}

// SavePracticeSession 落练习草稿。校验 mode 与试卷存在，数据序列化后 upsert。
func (s *ExamService) SavePracticeSession(userID, paperID uint64, mode string, payload *PaperSessionData) error {
	if mode != "practice" {
		return errors.New("暂不支持该模式")
	}
	if _, err := s.paperRepo.FindByID(paperID); err != nil {
		return errors.New("试卷不存在")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.paperSessionRepo.Save(&model.PaperPracticeSession{
		UserID:  userID,
		PaperID: paperID,
		Mode:    mode,
		Data:    string(data),
	})
}

// ClearPracticeSession 清练习草稿（「从头开始」/完成时）。
func (s *ExamService) ClearPracticeSession(userID, paperID uint64, mode string) error {
	return s.paperSessionRepo.Delete(userID, paperID, mode)
}
