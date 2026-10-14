package aiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestValidateUserBaseURL(t *testing.T) {
	bad := []string{
		"http://api.openai.com/v1",         // 非 https
		"https://169.254.169.254/latest",   // 云元数据
		"https://127.0.0.1:8080/v1",        // 环回
		"https://10.0.0.5/v1",              // 私有段
		"https://192.168.1.1/v1",           // 私有段
		"https://100.64.0.1/v1",            // CGNAT
		"https://[::1]/v1",                 // IPv6 环回
		"file:///etc/passwd",               // 非 http 协议
		"https:///v1",                      // 无域名
	}
	for _, u := range bad {
		if err := ValidateUserBaseURL(u); err == nil {
			t.Errorf("expected %q to be rejected", u)
		}
	}

	for _, u := range []string{"https://api.openai.com/v1", "https://ark.cn-beijing.volces.com/api/v3"} {
		if err := ValidateUserBaseURL(u); err != nil {
			t.Errorf("expected %q to pass, got %v", u, err)
		}
	}
}

// 域名解析到内网时，须在拨号阶段（DNS 之后）拦下 —— 这是 DNS rebinding 的防线。
func TestRestrictedDialBlocksResolvedPrivateIP(t *testing.T) {
	// 直接验证拨号钩子依据的判定，避免测试依赖外部 DNS
	for _, s := range []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.0.1", "169.254.169.254", "100.64.0.1", "::1", "0.0.0.0"} {
		if !isDisallowedIP(net.ParseIP(s)) {
			t.Errorf("expected %s to be disallowed", s)
		}
	}
	for _, s := range []string{"1.1.1.1", "104.18.0.1", "2606:4700::1"} {
		if isDisallowedIP(net.ParseIP(s)) {
			t.Errorf("expected %s to be allowed", s)
		}
	}
}

// Restricted 配置在 buildRequest 阶段就该拒掉非法 base_url，不发出任何请求。
func TestRestrictedRejectsBeforeDial(t *testing.T) {
	cfg := Config{BaseURL: "http://169.254.169.254/latest", APIKey: "sk-test", Model: "gpt-4", Restricted: true}
	_, _, err := ChatCompletion(context.Background(), cfg, []Message{{Role: "user", Content: "hi"}})
	if err == nil || !strings.Contains(err.Error(), "https://") {
		t.Fatalf("expected https rejection, got: %v", err)
	}
}

// 上游错误体不得回显给用户，否则等于一个带认证的任意 URL 读取器。
func TestUpstreamErrorHidesBodyWhenRestricted(t *testing.T) {
	body := []byte("ami-0abc SECRET-INSTANCE-METADATA")
	if strings.Contains((Config{Restricted: true}).upstreamError(403, body).Error(), "SECRET") {
		t.Error("restricted config must not echo upstream body")
	}
	if !strings.Contains((Config{}).upstreamError(403, body).Error(), "SECRET") {
		t.Error("admin config should keep upstream body for diagnostics")
	}
}

// ---- 流式超时与截断 ----
//
// 这一组守的是同一个坑：http.Client.Timeout 量的是「建连 + 重定向 + 读响应体」全程，
// 挂在流式请求上就等于给生成过程设了个硬死线 —— 与模型快慢无关，只与总时长有关。
// 论文这类长输出必然被腰斩，而且因为 HTTP 头早已发出，客户端拿不到任何错误。

// 流式客户端不得有总时长上限；非流式则相反，必须有。
func TestStreamClientsHaveNoTotalTimeout(t *testing.T) {
	for name, c := range map[string]*http.Client{
		"streamClient":           streamClient,
		"restrictedStreamClient": restrictedStreamClient,
	} {
		if c.Timeout != 0 {
			t.Errorf("%s.Timeout = %v，流式客户端不能设总时长上限（见 client.go 顶部超时策略）", name, c.Timeout)
		}
	}
	for name, c := range map[string]*http.Client{
		"httpClient":       httpClient,
		"restrictedClient": restrictedClient,
	} {
		if c.Timeout == 0 {
			t.Errorf("%s.Timeout = 0，非流式是一问一答，必须有总时长上限", name)
		}
	}
}

// 不设总时长不等于不设防：连上了却迟迟不来首字节，仍要快速失败。
func TestTransportsStillBoundFirstByte(t *testing.T) {
	for name, tr := range map[string]*http.Transport{
		"plainTransport":      plainTransport,
		"restrictedTransport": restrictedTransport,
	} {
		if tr.ResponseHeaderTimeout == 0 {
			t.Errorf("%s.ResponseHeaderTimeout = 0，上游卡住时连接会一直挂着", name)
		}
	}
}

// sseServer 起一个按 delay 逐条吐 SSE chunk 的测试服务，末尾发 finish_reason 与 [DONE]。
func sseServer(t *testing.T, chunks []string, finishReason string, delay time.Duration) *httptest.Server {
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
		for _, c := range chunks {
			b, _ := json.Marshal(map[string]any{
				"choices": []map[string]any{{"delta": map[string]string{"content": c}}},
			})
			fmt.Fprintf(w, "data: %s\n\n", b)
			flush()
			time.Sleep(delay)
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

func streamTestCfg(url string) Config {
	return Config{BaseURL: url, APIKey: "sk-test", Model: "test-model"}
}

// finish_reason=length 必须被标记出来。没有这一条，用户看到的就只是
// 「输出到一半停了」，既没报错也判断不出原因 —— 这正是本次修复前的表现。
func TestChatCompletionStreamReportsTruncation(t *testing.T) {
	srv := sseServer(t, []string{"前半段", "后半段"}, "length", time.Millisecond)

	var got strings.Builder
	res, err := ChatCompletionStream(context.Background(), streamTestCfg(srv.URL),
		[]Message{{Role: "user", Content: "写论文"}}, func(d string) error {
			got.WriteString(d)
			return nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != "前半段后半段" {
		t.Errorf("content = %q, want 前半段后半段", got.String())
	}
	if !res.Truncated {
		t.Error("finish_reason=length 未标记为 Truncated")
	}
}

func TestChatCompletionStreamNotTruncatedOnStop(t *testing.T) {
	srv := sseServer(t, []string{"完整内容"}, "stop", time.Millisecond)

	res, err := ChatCompletionStream(context.Background(), streamTestCfg(srv.URL),
		[]Message{{Role: "user", Content: "hi"}}, func(string) error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Truncated {
		t.Errorf("FinishReason=%q 却被标记为截断", res.FinishReason)
	}
	if res.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want stop", res.FinishReason)
	}
}

// 反向验证：把 Client.Timeout 挂回流式客户端，长流就会被腰斩、只留下一截内容。
// 这条测试是「为什么流式不能设 Timeout」的可执行说明 —— 别把它当成多余的洁癖。
func TestTotalTimeoutTruncatesStream(t *testing.T) {
	srv := sseServer(t, []string{"1", "2", "3", "4", "5"}, "stop", 40*time.Millisecond)

	orig := streamClient
	streamClient = &http.Client{Timeout: 60 * time.Millisecond, Transport: plainTransport}
	t.Cleanup(func() { streamClient = orig })

	var got strings.Builder
	_, err := ChatCompletionStream(context.Background(), streamTestCfg(srv.URL),
		[]Message{{Role: "user", Content: "hi"}}, func(d string) error {
			got.WriteString(d)
			return nil
		})
	if err == nil {
		t.Fatal("预期被 Client.Timeout 截断，却正常结束了 —— 这条测试的前提已不成立")
	}
	if got.Len() == 0 {
		t.Fatal("预期先收到部分内容再被截断")
	}
}
