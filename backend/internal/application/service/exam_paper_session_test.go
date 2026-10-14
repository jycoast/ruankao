package service

import (
	"testing"

	"software-exam/internal/domain/model"
)

type stubPaperSessionRepo struct{ s *model.PaperPracticeSession }

func (m *stubPaperSessionRepo) FindOne(_ uint64, _ uint64, _ string) (*model.PaperPracticeSession, error) {
	if m.s == nil {
		return nil, errNotFound
	}
	return m.s, nil
}
func (m *stubPaperSessionRepo) Save(s *model.PaperPracticeSession) error { m.s = s; return nil }
func (m *stubPaperSessionRepo) Delete(_ uint64, _ uint64, _ string) error {
	m.s = nil
	return nil
}

// 保存后能原样取回，判题结果与当前题位置都在。
func TestPaperSessionSaveAndLoad(t *testing.T) {
	psr := &stubPaperSessionRepo{}
	svc := NewExamService(&stubPaperRepo{}, nil, &stubExamRepo{}, nil, psr)

	correct := true
	payload := &PaperSessionData{
		Answers:         map[string]map[string]string{"1": {"0": "B"}, "2": {"0": "A"}},
		PracticeResults: map[string]PaperSessionResult{"1": {IsCorrect: &correct, CorrectAnswer: "B"}},
		CurrentIndex:    3,
	}
	if err := svc.SavePracticeSession(1, 100, "practice", payload); err != nil {
		t.Fatal(err)
	}
	data, ok := svc.GetPracticeSessionState(1, 100, "practice")
	if !ok {
		t.Fatal("应能取回会话")
	}
	if data.CurrentIndex != 3 || data.Answers["2"]["0"] != "A" {
		t.Fatalf("会话内容不符: %+v", data)
	}
	if data.PracticeResults["1"].IsCorrect == nil || !*data.PracticeResults["1"].IsCorrect {
		t.Fatal("判题结果未还原")
	}
}

// 无会话时返回 (nil, false)，不报错。
func TestPaperSessionStateWhenNone(t *testing.T) {
	svc := NewExamService(&stubPaperRepo{}, nil, &stubExamRepo{}, nil, &stubPaperSessionRepo{})
	if _, ok := svc.GetPracticeSessionState(1, 100, "practice"); ok {
		t.Fatal("无会话应返回 false")
	}
}

// 「从头开始」删除后会话消失。
func TestPaperSessionClear(t *testing.T) {
	psr := &stubPaperSessionRepo{s: &model.PaperPracticeSession{UserID: 1, PaperID: 100, Mode: "practice", Data: `{}`}}
	svc := NewExamService(&stubPaperRepo{}, nil, &stubExamRepo{}, nil, psr)
	if err := svc.ClearPracticeSession(1, 100, "practice"); err != nil {
		t.Fatal(err)
	}
	if _, ok := svc.GetPracticeSessionState(1, 100, "practice"); ok {
		t.Fatal("清除后不应再可恢复")
	}
}

// 非 practice 模式拒绝落库。
func TestPaperSessionRejectsOtherMode(t *testing.T) {
	psr := &stubPaperSessionRepo{}
	svc := NewExamService(&stubPaperRepo{}, nil, &stubExamRepo{}, nil, psr)
	if err := svc.SavePracticeSession(1, 100, "exam", &PaperSessionData{}); err == nil {
		t.Fatal("exam 模式应被拒绝")
	}
	if psr.s != nil {
		t.Fatal("不应落库")
	}
}
