// Package aiclient 是一个最小的 OpenAI 兼容 Chat Completions 客户端。
// 支持非流式与流式（SSE）调用，供 AIService 使用。base_url/api_key/model 由管理员在 DB 中配置。
package aiclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// Message 一条对话消息。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Usage 上游返回的 token 计数，用于成本核算。
// 并非所有 OpenAI 兼容服务都返回（尤其流式），拿不到时全为 0 —— 调用方据此判断是否可信。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Config 调用所需配置（来自 DB 的 AIConfig）。
type Config struct {
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float64
	MaxTokens   int
	// Restricted 标记 base_url 来自普通用户而非管理员。
	// 置位后强制 https 且拒绝连往私有网段，防止用户拿平台当跳板探测内网 / 云元数据。
	Restricted bool
}

// 超时策略分两套。关键在于 **流式绝不能用 http.Client.Timeout**：
// Go 的 Client.Timeout 覆盖「建连 + 重定向 + 读取响应体」全程，对 SSE 就意味着
// 生成到一半被硬掐 —— 与模型快慢无关，只与总时长有关。
//
//	非流式 —— 总时长可控（Client.Timeout），一问一答，超时即失败。
//	流式   —— Client.Timeout 置 0，改由两层兜底：
//	          ① Transport.ResponseHeaderTimeout：连上了但首字节不来，快速失败；
//	          ② streamMaxDuration：单次流的兜底总时长，防上游卡死后连接长期滞留。
var nonStreamTimeout = 120 * time.Second

// streamMaxDuration 单次流式调用的兜底总时长。量级远大于任何正常生成
// （2500 字论文即便在慢速模型上也只要数分钟），只为兜底，正常请求碰不到。
var streamMaxDuration = 10 * time.Minute

// plainTransport 沿用 http.DefaultTransport 全部默认值，只补一个首字节超时。
// 不自己拼 Transport，是为了不丢掉默认的代理、HTTP/2、连接池与 TLS 设置。
var plainTransport = func() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = 30 * time.Second
	return t
}()

// restrictedTransport 用于用户自填的 base_url：在 Control 里按实际待连 IP 拦截私有网段，
// 因此 DNS 解析后才判定，DNS rebinding 也挡得住。
var restrictedTransport = &http.Transport{
	ResponseHeaderTimeout: 30 * time.Second,
	DialContext: (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || isDisallowedIP(ip) {
				return fmt.Errorf("接口地址指向内网或保留地址，已拒绝")
			}
			return nil
		},
	}).DialContext,
}

// 重定向的目标不经 buildRequest 校验，直接禁掉，避免绕过 https 限制。
// 两个 restricted 客户端共用，别漏挂其中一个。
func denyRedirect(*http.Request, []*http.Request) error {
	return errors.New("接口地址不允许重定向")
}

var (
	httpClient       = &http.Client{Timeout: nonStreamTimeout, Transport: plainTransport}
	restrictedClient = &http.Client{
		Timeout:       nonStreamTimeout,
		Transport:     restrictedTransport,
		CheckRedirect: denyRedirect,
	}

	// 流式专用：Timeout 必须为 0，理由见上面的超时策略说明。
	streamClient           = &http.Client{Transport: plainTransport}
	restrictedStreamClient = &http.Client{Transport: restrictedTransport, CheckRedirect: denyRedirect}
)

// isDisallowedIP 私有 / 环回 / 链路本地（含 169.254.169.254 云元数据）/ 未指定地址。
func isDisallowedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	// 100.64.0.0/10 运营商级 NAT，云厂商内网服务常驻
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return true
	}
	return false
}

// ValidateUserBaseURL 校验用户自填的接口地址。保存与调用两处都要过。
func ValidateUserBaseURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return errors.New("接口地址格式不正确")
	}
	if u.Scheme != "https" {
		return errors.New("接口地址必须以 https:// 开头")
	}
	if u.Hostname() == "" {
		return errors.New("接口地址缺少域名")
	}
	// 直接填 IP 的情况在这里就拦掉；填域名的由 restrictedClient 在解析后拦截
	if ip := net.ParseIP(u.Hostname()); ip != nil && isDisallowedIP(ip) {
		return errors.New("接口地址指向内网或保留地址")
	}
	return nil
}

// client 取本次调用要用的 HTTP 客户端。streaming 必须如实传 ——
// 流式要用没有总时长上限的那个，否则长流会在生成途中被掐断。
func (cfg Config) client(streaming bool) *http.Client {
	if cfg.Restricted {
		if streaming {
			return restrictedStreamClient
		}
		return restrictedClient
	}
	if streaming {
		return streamClient
	}
	return httpClient
}

// endpoint 拼接 chat/completions；容忍 base_url 结尾带 / 或误填了完整 /chat/completions 路径。
func (c Config) endpoint() string {
	base := strings.TrimSpace(c.BaseURL)
	base = strings.TrimRight(base, "/")
	base = strings.TrimSuffix(base, "/chat/completions")
	base = strings.TrimRight(base, "/")
	return base + "/chat/completions"
}

// cleanKey 清洗 API Key：去空白、包裹引号、以及误粘贴的 "Bearer " 前缀。
func cleanKey(k string) string {
	k = strings.TrimSpace(k)
	k = strings.Trim(k, "\"'`")
	k = strings.TrimSpace(strings.TrimPrefix(k, "Bearer "))
	k = strings.TrimSpace(strings.TrimPrefix(k, "bearer "))
	return strings.TrimSpace(k)
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream"`
}

func (cfg Config) buildRequest(ctx context.Context, messages []Message, stream bool) (*http.Request, error) {
	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.Model == "" {
		return nil, errors.New("AI 未配置")
	}
	if cfg.Restricted {
		if err := ValidateUserBaseURL(cfg.BaseURL); err != nil {
			return nil, err
		}
	}
	body, err := json.Marshal(chatRequest{
		Model:       cfg.Model,
		Messages:    messages,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
		Stream:      stream,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.endpoint(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cleanKey(cfg.APIKey))
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// ChatCompletion 非流式调用，返回完整回复文本与 token 用量。
// 上游未返回 usage 时 Usage 为零值。
func ChatCompletion(ctx context.Context, cfg Config, messages []Message) (string, Usage, error) {
	req, err := cfg.buildRequest(ctx, messages, false)
	if err != nil {
		return "", Usage{}, err
	}
	resp, err := cfg.client(false).Do(req)
	if err != nil {
		return "", Usage{}, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", Usage{}, cfg.upstreamError(resp.StatusCode, raw)
	}

	var out struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", Usage{}, err
	}
	if len(out.Choices) == 0 {
		return "", out.Usage, errors.New("大模型未返回内容")
	}
	return out.Choices[0].Message.Content, out.Usage, nil
}

// StreamResult 一次流式调用的收尾信息。
type StreamResult struct {
	Usage Usage
	// FinishReason 上游给的结束原因：stop=正常说完，length=撞上 max_tokens，
	// content_filter 等。多数 OpenAI 兼容服务只在最后一个 chunk 里给。
	FinishReason string
	// Truncated 表示内容因长度上限而不完整。这种情况上游属**正常收尾**、不是错误，
	// 流会照常以 [DONE] 结束 —— 不主动报出来的话，前端只会看到「输出到一半停了」，
	// 既没有报错也无从判断原因。
	Truncated bool
}

// ChatCompletionStream 流式调用，逐块回调增量文本。onDelta 返回 error 可提前中止。
// 返回上游在末尾附带的 usage —— 多数服务流式下不返回，此时为零值。
// 不主动加 stream_options.include_usage：部分 OpenAI 兼容服务会因未知字段直接报错。
func ChatCompletionStream(ctx context.Context, cfg Config, messages []Message, onDelta func(string) error) (StreamResult, error) {
	var res StreamResult

	// 总时长兜底：上游卡死时靠它收场。正常生成远到不了这个量级。
	// 注意这里**不能**改用 Client.Timeout —— 那个会掐断正常的长流，见文件顶部的说明。
	ctx, cancel := context.WithTimeout(ctx, streamMaxDuration)
	defer cancel()

	req, err := cfg.buildRequest(ctx, messages, true)
	if err != nil {
		return res, err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := cfg.client(true).Do(req)
	if err != nil {
		return res, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return res, cfg.upstreamError(resp.StatusCode, raw)
	}

	reader := bufio.NewReader(resp.Body)
	for {
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
				if data == "[DONE]" {
					return res, nil
				}
				var chunk struct {
					Choices []struct {
						Delta struct {
							Content string `json:"content"`
						} `json:"delta"`
						FinishReason string `json:"finish_reason"`
					} `json:"choices"`
					Usage *Usage `json:"usage"`
				}
				if json.Unmarshal([]byte(data), &chunk) == nil {
					if chunk.Usage != nil && chunk.Usage.TotalTokens > 0 {
						res.Usage = *chunk.Usage
					}
					if len(chunk.Choices) > 0 {
						// 最后一个 chunk 会带 finish_reason，正文通常为空。
						// 只在非空时覆盖，避免后面的空值把已记下的原因抹掉。
						if fr := chunk.Choices[0].FinishReason; fr != "" {
							res.FinishReason = fr
							res.Truncated = fr == "length"
						}
						if c := chunk.Choices[0].Delta.Content; c != "" {
							if cbErr := onDelta(c); cbErr != nil {
								return res, cbErr
							}
						}
					}
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return res, nil
			}
			return res, err
		}
	}
}

// upstreamError 组装上游非 200 的错误。
// 用户自填地址时不回显响应体 —— 否则这个接口就成了带认证的任意 URL 读取器。
func (cfg Config) upstreamError(status int, body []byte) error {
	if cfg.Restricted {
		return fmt.Errorf("大模型接口返回 %d，请检查接口地址、API Key 与模型名", status)
	}
	return fmt.Errorf("大模型接口返回 %d：%s", status, truncate(string(body), 300))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
