package service

import (
	"errors"
	"testing"
	"time"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
	"software-exam/pkg/aiclient"
)

type fakeCallLogRepo struct {
	logs []model.AICallLog
}

func (f *fakeCallLogRepo) Create(l *model.AICallLog) error {
	f.logs = append(f.logs, *l)
	return nil
}
func (f *fakeCallLogRepo) List(string, int, int) ([]model.AICallLog, int64, error) {
	return nil, 0, nil
}
func (f *fakeCallLogRepo) StatByScene(int) ([]repository.AICallStat, error)   { return nil, nil }
func (f *fakeCallLogRepo) StatByDay(int) ([]repository.AICallStat, error)     { return nil, nil }
func (f *fakeCallLogRepo) TopUsers(int, int) ([]repository.AICallStat, error) { return nil, nil }
func (f *fakeCallLogRepo) Summary(int) (int64, int64, int64, int64, error) {
	return 0, 0, 0, 0, nil
}

// 流式拿不到 usage 时必须回落到估算，并把 estimated 标出来 ——
// 不标的话估算值会混进账单口径被当成真实计数。
func TestLogStreamMarksEstimatedWhenUpstreamOmitsUsage(t *testing.T) {
	repo := &fakeCallLogRepo{}
	s := &AIService{logRepo: repo}
	msgs := []aiclient.Message{{Role: "user", Content: "架构设计"}}

	s.logStream(7, "chat", aiclient.Config{Model: "gpt-4"}, msgs, "回答内容", aiclient.Usage{}, time.Now(), nil)

	if len(repo.logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(repo.logs))
	}
	l := repo.logs[0]
	if l.Estimated != 1 {
		t.Error("usage 缺失时必须置 estimated")
	}
	if l.TotalTokens == 0 {
		t.Error("估算值不应为 0")
	}
	if l.Success != 1 || l.UserID != 7 || l.Scene != "chat" {
		t.Errorf("unexpected log: %+v", l)
	}
}

// 上游给了真实 usage 就原样落库，不能被估算覆盖。
func TestLogStreamKeepsRealUsage(t *testing.T) {
	repo := &fakeCallLogRepo{}
	s := &AIService{logRepo: repo}

	s.logStream(1, "chat", aiclient.Config{}, nil, "x",
		aiclient.Usage{PromptTokens: 11, CompletionTokens: 22, TotalTokens: 33}, time.Now(), nil)

	l := repo.logs[0]
	if l.Estimated != 0 || l.TotalTokens != 33 || l.PromptTokens != 11 {
		t.Errorf("real usage was not preserved: %+v", l)
	}
}

// 自带 key 的调用不花平台的钱，必须能从记录里区分出来。
func TestCallLogFlagsOwnKeyAndFailure(t *testing.T) {
	repo := &fakeCallLogRepo{}
	s := &AIService{logRepo: repo}

	s.callLog(3, "explain", aiclient.Config{Restricted: true}, aiclient.Usage{}, time.Now(), false, errors.New("上游超时"))

	l := repo.logs[0]
	if l.OwnKey != 1 {
		t.Error("Restricted 配置应标记 own_key")
	}
	if l.Success != 0 || l.ErrMsg == "" {
		t.Errorf("失败调用应记录 err_msg: %+v", l)
	}
}

// 日志是旁路，仓储为 nil 时不能 panic。
func TestCallLogNoopWithoutRepo(t *testing.T) {
	(&AIService{}).callLog(1, "chat", aiclient.Config{}, aiclient.Usage{}, time.Now(), false, nil)
}
