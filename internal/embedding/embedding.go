// Embedding 客户端：基础能力层。
//
// OpenAI 兼容 /embeddings 端点；base_url/api_key/model 可独立配置，
// 未配置时回退复用 LLM 配置；仍不可用时 Available()=false（调用方降级）。
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"mirror-mian/internal/config"
)

// Cosine 余弦相似度（0-1）；维度不一致或零向量返回 0。
func Cosine(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// Embedder 文本向量化接口（基础能力层对编排层/画像暴露）。
type Embedder interface {
	// Embed 批量向量化文本，返回与输入等长的向量列表。
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Available 是否已配置且可调用。
	Available() bool
}

// Client OpenAI 兼容 embedding 客户端。
type Client struct {
	baseURL   string
	apiKey    string
	model     string
	http      *http.Client
	available bool
}

// New 创建 embedding 客户端。embedding 专用配置缺失时回退复用 LLM 配置。
func New(cfg *config.Config) *Client {
	baseURL, apiKey, model := cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel
	if cfg.EmbeddingBaseURL != "" {
		baseURL = cfg.EmbeddingBaseURL
	}
	if cfg.EmbeddingAPIKey != "" {
		apiKey = cfg.EmbeddingAPIKey
	}
	if cfg.EmbeddingModel != "" {
		model = cfg.EmbeddingModel
	}
	return &Client{
		baseURL:   baseURL,
		apiKey:    apiKey,
		model:     model,
		http:      &http.Client{Timeout: 60 * time.Second},
		available: baseURL != "" && apiKey != "",
	}
}

// Available 配置完整即可用（连通性在首次调用时验证）。
func (c *Client) Available() bool { return c.available }

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Embed 调用 /embeddings 批量向量化。
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if !c.available {
		return nil, fmt.Errorf("embedding: 未配置（设置 LLM_EMBEDDING_BASE_URL/API_KEY/MODEL 或 LLM_API_KEY）")
	}
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	body, err := json.Marshal(embedRequest{Model: c.model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("embedding: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embedding: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding: request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("embedding: read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding: HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var out embedResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("embedding: parse: %w", err)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("embedding: %s", out.Error.Message)
	}
	if len(out.Data) != len(texts) {
		return nil, fmt.Errorf("embedding: 返回 %d 个向量，期望 %d", len(out.Data), len(texts))
	}
	vectors := make([][]float32, 0, len(out.Data))
	for _, d := range out.Data {
		vectors = append(vectors, d.Embedding)
	}
	return vectors, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
