package gen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProviderCloudflare는 Cloudflare Workers AI 프로바이더 식별자입니다.
const ProviderCloudflare = "cloudflare"

func init() {
	registerProvider(providerDef{
		id:    ProviderCloudflare,
		label: "Cloudflare",
		// flux-1-schnell은 빠른 추론 속도와 안정적인 스프라이트 스트립 생성에
		// 유리한 기본 모델입니다. SDXL은 참조 이미지(img2img)를 지원하는 대안입니다.
		defaultModel: "@cf/black-forest-labs/flux-1-schnell",
		models:       []string{"@cf/black-forest-labs/flux-1-schnell", "@cf/stabilityai/stable-diffusion-xl-base-1.0"},
		sortOrder:    6,
		// AccountID + API 토큰 두 자격증명이 필요하므로 전용 생성자 NewCloudflare를
		// 사용합니다 (gen.New 대신). core.md의 계약을 참고하세요.
		newFunc: nil,
	})
}

// cloudflareRunURL은 Workers AI 텍스트-이미지 동기 실행 엔드포인트 포맷입니다.
const cloudflareRunURL = "https://api.cloudflare.com/client/v4/accounts/%s/ai/run/%s"

// Cloudflare는 Cloudflare Workers AI 이미지 생성 API 클라이언트입니다.
//
// Workers AI는 계정 ID + API 토큰 두 자격증명을 요구하며, 단일 API 키 방식의
// 다른 프로바이더와 달리 전용 생성자(NewCloudflare)로만 생성할 수 있습니다.
type Cloudflare struct {
	AccountID string
	APIToken  string
	Model     string // 예: @cf/black-forest-labs/flux-1-schnell
	HTTP      *http.Client

	endpoint string // 테스트용 오버라이드 (빈 값이면 cloudflareRunURL로 생성)
}

// NewCloudflare는 새 Cloudflare Workers AI 클라이언트를 생성합니다.
func NewCloudflare(accountID, apiToken, model string) *Cloudflare {
	if model == "" {
		model = DefaultModelFor(ProviderCloudflare)
	}
	return &Cloudflare{
		AccountID: accountID,
		APIToken:  apiToken,
		Model:     model,
		HTTP:      &http.Client{Timeout: 300 * time.Second},
	}
}

type cfRequest struct {
	Prompt string `json:"prompt"`
}

type cfResponse struct {
	Result struct {
		Image string `json:"image"`
		Data  string `json:"data"`
	} `json:"result"`
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

// runURL은 모델 실행 엔드포인트 URL을 반환합니다 (테스트 오버라이드 우선).
func (c *Cloudflare) runURL() string {
	if c.endpoint != "" {
		return c.endpoint
	}
	return fmt.Sprintf(cloudflareRunURL, c.AccountID, c.Model)
}

// GenerateImage는 프롬프트로 이미지를 생성합니다.
//
// Workers AI의 flux/SD 계열 모델은 종횡비 파라미터를 받지 않으므로 프롬프트에
// aspectHint를 덧붙입니다. 참조 이미지(img2img)는 SDXL 등 일부 모델에서만
// 지원되므로 현재는 전달하지 않습니다 (텍스트-이미지 경로로 통일).
func (c *Cloudflare) GenerateImage(ctx context.Context, prompt string, refImages [][]byte, aspectRatio string) ([]byte, error) {
	if c.AccountID == "" || c.APIToken == "" {
		return nil, errors.New("Cloudflare Account ID and API token are required")
	}

	fullPrompt := prompt + "\n\n" + aspectHint(aspectRatio)
	body, err := json.Marshal(cfRequest{Prompt: fullPrompt})
	if err != nil {
		return nil, fmt.Errorf("request serialization failed: %w", err)
	}

	var lastErr error
	backoff := 2 * time.Second
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}
		img, retryable, err := c.doRequest(ctx, c.runURL(), body)
		if err == nil {
			return img, nil
		}
		lastErr = err
		if !retryable {
			return nil, err
		}
	}
	return nil, lastErr
}

func (c *Cloudflare) doRequest(ctx context.Context, url string, body []byte) (img []byte, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, true, fmt.Errorf("response read failed: %w", err)
	}

	// 이진 출력 모델(예: stable-diffusion-xl-base-1.0)은 JSON 봉투 대신 PNG
	// 바이트를 직접 반환합니다 (Content-Type: image/png).
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "image/") {
		if resp.StatusCode != http.StatusOK {
			return nil, isRetryableStatus(resp.StatusCode), fmt.Errorf("Cloudflare error (HTTP %d)", resp.StatusCode)
		}
		return respBytes, false, nil
	}

	var parsed cfResponse
	if resp.StatusCode != http.StatusOK {
		retryable = isRetryableStatus(resp.StatusCode)
		_ = json.Unmarshal(respBytes, &parsed)
		if len(parsed.Errors) > 0 && parsed.Errors[0].Message != "" {
			return nil, retryable, fmt.Errorf("Cloudflare error (%d): %s", resp.StatusCode, parsed.Errors[0].Message)
		}
		return nil, retryable, fmt.Errorf("Cloudflare error (HTTP %d)", resp.StatusCode)
	}

	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		// JSON 봉투가 아니면(바이너리 본문이 image/* Content-Type 없이 온 경우)
		// PNG 시그니처를 확인해 원시 바이트로 취급합니다.
		if isPNG(respBytes) {
			return respBytes, false, nil
		}
		return nil, false, fmt.Errorf("response parse failed: %w", err)
	}

	b64 := parsed.Result.Image
	if b64 == "" {
		// 일부 Workers AI 모델/버전은 image 대신 data 필드를 반환합니다.
		b64 = parsed.Result.Data
	}
	if b64 == "" {
		return nil, true, errors.New("response has no image")
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, false, fmt.Errorf("image decode failed: %w", err)
	}
	return data, false, nil
}

// isRetryableStatus는 429(요청 과다) 또는 5xx(서버 오류)를 재시도 대상으로 봅니다.
func isRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

// isPNG는 바이트가 PNG 시그니처로 시작하는지 확인합니다.
func isPNG(b []byte) bool {
	return len(b) >= 8 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G'
}

// ValidateKey는 Cloudflare 계정 ID와 API 토큰 유효성을 확인합니다.
func (c *Cloudflare) ValidateKey(ctx context.Context) error {
	if c.AccountID == "" || c.APIToken == "" {
		return errors.New("Cloudflare Account ID and API token are required")
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s", c.AccountID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIToken)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return errors.New("API token is invalid or lacks permission")
	default:
		return fmt.Errorf("key check failed (HTTP %d)", resp.StatusCode)
	}
}
