package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env.local")
	content := "# 주석\n" +
		"FAL_KEY=abc123:secret\n" +
		"export OPENROUTER_API_KEY=\"sk-or-test\"\n" +
		"OPENAI_API_KEY='sk-openai-test'\n" +
		"GEMINI_API_KEY='AIza-test'\n" +
		"CLOUDFLARE_ACCOUNT_ID=cf-account\n" +
		"CLOUDFLARE_API_TOKEN=cf-token\n" +
		"CUSTOM_IMAGE_BASE_URL=https://example.com/v1\n" +
		"CUSTOM_IMAGE_API_KEY=custom-key\n" +
		"INVALID_LINE\n" +
		"EMPTY=\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	out := map[string]string{}
	parseEnvFile(path, out)

	if out["FAL_KEY"] != "abc123:secret" {
		t.Fatalf("FAL_KEY 파싱 실패: %q", out["FAL_KEY"])
	}
	if out["OPENROUTER_API_KEY"] != "sk-or-test" {
		t.Fatalf("export + 따옴표 파싱 실패: %q", out["OPENROUTER_API_KEY"])
	}
	if out["OPENAI_API_KEY"] != "sk-openai-test" {
		t.Fatalf("OPENAI_API_KEY 파싱 실패: %q", out["OPENAI_API_KEY"])
	}
	if out["GEMINI_API_KEY"] != "AIza-test" {
		t.Fatalf("작은따옴표 파싱 실패: %q", out["GEMINI_API_KEY"])
	}
	if out["CLOUDFLARE_ACCOUNT_ID"] != "cf-account" {
		t.Fatalf("CLOUDFLARE_ACCOUNT_ID 파싱 실패: %q", out["CLOUDFLARE_ACCOUNT_ID"])
	}
	if out["CLOUDFLARE_API_TOKEN"] != "cf-token" {
		t.Fatalf("CLOUDFLARE_API_TOKEN 파싱 실패: %q", out["CLOUDFLARE_API_TOKEN"])
	}
	if out["CUSTOM_IMAGE_BASE_URL"] != "https://example.com/v1" {
		t.Fatalf("CUSTOM_IMAGE_BASE_URL 파싱 실패: %q", out["CUSTOM_IMAGE_BASE_URL"])
	}
	if out["CUSTOM_IMAGE_API_KEY"] != "custom-key" {
		t.Fatalf("CUSTOM_IMAGE_API_KEY 파싱 실패: %q", out["CUSTOM_IMAGE_API_KEY"])
	}
	if _, ok := out["EMPTY"]; ok {
		t.Fatal("빈 값은 무시되어야 합니다")
	}
}

func TestSettingsCfg(t *testing.T) {
	s := Settings{
		Gemini:     ProviderCfg{APIKey: "g"},
		OpenAI:     ProviderCfg{APIKey: "ai"},
		OpenRouter: ProviderCfg{APIKey: "o"},
		Fal:        ProviderCfg{APIKey: "f"},
		Cloudflare: ProviderCfg{AccountID: "cf-acct", APIKey: "cf-tok"},
		Custom:     ProviderCfg{BaseURL: "https://example.com", APIKey: "custom-tok"},
	}
	if s.Cfg("openai").APIKey != "ai" || s.Cfg("openrouter").APIKey != "o" || s.Cfg("fal").APIKey != "f" {
		t.Fatal("프로바이더별 설정 매핑 오류")
	}
	if s.Cfg("cloudflare").AccountID != "cf-acct" || s.Cfg("cloudflare").APIKey != "cf-tok" {
		t.Fatal("cloudflare 설정 매핑 오류")
	}
	if s.Cfg("custom").BaseURL != "https://example.com" || s.Cfg("custom").APIKey != "custom-tok" {
		t.Fatal("custom 설정 매핑 오류")
	}
	// 알 수 없는 프로바이더는 gemini로 폴백
	if s.Cfg("unknown").APIKey != "g" {
		t.Fatal("기본 폴백 오류")
	}
	// 포인터 반환이므로 수정이 반영되어야 함
	s.Cfg("fal").Model = "m"
	if s.Fal.Model != "m" {
		t.Fatal("Cfg는 포인터를 반환해야 합니다")
	}
}

// TestLoadEnvFallbackNewVars는 신규 환경변수(Cloudflare/Custom) 폴백을 검증합니다.
func TestLoadEnvFallbackNewVars(t *testing.T) {
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "cf-acct")
	t.Setenv("CLOUDFLARE_API_TOKEN", "cf-tok")
	t.Setenv("CUSTOM_IMAGE_BASE_URL", "https://img.example.com")
	t.Setenv("CUSTOM_IMAGE_API_KEY", "custom-tok")

	env := loadEnvFallback()
	if env["CLOUDFLARE_ACCOUNT_ID"] != "cf-acct" {
		t.Fatalf("CLOUDFLARE_ACCOUNT_ID 환경변수 폴백 실패: %q", env["CLOUDFLARE_ACCOUNT_ID"])
	}
	if env["CLOUDFLARE_API_TOKEN"] != "cf-tok" {
		t.Fatalf("CLOUDFLARE_API_TOKEN 환경변수 폴백 실패: %q", env["CLOUDFLARE_API_TOKEN"])
	}
	if env["CUSTOM_IMAGE_BASE_URL"] != "https://img.example.com" {
		t.Fatalf("CUSTOM_IMAGE_BASE_URL 환경변수 폴백 실패: %q", env["CUSTOM_IMAGE_BASE_URL"])
	}
	if env["CUSTOM_IMAGE_API_KEY"] != "custom-tok" {
		t.Fatalf("CUSTOM_IMAGE_API_KEY 환경변수 폴백 실패: %q", env["CUSTOM_IMAGE_API_KEY"])
	}
}

// TestLegacyConfigMigration은 기존 config.json(신규 필드 없음) 로딩 안전성과
// 빈 신규 필드의 omitempty 라운드트립을 검증합니다.
func TestLegacyConfigMigration(t *testing.T) {
	old := `{"provider":"openai","gemini":{"apiKey":"g","model":""},"openai":{"apiKey":"sk-ai","model":"gpt-image-1"}}`
	var s Settings
	if err := json.Unmarshal([]byte(old), &s); err != nil {
		t.Fatalf("기존 설정 로딩 실패: %v", err)
	}
	if s.OpenAI.APIKey != "sk-ai" || s.OpenAI.Model != "gpt-image-1" {
		t.Fatal("기존 필드 보존 오류")
	}
	if s.Cloudflare.AccountID != "" || s.Cloudflare.APIKey != "" || s.Cloudflare.BaseURL != "" {
		t.Fatal("신규 cloudflare 필드는 기본 빈 값이어야 합니다")
	}
	if s.Custom.BaseURL != "" || s.Custom.APIKey != "" || s.Custom.AccountID != "" {
		t.Fatal("신규 custom 필드는 기본 빈 값이어야 합니다")
	}

	// 빈 baseUrl/accountId는 omitempty로 인해 출력에서 빠져야 합니다.
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("마샬링 실패: %v", err)
	}
	text := string(data)
	if strings.Contains(text, "baseUrl") || strings.Contains(text, "accountId") {
		t.Fatalf("빈 baseUrl/accountId가 출력에 포함됨: %s", text)
	}
}
