package gen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

// ProviderCustom은 OpenAI 호환(custom) 프로바이더 식별자입니다.
const ProviderCustom = "custom"

func init() {
	registerProvider(providerDef{
		id:           ProviderCustom,
		label:        "Custom (OpenAI-compatible)",
		defaultModel: "gpt-image-1",
		// gpt-image-1은 중립적인 기본 모델이며, 실제 엔진(LocalAI, ComfyUI-proxy,
		// Together 등)의 모델명은 사용자가 자유롭게 입력할 수 있습니다.
		models:    []string{"gpt-image-1"},
		sortOrder: 7,
		// BaseURL(+선택 API 키)이 필요한 프로바이더이므로 전용 생성자 NewCustom을
		// 사용합니다 (gen.New 대신). core.md의 계약을 참고하세요.
		newFunc: nil,
	})
}

// Custom은 OpenAI 호환 이미지 생성 API 클라이언트입니다.
// LocalAI, ComfyUI-proxy, Together 등 BaseURL만 다른 모든 엔진을 포괄합니다.
type Custom struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client

	generationEndpoint string // 테스트용 오버라이드
	editEndpoint       string // 테스트용 오버라이드
}

// NewCustom은 새 OpenAI 호환 클라이언트를 생성합니다 (BaseURL의 trailing slash 제거).
func NewCustom(baseURL, apiKey, model string) *Custom {
	if model == "" {
		model = DefaultModelFor(ProviderCustom)
	}
	return &Custom{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		HTTP:    &http.Client{Timeout: 300 * time.Second},
	}
}

type customImageRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n"`
	Size           string `json:"size,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
}

type customImageResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
		URL     string `json:"url"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error"`
}

func (c *Custom) generationURL() string {
	if c.generationEndpoint != "" {
		return c.generationEndpoint
	}
	return strings.TrimRight(c.BaseURL, "/") + "/v1/images/generations"
}

func (c *Custom) editURL() string {
	if c.editEndpoint != "" {
		return c.editEndpoint
	}
	return strings.TrimRight(c.BaseURL, "/") + "/v1/images/edits"
}

// GenerateImage는 OpenAI 호환 API로 이미지를 생성하거나 참조 이미지를 편집합니다.
func (c *Custom) GenerateImage(ctx context.Context, prompt string, refImages [][]byte, aspectRatio string) ([]byte, error) {
	if c.BaseURL == "" {
		return nil, errors.New("Custom Base URL is not configured")
	}

	fullPrompt := prompt + "\n\n" + aspectHint(aspectRatio)
	size := openAISizeFor(aspectRatio)

	var body []byte
	var contentType string
	var endpoint string
	var err error
	if len(refImages) == 0 {
		endpoint = c.generationURL()
		body, err = json.Marshal(customImageRequest{
			Model:          c.Model,
			Prompt:         fullPrompt,
			N:              1,
			Size:           size,
			ResponseFormat: "b64_json",
		})
		contentType = "application/json"
	} else {
		endpoint = c.editURL()
		body, contentType, err = c.buildEditBody(fullPrompt, refImages, size)
	}
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
		img, retryable, err := c.doRequest(ctx, endpoint, contentType, body)
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

func (c *Custom) buildEditBody(prompt string, refImages [][]byte, size string) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fields := map[string]string{
		"model":  c.Model,
		"prompt": prompt,
		"n":      "1",
		"size":   size,
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return nil, "", err
		}
	}
	for i, img := range refImages {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image[]"; filename="reference-%d.png"`, i+1))
		h.Set("Content-Type", "image/png")
		part, err := w.CreatePart(h)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(img); err != nil {
			return nil, "", err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}

func (c *Custom) doRequest(ctx context.Context, endpoint, contentType string, body []byte) (img []byte, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	// API 키는 선택 사항입니다 (LocalAI 등 키리스 엔진은 Authorization 헤더를 보내지 않음).
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, true, fmt.Errorf("response read failed: %w", err)
	}

	var parsed customImageResponse
	_ = json.Unmarshal(respBytes, &parsed)

	if resp.StatusCode != http.StatusOK {
		retryable = resp.StatusCode == 429 || resp.StatusCode >= 500
		if parsed.Error != nil && parsed.Error.Message != "" {
			return nil, retryable, fmt.Errorf("Custom error (%d): %s", resp.StatusCode, parsed.Error.Message)
		}
		return nil, retryable, fmt.Errorf("Custom error (HTTP %d)", resp.StatusCode)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, true, fmt.Errorf("Custom error: %s", parsed.Error.Message)
	}
	if len(parsed.Data) == 0 {
		return nil, true, errors.New("response has no image")
	}
	d := parsed.Data[0]
	if d.B64JSON != "" {
		data, err := base64.StdEncoding.DecodeString(d.B64JSON)
		if err != nil {
			return nil, false, fmt.Errorf("image decode failed: %w", err)
		}
		return data, false, nil
	}
	if d.URL != "" {
		data, err := decodeDataOrDownload(c.HTTP, d.URL)
		if err != nil {
			return nil, false, err
		}
		return data, false, nil
	}
	return nil, true, errors.New("response has no image")
}

// ValidateKey는 API 키 유효성을 확인합니다.
//
// 키리스 엔진(LocalAI 등)은 APIKey가 비어 있으면 유효로 간주합니다. 키가 있을 때는
// GET {BaseURL}/v1/models로 확인하며, 200이면 유효, 401/403이면 무효입니다. 일부
// 엔진은 /v1/models를 구현하지 않아 404를 반환하는데, 이 경우 "검증 불가"로 보고
// 하드 실패하지 않고 길이(leniency) 검사로 폴백합니다 (fal/byteplus와 동일한 방식).
func (c *Custom) ValidateKey(ctx context.Context) error {
	if c.BaseURL == "" {
		return errors.New("Custom Base URL is not configured")
	}
	key := strings.TrimSpace(c.APIKey)
	if key == "" {
		return nil
	}

	ep := strings.TrimRight(c.BaseURL, "/") + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return errors.New("API key is invalid or lacks permission")
	case http.StatusNotFound:
		// /v1/models 미지원 엔진: 검증 불가 → 길이(leniency) 검사로 폴백.
		if len(key) < 10 {
			return errors.New("API key is too short")
		}
		return nil
	default:
		return fmt.Errorf("key check failed (HTTP %d)", resp.StatusCode)
	}
}
