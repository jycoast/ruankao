package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"software-exam/config"
	"software-exam/internal/application/service"
	"software-exam/pkg/response"

	"github.com/gin-gonic/gin"
)

// oauthStateStore 服务端存储 OAuth state，避免跨域 Cookie 问题
var oauthStateStore = &stateStore{m: make(map[string]time.Time)}

type stateStore struct {
	mu sync.Mutex
	m  map[string]time.Time
}

func (s *stateStore) Set(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 顺带清理超过 15 分钟的旧 state
	now := time.Now()
	for k, t := range s.m {
		if now.Sub(t) > 15*time.Minute {
			delete(s.m, k)
		}
	}
	s.m[state] = now
}

func (s *stateStore) Verify(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.m[state]
	if !ok {
		return false
	}
	delete(s.m, state)
	return time.Since(t) <= 10*time.Minute
}

type OAuthHandler struct {
	userService *service.UserService
}

func NewOAuthHandler(userService *service.UserService) *OAuthHandler {
	return &OAuthHandler{userService: userService}
}

type linuxDoUser struct {
	ID         uint64 `json:"id"`
	Username   string `json:"username"`
	Name       string `json:"name"`
	AvatarURL  string `json:"avatar_url"`
	Email      string `json:"email"`
	TrustLevel int    `json:"trust_level"`
	Active     bool   `json:"active"`
	Silenced   bool   `json:"silenced"`
}

// LinuxDoInitiate 发起 OAuth 授权，浏览器跳转到 LinuxDo
// GET /api/auth/oauth/linuxdo
func (h *OAuthHandler) LinuxDoInitiate(c *gin.Context) {
	state := randomState()
	oauthStateStore.Set(state)

	cfg := config.AppConfig.OAuth.LinuxDo
	params := url.Values{
		"client_id":     {cfg.ClientID},
		"redirect_uri":  {cfg.RedirectURL},
		"response_type": {"code"},
		"scope":         {"read"},
		"state":         {state},
	}
	c.Redirect(http.StatusFound, cfg.AuthorizeEndpoint+"?"+params.Encode())
}

// LinuxDoCallback 处理授权回调
// GET /api/auth/oauth/linuxdo/callback
func (h *OAuthHandler) LinuxDoCallback(c *gin.Context) {
	frontendURL := config.AppConfig.OAuth.FrontendURL
	fail := func(reason string) {
		c.Redirect(http.StatusFound, frontendURL+"/#/login?oauth_error="+url.QueryEscape(reason))
	}

	// 1. 验证 state（服务端存储，不依赖 Cookie）
	state := c.Query("state")
	if state == "" || !oauthStateStore.Verify(state) {
		fail("state_mismatch")
		return
	}

	// 2. 检查 error 参数（用户拒绝授权等）
	if errParam := c.Query("error"); errParam != "" {
		fail(errParam)
		return
	}

	code := c.Query("code")
	if code == "" {
		fail("missing_code")
		return
	}

	// 3. 用 code 换 access_token
	accessToken, err := exchangeLinuxDoToken(code)
	if err != nil {
		log.Printf("[oauth] token exchange failed: %v", err)
		fail("token_exchange_failed")
		return
	}

	// 4. 获取 LinuxDo 用户信息
	ldUser, err := fetchLinuxDoUser(accessToken)
	if err != nil {
		fail("userinfo_failed")
		return
	}
	if !ldUser.Active {
		fail("account_inactive")
		return
	}
	if ldUser.Silenced {
		fail("account_silenced")
		return
	}

	// 5. 在本地库找或创建用户，颁发 JWT
	_, token, err := h.userService.FindOrCreateByLinuxDo(
		ldUser.ID, ldUser.Username, ldUser.Email, ldUser.Name, ldUser.AvatarURL,
	)
	if err != nil {
		fail("db_error")
		return
	}

	// 6. 跳转回前端，带上 token（前端已迁到 history 路由，不再是 /#/oauth/callback）
	c.Redirect(http.StatusFound, frontendURL+"/oauth/callback?token="+token)
}

// LinuxDoStatus 检查当前配置是否已启用（前端用来决定是否显示登录按钮）
// GET /api/auth/oauth/linuxdo/status
func (h *OAuthHandler) LinuxDoStatus(c *gin.Context) {
	cfg := config.AppConfig.OAuth.LinuxDo
	enabled := cfg.ClientID != "" && cfg.ClientSecret != ""
	response.Success(c, gin.H{"enabled": enabled})
}

// ---- helpers ----

func randomState() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

var oauthHTTPClient = &http.Client{Timeout: 15 * time.Second}

func exchangeLinuxDoToken(code string) (string, error) {
	cfg := config.AppConfig.OAuth.LinuxDo

	body := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {cfg.RedirectURL},
	}

	req, err := http.NewRequest(http.MethodPost, cfg.TokenEndpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(cfg.ClientID, cfg.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(raw))
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("empty access_token in response")
	}
	return result.AccessToken, nil
}

func fetchLinuxDoUser(accessToken string) (*linuxDoUser, error) {
	cfg := config.AppConfig.OAuth.LinuxDo

	req, err := http.NewRequest(http.MethodGet, cfg.UserEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user endpoint returned %d: %s", resp.StatusCode, string(raw))
	}

	var u linuxDoUser
	if err := json.Unmarshal(raw, &u); err != nil {
		return nil, err
	}
	return &u, nil
}
