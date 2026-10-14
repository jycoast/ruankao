package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"software-exam/internal/application/service"
	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"

	"github.com/gin-gonic/gin"
)

// ---- 最小假 repo：只覆盖 GenerateEssayStream 真正用到的 cfgRepo 与 logRepo ----
// userID 传 0 时 resolveConfig 会跳过会员配置，其余 repo 可以留 nil。

type fakeCfgRepo struct{ cfg *model.AIConfig }

func (f *fakeCfgRepo) Get() (*model.AIConfig, error) { return f.cfg, nil }
func (f *fakeCfgRepo) Update(*model.AIConfig) error  { return nil }

type fakeLogRepo struct{ n int }

func (f *fakeLogRepo) Create(*model.AICallLog) error                           { f.n++; return nil }
func (f *fakeLogRepo) List(string, int, int) ([]model.AICallLog, int64, error) { return nil, 0, nil }
func (f *fakeLogRepo) StatByScene(int) ([]repository.AICallStat, error)        { return nil, nil }
func (f *fakeLogRepo) StatByDay(int) ([]repository.AICallStat, error)          { return nil, nil }
func (f *fakeLogRepo) TopUsers(int, int) ([]repository.AICallStat, error)      { return nil, nil }
func (f *fakeLogRepo) Summary(int) (int64, int64, int64, int64, error)         { return 0, 0, 0, 0, nil }

// fakeUpstream 模拟大模型上游，最后一帧带 finish_reason —— 截断判定就靠它。
func fakeUpstream(t *testing.T, finishReason string) *httptest.Server {
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
		for _, chunk := range []string{"摘要：", "正文……"} {
			b, _ := json.Marshal(map[string]any{
				"choices": []map[string]any{{"delta": map[string]string{"content": chunk}}},
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

func newEssayHandler(t *testing.T, upstreamURL string) *AIHandler {
	t.Helper()
	svc := service.NewAIService(
		&fakeCfgRepo{cfg: &model.AIConfig{
			BaseURL: upstreamURL, APIKey: "sk-test", Model: "test-model",
			MaxTokens: 4095, Enabled: 1,
		}},
		nil, nil, nil, nil, &fakeLogRepo{}, nil, nil,
	)
	return &AIHandler{aiService: svc}
}

func callGenerateEssay(t *testing.T, h *AIHandler) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/ai/generate-essay",
		strings.NewReader(`{"topic":"论云原生架构及其应用"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", uint64(0))
	h.GenerateEssay(c)
	return w.Body.String()
}

// 截断要一路穿透 SSE 送到前端。后端发 "truncated"、前端读 obj.truncated,
// 这层约定编译器管不到 —— 这条测试就是它的守门人，改字段名必须同步改 aiStream.ts。
func TestGenerateEssaySSESurfacesTruncation(t *testing.T) {
	body := callGenerateEssay(t, newEssayHandler(t, fakeUpstream(t, "length").URL))

	if !strings.Contains(body, `"truncated":true`) {
		t.Fatalf("finish_reason=length 未透出 truncated，前端只会看到「输出到一半停了」：\n%s", body)
	}
	// 正文照旧要送达，别为了报告截断把内容吞了
	if !strings.Contains(body, "摘要：") || !strings.Contains(body, "正文……") {
		t.Errorf("正文未完整送达：\n%s", body)
	}
}

func TestGenerateEssaySSENotTruncatedOnStop(t *testing.T) {
	body := callGenerateEssay(t, newEssayHandler(t, fakeUpstream(t, "stop").URL))

	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("正常收尾未发送 done：\n%s", body)
	}
	if strings.Contains(body, `"truncated":true`) {
		t.Errorf("finish_reason=stop 不应标记截断：\n%s", body)
	}
}
