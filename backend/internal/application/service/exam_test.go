package service

import (
	"testing"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

type stubPaperRepo struct{ questions []model.Question }

func (s *stubPaperRepo) FindByID(id uint64) (*model.Paper, error) {
	return &model.Paper{ID: id}, nil
}
func (s *stubPaperRepo) FindQuestionsByPaperID(uint64) ([]model.Question, error) {
	return s.questions, nil
}
func (s *stubPaperRepo) Create(*model.Paper) error                        { panic("unused") }
func (s *stubPaperRepo) CreateWithQuestions(*model.Paper, []uint64) error { panic("unused") }
func (s *stubPaperRepo) Update(*model.Paper) error                        { panic("unused") }
func (s *stubPaperRepo) Delete(uint64) error                              { panic("unused") }
func (s *stubPaperRepo) DeleteMockByUser(uint64) error                    { panic("unused") }
func (s *stubPaperRepo) List(int8, int, int, uint64) ([]model.Paper, int64, error) {
	panic("unused")
}
func (s *stubPaperRepo) ListAdmin(repository.AdminPaperListReq) ([]model.Paper, int64, error) {
	panic("unused")
}
func (s *stubPaperRepo) CountQuestionsByPaperIDs([]uint64) (map[uint64]int64, error) {
	panic("unused")
}
func (s *stubPaperRepo) ReplaceQuestions(uint64, []uint64) error { panic("unused") }
func (s *stubPaperRepo) DeleteFull(uint64) error                 { panic("unused") }

type stubExamRepo struct {
	records     []*model.ExamRecord
	nextID      uint64
	total       int64
	gotPage     int
	gotPageSize int
}

func (s *stubExamRepo) Create(r *model.ExamRecord) error {
	s.nextID++
	r.ID = s.nextID
	s.records = append(s.records, r)
	return nil
}
func (s *stubExamRepo) FindByID(id uint64) (*model.ExamRecord, error) {
	for _, r := range s.records {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, errNotFound
}
func (s *stubExamRepo) FindInProgress(userID, paperID uint64) (*model.ExamRecord, error) {
	for i := len(s.records) - 1; i >= 0; i-- {
		r := s.records[i]
		if r.UserID == userID && r.PaperID == paperID && r.Status == 0 {
			return r, nil
		}
	}
	return nil, nil
}
func (s *stubExamRepo) Update(*model.ExamRecord) error { panic("unused") }
func (s *stubExamRepo) FinishOnce(r *model.ExamRecord) (bool, error) {
	for _, rec := range s.records {
		if rec.ID == r.ID && rec.Status == 0 {
			rec.Status = 1
			return true, nil
		}
	}
	return false, nil
}
func (s *stubExamRepo) FindByUserID(userID uint64, page, pageSize int) ([]model.ExamRecord, int64, error) {
	s.gotPage, s.gotPageSize = page, pageSize
	out := make([]model.ExamRecord, 0, len(s.records))
	for _, r := range s.records {
		out = append(out, *r)
	}
	return out, s.total, nil
}

// 考试中途刷新浏览器会重新走一遍 StartExam。原实现每次都 Create，
// 于是每刷新一次就多一条永远不会被交卷的僵尸记录。
func TestStartExamReusesInProgressRecord(t *testing.T) {
	examRepo := &stubExamRepo{}
	svc := NewExamService(&stubPaperRepo{questions: make([]model.Question, 5)}, nil, examRepo, nil, &stubPaperSessionRepo{})

	first, err := svc.StartExam(1, 100)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.StartExam(1, 100) // 模拟刷新
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("刷新应复用同一条记录，得到 %d 和 %d", first.ID, second.ID)
	}
	if len(examRepo.records) != 1 {
		t.Fatalf("不应产生僵尸记录，实际 %d 条", len(examRepo.records))
	}
}

// 交完卷再考同一套卷子必须是新的一场，否则成绩会覆盖上一次。
func TestStartExamCreatesNewRecordAfterFinish(t *testing.T) {
	examRepo := &stubExamRepo{}
	svc := NewExamService(&stubPaperRepo{questions: make([]model.Question, 5)}, nil, examRepo, nil, &stubPaperSessionRepo{})

	first, _ := svc.StartExam(1, 100)
	examRepo.FinishOnce(first)

	second, err := svc.StartExam(1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == first.ID {
		t.Fatal("已交卷的记录不应被复用")
	}
}

// 不同用户各自独立，不能复用别人的记录。
func TestStartExamIsolatesUsers(t *testing.T) {
	examRepo := &stubExamRepo{}
	svc := NewExamService(&stubPaperRepo{questions: make([]model.Question, 5)}, nil, examRepo, nil, &stubPaperSessionRepo{})

	a, _ := svc.StartExam(1, 100)
	b, _ := svc.StartExam(2, 100)
	if a.ID == b.ID {
		t.Fatal("不同用户应各自建立记录")
	}
}

// 交卷后已作答的客观题要逐题记入学习流水与错题本（bug1 修复）：
// 答对进流水不进错题本，答错进流水也进错题本；未作答的题只按错计分、
// 不写流水，避免整卷跳过时把错题本/做题记录灌满；主观题不进流水。
func TestSubmitExamRecordsAnswersAndWrongQuestions(t *testing.T) {
	paperQ := []model.Question{
		{ID: 1, SubjectID: 2, QuestionType: 1, Options: []model.Option{{Label: "A"}, {Label: "B", IsCorrect: 1}}},
		{ID: 2, SubjectID: 2, QuestionType: 1, Options: []model.Option{{Label: "A", IsCorrect: 1}, {Label: "B"}}},
		{ID: 3, SubjectID: 2, QuestionType: 1, Options: []model.Option{{Label: "A", IsCorrect: 1}, {Label: "B"}}},
		{ID: 4, SubjectID: 2, QuestionType: 3}, // 案例题，不计分也不记录
	}

	pr := &stubPracticeRepo{}
	wr := &stubWrongRepo{}
	svc := NewExamService(&stubPaperRepo{questions: paperQ}, nil, &stubExamRepo{},
		NewPracticeService(&fakeQuestionRepo{}, wr, pr,
			&stubProfileRepo{p: &model.UserProfile{UserID: 1}}, &stubSubjectStatRepo{}, &stubSessionRepo{}, &stubPracticeQueryRepo{}),
		&stubPaperSessionRepo{})

	record, err := svc.StartExam(1, 100)
	if err != nil {
		t.Fatal(err)
	}
	// 第1题答对(B)、第2题答错(B)、第3题跳过不答
	res, err := svc.SubmitExam(1, record.ID, []ExamAnswer{
		{QuestionID: 1, Answer: "B"},
		{QuestionID: 2, Answer: "B"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCount != 3 || res.CorrectCount != 1 {
		t.Fatalf("判分有误: total=%d correct=%d", res.TotalCount, res.CorrectCount)
	}

	// 只记已作答的 2 题；跳过与主观题都不写流水
	if len(pr.records) != 2 {
		t.Fatalf("应只记录已作答的 2 题，实际 %d 条", len(pr.records))
	}
	// 只 1 道错题进错题本
	if wr.created != 1 {
		t.Fatalf("应只有 1 道错题进错题本，实际 %d", wr.created)
	}
	// 流水应带上 paper_id，方便回溯是哪场考试
	for _, r := range pr.records {
		if r.PaperID != record.PaperID {
			t.Fatalf("流水应记录 paper_id=%d，实际 %d", record.PaperID, r.PaperID)
		}
	}
}

// GetExamRecords 是分页 delegate：page/pageSize 原样交给 repo，total 透传。
func TestGetExamRecords_PassesPagination(t *testing.T) {
	examRepo := &stubExamRepo{records: []*model.ExamRecord{{ID: 1, UserID: 1, PaperID: 100, Status: 1}}}
	examRepo.total = 5
	svc := NewExamService(&stubPaperRepo{}, nil, examRepo, nil, &stubPaperSessionRepo{})

	records, total, err := svc.GetExamRecords(1, 2, 5)
	if err != nil {
		t.Fatalf("GetExamRecords 应无错，得到 %v", err)
	}
	if total != 5 || len(records) != 1 {
		t.Fatalf("应透传 total=5 / 1 条，实际 total=%d len=%d", total, len(records))
	}
	if examRepo.gotPage != 2 || examRepo.gotPageSize != 5 {
		t.Fatalf("分页参数未透传给 repo: page=%d size=%d", examRepo.gotPage, examRepo.gotPageSize)
	}
}
