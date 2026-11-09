package gen

import (
	"bytes"
	"context"
	"image/jpeg"
	"image/png"
	"os"
	"testing"
	"time"
)

// TestCloudflareLive는 실제 Cloudflare Workers AI를 호출하는 스모크 테스트입니다.
// PP_LIVE_TEST=1 과 CLOUDFLARE_ACCOUNT_ID + CLOUDFLARE_API_TOKEN 이 설정된
// 경우에만 실행됩니다 (크레딧 소모 주의). 키는 절대 로그에 남기지 않습니다.
func TestCloudflareLive(t *testing.T) {
	if os.Getenv("PP_LIVE_TEST") != "1" {
		t.Skip("PP_LIVE_TEST=1 설정 시에만 실행")
	}
	accountID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")
	if accountID == "" || apiToken == "" {
		t.Skip("CLOUDFLARE_ACCOUNT_ID/CLOUDFLARE_API_TOKEN 미설정")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	c := NewCloudflare(accountID, apiToken, "")
	data, err := c.GenerateImage(ctx,
		"A single small red circle centered on a solid magenta (#FF00FF) background. Flat colors, no gradients.",
		nil, "1:1")
	if err != nil {
		t.Fatalf("cloudflare 이미지 생성 실패: %v", err)
	}

	// flux-1-schnell은 문서상 JPEG로 반환할 수 있어 PNG 우선, 실패 시 JPEG로 폴백합니다.
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		img, err = jpeg.Decode(bytes.NewReader(data))
	}
	if err != nil {
		t.Fatalf("이미지 디코딩 실패 (len=%d): %v", len(data), err)
	}
	b := img.Bounds()
	if b.Dx() < 64 || b.Dy() < 64 {
		t.Fatalf("이미지 크기가 비정상적으로 작음: %v", b)
	}
	t.Logf("cloudflare 생성 성공: %dx%d (%d bytes)", b.Dx(), b.Dy(), len(data))
}
