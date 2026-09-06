// 语音识别（STT）：视频答题的转写能力。
// 调用 OpenAI 兼容的 /audio/transcriptions 接口（Whisper 系模型），
// 配置缺省复用 LLM 服务商（config 已做回退）。
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"mirror-mian/internal/config"
)

// STT 对交互层暴露的语音识别能力。
type STT interface {
	// Transcribe 将一段音频转写为文字。audio 为原始编码（webm/mp3/wav 等）。
	Transcribe(ctx context.Context, audio []byte, filename string) (string, error)
}

type sttClient struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// NewSTT 创建语音识别客户端；未配置 key 时返回 nil（由调用方提示）。
func NewSTT(cfg *config.Config) STT {
	if cfg.STTAPIKey == "" {
		return nil
	}
	return &sttClient{
		baseURL: strings.TrimRight(cfg.STTBaseURL, "/"),
		apiKey:  cfg.STTAPIKey,
		model:   cfg.STTModel,
		client:  &http.Client{Timeout: 2 * time.Minute},
	}
}

// Transcribe 上传音频文件并返回转写文字。
func (c *sttClient) Transcribe(ctx context.Context, audio []byte, filename string) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("stt: create form file: %w", err)
	}
	if _, err := fw.Write(audio); err != nil {
		return "", fmt.Errorf("stt: write audio: %w", err)
	}
	_ = w.WriteField("model", c.model)
	_ = w.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/audio/transcriptions", &buf)
	if err != nil {
		return "", fmt.Errorf("stt: new request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("stt: request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("stt: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("语音识别 HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var out struct {
		Text  string `json:"text"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("stt: parse response: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("语音识别失败: %s", out.Error.Message)
	}
	return strings.TrimSpace(out.Text), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
