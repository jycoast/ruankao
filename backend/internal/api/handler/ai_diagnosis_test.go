package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"software-exam/internal/application/service"
	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"

	"github.com/gin-gonic/gin"
)

// 会员配置仓库:一律「无自带 key」,让 resolveConfig 回落到平台全局配置。
type fakeNoUserCfgRepo struct{}

func (f *fakeNoUserCfgRepo) GetByUser(uint64) (*model.UserAIConfig, error) {
	return nil, errors.New("no-user-config")
}
func (f *fakeNoUserCfgRepo) Upsert(*model.UserAIConfig) error { return nil }

// 诊断流式端点的 handler 测试:SSE 帧契约(delta/done/truncated/error)。
// fakeUpstream / fakeCfgRepo / fakeLogRepo 复用 ai_stream_test.go。
// 复用 newEssayHandler 同款结构,再加一个 fakePQRepo 提供诊断数据。

// fakePQRepo 只覆盖 DiagnosisStream 用到的两个查询,其余不可达即 panic。
type fakePQRepo struct {
	weak    []repository.WeaknessRow
	samples []repository.WrongSampleRow
}

func (f *fakePQRepo) AccuracyByUser(uint64) (repository.AccuracyRow, error) { panic("unused") }
func (f *fakePQRepo) CategoryProgress(uint64) ([]repository.ProgressRow, error) {
	panic("unused")
}
func (f *fakePQRepo) AccuracyBySubject(uint64, uint64) (repository.AccuracyRow, error) {
	panic("unused")
}
func (f *fakePQRepo) TrendBySubject(uint64, uint64) ([]repository.TrendRow, error) {
	panic("unused")
}
func (f *fakePQRepo) KnowledgeAccuracyBySubject(uint64, uint64) ([]repository.KpRow, error) {
	panic("unused")
}
func (f *fakePQRepo) SpeedBySubject(uint64, uint64, uint64, uint64) (repository.SpeedRow, error) {
	panic("unused")
}
func (f *fakePQRepo) SpeedByUser(uint64, uint64, uint64) (repository.SpeedRow, error) {
	panic("unused")
}
func (f *fakePQRepo) CategoryAccuracyBySubject(uint64, uint64) ([]repository.WeaknessRow, error) {
	return f.weak, nil
}
func (f *fakePQRepo) CategoryQuestionCountsBySubject(uint64) ([]repository.CategoryCountRow, error) {
	panic("unused")
}
func (f *fakePQRepo) WrongBookStatsBySubject(uint64, uint64) (repository.WrongBookStats, error) {
	panic("unused")
}
func (f *fakePQRepo) RecentWrongSamplesBySubject(uint64, uint64, int) ([]repository.WrongSampleRow, error) {
	return f.samples, nil
}

func newDiagnosisHandler(t *testing.T, upstreamURL string, pq repository.PracticeQueryRepository) *AIHandler {
	t.Helper()
	svc := service.NewAIService(
		&fakeCfgRepo{cfg: &model.AIConfig{
			BaseURL: upstreamURL, APIKey: "sk-test", Model: "test-model",
			MaxTokens: 4095, Enabled: 1,
		}},
		&fakeNoUserCfgRepo{}, nil, nil, nil, &fakeLogRepo{}, nil, pq,
	)
	return &AIHandler{aiService: svc}
}

func callDiagnosisStream(t *testing.T, h *AIHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/diagnosis/ai", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", uint64(7))
	h.DiagnosisStream(c)
	return w
}

// 正常流:delta 逐块出现,收尾 done:true,无 error 帧。
func TestDiagnosisStream_DeltaAndDoneFrames(t *testing.T) {
	up := fakeUpstream(t, "stop")
	h := newDiagnosisHandler(t, up.URL, &fakePQRepo{
		weak: []repository.WeaknessRow{{Category: "操作系统", Done: 20, Correct: 8, Wrong: 12}},
	})

	w := callDiagnosisStream(t, h, `{"subject_id":1}`)
	body := w.Body.String()
	for _, frag := range []string{`"delta":"摘要："`, `"delta":"正文……"`, `"done":true`} {
		if !strings.Contains(body, frag) {
			t.Fatalf("SSE 响应缺 %q:\n%s", frag, body)
		}
	}
	if strings.Contains(body, `"error"`) {
		t.Fatalf("正常流不应有 error 帧:\n%s", body)
	}
}

// 上游 finish_reason=length:truncated:true 透出。
func TestDiagnosisStream_TruncatedFrame(t *testing.T) {
	up := fakeUpstream(t, "length")
	h := newDiagnosisHandler(t, up.URL, &fakePQRepo{
		weak: []repository.WeaknessRow{{Category: "操作系统", Done: 20, Correct: 8, Wrong: 12}},
	})

	w := callDiagnosisStream(t, h, `{"subject_id":1}`)
	if !strings.Contains(w.Body.String(), `"truncated":true`) {
		t.Fatalf("应带 truncated:true:\n%s", w.Body.String())
	}
}

// 无数据:业务错误必须以 SSE error 帧下发(流已开 200 头,不能解析 JSON body),
// 且不允许出现 delta/done。
func TestDiagnosisStream_NoDataSendsErrorFrame(t *testing.T) {
	up := fakeUpstream(t, "stop")
	h := newDiagnosisHandler(t, up.URL, &fakePQRepo{}) // weak 空 → ErrDiagnosisNoData

	w := callDiagnosisStream(t, h, `{"subject_id":1}`)
	body := w.Body.String()
	if !strings.Contains(body, `"error":"暂无诊断数据`) {
		t.Fatalf("无数据应发 error 帧:\n%s", body)
	}
	if strings.Contains(body, `"delta"`) || strings.Contains(body, `"done"`) {
		t.Fatalf("无数据不应有 delta/done:\n%s", body)
	}
}

// subject_id 必填:0 直接 400,不进服务。
func TestDiagnosisStream_RequiresSubject(t *testing.T) {
	up := fakeUpstream(t, "stop")
	h := newDiagnosisHandler(t, up.URL, &fakePQRepo{
		weak: []repository.WeaknessRow{{Category: "操作系统", Done: 20, Correct: 8, Wrong: 12}},
	})

	w := callDiagnosisStream(t, h, `{"subject_id":0}`)
	if !strings.Contains(w.Body.String(), `"code":400`) {
		t.Fatalf("subject_id=0 应 400:\n%s", w.Body.String())
	}
}
