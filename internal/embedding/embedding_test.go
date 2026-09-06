package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mirror-mian/internal/config"
)

func TestAvailableFallback(t *testing.T) {
	// 未配置 embedding，回退复用 LLM 配置
	c := New(&config.Config{LLMBaseURL: "http://x", LLMAPIKey: "k", LLMModel: "m"})
	if !c.Available() {
		t.Fatal("should be available via LLM fallback")
	}
	// 完全未配置
	c2 := New(&config.Config{})
	if c2.Available() {
		t.Fatal("should not be available")
	}
	// 独立 embedding 配置优先
	c3 := New(&config.Config{LLMBaseURL: "http://x", EmbeddingBaseURL: "http://e", EmbeddingAPIKey: "ek", EmbeddingModel: "em"})
	if c3.baseURL != "http://e" || c3.model != "em" {
		t.Fatalf("dedicated config not used: %+v", c3)
	}
}

func TestEmbed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer ek" {
			t.Fatalf("auth = %q", r.Header.Get("Authorization"))
		}
		var req embedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "em" || len(req.Input) != 2 {
			t.Fatalf("req = %+v", req)
		}
		_ = json.NewEncoder(w).Encode(embedResponse{Data: []struct {
			Embedding []float32 `json:"embedding"`
		}{{Embedding: []float32{0.1, 0.2}}, {Embedding: []float32{0.3, 0.4}}}})
	}))
	defer srv.Close()

	c := New(&config.Config{
		LLMBaseURL: "http://unused", LLMAPIKey: "k", LLMModel: "m",
		EmbeddingBaseURL: srv.URL, EmbeddingAPIKey: "ek", EmbeddingModel: "em",
	})
	vecs, err := c.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 2 || vecs[0][0] != 0.1 || vecs[1][1] != 0.4 {
		t.Fatalf("vecs = %+v", vecs)
	}
}

func TestEmbedErrors(t *testing.T) {
	// 未配置 → 明确错误
	c := New(&config.Config{})
	if _, err := c.Embed(context.Background(), []string{"a"}); err == nil {
		t.Fatal("expected error when not configured")
	}
	// 服务端错误
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid key"}}`))
	}))
	defer srv.Close()
	c2 := New(&config.Config{EmbeddingBaseURL: srv.URL, EmbeddingAPIKey: "bad"})
	if _, err := c2.Embed(context.Background(), []string{"a"}); err == nil {
		t.Fatal("expected error on HTTP 401")
	}
}
