// 复盘 Agent（reviewer）：面试结束后生成彻底复盘（仿 TechSpar）。
//
// 输出双份：
//   - Markdown 报告：整体评价 + 平均分 + 逐题评分点评 + 改进建议 + 薄弱点/亮点；
//   - 结构化 JSON：ReviewReport，供画像沉淀使用。
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mirror-mian/internal/llm"
	"mirror-mian/internal/model"
)

// Reviewer 复盘 Agent。
type Reviewer struct {
	llm llm.Client
}

// NewReviewer 创建复盘 Agent。
func NewReviewer(llm llm.Client) *Reviewer {
	return &Reviewer{llm: llm}
}

// Review 基于整场会话生成复盘（Markdown + 结构化 JSON）。
func (r *Reviewer) Review(ctx context.Context, sess *model.TrainingSession) (reviewMD string, report model.ReviewReport, err error) {
	transcript := buildTranscript(sess)

	const system = `你是技术面试的复盘教练。基于整场面试的逐题记录生成彻底复盘（仿 TechSpar）。
输出两份内容（同一 LLM 调用中给出，用 JSON 包裹）：
- markdown：整体评价（含平均分）+ 逐题评分与点评 + 改进建议 + 薄弱点与亮点，用 Markdown 格式；
- report：结构化 JSON 字段 per_question 逐题（question_id/question_text/score/assessment/improvement）。
注意：per_question 必须与面试记录中的题目逐题对应、顺序一致；question_id 输出字符串（无法确定时输出空字符串""），score 为 0-10 数字。

只输出 JSON：{"markdown": "...", "overall_score": 0-10, "summary": "...", "per_question": [...], "weak_points": [...], "strong_points": [...], "improvements": [...]}，不要任何解释文字。`

	type out struct {
		Markdown     string   `json:"markdown"`
		OverallScore float64  `json:"overall_score"`
		Summary      string   `json:"summary"`
		PerQuestion  []struct {
			QuestionID   flexString `json:"question_id"`
			QuestionText string     `json:"question_text"`
			Score        float64    `json:"score"`
			Assessment   string     `json:"assessment"`
			Improvement  string     `json:"improvement"`
		} `json:"per_question"`
		WeakPoints   []string `json:"weak_points"`
		StrongPoints []string `json:"strong_points"`
		Improvements []string `json:"improvements"`
	}

	var o out
	if err := r.llm.GenerateJSON(ctx, system, fmt.Sprintf("面试记录：\n%s", transcript), &o); err != nil {
		return "", model.ReviewReport{}, err
	}

	report = model.ReviewReport{
		OverallScore: o.OverallScore,
		Summary:      o.Summary,
		WeakPoints:   o.WeakPoints,
		StrongPoints: o.StrongPoints,
		Improvements: o.Improvements,
	}
	for _, pq := range o.PerQuestion {
		report.PerQuestion = append(report.PerQuestion, model.PerQuestionReview{
			QuestionID: string(pq.QuestionID), QuestionText: pq.QuestionText,
			Score: pq.Score, Assessment: pq.Assessment, Improvement: pq.Improvement,
		})
	}
	return o.Markdown, report, nil
}

// flexString 容错解析：模型可能输出 string 或 number 形式的 id。
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err == nil {
		*f = flexString(n.String())
		return nil
	}
	*f = ""
	return nil
}

// buildTranscript 把会话转为逐题记录文本（题目/回答/评分）。
func buildTranscript(sess *model.TrainingSession) string {
	answerByQ := map[string]string{}
	for _, a := range sess.Answers {
		answerByQ[a.QuestionID] = a.Text
	}
	var b strings.Builder
	fmt.Fprintf(&b, "面试模式：%s\n主题：%s\n\n", sess.Mode, sess.Direction.Topic)
	for _, q := range sess.Questions {
		fmt.Fprintf(&b, "第%d轮（知识点：%s）：%s\n", q.Round+1, q.KnowledgePt, q.Text)
		if ans, ok := answerByQ[q.ID]; ok {
			fmt.Fprintf(&b, "回答：%s\n", ans)
		}
	}
	return b.String()
}
