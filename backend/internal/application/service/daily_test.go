package service

import (
	"fmt"
	"testing"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

// dailyQuestionRepo 只实现每日一练要用的种子抽题。
// GetRandomQuestions 特意 panic：每日一练一旦退回纯随机抽题，
// 换设备/刷新就会拿到另一组题，必须让测试立刻暴露。
type dailyQuestionRepo struct {
	seed        uint32
	seededCalls int
	questions   []model.Question
}

func (f *dailyQuestionRepo) GetSeededRandomQuestions(_ int8, _ int, _ string, _ uint64, seed uint32) ([]model.Question, error) {
	f.seed = seed
	f.seededCalls++
	return f.questions, nil
}

func (f *dailyQuestionRepo) GetRandomQuestions(int8, int, string, uint64) ([]model.Question, error) {
	panic("每日一练必须走种子抽题，不应调用纯随机抽题")
}
func (f *dailyQuestionRepo) Create(*model.Question) error              { panic("unused") }
func (f *dailyQuestionRepo) FindByID(uint64) (*model.Question, error)  { panic("unused") }
func (f *dailyQuestionRepo) Update(*model.Question) error              { panic("unused") }
func (f *dailyQuestionRepo) ReplaceOptions(uint64, []model.Option) error { panic("unused") }
func (f *dailyQuestionRepo) Delete(uint64) error                       { panic("unused") }
func (f *dailyQuestionRepo) List(repository.QuestionListReq) ([]model.Question, int64, error) {
	panic("unused")
}
func (f *dailyQuestionRepo) FindByIDs([]uint64) ([]model.Question, error) { panic("unused") }
func (f *dailyQuestionRepo) GetRandomByTypeYear(int8, int, uint64, int16) ([]model.Question, error) {
	panic("unused")
}
func (f *dailyQuestionRepo) CountQuestions(int8, string, uint64) (int64, error) { panic("unused") }
func (f *dailyQuestionRepo) UpdateSortOrders([]uint64) error { panic("unused") }
func (f *dailyQuestionRepo) GetCategories(uint64) ([]repository.Category, error) {
	panic("unused")
}

type stubCheckinRepo struct{ record *model.DailyCheckin }

func (s *stubCheckinRepo) FindByUserAndDate(uint64, string) (*model.DailyCheckin, error) {
	if s.record == nil {
		return nil, errNotFound
	}
	return s.record, nil
}
func (s *stubCheckinRepo) Upsert(c *model.DailyCheckin) error { s.record = c; return nil }
func (s *stubCheckinRepo) ListByUserInRange(uint64, string, string) ([]model.DailyCheckin, error) {
	return nil, nil
}

func newDailySvc(qr *dailyQuestionRepo) *DailyService {
	return NewDailyService(&stubCheckinRepo{}, qr, &stubProfileRepo{p: &model.UserProfile{UserID: 1}}, &stubDailySessionRepo{})
}

// 每日一练必须按「日期+用户+科目」的固定种子抽题：
// 同一天、同一账号在不同设备上调用才可能拿到同一组题。
func TestGetTodayUsesSeededRandom(t *testing.T) {
	qr := &dailyQuestionRepo{questions: []model.Question{{ID: 100}}}
	svc := newDailySvc(qr)

	res, err := svc.GetToday(42, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Questions) != 1 || res.Questions[0].ID != 100 {
		t.Fatalf("应返回种子抽题结果: %+v", res.Questions)
	}
	if qr.seededCalls != 1 {
		t.Fatalf("应走种子抽题一次，实际 %d 次", qr.seededCalls)
	}
	if qr.seed != dailySeed(localToday(), 42, 2) {
		t.Fatalf("种子应为 date+userID+subjectID 的哈希，实际 %d", qr.seed)
	}
}

// 同一天连续两次取题，种子必须一致（种子一致 + 题库不变 = 题目一致）。
// 这正是「换设备/刷新不换题」的服务端保证。
func TestGetTodaySeedStableWithinDay(t *testing.T) {
	qr := &dailyQuestionRepo{questions: []model.Question{{ID: 100}}}
	svc := newDailySvc(qr)

	if _, err := svc.GetToday(42, 2); err != nil {
		t.Fatal(err)
	}
	first := qr.seed
	if _, err := svc.GetToday(42, 2); err != nil {
		t.Fatal(err)
	}
	if qr.seed != first {
		t.Fatalf("同一天种子应稳定: %d != %d", first, qr.seed)
	}
}

// 换账号 / 换日期 / 换科目，种子必须跟着变 —— 否则所有用户一天做同一套题。
func TestDailySeedVariesByUserDateSubject(t *testing.T) {
	base := dailySeed("2026-08-30", 42, 2)
	if base == dailySeed("2026-08-30", 43, 2) {
		t.Fatal("不同用户种子应不同")
	}
	if base == dailySeed("2026-08-31", 42, 2) {
		t.Fatal("不同日期种子应不同")
	}
	if base == dailySeed("2026-08-30", 42, 3) {
		t.Fatal("不同科目种子应不同")
	}
	// 同参数重复调用必须可复现
	if base != dailySeed("2026-08-30", 42, 2) {
		t.Fatal("同参数种子应可复现")
	}
}

// ---- 每日一练会话（跨设备进度同步）----

type stubDailySessionRepo struct {
	sessions map[string]*model.DailySession
}

func (m *stubDailySessionRepo) key(userID uint64, date string, subjectID uint64) string {
	return fmt.Sprintf("%d:%s:%d", userID, date, subjectID)
}

func (m *stubDailySessionRepo) FindOne(userID uint64, date string, subjectID uint64) (*model.DailySession, error) {
	if m.sessions == nil {
		return nil, errNotFound
	}
	s, ok := m.sessions[m.key(userID, date, subjectID)]
	if !ok {
		return nil, errNotFound
	}
	return s, nil
}
func (m *stubDailySessionRepo) Save(s *model.DailySession) error {
	if m.sessions == nil {
		m.sessions = map[string]*model.DailySession{}
	}
	m.sessions[m.key(s.UserID, s.Date, s.SubjectID)] = s
	return nil
}
func (m *stubDailySessionRepo) Delete(userID uint64, date string, subjectID uint64) error {
	if m.sessions != nil {
		delete(m.sessions, m.key(userID, date, subjectID))
	}
	return nil
}

func newDailySvcWithSession(qr *dailyQuestionRepo, sr *stubDailySessionRepo) *DailyService {
	return NewDailyService(&stubCheckinRepo{}, qr, &stubProfileRepo{p: &model.UserProfile{UserID: 1}}, sr)
}

// 新开每日一练（无会话）：session 为 null
func TestGetTodayNoSession(t *testing.T) {
	qr := &dailyQuestionRepo{questions: []model.Question{{ID: 100}}}
	svc := newDailySvcWithSession(qr, &stubDailySessionRepo{})

	res, err := svc.GetToday(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.Session != nil {
		t.Fatal("首次打开不应有 session")
	}
}

// 保存后能取回完整进度
func TestSaveSessionRoundtrip(t *testing.T) {
	qr := &dailyQuestionRepo{questions: []model.Question{{ID: 100}}}
	sr := &stubDailySessionRepo{}
	svc := newDailySvcWithSession(qr, sr)

	payload := &DailySessionData{
		CurrentIndex: 3,
		CurrentBlank: 1,
		Answers:      map[string]string{"0": "A", "1": "B"},
		ShowResult:   true,
		Result:       map[string]interface{}{"is_correct": true},
		CorrectCount: 2,
	}
	if err := svc.SaveSession(1, 2, payload); err != nil {
		t.Fatal(err)
	}
	// 再次 GetToday 应带 session
	res, err := svc.GetToday(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.Session == nil {
		t.Fatal("保存后应能取回 session")
	}
	if res.Session.CurrentIndex != 3 || res.Session.CurrentBlank != 1 {
		t.Fatalf("进度不符: currentIndex=%d currentBlank=%d", res.Session.CurrentIndex, res.Session.CurrentBlank)
	}
	if res.Session.Answers["0"] != "A" || res.Session.ShowResult != true {
		t.Fatalf("答案/状态不符: %+v", res.Session)
	}
}

// 签到成功后删当日会话
func TestCheckinClearsSession(t *testing.T) {
	qr := &dailyQuestionRepo{questions: []model.Question{{ID: 100}}}
	sr := &stubDailySessionRepo{}
	// 预存一个会话
	_ = sr.Save(&model.DailySession{
		UserID: 1, Date: localToday(), SubjectID: 2,
		Data: `{"current_index":0}`,
	})
	svc := newDailySvcWithSession(qr, sr)

	if err := svc.Checkin(1, 2, 15, 10); err != nil {
		t.Fatal(err)
	}
	// 签到后会话应被删除
	if _, err := sr.FindOne(1, localToday(), 2); err == nil {
		t.Fatal("签到后会话应被删除")
	}
}

// 不同用户/日期的 session 隔离
func TestDailySessionScopeIsolation(t *testing.T) {
	qr := &dailyQuestionRepo{questions: []model.Question{{ID: 100}}}
	sr := &stubDailySessionRepo{}
	svc := newDailySvcWithSession(qr, sr)

	_ = svc.SaveSession(1, 2, &DailySessionData{CurrentIndex: 1})
	_ = svc.SaveSession(2, 2, &DailySessionData{CurrentIndex: 5})
	_ = svc.SaveSession(1, 3, &DailySessionData{CurrentIndex: 3})

	// 用户1科目2
	res, _ := svc.GetToday(1, 2)
	if res.Session == nil || res.Session.CurrentIndex != 1 {
		t.Fatalf("用户1科目2: 应取回 index=1, 实际 %+v", res.Session)
	}
	// 用户2科目2
	res, _ = svc.GetToday(2, 2)
	if res.Session == nil || res.Session.CurrentIndex != 5 {
		t.Fatalf("用户2科目2: 应取回 index=5, 实际 %+v", res.Session)
	}
	// 用户1科目3
	res, _ = svc.GetToday(1, 3)
	if res.Session == nil || res.Session.CurrentIndex != 3 {
		t.Fatalf("用户1科目3: 应取回 index=3, 实际 %+v", res.Session)
	}
}
