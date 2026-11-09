package gen

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"testing"
	"time"
)

// TestCustomLive는 실제 OpenAI 호환(custom) 엔진을 호출하는 스모크 테스트입니다.
// PP_LIVE_TEST=1 과 CUSTOM_IMAGE_BASE_URL 이 설정된 경우에만 실행됩니다.
// (CUSTOM_IMAGE_API_KEY는 선택 사항이며, 키리스 엔진이면 생략 가능합니다.)
func TestCustomLive(t *testing.T) {
	if os.Getenv("PP_LIVE_TEST") != "1" {
		t.Skip("PP_LIVE_TEST=1 설정 시에만 실행")
	}
	baseURL := os.Getenv("CUSTOM_IMAGE_BASE_URL")
	if baseURL == "" {
		t.Skip("CUSTOM_IMAGE_BASE_URL 미설정")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	c := NewCustom(baseURL, os.Getenv("CUSTOM_IMAGE_API_KEY"), "")
	data, err := c.GenerateImage(ctx,
		"A single small red circle centered on a solid magenta (#FF00FF) background. Flat colors, no gradients.",
		nil, "1:1")
	if err != nil {
		t.Fatalf("custom 이미지 생성 실패: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("PNG 디코딩 실패 (len=%d): %v", len(data), err)
	}
	b := img.Bounds()
	if b.Dx() < 64 || b.Dy() < 64 {
		t.Fatalf("이미지 크기가 비정상적으로 작음: %v", b)
	}
	t.Logf("custom 생성 성공: %dx%d (%d bytes)", b.Dx(), b.Dy(), len(data))
}
