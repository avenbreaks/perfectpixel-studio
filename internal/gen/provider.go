package gen

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// 지원 프로바이더 식별자
const (
	ProviderGemini     = "gemini"
	ProviderOpenAI     = "openai"
	ProviderOpenRouter = "openrouter"
	ProviderFal        = "fal"
	ProviderBytePlus   = "byteplus"
)

// providerDef는 단일 프로바이더의 레지스트리 항목입니다.
// 각 프로바이더 파일은 자신의 init()에서 registerProvider를 호출해 자기 자신을
// 등록합니다. 이로써 새 프로바이더(cloudflare, custom 등)는 이 공유 파일을
// 수정하지 않고도 독립된 파일 하나로 추가할 수 있습니다 (머지 충돌 회피).
type providerDef struct {
	id           string   // 레지스트리 식별자 (예: "gemini")
	label        string   // UI 표시용 이름
	defaultModel string   // 기본 모델 (models[0]과 일치해야 함)
	models       []string // 선택 가능한 모델 목록 (최신 모델이 맨 앞)
	sortOrder    int      // UI 노출 순서 (작을수록 앞)
	// newFunc는 apiKey + model로 단일 키 프로바이더를 생성합니다.
	//
	// 추가 필드가 필요한 프로바이더(cloudflare: AccountID+APIKey, custom:
	// BaseURL+선택 키)는 newFunc를 nil로 두고 전용 생성자(NewCloudflare /
	// NewCustom)를 노출합니다. 이 경우 앱 레이어는 gen.New 대신 해당 전용
	// 생성자를 ProviderCfg에서 직접 호출해야 합니다.
	// 자세한 계약은 .orca/handoff/core.md를 참고하세요.
	newFunc func(apiKey, model string) Provider
}

// providerRegistry는 등록된 프로바이더 맵입니다 (id → 정의).
var providerRegistry = map[string]*providerDef{}

// registerProvider는 프로바이더 파일의 init()에서 호출해 레지스트리에 등록합니다.
func registerProvider(def providerDef) {
	if def.id == "" {
		panic("gen: provider id must not be empty")
	}
	if len(def.models) == 0 || def.models[0] == "" {
		panic("gen: provider " + def.id + " must declare at least one model")
	}
	if def.defaultModel == "" {
		def.defaultModel = def.models[0]
	}
	if _, dup := providerRegistry[def.id]; dup {
		panic("gen: provider " + def.id + " registered twice")
	}
	def.models = append([]string(nil), def.models...)
	providerRegistry[def.id] = &def
}

// SupportedProviders는 지원 프로바이더 식별자 목록입니다 (UI 노출 순서).
// init()에서 sortOrder 기준으로 정렬해 채우므로, 등록(init) 순서와 무관하게
// UI 순서가 항상 일정합니다.
var SupportedProviders []string

func init() {
	ids := make([]string, 0, len(providerRegistry))
	for id := range providerRegistry {
		ids = append(ids, id)
	}
	// UI 순서는 sortOrder로 고정 (동률이면 id 오름차순으로 결정적 정렬).
	sort.Slice(ids, func(i, j int) bool {
		lo, ro := providerRegistry[ids[i]].sortOrder, providerRegistry[ids[j]].sortOrder
		if lo != ro {
			return lo < ro
		}
		return ids[i] < ids[j]
	})
	SupportedProviders = ids
}

// ModelsFor는 프로바이더가 제공하는 선택 가능한 모델 목록을 반환합니다 (최신 모델이 맨 앞).
func ModelsFor(provider string) []string {
	if def, ok := providerRegistry[provider]; ok {
		return append([]string(nil), def.models...)
	}
	return nil
}

// Provider는 이미지 생성 백엔드 공통 인터페이스입니다.
type Provider interface {
	// GenerateImage는 프롬프트와 참조 이미지(PNG)로 이미지를 생성합니다.
	GenerateImage(ctx context.Context, prompt string, refImages [][]byte, aspectRatio string) ([]byte, error)
	// ValidateKey는 API 키 유효성을 확인합니다.
	ValidateKey(ctx context.Context) error
}

// DefaultModelFor는 프로바이더별 기본 모델을 반환합니다.
func DefaultModelFor(provider string) string {
	if def, ok := providerRegistry[provider]; ok && def.defaultModel != "" {
		return def.defaultModel
	}
	// 기본 폴백: gemini 기본 모델
	return DefaultModel
}

// ProviderLabel은 UI 표시용 이름입니다.
func ProviderLabel(provider string) string {
	if def, ok := providerRegistry[provider]; ok && def.label != "" {
		return def.label
	}
	// 기본 폴백: gemini
	return "Gemini"
}

// New는 프로바이더 구현체를 생성합니다.
//
// 단일 API 키 프로바이더 전용 팩토리입니다. cloudflare(AccountID)나
// custom(BaseURL)처럼 키 외에 추가 필드가 필요한 프로바이더는 여기에
// 등록하지 않고 전용 생성자(NewCloudflare / NewCustom)를 사용합니다.
// 그런 프로바이더는 providerDef.newFunc를 nil로 두면 New가 명확한 오류를
// 반환하므로, 앱 레이어가 전용 생성자를 직접 호출해야 합니다.
func New(provider, apiKey, model string) (Provider, error) {
	if provider == "" {
		provider = ProviderGemini
	}
	def, ok := providerRegistry[provider]
	if !ok {
		return nil, fmt.Errorf("지원하지 않는 프로바이더입니다: %s", provider)
	}
	if def.newFunc == nil {
		return nil, fmt.Errorf("%s 프로바이더는 전용 생성자가 필요합니다", provider)
	}
	if model == "" {
		model = DefaultModelFor(provider)
	}
	return def.newFunc(apiKey, model), nil
}

// aspectHint는 종횡비 파라미터를 지원하지 않는 API용 프롬프트 보조 문구입니다.
func aspectHint(aspectRatio string) string {
	switch aspectRatio {
	case "", "1:1":
		return "Render on a square 1:1 canvas."
	case "16:9":
		return "Render on a wide 16:9 landscape canvas, much wider than tall."
	case "21:9":
		return "Render on an extra-wide 21:9 panorama canvas, with plenty of horizontal room for a long row of poses."
	default:
		return fmt.Sprintf("Render on a wide %s landscape canvas.", aspectRatio)
	}
}

// decodeDataOrDownload는 data: URL이면 디코딩하고, http(s) URL이면 다운로드합니다.
func decodeDataOrDownload(httpClient *http.Client, url string) ([]byte, error) {
	if strings.HasPrefix(url, "data:") {
		idx := strings.Index(url, "base64,")
		if idx < 0 {
			return nil, errors.New("이미지 데이터 형식을 해석할 수 없습니다")
		}
		return base64.StdEncoding.DecodeString(url[idx+7:])
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("결과 이미지 다운로드 실패: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("결과 이미지 다운로드 실패 (HTTP %d)", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}
