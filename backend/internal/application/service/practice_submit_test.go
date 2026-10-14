package service

import (
	"errors"
	"testing"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

var errNotFound = errors.New("not found")

type stubPracticeRepo struct{ records []model.PracticeRecord }

func (s *stubPracticeRepo) Create(r *model.PracticeRecord) error {
	s.records = append(s.records, *r)
	return nil
}
func (s *stubPracticeRepo) FindByUserID(uint64, int8, int, int) ([]model.PracticeRecord, int64, error) {
	panic("unused")
}

// stubWrongRepo 分页列表桩：FindByUserID/FindDueReview 记录收到的分页参数，
// 返回调用方填好的 wqs/total（practice_list_test 用它锁透传契约）。
type stubWrongRepo struct {
	created     int
	wqs         []model.WrongQuestion
	total       int64
	gotStatus   int8
	gotPage     int
	gotPageSize int
}

func (s *stubWrongRepo) Create(*model.WrongQuestion) error { s.created++; return nil }
func (s *stubWrongRepo) FindByUserAndQuestion(uint64, uint64) (*model.WrongQuestion, error) {
	return nil, errNotFound
}
func (s *stubWrongRepo) Update(*model.WrongQuestion) error { return nil }
func (s *stubWrongRepo) Delete(uint64, uint64) error       { panic("unused") }
func (s *stubWrongRepo) FindByUserID(userID uint64, status int8, page, pageSize int) ([]model.WrongQuestion, int64, error) {
	s.gotStatus, s.gotPage, s.gotPageSize = status, page, pageSize
	return s.wqs, s.total, nil
}
func (s *stubWrongRepo) FindDueReview(userID uint64, page, pageSize int) ([]model.WrongQuestion, int64, error) {
	s.gotPage, s.gotPageSize = page, pageSize
	return s.wqs, s.total, nil
}

type stubProfileRepo struct{ p *model.UserProfile }

func (s *stubProfileRepo) Create(p *model.UserProfile) error { s.p = p; return nil }
func (s *stubProfileRepo) FindByUserID(uint64) (*model.UserProfile, error) {
	if s.p == nil {
		return nil, errNotFound
	}
	return s.p, nil
}
func (s *stubProfileRepo) Update(p *model.UserProfile) error { s.p = p; return nil }

type stubSubjectStatRepo struct {
	stats map[uint64]*model.UserSubjectStat
}

func (s *stubSubjectStatRepo) FindOne(_, subjectID uint64) (*model.UserSubjectStat, error) {
	if st, ok := s.stats[subjectID]; ok {
		return st, nil
	}
	return nil, errNotFound
}

func (s *stubSubjectStatRepo) Create(st *model.UserSubjectStat) error { return s.Save(st) }

func (s *stubSubjectStatRepo) Save(st *model.UserSubjectStat) error {
	if s.stats == nil {
		s.stats = map[uint64]*model.UserSubjectStat{}
	}
	s.stats[st.SubjectID] = st
	return nil
}

func newSubmitSvc(q *model.Question) (*PracticeService, *stubPracticeRepo, *stubWrongRepo, *stubProfileRepo) {
	qr := &fakeQuestionRepo{q: q}
	pr := &stubPracticeRepo{}
	wr := &stubWrongRepo{}
	pf := &stubProfileRepo{p: &model.UserProfile{UserID: 1}}
	return NewPracticeService(qr, wr, pr, pf, &stubSubjectStatRepo{}, &stubSessionRepo{}, &stubPracticeQueryRepo{}), pr, wr, pf
}

// stubSessionRepo 断点草稿不参与判分，提交路径上给个空实现即可。
type stubSessionRepo struct{}

func (s *stubSessionRepo) FindOne(uint64, uint64, int8, string) (*model.PracticeSession, error) {
	return nil, errNotFound
}
func (s *stubSessionRepo) Save(*model.PracticeSession) error         { return nil }
func (s *stubSessionRepo) Delete(uint64, uint64, int8, string) error { return nil }

// stubPracticeQueryRepo 统计口径查询的桩。practice 单测大多不触达这些方法,
// 默认返回零值;需要特定行的测试(GetAccuracy 等)直接改对应字段。
type stubPracticeQueryRepo struct {
	acc      repository.AccuracyRow
	progr    []repository.ProgressRow
	trend    []repository.TrendRow
	kp       []repository.KpRow
	speed    repository.SpeedRow
	weak     []repository.WeaknessRow
	counts   []repository.CategoryCountRow
	stats    repository.WrongBookStats
	samples  []repository.WrongSampleRow
	accErr   error
	progrErr error
	trendErr error
	kpErr    error
	speedErr error
	weakErr  error
}

func (s *stubPracticeQueryRepo) AccuracyByUser(uint64) (repository.AccuracyRow, error) {
	return s.acc, s.accErr
}
func (s *stubPracticeQueryRepo) CategoryProgress(uint64) ([]repository.ProgressRow, error) {
	return s.progr, s.progrErr
}
func (s *stubPracticeQueryRepo) AccuracyBySubject(uint64, uint64) (repository.AccuracyRow, error) {
	return s.acc, s.accErr
}
func (s *stubPracticeQueryRepo) TrendBySubject(uint64, uint64) ([]repository.TrendRow, error) {
	return s.trend, s.trendErr
}
func (s *stubPracticeQueryRepo) KnowledgeAccuracyBySubject(uint64, uint64) ([]repository.KpRow, error) {
	return s.kp, s.kpErr
}
func (s *stubPracticeQueryRepo) SpeedBySubject(uint64, uint64, uint64, uint64) (repository.SpeedRow, error) {
	return s.speed, s.speedErr
}
func (s *stubPracticeQueryRepo) SpeedByUser(uint64, uint64, uint64) (repository.SpeedRow, error) {
	return s.speed, s.speedErr
}
func (s *stubPracticeQueryRepo) CategoryAccuracyBySubject(uint64, uint64) ([]repository.WeaknessRow, error) {
	return s.weak, s.weakErr
}
func (s *stubPracticeQueryRepo) CategoryQuestionCountsBySubject(uint64) ([]repository.CategoryCountRow, error) {
	return s.counts, nil
}
func (s *stubPracticeQueryRepo) WrongBookStatsBySubject(uint64, uint64) (repository.WrongBookStats, error) {
	return s.stats, nil
}
func (s *stubPracticeQueryRepo) RecentWrongSamplesBySubject(uint64, uint64, int) ([]repository.WrongSampleRow, error) {
	return s.samples, nil
}

// newPracticeQuerySvc 构造一个带 stubPracticeQueryRepo 的 PracticeService,
// 供统计口径相关测试(GetAccuracy/GetCategoryProgress/GetReport/Speed)使用。
func newPracticeQuerySvc(pq *stubPracticeQueryRepo) *PracticeService {
	return NewPracticeService(&fakeQuestionRepo{}, &stubWrongRepo{}, &stubPracticeRepo{},
		&stubProfileRepo{p: &model.UserProfile{UserID: 1}}, &stubSubjectStatRepo{}, &stubSessionRepo{}, pq)
}

// 案例/论文没有标准选项，字符串比对必然不等。原实现把它们一律判错，
// 既拉低正确率又把每道大题塞进错题本。
func TestSubjectiveQuestionNotGradedWrong(t *testing.T) {
	svc, pr, wr, pf := newSubmitSvc(&model.Question{ID: 1, QuestionType: 3})

	res, err := svc.SubmitAnswer(1, 1, "我的论述……", 5, 300)
	if err != nil {
		t.Fatal(err)
	}
	if res.Objective {
		t.Fatal("案例题不应被当作客观题判分")
	}
	if wr.created != 0 {
		t.Fatal("主观题不应进错题本")
	}
	// 学习时长/连续天数仍要累计 —— 写论文是耗时最长的练习，不记等于白练。
	// 正确率的分母不在这里，而在 latestAttemptsSQL（只取客观题）。
	if pf.p.TotalTime != 300 {
		t.Fatalf("主观题的学习时长应计入，实际 %d", pf.p.TotalTime)
	}
	if pf.p.TotalCorrect != 0 {
		t.Fatalf("主观题不应被算作答对，实际 %d", pf.p.TotalCorrect)
	}
	// 作答本身仍要留痕，否则用户看不到自己写过什么
	if len(pr.records) != 1 {
		t.Fatalf("作答记录应保留，实际 %d 条", len(pr.records))
	}
}

func TestObjectiveQuestionStillGraded(t *testing.T) {
	q := &model.Question{ID: 1, QuestionType: 1, Options: []model.Option{
		{Label: "A"}, {Label: "B", IsCorrect: 1},
	}}
	svc, _, wr, pf := newSubmitSvc(q)

	res, err := svc.SubmitAnswer(1, 1, "A", 1, 30)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Objective || res.IsCorrect {
		t.Fatalf("答错的单选题应判错: %+v", res)
	}
	if wr.created != 1 {
		t.Fatal("答错的客观题应进错题本")
	}
	if pf.p.TotalPractice != 1 || pf.p.TotalCorrect != 0 {
		t.Fatalf("统计有误: %+v", pf.p)
	}
}
