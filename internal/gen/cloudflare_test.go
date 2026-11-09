package gen

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// cfImageResponse는 fakePNG를 Cloudflare Workers AI 응답 형태로 담아 반환합니다.
func cfImageResponse() string {
	b64 := base64.StdEncoding.EncodeToString(fakePNG)
	return `{"result":{"image":"` + b64 + `"},"success":true}`
}

func TestCloudflareGenerateSuccess(t *testing.T) {
	var got cfRequest
	var gotAuth, gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(cfImageResponse()))
	}))
	defer srv.Close()

	c := NewCloudflare("acct-123", "test-token", "")
	c.endpoint = srv.URL

	img, err := c.GenerateImage(context.Background(), "hello", nil, "1:1")
	if err != nil {
		t.Fatalf("GenerateImage: %v", err)
	}
	if string(img) != string(fakePNG) {
		t.Fatalf("image mismatch: %q", img)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("auth = %q, want Bearer test-token", gotAuth)
	}
	if gotContentType != "application/json" {
		t.Fatalf("content-type = %q", gotContentType)
	}
	// 종횡비 힌트가 프롬프트에 붙어야 합니다.
	if !strings.Contains(got.Prompt, "hello") || !strings.Contains(got.Prompt, "square 1:1 canvas") {
		t.Fatalf("prompt = %q, want prompt + aspect hint", got.Prompt)
	}
}

func TestCloudflareRunURL(t *testing.T) {
	var gotURL string
	c := NewCloudflare("acct-123", "tok", "")
	c.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(cfImageResponse())),
		}, nil
	})}

	if _, err := c.GenerateImage(context.Background(), "p", nil, ""); err != nil {
		t.Fatalf("GenerateImage: %v", err)
	}
	want := "https://api.cloudflare.com/client/v4/accounts/acct-123/ai/run/@cf/black-forest-labs/flux-1-schnell"
	if gotURL != want {
		t.Fatalf("url = %q, want %q", gotURL, want)
	}
}

func TestCloudflareBinaryResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(fakePNG)
	}))
	defer srv.Close()

	c := NewCloudflare("acct-123", "tok", "")
	c.endpoint = srv.URL
	img, err := c.GenerateImage(context.Background(), "p", nil, "")
	if err != nil {
		t.Fatalf("GenerateImage: %v", err)
	}
	if string(img) != string(fakePNG) {
		t.Fatalf("image mismatch: %q", img)
	}
}

func TestCloudflareDataFieldFallback(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString(fakePNG)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"data":"` + b64 + `"},"success":true}`))
	}))
	defer srv.Close()

	c := NewCloudflare("acct-123", "tok", "")
	c.endpoint = srv.URL
	img, err := c.GenerateImage(context.Background(), "p", nil, "")
	if err != nil {
		t.Fatalf("GenerateImage: %v", err)
	}
	if string(img) != string(fakePNG) {
		t.Fatalf("image mismatch: %q", img)
	}
}

func TestCloudflareMissingCreds(t *testing.T) {
	c := NewCloudflare("", "tok", "")
	if _, err := c.GenerateImage(context.Background(), "p", nil, ""); err == nil {
		t.Fatal("missing AccountID should error on generate")
	}
	if err := c.ValidateKey(context.Background()); err == nil {
		t.Fatal("missing AccountID should error on validate")
	}

	c2 := NewCloudflare("acct", "", "")
	if _, err := c2.GenerateImage(context.Background(), "p", nil, ""); err == nil {
		t.Fatal("missing token should error on generate")
	}
	if err := c2.ValidateKey(context.Background()); err == nil {
		t.Fatal("missing token should error on validate")
	}
}

func TestCloudflareNon200Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":10000,"message":"bad request"}]}`))
	}))
	defer srv.Close()

	c := NewCloudflare("acct", "tok", "")
	c.endpoint = srv.URL
	_, err := c.GenerateImage(context.Background(), "p", nil, "")
	if err == nil {
		t.Fatal("non-200 should error")
	}
	if !strings.Contains(err.Error(), "bad request") {
		t.Fatalf("error = %q, want Cloudflare error message", err)
	}
}

func TestCloudflareRetry429(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(cfImageResponse()))
	}))
	defer srv.Close()

	c := NewCloudflare("acct", "tok", "")
	c.endpoint = srv.URL
	img, err := c.GenerateImage(context.Background(), "p", nil, "")
	if err != nil {
		t.Fatalf("GenerateImage: %v", err)
	}
	if string(img) != string(fakePNG) {
		t.Fatalf("image mismatch: %q", img)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3 (two retries after 429)", calls)
	}
}

func TestCloudflareValidateKey200(t *testing.T) {
	var gotURL, gotAuth string
	c := NewCloudflare("acct-123", "tok", "")
	c.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		gotAuth = r.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"success":true}`)),
		}, nil
	})}

	if err := c.ValidateKey(context.Background()); err != nil {
		t.Fatalf("ValidateKey: %v", err)
	}
	if gotURL != "https://api.cloudflare.com/client/v4/accounts/acct-123" {
		t.Fatalf("url = %q", gotURL)
	}
	if gotAuth != "Bearer tok" {
		t.Fatalf("auth = %q, want Bearer tok", gotAuth)
	}
}

func TestCloudflareValidateKey401(t *testing.T) {
	c := NewCloudflare("acct-123", "bad-token", "")
	c.HTTP = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"success":false}`)),
		}, nil
	})}

	if err := c.ValidateKey(context.Background()); err == nil {
		t.Fatal("401 should error")
	}
}
