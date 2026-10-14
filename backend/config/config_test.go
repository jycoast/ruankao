package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain 在跑任何用例前，清掉进程里继承来的 SOFTWARE_EXAM_* 环境变量。
//
// 本文件大半用例断言的是「这个值来自 yaml」或「来自 config.local.yaml 覆盖层」，
// 而 viper 的 SetEnvPrefix + AutomaticEnv（见 config.go 的 Load）会把同名环境变量
// 当成优先级更高的配置来源。只要环境里有，这些断言就会拿到环境变量的值而假红：
//   - CI 的 Test 步骤曾经注入 SOFTWARE_EXAM_DATABASE_PASSWORD: root，
//     直接把 TestLocalOverrideMergesOntoBase 顶红；
//   - 开发者本机 export 过数据库口令/密钥也一样会中招。
//
// 需要环境变量的用例自己用 t.Setenv 显式设置，在这里清完之后设置，不受影响。
// 清理放在 TestMain 而不是逐个用例里，是为了让以后新加的用例默认就是封闭的。
func TestMain(m *testing.M) {
	for _, kv := range os.Environ() {
		if key, _, ok := strings.Cut(kv, "="); ok && strings.HasPrefix(key, envPrefix+"_") {
			os.Unsetenv(key)
		}
	}
	os.Exit(m.Run())
}

func writeCfg(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const baseCfg = `
server:
  mode: release
database:
  host: yaml-host
  password: yaml-pass
jwt:
  secret: "yaml-secret-that-is-long-enough-to-pass-32"
oauth:
  linuxdo:
    client_secret: yaml-client-secret
`

// 凭据必须能完全由环境变量注入，否则无法把 yaml 移出版本控制。
func TestEnvOverridesYAML(t *testing.T) {
	t.Setenv("SOFTWARE_EXAM_JWT_SECRET", "env-secret-that-is-also-long-enough-here")
	t.Setenv("SOFTWARE_EXAM_DATABASE_PASSWORD", "env-pass")
	t.Setenv("SOFTWARE_EXAM_OAUTH_LINUXDO_CLIENT_SECRET", "env-client-secret")

	if err := Load(writeCfg(t, baseCfg)); err != nil {
		t.Fatalf("load: %v", err)
	}
	if AppConfig.JWT.Secret != "env-secret-that-is-also-long-enough-here" {
		t.Errorf("jwt.secret not overridden: %q", AppConfig.JWT.Secret)
	}
	if AppConfig.Database.Password != "env-pass" {
		t.Errorf("database.password not overridden: %q", AppConfig.Database.Password)
	}
	if AppConfig.OAuth.LinuxDo.ClientSecret != "env-client-secret" {
		t.Errorf("linuxdo client_secret not overridden: %q", AppConfig.OAuth.LinuxDo.ClientSecret)
	}
	// 未被环境变量覆盖的字段仍应来自 yaml
	if AppConfig.Database.Host != "yaml-host" {
		t.Errorf("database.host should come from yaml, got %q", AppConfig.Database.Host)
	}
}

// 忘了轮换密钥应在启动时炸掉，而不是上线后被人自签管理员 token。
func TestReleaseRejectsPlaceholderSecret(t *testing.T) {
	cfg := strings.Replace(baseCfg,
		`"yaml-secret-that-is-long-enough-to-pass-32"`, `"`+placeholderJWTSecret+`"`, 1)
	err := Load(writeCfg(t, cfg))
	if err == nil || !strings.Contains(err.Error(), "占位值") {
		t.Fatalf("expected placeholder rejection, got %v", err)
	}
}

func TestReleaseRejectsShortSecret(t *testing.T) {
	cfg := strings.Replace(baseCfg, `"yaml-secret-that-is-long-enough-to-pass-32"`, `"short"`, 1)
	if err := Load(writeCfg(t, cfg)); err == nil {
		t.Fatal("expected short secret to be rejected in release mode")
	}
}

// debug 模式不做强校验，本地开发不该被拦住。
func TestDebugAllowsPlaceholder(t *testing.T) {
	cfg := strings.Replace(baseCfg, "mode: release", "mode: debug", 1)
	cfg = strings.Replace(cfg,
		`"yaml-secret-that-is-long-enough-to-pass-32"`, `"`+placeholderJWTSecret+`"`, 1)
	if err := Load(writeCfg(t, cfg)); err != nil {
		t.Fatalf("debug mode should not enforce secret rules: %v", err)
	}
}

// 主配置存容器里的服务名，本机靠 config.local.yaml 覆盖成可达地址；
// 覆盖层只写差异项，未提及的字段必须保持主配置的值。
func TestLocalOverrideMergesOntoBase(t *testing.T) {
	p := writeCfg(t, baseCfg)
	local := filepath.Join(filepath.Dir(p), "config.local.yaml")
	if err := os.WriteFile(local, []byte("database:\n  host: local-host\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Load(p); err != nil {
		t.Fatalf("load: %v", err)
	}
	if AppConfig.Database.Host != "local-host" {
		t.Errorf("database.host should come from override, got %q", AppConfig.Database.Host)
	}
	if AppConfig.Database.Password != "yaml-pass" {
		t.Errorf("database.password should survive the merge, got %q", AppConfig.Database.Password)
	}
}

// 容器里没有覆盖层，缺文件是正常状态而非错误。
func TestLoadWithoutLocalOverride(t *testing.T) {
	if err := Load(writeCfg(t, baseCfg)); err != nil {
		t.Fatalf("missing override should not fail: %v", err)
	}
	if AppConfig.Database.Host != "yaml-host" {
		t.Errorf("database.host should come from base, got %q", AppConfig.Database.Host)
	}
}
