// LLM 模型接入：封装 Eino ChatModel（OpenAI 兼容端点）。
// 提供出题/评估/追问/复盘共用的非流式生成与结构化 JSON 输出接口。
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"mirror-mian/internal/config"
)

// Client 对编排层暴露的 LLM 能力。
type Client interface {
	// Generate 以 system+user 消息生成文本回答。
	Generate(ctx context.Context, system, user string) (string, error)
	// GenerateJSON 生成并解析为指定结构（要求模型输出 JSON）。
	GenerateJSON(ctx context.Context, system, user string, out any) error
	// ModelName 返回当前模型名（健康自检用）。
	ModelName() string
}

type einoClient struct {
	cm     model.ChatModel
	model  string
}

// New 创建 Eino 封装的 LLM 客户端。
func New(cfg *config.Config) (Client, error) {
	cm, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		BaseURL: cfg.LLMBaseURL,
		APIKey:  cfg.LLMAPIKey,
		Model:   cfg.LLMModel,
	})
	if err != nil {
		return nil, fmt.Errorf("llm: create chat model: %w", err)
	}
	return &einoClient{cm: cm, model: cfg.LLMModel}, nil
}

func (c *einoClient) Generate(ctx context.Context, system, user string) (string, error) {
	resp, err := c.cm.Generate(ctx, messages(system, user))
	if err != nil {
		return "", fmt.Errorf("llm: generate: %w", err)
	}
	return resp.Content, nil
}

func (c *einoClient) GenerateJSON(ctx context.Context, system, user string, out any) error {
	text, err := c.Generate(ctx, system, user)
	if err != nil {
		return err
	}
	return parseJSON(text, out)
}

// parseJSON 去除代码围栏后解析 JSON。
func parseJSON(text string, out any) error {
	text = stripCodeFence(text)
	if err := json.Unmarshal([]byte(text), out); err != nil {
		return fmt.Errorf("llm: parse json (got %d chars): %w", len(text), err)
	}
	return nil
}

func (c *einoClient) ModelName() string { return c.model }

func messages(system, user string) []*schema.Message {
	msgs := make([]*schema.Message, 0, 2)
	if system != "" {
		msgs = append(msgs, schema.SystemMessage(system))
	}
	msgs = append(msgs, schema.UserMessage(user))
	return msgs
}

// stripCodeFence 去掉模型常见的 ```json ... ``` 包裹。
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if i := strings.Index(s, "\n"); i >= 0 {
			s = s[i+1:]
		}
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
