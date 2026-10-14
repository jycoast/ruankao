package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

// 记录的 AI 调用日志,供 scene/success 断言;其余方法不可达即 panic。
type recLogRepo struct {
	logs []*model.AICallLog
}

func (s *recLogRepo) Create(l *model.AICallLog) error { s.logs = append(s.logs, l); return nil }
func (s *recLogRepo) List(string, int, int) ([]model.AICallLog, int64, error) {
	panic("unused")
}
func (s *recLogRepo) StatByScene(int) ([]repository.AICallStat, error) { panic("unused") }
func (s *recLogRepo) StatByDay(int) ([]repository.AICallStat, error)   { panic("unused") }
func (s *recLogRepo) TopUsers(int, int) ([]repository.AICallStat, error) {
	panic("unused")
}
func (s *recLogRepo) Summary(int) (int64, int64, int64, int64, error) { panic("unused") }

// sseFakeUpstream 模拟大模型上游的 SSE 逐块输出,可选末尾 finish_reason(stop/length)。
func sseFakeUpstream(t *testing.T, deltas []string, finishReason string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		flush := func() {
			if fl != nil {
				fl.Flush()
			}
		}
		for _, d := range deltas {
			b, _ := json.Marshal(map[string]any{
				"choices": []map[string]any{{"delta": map[string]string{"content": d}}},
			})
			fmt.Fprintf(w, "data: %s\n\n", b)
			flush()
		}
		if finishReason != "" {
			b, _ := json.Marshal(map[string]any{
				"choices": []map[string]any{{"delta": map[string]string{}, "finish_reason": finishReason}},
			})
			fmt.Fprintf(w, "data: %s\n\n", b)
			flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flush()
	}))
	t.Cleanup(srv.Close)
	return srv
}

func diagAISvc(cfgRepo *stubAIConfigRepo, pq repository.PracticeQueryRepository, logs *recLogRepo) *AIService {
	return NewAIService(cfgRepo, &stubUserAIConfigRepo{}, nil, nil, nil, logs, nil, pq)
}

// buildDiagnosisPrompt:大纲含作答表头与错题样例;标题/解析按 rune 截断。
func TestBuildDiagnosisPrompt_SectionsAndTruncation(t *testing.T) {
	rows := []repository.WeaknessRow{
		{Category: "操作系统", Done: 20, Correct: 8, Wrong: 12},
	}
	longTitle := strings.Repeat("架", 100)
	samples := []repository.WrongSampleRow{
		{Title: longTitle, KnowledgePoint: "操作系统>微内核", Explanation: strings.Repeat("因", 200)},
	}
	got := buildDiagnosisPrompt(rows, samples)

	for _, frag := range []string{"知识大类", "做题数", "答对数", "正确率", "操作系统", "最近答错的题目", "知识点：操作系统>微内核"} {
		if !strings.Contains(got, frag) {
			t.Fatalf("prompt 缺片段 %q:\n%s", frag, got)
		}
	}
	// 标题 60 rune、解析 120 rune 截断后,超长原文不应整体出现在 prompt 里
	if strings.Contains(got, longTitle) {
		t.Fatal("超长标题未被截断")
	}
	if strings.Contains(got, strings.Repeat("因", 200)) {
		t.Fatal("超长解析未被截断")
	}
}

func TestBuildDiagnosisPrompt_NoRowsNoSamples(t *testing.T) {
	got := buildDiagnosisPrompt(nil, nil)
	if !strings.Contains(got, "暂无作答记录") || !strings.Contains(got, "（暂无）") {
		t.Fatalf("空输入应给出占位文案:\n%s", got)
	}
}

// 无数据:返回 ErrDiagnosisNoData 且一次都不调上游。
func TestDiagnosisStream_NoDataSkipsUpstream(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := diagAISvc(&stubAIConfigRepo{cfg: &model.AIConfig{
		BaseURL: srv.URL, APIKey: "sk", Model: "m", Enabled: 1,
	}}, &stubPracticeQueryRepo{}, &recLogRepo{})

	_, err := svc.DiagnosisStream(context.Background(), 0, 1, func(string) error { return nil })
	if err != ErrDiagnosisNoData {
		t.Fatalf("应返 ErrDiagnosisNoData,得到 %v", err)
	}
	if hits != 0 {
		t.Fatalf("无数据不应调上游,得到 %d 次", hits)
	}
}

// 正常流式:逐块回调拼接完整输出,scene=diagnosis 落库且 success=1。
func TestDiagnosisStream_StreamsAndLogsScene(t *testing.T) {
	up := sseFakeUpstream(t, []string{"一句话诊断：", "你的弱项在", "操作系统"}, "stop")
	svc := diagAISvc(&stubAIConfigRepo{cfg: &model.AIConfig{
		BaseURL: up.URL, APIKey: "sk", Model: "m", Enabled: 1, MaxTokens: 4095,
	}}, &stubPracticeQueryRepo{weak: []repository.WeaknessRow{
		{Category: "操作系统", Done: 20, Correct: 8, Wrong: 12},
	}}, &recLogRepo{})

	var out strings.Builder
	truncated, err := svc.DiagnosisStream(context.Background(), 0, 1, func(d string) error {
		out.WriteString(d)
		return nil
	})
	if err != nil {
		t.Fatalf("应无错,得到 err=%v", err)
	}
	if truncated {
		t.Fatal("finish_reason=stop 不应 truncated")
	}
	if out.String() != "一句话诊断：你的弱项在操作系统" {
		t.Fatalf("流式输出错: %q", out.String())
	}
}

// 上游返回截断(finish_reason=length):truncated=true 且正常落库。
func TestDiagnosisStream_Truncated(t *testing.T) {
	up := sseFakeUpstream(t, []string{"正文"}, "length")
	svc := diagAISvc(&stubAIConfigRepo{cfg: &model.AIConfig{
		BaseURL: up.URL, APIKey: "sk", Model: "m", Enabled: 1, MaxTokens: 100,
	}}, &stubPracticeQueryRepo{weak: []repository.WeaknessRow{
		{Category: "操作系统", Done: 20, Correct: 8, Wrong: 12},
	}}, &recLogRepo{})

	truncated, err := svc.DiagnosisStream(context.Background(), 0, 1, func(string) error { return nil })
	if err != nil {
		t.Fatalf("应无错,得到 err=%v", err)
	}
	if !truncated {
		t.Fatal("finish_reason=length 应 truncated=true")
	}
}

// 上游非 200:错误透传,日志记为失败。
func TestDiagnosisStream_UpstreamErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	logs := &recLogRepo{}
	svc := diagAISvc(&stubAIConfigRepo{cfg: &model.AIConfig{
		BaseURL: srv.URL, APIKey: "sk", Model: "m", Enabled: 1,
	}}, &stubPracticeQueryRepo{weak: []repository.WeaknessRow{
		{Category: "操作系统", Done: 20, Correct: 8, Wrong: 12},
	}}, logs)

	_, err := svc.DiagnosisStream(context.Background(), 0, 1, func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "大模型接口返回 500") {
		t.Fatalf("上游 500 应透传错误,得到 %v", err)
	}
	if len(logs.logs) != 1 {
		t.Fatalf("应有 1 条调用日志,得到 %d", len(logs.logs))
	}
	if logs.logs[0].Scene != "diagnosis" || logs.logs[0].Success != 0 {
		t.Fatalf("日志应 scene=diagnosis 且 success=0,得到 %+v", logs.logs[0])
	}
}
