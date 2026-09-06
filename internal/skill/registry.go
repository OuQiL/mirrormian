// SkillRegistry：技能注册、优先级匹配、会话分发。
package skill

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Registry 技能注册中心。
type Registry struct {
	skills   []Skill
	store    Store
	fallback Skill // 未命中任何技能时的兜底（普通对话）
}

// NewRegistry 创建注册中心。
func NewRegistry(store Store) *Registry {
	return &Registry{store: store, skills: []Skill{}}
}

// SetFallback 设置兜底技能：意图未命中时降级为该技能（如普通对话）。
func (r *Registry) SetFallback(s Skill) {
	r.fallback = s
}

// Register 注册技能。
func (r *Registry) Register(s ...Skill) {
	r.skills = append(r.skills, s...)
}

// List 技能列表（按名称排序）。
func (r *Registry) List() []Skill {
	out := append([]Skill{}, r.skills...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// Match 按输入意图返回最高匹配分技能（低于阈值返回 nil）。
func (r *Registry) Match(ctx context.Context, input string) (Skill, int) {
	var best Skill
	bestScore := 0
	for _, s := range r.skills {
		score := s.MatchScore(ctx, input)
		if score > bestScore {
			best = s
			bestScore = score
		}
	}
	if best == nil || bestScore < MatchThreshold {
		return nil, 0
	}
	return best, bestScore
}

// StartSkill 启动指定技能（按名称）；不存在返回错误。
// 启动后把用户输入与首条回复写入会话记录，形成完整对话转写（供历史回放）。
func (r *Registry) StartSkill(ctx context.Context, name, input string) (*Session, *TurnResult, error) {
	for _, s := range r.skills {
		if s.Name() == name {
			sess, res, err := s.Start(ctx, input)
			if err != nil {
				return nil, nil, err
			}
			sess.Messages = []string{input, res.Reply}
			if err := r.store.SaveSkillSession(sess); err != nil {
				return nil, nil, fmt.Errorf("skill: 保存会话: %w", err)
			}
			return sess, res, nil
		}
	}
	return nil, nil, fmt.Errorf("技能 %q 不存在（可用：skill list）", name)
}

// StartMatched 按意图匹配启动技能；未命中时降级为兜底大模型调用（普通对话），
// 兜底也不存在时才返回可读提示（含技能列表）。
func (r *Registry) StartMatched(ctx context.Context, input string) (*Session, *TurnResult, error) {
	s, score := r.Match(ctx, input)
	if s == nil {
		if r.fallback != nil {
			sess, res, err := r.fallback.Start(ctx, input)
			if err != nil {
				return nil, nil, err
			}
			sess.Messages = []string{input, res.Reply}
			if err := r.store.SaveSkillSession(sess); err != nil {
				return nil, nil, fmt.Errorf("skill: 保存会话: %w", err)
			}
			return sess, res, nil
		}
		var b strings.Builder
		for i, sk := range r.List() {
			if i > 0 {
				b.WriteString("、")
			}
			b.WriteString(sk.Name())
		}
		return nil, nil, fmt.Errorf("无法匹配技能（得分 %d < %d）。可用技能：%s", score, MatchThreshold, b.String())
	}
	return r.StartSkill(ctx, s.Name(), input)
}

// Turn 推进会话（自动恢复持久化状态）。
func (r *Registry) Turn(ctx context.Context, sessionID, input string) (*TurnResult, error) {
	sess, err := r.store.GetSkillSession(sessionID)
	if err != nil {
		return nil, err
	}
	if sess.Finished {
		return nil, fmt.Errorf("会话 %s 已结束", sessionID)
	}
	var s Skill
	if r.fallback != nil && sess.SkillName == r.fallback.Name() {
		s = r.fallback
	} else {
		for _, sk := range r.skills {
			if sk.Name() == sess.SkillName {
				s = sk
				break
			}
		}
	}
	if s == nil {
		return nil, fmt.Errorf("技能 %q 未注册", sess.SkillName)
	}
	res, err := s.Turn(ctx, sess, input)
	if err != nil {
		return nil, err
	}
	// 追加用户输入与 AI 回复，形成完整对话转写（供历史回放）。
	sess.Messages = append(sess.Messages, input, res.Reply)
	if res.Finished {
		sess.Finished = true
	}
	if err := r.store.SaveSkillSession(sess); err != nil {
		return nil, fmt.Errorf("skill: 保存会话: %w", err)
	}
	return res, nil
}
