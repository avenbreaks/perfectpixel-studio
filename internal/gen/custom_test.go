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

// customB64Response는 fakePNG를 b64_json 형태로 담은 OpenAI 호환 응답을 반환합니다.
func customB64Response() string {
	b64 := base64.StdEncoding.EncodeToString(fakePNG)
	return `{"data":[{"b64_json":"` + b64 + `"}]}`
}

func TestCustomGenerationsB64(t *testing.T) {
	var got customImageRequest
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(customB64Response()))
	}))
	defer srv.Close()

	c := NewCustom(srv.URL, "test-key", "")
	img, err := c.GenerateImage(context.Background(), "hello", nil, "1:1")
	if err != nil {
		t.Fatalf("GenerateImage: %v", err)
	}
	if string(img) != string(fakePNG) {
		t.Fatalf("image mismatch: %q", img)
	}
	if gotPath != "/v1/images/generations" {
		t.Fatalf("path = %q, want /v1/images/generations", gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if got.ResponseFormat != "b64_json" {
		t.Fatalf("response_format = %q, want b64_json", got.ResponseFormat)
	}
	if got.Size != openAISizeFor("1:1") {
		t.Fatalf("size = %q, want %q", got.Size, openAISizeFor("1:1"))
	}
	if got.N != 1 {
		t.Fatalf("n = %d, want 1", got.N)
	}
	if got.Model == "" {
		t.Fatal("model must be set")
	}
}

func TestCustomGenerationsURL(t *testing.T) {
	var resultURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/images/generations", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"url": resultURL}},
		})
	})
	mux.HandleFunc("/result.png", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(fakePNG)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resultURL = srv.URL + "/result.png"

	c := NewCustom(srv.URL, "k", "")
	img, err := c.GenerateImage(context.Background(), "p", nil, "1:1")
	if err != nil {
		t.Fatalf("GenerateImage: %v", err)
	}
	if string(img) != string(fakePNG) {
		t.Fatalf("image mismatch: %q", img)
	}
}

func TestCustomEditsMultipart(t *testing.T) {
	var gotBody, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte(customB64Response()))
	}))
	defer srv.Close()

	c := NewCustom(srv.URL, "k", "")
	img, err := c.GenerateImage(context.Background(), "p", [][]byte{fakePNG, fakePNG}, "1:1")
	if err != nil {
		t.Fatalf("GenerateImage: %v", err)
	}
	if string(img) != string(fakePNG) {
		t.Fatalf("image mismatch: %q", img)
	}
	if gotPath != "/v1/images/edits" {
		t.Fatalf("path = %q, want /v1/images/edits", gotPath)
	}
	if !strings.Contains(gotBody, `name="image[]"`) {
		t.Fatalf("multipart body missing image[]: %s", gotBody)
	}
	if n := strings.Count(gotBody, `name="image[]"`); n != 2 {
		t.Fatalf("image[] count = %d, want 2", n)
	}
}

func TestCustomMissingBaseURL(t *testing.T) {
	c := NewCustom("", "k", "")
	_, err := c.GenerateImage(context.Background(), "p", nil, "1:1")
	if err == nil {
		t.Fatal("missing base URL should error")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("error = %q", err)
	}
}

func TestCustomKeylessOmitsAuth(t *testing.T) {
	var gotAuth string
	c := NewCustom("https://example.test", "", "")
	c.generationEndpoint = "https://example.test/v1/images/generations"
	c.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotAuth = r.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(customB64Response())),
		}, nil
	})}

	img, err := c.GenerateImage(context.Background(), "p", nil, "1:1")
	if err != nil {
		t.Fatalf("GenerateImage: %v", err)
	}
	if string(img) != string(fakePNG) {
		t.Fatalf("image mismatch: %q", img)
	}
	if gotAuth != "" {
		t.Fatalf("keyless request must omit Authorization, got %q", gotAuth)
	}
}

func TestCustomNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer srv.Close()

	c := NewCustom(srv.URL, "k", "")
	if _, err := c.GenerateImage(context.Background(), "p", nil, "1:1"); err == nil {
		t.Fatal("non-200 should error")
	}
}

func TestCustomRetry429(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(customB64Response()))
	}))
	defer srv.Close()

	c := NewCustom(srv.URL, "k", "")
	img, err := c.GenerateImage(context.Background(), "p", nil, "1:1")
	if err != nil {
		t.Fatalf("GenerateImage: %v", err)
	}
	if string(img) != string(fakePNG) {
		t.Fatalf("image mismatch: %q", img)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 (one retry after 429)", calls)
	}
}

func TestCustomValidateKeyKeyless(t *testing.T) {
	c := NewCustom("https://example.test", "", "")
	if err := c.ValidateKey(context.Background()); err != nil {
		t.Fatalf("keyless should validate: %v", err)
	}
}

func TestCustomValidateKey401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewCustom(srv.URL, "some-api-key", "")
	if err := c.ValidateKey(context.Background()); err == nil {
		t.Fatal("401 should error")
	}
}

func TestCustomValidateKey404Fallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewCustom(srv.URL, "a-long-enough-key", "")
	if err := c.ValidateKey(context.Background()); err != nil {
		t.Fatalf("404 should fall back to leniency, got: %v", err)
	}
}
