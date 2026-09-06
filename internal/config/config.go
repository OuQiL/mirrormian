// 配置加载与校验：从 .env（godotenv）与显式字段加载。
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Config 运行时配置。
type Config struct {
	LLMBaseURL string `json:"llm_base_url"` // OpenAI 兼容端点
	LLMAPIKey  string `json:"llm_api_key"`
	LLMModel   string `json:"llm_model"`
	// Embedding 独立配置（可选，缺省回退复用 LLM 配置）
	EmbeddingBaseURL string `json:"embedding_base_url,omitempty"`
	EmbeddingAPIKey  string `json:"embedding_api_key,omitempty"`
	EmbeddingModel   string `json:"embedding_model,omitempty"`
	// STT 语音识别（视频答题转写；可选，缺省回退复用 LLM 配置）
	STTBaseURL string `json:"stt_base_url,omitempty"`
	STTAPIKey  string `json:"stt_api_key,omitempty"`
	STTModel   string `json:"stt_model,omitempty"`
	DBPath     string `json:"db_path"`     // SQLite 文件路径
	KBPath     string `json:"kb_path"`     // 领域内容目录（kb/<topic>/）
	MilvusAddr string `json:"milvus_addr"` // Milvus 地址（默认 localhost:19530，空=禁用）
}

// Load 加载配置：先读 .env（可选），再读环境变量覆盖。
// 返回 error 时给出可读的缺失信息。
func Load(envPath string) (*Config, error) {
	if envPath != "" {
		if err := godotenv.Load(envPath); err != nil {
			return nil, fmt.Errorf("load env file %s: %w", envPath, err)
		}
	} else {
		// .env 存在则加载，不存在则忽略（允许纯环境变量方式）
		if _, err := os.Stat(".env"); err == nil {
			_ = godotenv.Load()
		}
	}

	llmBase := getenv("LLM_BASE_URL", "https://api.openai.com/v1")
	llmKey := os.Getenv("LLM_API_KEY")

	cfg := &Config{
		LLMBaseURL:       llmBase,
		LLMAPIKey:        llmKey,
		LLMModel:         getenv("LLM_MODEL", "gpt-4o-mini"),
		EmbeddingBaseURL: os.Getenv("LLM_EMBEDDING_BASE_URL"),
		EmbeddingAPIKey:  os.Getenv("LLM_EMBEDDING_API_KEY"),
		EmbeddingModel:   os.Getenv("LLM_EMBEDDING_MODEL"),
		STTBaseURL:       getenv("STT_BASE_URL", llmBase),
		STTAPIKey:        getenv("STT_API_KEY", llmKey),
		STTModel:         getenv("STT_MODEL", "whisper-1"),
		DBPath:           getenv("MIRROR_DB_PATH", "data/mirror-mian.db"),
		KBPath:           getenv("MIRROR_KB_PATH", "kb"),
		MilvusAddr:       getenv("MILVUS_ADDR", "localhost:19530"),
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate 校验必需配置，缺失时返回可读错误。
func (c *Config) Validate() error {
	if c.LLMBaseURL == "" {
		return fmt.Errorf("config: LLM_BASE_URL is required")
	}
	if c.LLMModel == "" {
		return fmt.Errorf("config: LLM_MODEL is required")
	}
	if c.DBPath == "" {
		return fmt.Errorf("config: MIRROR_DB_PATH is required")
	}
	return nil
}

// EnsureDataDir 确保 SQLite 父目录存在。
func (c *Config) EnsureDataDir() error {
	dir := filepath.Dir(c.DBPath)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
