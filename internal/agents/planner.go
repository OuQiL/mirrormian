// 方向规划 Agent（direction_planner）：准备阶段确定出题方向。
//
// 专项模式：按主题生成知识点方向清单；
// 综合模式：解析简历 + JD，生成岗位关键能力点方向（含 JD 分析摘要与简历匹配摘要）。
package agents

import (
	"context"
	"fmt"
	"strings"

	"mirror-mian/internal/llm"
	"mirror-mian/internal/model"
)

// DirectionPlanner 出题方向规划 Agent。
type DirectionPlanner struct {
	llm      llm.Client
	analyzer *JDResumeAnalyzer
}

// NewDirectionPlanner 创建方向规划 Agent。
func NewDirectionPlanner(llm llm.Client) *DirectionPlanner {
	return &DirectionPlanner{llm: llm, analyzer: NewJDResumeAnalyzer(llm)}
}

// PlanSpecial 专项面试：按主题生成知识点方向清单。
func (p *DirectionPlanner) PlanSpecial(ctx context.Context, topic string) (model.Direction, error) {
	const system = `你是技术面试的出题方向规划师。给定一个训练主题，输出该主题应覆盖的知识点清单（出题方向）。
要求：
- 每个知识点必须聚焦单一概念（最小单位），可独立出题；
- 覆盖该主题的核心概念、常见难点与高频考点，6-10 个；
- 只输出 JSON，不要任何解释文字。`

	user := fmt.Sprintf(`主题：%s
请输出：{"key_points": ["知识点1", "知识点2", ...]}`, topic)

	var out struct {
		KeyPoints []string `json:"key_points"`
	}
	if err := p.llm.GenerateJSON(ctx, system, user, &out); err != nil {
		return model.Direction{}, err
	}
	if len(out.KeyPoints) == 0 {
		return model.Direction{}, fmt.Errorf("direction_planner: empty key points for topic %q", topic)
	}
	return model.Direction{Mode: model.ModeSpecial, Topic: topic, KeyPoints: out.KeyPoints}, nil
}

// PlanFull 综合面试：先执行 JD/简历匹配度分析，再基于差距生成出题方向（差距优先）。
func (p *DirectionPlanner) PlanFull(ctx context.Context, resumeText, jdText string) (model.Direction, error) {
	// 1) 匹配度分析（JD/简历分析 Agent）
	analysis, err := p.analyzer.Analyze(ctx, resumeText, jdText)
	if err != nil {
		return model.Direction{}, fmt.Errorf("match analysis: %w", err)
	}

	// 2) 基于分析结果生成出题方向（差距项优先）
	const system = `你是技术面试的出题方向规划师。基于 JD 与简历的匹配度分析结果，输出综合面试的出题方向。
要求：
- 提炼岗位最看重的关键能力点（6-10 个，聚焦单一概念的最小单位）；
- 匹配差距（high/medium 严重度）对应的知识点必须优先进入方向；
- 只输出 JSON，不要任何解释文字。`

	user := fmt.Sprintf(`匹配度分析：
- 匹配评分：%d/100
- 差距清单（按严重度）：%s
- 技术栈：%s
- 总评：%s

请输出：{"key_points": ["能力点1", ...], "jd_summary": "JD 分析摘要", "resume_summary": "匹配总评"}`, analysis.MatchScore, formatGaps(analysis.Gaps), joinSlice(analysis.TechStack), analysis.Summary)

	var out struct {
		KeyPoints     []string `json:"key_points"`
		JDSummary     string   `json:"jd_summary"`
		ResumeSummary string   `json:"resume_summary"`
	}
	if err := p.llm.GenerateJSON(ctx, system, user, &out); err != nil {
		return model.Direction{}, err
	}
	if len(out.KeyPoints) == 0 {
		return model.Direction{}, fmt.Errorf("direction_planner: empty key points")
	}
	return model.Direction{
		Mode: model.ModeFull, KeyPoints: out.KeyPoints,
		JDSummary: out.JDSummary, ResumeSummary: out.ResumeSummary,
	}, nil
}

// formatGaps 差距清单 → 文本。
func formatGaps(gaps []Gap) string {
	if len(gaps) == 0 {
		return "（无差距）"
	}
	var b strings.Builder
	for i, g := range gaps {
		if i >= 10 {
			break
		}
		fmt.Fprintf(&b, "[%s] %s（建议：%s）; ", g.Severity, g.Item, g.Suggestion)
	}
	return b.String()
}

func joinSlice(items []string) string {
	if len(items) == 0 {
		return "（无）"
	}
	return strings.Join(items, ", ")
}
