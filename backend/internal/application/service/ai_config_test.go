package service

import (
	"encoding/json"
	"errors"
	"testing"

	"software-exam/internal/domain/model"
)

// ---- AI 配置 DTO 出参契约（管理员全局 / 会员自定义） ----

type stubAIConfigRepo struct {
	cfg *model.AIConfig
	err error
}

func (s *stubAIConfigRepo) Get() (*model.AIConfig, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.cfg == nil {
		s.cfg = &model.AIConfig{}
	}
	cp := *s.cfg
	return &cp, nil
}

func (s *stubAIConfigRepo) Update(*model.AIConfig) error { panic("unused") }

type stubUserAIConfigRepo struct {
	cfg *model.UserAIConfig
	err error
}

func (s *stubUserAIConfigRepo) GetByUser(uint64) (*model.UserAIConfig, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.cfg == nil {
		s.cfg = &model.UserAIConfig{}
	}
	cp := *s.cfg
	return &cp, nil
}

func (s *stubUserAIConfigRepo) Upsert(*model.UserAIConfig) error { panic("unused") }

// GetConfigMasked：脱敏 DTO 字段类型正确；key 以 key_set/key_masked 输出，
// free_trial_limit 恒输出（含 0，避免管理端表单因 omitempty 丢失 0）。
func TestGetConfigMasked_TypedFields(t *testing.T) {
	cfgRepo := &stubAIConfigRepo{cfg: &model.AIConfig{
		BaseURL: "https://api.example.com", APIKey: "sk-secret-123456",
		Model: "gpt-4o-mini", Temperature: 0.7, MaxTokens: 2048,
		Enabled: 1, FreeTrialLimit: 5,
	}}
	svc := NewAIService(cfgRepo, &stubUserAIConfigRepo{}, nil, nil, nil, nil, nil, nil)

	cfg, err := svc.GetConfigMasked()
	if err != nil {
		t.Fatalf("应无错返回，得到 %v", err)
	}
	if cfg.BaseURL != "https://api.example.com" || cfg.Model != "gpt-4o-mini" {
		t.Fatalf("base_url/model 透传错: %+v", cfg)
	}
	if cfg.Temperature != 0.7 || cfg.MaxTokens != 2048 || cfg.Enabled != 1 {
		t.Fatalf("temperature/max_tokens/enabled 透传错: %+v", cfg)
	}
	if cfg.FreeTrialLimit != 5 {
		t.Fatalf("free_trial_limit 应透传 5，得到 %d", cfg.FreeTrialLimit)
	}
	if !cfg.KeySet {
		t.Fatal("有 key 时 key_set 应为 true")
	}
	if cfg.KeyMasked == "" || cfg.KeyMasked == "sk-secret-123456" {
		t.Fatalf("key_masked 应脱敏且非空，得到 %q", cfg.KeyMasked)
	}

	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	for _, k := range []string{"base_url", "model", "temperature", "max_tokens", "enabled", "free_trial_limit", "key_set", "key_masked"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("JSON 缺 key %q: %s", k, b)
		}
	}
}

// GetConfigMasked：free_trial_limit=0 时 key 也必须在（不加 omitempty）。
func TestGetConfigMasked_ZeroFreeTrialStillSerialized(t *testing.T) {
	cfgRepo := &stubAIConfigRepo{cfg: &model.AIConfig{
		BaseURL: "https://api.example.com", Model: "gpt-4o-mini", FreeTrialLimit: 0,
	}}
	svc := NewAIService(cfgRepo, &stubUserAIConfigRepo{}, nil, nil, nil, nil, nil, nil)

	cfg, err := svc.GetConfigMasked()
	if err != nil {
		t.Fatalf("应无错返回，得到 %v", err)
	}
	if cfg.FreeTrialLimit != 0 {
		t.Fatalf("free_trial_limit=0 应保留，得到 %d", cfg.FreeTrialLimit)
	}
	if cfg.KeySet {
		t.Fatal("无 key 时 key_set 应为 false")
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if string(m["free_trial_limit"]) != "0" {
		t.Fatalf("free_trial_limit=0 必须序列化出现，得到 %s", m["free_trial_limit"])
	}
}

// GetUserConfigMasked：会员自定义出参不含 free_trial_limit（那是全局配置属性）。
func TestGetUserConfigMasked_NoFreeTrialLimit(t *testing.T) {
	userCfgRepo := &stubUserAIConfigRepo{cfg: &model.UserAIConfig{
		BaseURL: "https://user.example.com", APIKey: "user-secret-abcdef",
		Model: "deepseek-chat", Temperature: 0.2, MaxTokens: 4096, Enabled: 1,
	}}
	svc := NewAIService(&stubAIConfigRepo{}, userCfgRepo, nil, nil, nil, nil, nil, nil)

	cfg, err := svc.GetUserConfigMasked(7)
	if err != nil {
		t.Fatalf("应无错返回，得到 %v", err)
	}
	if cfg.BaseURL != "https://user.example.com" || cfg.Model != "deepseek-chat" ||
		cfg.Temperature != 0.2 || cfg.MaxTokens != 4096 || cfg.Enabled != 1 {
		t.Fatalf("字段透传错: %+v", cfg)
	}
	if !cfg.KeySet || cfg.KeyMasked == "" || cfg.KeyMasked == "user-secret-abcdef" {
		t.Fatalf("key_set/key_masked 脱敏错: %+v", cfg)
	}

	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if _, ok := m["free_trial_limit"]; ok {
		t.Fatalf("会员自定义出参不应含 free_trial_limit: %s", b)
	}
	for _, k := range []string{"base_url", "model", "temperature", "max_tokens", "enabled", "key_set", "key_masked"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("JSON 缺 key %q: %s", k, b)
		}
	}
}

// 两个脱敏出参均透传 repo 错误。
func TestGetConfigMasked_PropagatesRepoError(t *testing.T) {
	cfgRepo := &stubAIConfigRepo{err: errors.New("db down")}
	svc := NewAIService(cfgRepo, &stubUserAIConfigRepo{}, nil, nil, nil, nil, nil, nil)

	if _, err := svc.GetConfigMasked(); err == nil {
		t.Fatal("应传播 repo 错误，得到 nil")
	}
}