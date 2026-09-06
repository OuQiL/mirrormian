// Eino Graph 三阶段编排：准备 → 面试 → 复盘。
//
// 阶段内循环（追问）由状态机驱动（面试交互需要等待用户回答），
// Graph 负责三阶段的 DAG 顺序；状态（会话/方向/追问轮数）外置持久化到 store，
// Graph 节点保持无状态。
package orchestration

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"

	"mirror-mian/internal/agents"
	"mirror-mian/internal/domain"
	"mirror-mian/internal/model"
	"mirror-mian/internal/rag"
	"mirror-mian/internal/store"
)

// graphInput 编排图输入：模式 + 参数。
type graphInput struct {
	mode   string
	topic  string
	resume string
	jd     string
	ask    AskFunc
}

// graphNodes 三阶段节点实现的上下文（Agent 与持久化）。
type graphNodes struct {
	planner     *agents.DirectionPlanner
	questioner  *agents.Questioner
	interviewer *agents.Interviewer
	reviewer    *agents.Reviewer
	store       store.Store
	rag         *rag.Service
	domain      *domain.Service
}

// knowledgeContext 检索知识库上下文 + 高频题库，供出题注入；无结果或降级时返回空串。
func (ns *graphNodes) knowledgeContext(ctx context.Context, topic, kp string) string {
	var parts []string
	if kb, err := ns.rag.Context(ctx, topic, kp, rag.DefaultBudget()); err == nil && kb != "" {
		parts = append(parts, kb)
	}
	if ns.domain != nil {
		if hf := ns.domain.HighFreqContext(topic); hf != "" {
			parts = append(parts, "[高频考点]\n"+hf)
		}
	}
	return strings.Join(parts, "\n\n")
}

// runWithGraph 以 Eino Graph 执行三阶段流程，返回最终会话。
func (s *Service) runWithGraph(ctx context.Context, in graphInput) (*model.TrainingSession, error) {
	ns := s.nodes

	g := compose.NewGraph[graphInput, *model.TrainingSession]()

	// 准备阶段：确定出题方向
	_ = g.AddLambdaNode("prepare", compose.InvokableLambda(func(ctx context.Context, in graphInput) (*model.Direction, error) {
		switch in.mode {
		case model.ModeSpecial:
			d, err := ns.planner.PlanSpecial(ctx, in.topic)
			return &d, err
		case model.ModeFull:
			d, err := ns.planner.PlanFull(ctx, in.resume, in.jd)
			return &d, err
		default:
			return nil, fmt.Errorf("unknown mode %q", in.mode)
		}
	}))

	// 面试阶段：状态机驱动逐题问答（交互等待用户回答）
	_ = g.AddLambdaNode("interview", compose.InvokableLambda(func(ctx context.Context, d *model.Direction) (*model.TrainingSession, error) {
		return ns.runInterview(ctx, *d, in.ask)
	}))

	// 复盘阶段：彻底复盘 + 画像沉淀 + 最终持久化
	_ = g.AddLambdaNode("review", compose.InvokableLambda(func(ctx context.Context, sess *model.TrainingSession) (*model.TrainingSession, error) {
		return ns.runReview(ctx, sess)
	}))

	_ = g.AddEdge(compose.START, "prepare")
	_ = g.AddEdge("prepare", "interview")
	_ = g.AddEdge("interview", "review")
	_ = g.AddEdge("review", compose.END)

	compiled, err := g.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("graph compile: %w", err)
	}
	return compiled.Invoke(ctx, in)
}
