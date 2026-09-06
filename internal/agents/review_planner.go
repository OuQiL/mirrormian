// 复习规划 Agent：基于画像（薄弱点 + SM-2 + 掌握度 + 到期复习）生成行动化复习计划。
package agents

import (
	"context"
	"encoding/json"
	"fmt"

	"mirror-mian/internal/llm"
	"mirror-mian/internal/model"
)

// PlanItem 一项复习行动。
type PlanItem struct {
	Topic      string `json:"topic"`
	Action     string `json:"action"`     // 练什么（到期复习/专项训练/补知识点）
	Priority   string `json:"priority"`   // high / medium / low
	Suggestion string `json:"suggestion"` // 练习方式建议
	Minutes    int    `json:"minutes"`    // 预计耗时（分钟）
}

// ReviewPlan 复习计划。
type ReviewPlan struct {
	Summary string     `json:"summary"`
	Items   []PlanItem `json:"items"`
}

// ReviewPlanner 复习规划 Agent。
type ReviewPlanner struct {
	llm llm.Client
}

// NewReviewPlanner 创建复习规划 Agent。
func NewReviewPlanner(llm llm.Client) *ReviewPlanner {
	return &ReviewPlanner{llm: llm}
}

// Plan 基于画像生成复习计划。画像为空时返回提示（不调 LLM）。
func (p *ReviewPlanner) Plan(ctx context.Context, profile *model.Profile, dueCount int) (ReviewPlan, error) {
	if profile == nil || (len(profile.WeakPoints) == 0 && len(profile.Mastery) == 0) {
		return ReviewPlan{
			Summary: "画像为空——先完成几场面试积累数据，再生成复习计划。",
			Items:   []PlanItem{},
		}, nil
	}
	profileJSON, err := json.Marshal(profile)
	if err != nil {
		return ReviewPlan{}, err
	}

	const system = `你是复习规划教练。基于候选人画像（薄弱点含 SM-2 复习状态、主题掌握度、到期复习数），生成今天的行动化复习计划。
要求：
- 到期复习项优先（Priority=high），随后是掌握度最低/薄弱点最多的主题；
- 每项给出：练什么（到期复习 / 专项训练 / 补知识点）、练习方式建议、预计耗时（分钟）；
- items 3-8 项，按优先级排序；
- summary 用 1-2 句概述今日计划。
只输出 JSON，不要任何解释文字。`

	user := fmt.Sprintf("画像：%s\n到期复习项数量：%d", string(profileJSON), dueCount)

	var out ReviewPlan
	if err := p.llm.GenerateJSON(ctx, system, user, &out); err != nil {
		return out, err
	}
	if len(out.Items) == 0 {
		out.Items = []PlanItem{}
	}
	return out, nil
}
