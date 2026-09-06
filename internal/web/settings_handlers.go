// 设置接口：LLM/Embedding 配置的读取（脱敏）、保存（.env）与连接测试。
package web

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mirror-mian/internal/config"
)

// settings 配置键（.env 中的 key）。
const (
	envBase = ".env"
)

// envKeys LLM/Embedding/STT 配置键对。
var envKeys = map[string][3]string{
	"llm":       {"LLM_BASE_URL", "LLM_API_KEY", "LLM_MODEL"},
	"embedding": {"LLM_EMBEDDING_BASE_URL", "LLM_EMBEDDING_API_KEY", "LLM_EMBEDDING_MODEL"},
	"stt":       {"STT_BASE_URL", "STT_API_KEY", "STT_MODEL"},
}

// providerSettings 一组服务配置。
type providerSettings struct {
	BaseURL      string `json:"base_url"`
	APIKey       string `json:"api_key,omitempty"` // 保存/测试用；读取时脱敏
	APIKeyMasked string `json:"api_key_masked,omitempty"`
	KeyConfigured bool   `json:"key_configured"`
	Model        string `json:"model"`
}

// envFilePath 定位 .env（项目根，与 config.Load 默认一致）。
func envFilePath() string {
	return filepath.Join(".env")
}

// handleSettingsGet 读取当前生效配置（完整 key——本地单用户设置页，用户要求可见可复制可修改）。
func (s *Server) handleSettingsGet(w http.ResponseWriter, _ *http.Request) {
	env, _ := config.ReadEnvFile(envFilePath())
	// 环境变量优先于 .env（与 config.Load 行为一致）
	env = overlayEnv(env)

	out := map[string]providerSettings{}
	for group, keys := range envKeys {
		base, key, model := env[keys[0]], env[keys[1]], env[keys[2]]
		// STT 缺省回退复用 LLM 服务商（与 config.Load 行为一致）
		if group == "stt" {
			if base == "" {
				base = env[envKeys["llm"][0]]
			}
			if key == "" {
				key = env[envKeys["llm"][1]]
			}
			if model == "" {
				model = "whisper-1"
			}
		}
		p := providerSettings{
			BaseURL:       base,
			APIKey:        key,
			KeyConfigured: key != "",
			Model:         model,
		}
		if p.KeyConfigured {
			p.APIKeyMasked = maskKey(key)
		}
		out[group] = p
	}
	writeJSON(w, out)
}

// overlayEnv 用环境变量覆盖 .env 值。
func overlayEnv(env map[string]string) map[string]string {
	out := maps.Clone(env)
	for _, keys := range envKeys {
		for _, k := range keys {
			if v := os.Getenv(k); v != "" {
				out[k] = v
			}
		}
	}
	return out
}

// maskKey 脱敏：sk-abc****h1234（保留前缀 6 字符与末尾 5 字符）。
func maskKey(key string) string {
	if len(key) <= 10 {
		return "****"
	}
	return key[:min(6, len(key)-5)] + "****" + key[len(key)-5:]
}

// handleSettingsSave 保存配置到 .env（空 key = 保持现有）。
func (s *Server) handleSettingsSave(w http.ResponseWriter, r *http.Request) {
	var req map[string]providerSettings
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	env, err := config.ReadEnvFile(envFilePath())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	for group, keys := range envKeys {
		p, ok := req[group]
		if !ok {
			continue
		}
		if p.BaseURL == "" || p.Model == "" {
			writeErr(w, http.StatusBadRequest, fmt.Sprintf("%s 的 base_url 与 model 不能为空", group))
			return
		}
		env[keys[0]] = p.BaseURL
		env[keys[2]] = p.Model
		if p.APIKey != "" {
			env[keys[1]] = p.APIKey
		}
		// 显式清除：api_key == " "（页面清除按钮约定）
		if p.APIKey == " " {
			delete(env, keys[1])
		}
	}
	if err := config.WriteEnvFile(envFilePath(), env); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "need_restart": true, "message": "配置已保存，重启服务后生效"})
}

// handleSettingsTest 用提交配置（不落盘）测试 LLM/Embedding 连通性。
func (s *Server) handleSettingsTest(w http.ResponseWriter, r *http.Request) {
	var req map[string]providerSettings
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	client := &http.Client{Timeout: 15 * time.Second}

	llmRes := map[string]any{"ok": false}
	if p, ok := req["llm"]; ok && p.BaseURL != "" && p.Model != "" {
		llmRes = testLLM(client, p)
	}
	embRes := map[string]any{"ok": false}
	if p, ok := req["embedding"]; ok && p.BaseURL != "" && p.Model != "" {
		embRes = testEmbedding(client, p)
	}
	sttRes := map[string]any{"ok": false}
	if p, ok := req["stt"]; ok && p.BaseURL != "" && p.Model != "" {
		sttRes = testSTT(client, p)
	}
	writeJSON(w, map[string]any{"llm": llmRes, "embedding": embRes, "stt": sttRes})
}

// testLLM 最小 chat 请求验证连通性。
func testLLM(client *http.Client, p providerSettings) map[string]any {
	body, _ := json.Marshal(map[string]any{
		"model": p.Model,
		"messages": []map[string]string{{"role": "user", "content": "ping"}},
		"max_tokens": 1,
	})
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		p.BaseURL+"/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode != http.StatusOK {
		return map[string]any{"ok": false, "error": fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))}
	}
	return map[string]any{"ok": true, "model": p.Model}
}

// testEmbedding 最小 embeddings 请求验证连通性（返回维度）。
func testEmbedding(client *http.Client, p providerSettings) map[string]any {
	body, _ := json.Marshal(map[string]any{"model": p.Model, "input": "ping"})
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		p.BaseURL+"/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return map[string]any{"ok": false, "error": fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))}
	}
	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Data) == 0 {
		return map[string]any{"ok": false, "error": "返回格式无法解析"}
	}
	return map[string]any{"ok": true, "model": p.Model, "dim": len(out.Data[0].Embedding)}
}

// testSTT 上传一段 0.1s 静音 WAV 验证 /audio/transcriptions 连通性。
func testSTT(client *http.Client, p providerSettings) map[string]any {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", "ping.wav")
	_, _ = fw.Write(tinyWAV())
	_ = w.WriteField("model", p.Model)
	_ = w.Close()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		strings.TrimRight(p.BaseURL, "/")+"/audio/transcriptions", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode != http.StatusOK {
		return map[string]any{"ok": false, "error": fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))}
	}
	return map[string]any{"ok": true, "model": p.Model}
}

// tinyWAV 生成 0.1s 静音 PCM WAV（8kHz 单声道 16bit），用于 STT 连通性测试。
func tinyWAV() []byte {
	const (
		sampleRate = 8000
		dataLen    = sampleRate / 10 * 2 // 0.1s * 2 字节
	)
	buf := make([]byte, 44+dataLen)
	copy(buf[0:], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:], uint32(36+dataLen))
	copy(buf[8:], "WAVE")
	copy(buf[12:], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:], 16)
	binary.LittleEndian.PutUint16(buf[20:], 1) // PCM
	binary.LittleEndian.PutUint16(buf[22:], 1) // mono
	binary.LittleEndian.PutUint32(buf[24:], sampleRate)
	binary.LittleEndian.PutUint32(buf[28:], sampleRate*2)
	binary.LittleEndian.PutUint16(buf[32:], 2)
	binary.LittleEndian.PutUint16(buf[34:], 16)
	copy(buf[36:], "data")
	binary.LittleEndian.PutUint32(buf[40:], uint32(dataLen))
	return buf
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
