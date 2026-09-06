// JD/简历分析 Agent：解析 JD 与简历，输出结构化匹配度分析。
//
// 产出：岗位职责/任职要求（逐项对位）/技术栈/匹配评分/差距清单（严重度排序），
// 综合面试准备阶段基于差距生成出题方向。
package agents

import (
	"context"
	"fmt"

	"mirror-mian/internal/llm"
)

// Requirement 任职要求逐项对位。
type Requirement struct {
	Item     string `json:"item"`
	Level    string `json:"level"`    // hard / plus
	Status   string `json:"status"`   // met / partial / missing
	Evidence string `json:"evidence"` // 简历中的支撑（无则空）
}

// Gap 匹配差距。
type Gap struct {
	Item       string `json:"item"`
	Severity   string `json:"severity"` // high / medium / low
	Suggestion string `json:"suggestion"`
}

// JDAnalysis 匹配度分析结果。
type JDAnalysis struct {
	Responsibilities []string      `json:"responsibilities"`
	Requirements     []Requirement `json:"requirements"`
	TechStack        []string      `json:"tech_stack"`
	MatchScore       int           `json:"match_score"` // 0-100
	Gaps             []Gap         `json:"gaps"`
	Summary          string        `json:"summary"`
}

// JDResumeAnalyzer JD/简历分析 Agent。
type JDResumeAnalyzer struct {
	llm llm.Client
}

// NewJDResumeAnalyzer 创建分析 Agent。
func NewJDResumeAnalyzer(llm llm.Client) *JDResumeAnalyzer {
	return &JDResumeAnalyzer{llm: llm}
}

// Analyze 解析 JD 与简历并输出匹配度分析。
func (a *JDResumeAnalyzer) Analyze(ctx context.Context, resumeText, jdText string) (JDAnalysis, error) {
	const system = `你是资深招聘专家与面试官，擅长 JD 与简历的匹配度分析。
对 JD 与候选人简历进行结构化分析：
- responsibilities：岗位职责要点（3-8 条）；
- requirements：逐项任职要求（含硬性要求与加分项），每项标注 status：
  met（简历有明确支撑）/ partial（部分满足）/ missing（缺失），evidence 引用简历原文；
- tech_stack：JD 涉及的技术栈清单；
- match_score：0-100 匹配评分（硬性要求满足率加权，加分项少量加分）；
- gaps：差距清单，按严重度降序（high/medium/low），每项给出 suggestion 补强建议；
- summary：2-3 句总评。

只输出 JSON，不要任何解释文字。`

	user := fmt.Sprintf(`JD：
%s

简历：
%s`, jdText, resumeText)

	var out JDAnalysis
	if err := a.llm.GenerateJSON(ctx, system, user, &out); err != nil {
		return out, err
	}
	if out.MatchScore < 0 || out.MatchScore > 100 {
		return out, fmt.Errorf("jd_resume_analyzer: 匹配评分越界 %d", out.MatchScore)
	}
	return out, nil
}
