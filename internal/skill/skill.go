// Skill 技能系统：有状态多轮交互技能（区别于无状态 Tool）。
//
// Skill 接口 + Session 持久化 + SkillRegistry 优先级匹配分发。
package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"mirror-mian/internal/model"
)

// MatchThreshold 匹配分阈值（低于视为不匹配）。
const MatchThreshold = 30

// Session 技能会话（model.SkillSession 别名，避免 skill↔store 循环依赖）。
type Session = model.SkillSession

// TurnResult 一轮交互的结果。
type TurnResult struct {
	Reply    string // 展示文本
	Finished bool
}

// Skill 有状态多轮技能接口。
type Skill interface {
	Name() string
	Description() string
	// MatchScore 输入意图匹配分（0-100；低于 MatchThreshold 视为不匹配）。
	MatchScore(ctx context.Context, input string) int
	// Start 创建会话并返回初始提示。
	Start(ctx context.Context, input string) (*Session, *TurnResult, error)
	// Turn 推进一轮（修改会话状态后返回输出）。
	Turn(ctx context.Context, sess *Session, userInput string) (*TurnResult, error)
}

// Store 会话存储接口。
type Store interface {
	SaveSkillSession(s *Session) error
	GetSkillSession(id string) (*Session, error)
}

// newSession 创建会话骨架。
func newSession(skillName string, state any) (*Session, error) {
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("skill: 序列化状态: %w", err)
	}
	now := time.Now()
	return &Session{
		ID: uuid.NewString(), SkillName: skillName,
		State: raw, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// saveState 更新会话状态与时间戳。store 为 nil 时仅更新内存
// （持久化由 Registry.Turn 统一执行）；非 nil 时立即落库。
func saveState(s *Session, state any, store Store) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("skill: 序列化状态: %w", err)
	}
	s.State = raw
	s.UpdatedAt = time.Now()
	if store != nil {
		return store.SaveSkillSession(s)
	}
	return nil
}
