package service

import (
	"encoding/json"
	"testing"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

// sessionQuestionRepo 只实现续做用到的两个方法。
// FindByIDs 特意按 id 升序返回而非按传入顺序 —— 真实的 IN 查询就是这样，
// 服务层必须自己把顺序还原回草稿里存的样子。
type sessionQuestionRepo struct {
	all    map[uint64]model.Question
	random []model.Question
	// drawnCount 记录最近一次抽题请求的数量参数，供测试断言「不限量」
	drawnCount int
}

func (f *sessionQuestionRepo) FindByIDs(ids []uint64) ([]model.Question, error) {
	var out []model.Question
	for id := uint64(1); id <= 100; id++ {
		for _, want := range ids {
			if want == id {
				if q, ok := f.all[id]; ok {
					out = append(out, q)
				}
			}
		}
	}
	return out, nil
}

func (f *sessionQuestionRepo) GetRandomQuestions(_ int8, count int, _ string, _ uint64) ([]model.Question, error) {
	f.drawnCount = count
	return f.random, nil
}
func (f *sessionQuestionRepo) GetSeededRandomQuestions(int8, int, string, uint64, uint32) ([]model.Question, error) {
	panic("unused")
}

// CountQuestions 用题库全量（all 的条数）当候选集题数。测试里 all 代表当前全量题库，
// 残缺草稿（题数对不上）就能被「继续刷题」识别为过期。
func (f *sessionQuestionRepo) CountQuestions(int8, string, uint64) (int64, error) {
	return int64(len(f.all)), nil
}

func (f *sessionQuestionRepo) Create(*model.Question) error              { panic("unused") }
func (f *sessionQuestionRepo) FindByID(uint64) (*model.Question, error)  { panic("unused") }
func (f *sessionQuestionRepo) Update(*model.Question) error              { panic("unused") }
func (f *sessionQuestionRepo) ReplaceOptions(uint64, []model.Option) error { panic("unused") }
func (f *sessionQuestionRepo) Delete(uint64) error                       { panic("unused") }
func (f *sessionQuestionRepo) List(repository.QuestionListReq) ([]model.Question, int64, error) {
	panic("unused")
}
func (f *sessionQuestionRepo) GetRandomByTypeYear(int8, int, uint64, int16) ([]model.Question, error) {
	panic("unused")
}
func (f *sessionQuestionRepo) GetCategories(uint64) ([]repository.Category, error) {
	panic("unused")
}
func (f *sessionQuestionRepo) UpdateSortOrders([]uint64) error { panic("unused") }

type memSessionRepo struct{ s *model.PracticeSession }

func (m *memSessionRepo) FindOne(uint64, uint64, int8, string) (*model.PracticeSession, error) {
	if m.s == nil {
		return nil, errNotFound
	}
	return m.s, nil
}
func (m *memSessionRepo) Save(s *model.PracticeSession) error { m.s = s; return nil }
func (m *memSessionRepo) Delete(uint64, uint64, int8, string) error {
	m.s = nil
	return nil
}

func newSessionSvc(qr *sessionQuestionRepo, sr *memSessionRepo) *PracticeService {
	return NewPracticeService(qr, &stubWrongRepo{}, &stubPracticeRepo{},
		&stubProfileRepo{p: &model.UserProfile{UserID: 1}}, &stubSubjectStatRepo{}, sr, &stubPracticeQueryRepo{})
}

func questionSet(ids ...uint64) map[uint64]model.Question {
	m := map[uint64]model.Question{}
	for _, id := range ids {
		m[id] = model.Question{ID: id}
	}
	return m
}

// 「继续刷题」必须还原上次的题序。FindByIDs 走 IN 查询，
// 返回顺序由数据库决定，直接拿来用会让用户看到一份重新洗过牌的题目。
func TestResumeKeepsOriginalOrder(t *testing.T) {
	qr := &sessionQuestionRepo{all: questionSet(3, 7, 9)}
	sr := &memSessionRepo{s: &model.PracticeSession{
		QuestionIDs: `[9,3,7]`,
		Answers:     `{}`,
	}}
	svc := newSessionSvc(qr, sr)

	data, err := svc.StartPractice(1, 1, 1, "数据结构", false)
	if err != nil {
		t.Fatal(err)
	}
	if !data.Resumed {
		t.Fatal("应识别为续做")
	}
	got := []uint64{}
	for _, q := range data.Questions {
		got = append(got, q.ID)
	}
	if len(got) != 3 || got[0] != 9 || got[1] != 3 || got[2] != 7 {
		t.Fatalf("题序未还原: %v", got)
	}
}

// 续做要停在第一道没答过的题上，而不是从头再答一遍已经答过的。
func TestResumeJumpsToFirstUnanswered(t *testing.T) {
	qr := &sessionQuestionRepo{all: questionSet(1, 2, 3, 4)}
	sr := &memSessionRepo{s: &model.PracticeSession{
		QuestionIDs: `[1,2,3,4]`,
		Answers:     `{"1":"A","2":"B"}`,
	}}
	svc := newSessionSvc(qr, sr)

	data, err := svc.StartPractice(1, 1, 1, "数据结构", false)
	if err != nil {
		t.Fatal(err)
	}
	if data.Index != 2 {
		t.Fatalf("应从第 3 题（下标 2）继续，实际下标 %d", data.Index)
	}
	if data.Answers["1"] != "A" || data.Answers["2"] != "B" {
		t.Fatalf("上次作答未还原: %v", data.Answers)
	}
}

// 「从头开始」必须丢弃旧作答并重新抽题，否则用户点了重来还是老样子。
func TestFreshStartDiscardsPreviousAnswers(t *testing.T) {
	qr := &sessionQuestionRepo{
		all:    questionSet(1, 2),
		random: []model.Question{{ID: 5}, {ID: 6}},
	}
	sr := &memSessionRepo{s: &model.PracticeSession{
		QuestionIDs: `[1,2]`,
		Answers:     `{"1":"A"}`,
	}}
	svc := newSessionSvc(qr, sr)

	data, err := svc.StartPractice(1, 1, 1, "数据结构", true)
	if err != nil {
		t.Fatal(err)
	}
	if data.Resumed || len(data.Answers) != 0 {
		t.Fatalf("从头开始不应带出旧作答: %+v", data.Answers)
	}
	if len(data.Questions) != 2 || data.Questions[0].ID != 5 {
		t.Fatalf("应重新抽题: %+v", data.Questions)
	}
	// 旧草稿要被新的一轮覆盖，否则下次「继续」会捡回已经作废的答题卡
	var ids []uint64
	json.Unmarshal([]byte(sr.s.QuestionIDs), &ids)
	if len(ids) != 2 || ids[0] != 5 || sr.s.Answers != "{}" {
		t.Fatalf("草稿未被覆盖: ids=%v answers=%s", ids, sr.s.Answers)
	}
}

// 分类刷题一次要覆盖该知识点全部题目（列表显示 37 题，答题卡就要有 37 题）。
// 抽题请求必须不下发 20 道上限，否则答题卡提前「完成」、后面的题永远不出现。
func TestStartPracticeRequestsAllQuestions(t *testing.T) {
	qr := &sessionQuestionRepo{
		// 模拟 37 道题的知识点：返回数量远超旧版 20 道上限
		random: func() []model.Question {
			qs := make([]model.Question, 37)
			for i := range qs {
				qs[i] = model.Question{ID: uint64(i + 1)}
			}
			return qs
		}(),
	}
	sr := &memSessionRepo{}
	svc := newSessionSvc(qr, sr)

	data, err := svc.StartPractice(1, 1, 1, "软件工程>开发模型", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Questions) != 37 {
		t.Fatalf("应一次返回全部 37 题，实际 %d 题", len(data.Questions))
	}
	if qr.drawnCount > 0 {
		t.Fatalf("抽题应不限量（count<=0 表示不设上限），实际 count=%d", qr.drawnCount)
	}
	// 草稿里也得是全量题目，否则「继续刷题」恢复回来还是残缺的答题卡
	var ids []uint64
	json.Unmarshal([]byte(sr.s.QuestionIDs), &ids)
	if len(ids) != 37 {
		t.Fatalf("草稿应存全部 37 题，实际 %d 题", len(ids))
	}
}

// 旧版 20 道上限存下的草稿（知识点评 37 题却只存 20）在「继续刷题」时必须被识别为
// 过期、丢弃重抽，否则恢复出来的还是残缺答题卡 —— 这正是用户反馈的「做完 20 就完成，
// 剩下 17 道永远不出现」。判定标准：草稿题数 != 当前候选集全量题数（CountQuestions）。
func TestResumeRegeneratesStaleDraft(t *testing.T) {
	qr := &sessionQuestionRepo{
		all:    questionSet(1, 2, 3, 4, 5), // 当前全量题库 5 题
		random: []model.Question{{ID: 10}, {ID: 11}, {ID: 12}, {ID: 13}, {ID: 14}},
	}
	sr := &memSessionRepo{s: &model.PracticeSession{
		QuestionIDs: `[1,2]`, // 旧版上限留下的残缺草稿（只截了 2 题切片示意）
		Answers:     `{"1":"A"}`,
	}}
	svc := newSessionSvc(qr, sr)

	data, err := svc.StartPractice(1, 1, 1, "软件工程>开发模型", false)
	if err != nil {
		t.Fatal(err)
	}
	if data.Resumed || len(data.Questions) != 5 {
		t.Fatalf("残缺草稿应被丢弃并重抽全量，实际 resumed=%v len=%d", data.Resumed, len(data.Questions))
	}
	if len(data.Answers) != 0 {
		t.Fatalf("过期草稿的作答不得恢复: %+v", data.Answers)
	}
	// 新草稿应覆盖全量、作答应清空，否则下次「继续」又捡回残缺答题卡
	var ids []uint64
	json.Unmarshal([]byte(sr.s.QuestionIDs), &ids)
	if len(ids) != 5 {
		t.Fatalf("新草稿应覆盖全量 5 题，实际 %v", ids)
	}
	if sr.s.Answers != "{}" {
		t.Fatalf("新草稿作答应清空: %s", sr.s.Answers)
	}
}

// 草稿里的题目被下架后不能整轮失败，要回落到重新抽题。
func TestResumeFallsBackWhenQuestionsGone(t *testing.T) {
	qr := &sessionQuestionRepo{
		all:    map[uint64]model.Question{}, // 全部下架
		random: []model.Question{{ID: 8}},
	}
	sr := &memSessionRepo{s: &model.PracticeSession{
		QuestionIDs: `[1,2]`,
		Answers:     `{"1":"A"}`,
	}}
	svc := newSessionSvc(qr, sr)

	data, err := svc.StartPractice(1, 1, 1, "数据结构", false)
	if err != nil {
		t.Fatal(err)
	}
	if data.Resumed || len(data.Questions) != 1 || data.Questions[0].ID != 8 {
		t.Fatalf("应回落到重新抽题: %+v", data)
	}
}

// 作答要即时落进草稿，否则刷新页面就丢了 —— 这正是「继续刷题」要解决的问题。
func TestSaveSessionAnswerPersists(t *testing.T) {
	qr := &sessionQuestionRepo{all: questionSet(1, 2)}
	sr := &memSessionRepo{s: &model.PracticeSession{
		QuestionIDs: `[1,2]`,
		Answers:     `{}`,
	}}
	svc := newSessionSvc(qr, sr)

	if err := svc.SaveSessionAnswer(1, 1, 1, "数据结构", 2, "C"); err != nil {
		t.Fatal(err)
	}
	answers := map[string]string{}
	json.Unmarshal([]byte(sr.s.Answers), &answers)
	if answers["2"] != "C" {
		t.Fatalf("作答未落草稿: %v", answers)
	}
}
