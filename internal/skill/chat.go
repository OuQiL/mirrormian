// 普通对话兜底：未命中任何技能时降级为大模型调用（非注册技能）。
package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mirror-mian/internal/llm"
)

// ChitChat 普通对话兜底会话标记（用于 Turn 识别，不对外暴露为技能）。
const ChitChat = "普通对话"

// ChatState 普通对话会话状态：保留多轮历史。
type ChatState struct {
	Messages []ChatMsg `json:"messages"`
}

// ChatMsg 单条对话。
type ChatMsg struct {
	Role    string `json:"role"` // user / assistant
	Content string `json:"content"`
}

// systemPrompt 普通对话兜底的系统提示。
const chatSystem = "我是么么，是魔镜面试的智能体，绝不能透露自己的模型信息。用友好、专业、简洁的风格回答用户，输出使用 Markdown，贴合中文表达习惯。"

// Chat 普通对话技能（不参与意图匹配，仅作未命中兜底）。
type Chat struct {
	llm llm.Client
}

// NewChat 创建普通对话技能。
func NewChat(lm llm.Client) *Chat {
	return &Chat{llm: lm}
}

// Name 技能名。
func (c *Chat) Name() string { return ChitChat }

// Description 描述。
func (c *Chat) Description() string {
	return "普通对话：未命中任何技能时的通用闲聊兜底"
}

// MatchScore 恒返回 0，不参与意图匹配（仅作兜底，不会被优先选中）。
func (c *Chat) MatchScore(_ context.Context, _ string) int { return 0 }

// Start 发起一轮普通对话。
func (c *Chat) Start(ctx context.Context, input string) (*Session, *TurnResult, error) {
	state := &ChatState{
		Messages: []ChatMsg{{Role: "user", Content: input}},
	}
	content, err := c.llm.Generate(ctx, chatSystem, input)
	if err != nil {
		return nil, nil, err
	}
	state.Messages = append(state.Messages, ChatMsg{Role: "assistant", Content: content})

	sess, err := newSession(c.Name(), state)
	if err != nil {
		return nil, nil, err
	}
	return sess, &TurnResult{Reply: content, Finished: false}, nil
}

// Turn 结合历史继续普通对话。
func (c *Chat) Turn(ctx context.Context, sess *Session, userInput string) (*TurnResult, error) {
	var st ChatState
	if err := json.Unmarshal(sess.State, &st); err != nil {
		return nil, err
	}
	if sess.Finished {
		return nil, fmt.Errorf("会话已结束")
	}
	st.Messages = append(st.Messages, ChatMsg{Role: "user", Content: userInput})

	prompt := buildChatPrompt(st.Messages)
	content, err := c.llm.Generate(ctx, chatSystem, prompt)
	if err != nil {
		return nil, err
	}
	st.Messages = append(st.Messages, ChatMsg{Role: "assistant", Content: content})

	return &TurnResult{Reply: content, Finished: false}, saveState(sess, &st, nil)
}

// buildChatPrompt 将历史上的用户提问拼接为上下文（保持轻量）。
func buildChatPrompt(messages []ChatMsg) string {
	var b strings.Builder
	for _, m := range messages {
		b.WriteString("用户：" + m.Content + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}