// 面试官 Agent（interviewer）：评估用户回答，输出结构化判定与追问方向。
//
// 判定（spec：LLM 判断用户是游刃有余还是无力回答）：
//   - continue：游刃有余，可以进入下一轮追问；
//   - advance：知识点已过关，停止追问；
//   - stop：无力回答，放弃追问。
//
// 追问轮数上限由编排层状态机强制（默认 3 轮），本 Agent 只负责判定与内容。
package agents

import (
	"context"
	"fmt"
	"strings"

	"mirror-mian/internal/llm"
	"mirror-mian/internal/model"
)

// InterviewJudge 面试官对一轮回答的结构化判定。
type InterviewJudge struct {
	Judge string `json:"judge"` // continue / advance / stop
	Score float64 `json:"score"` // 本轮回答评分 0-10
	Reason string `json:"reason"` // 判定理由（简短）
	// FollowUp 仅在 judge=continue 时有意义：建议的追问方向要点（由 questioner 落成题目）
	FollowUpHint string `json:"follow_up_hint,omitempty"`
}

// Interviewer 面试官 Agent。
type Interviewer struct {
	llm llm.Client
}

// NewInterviewer 创建面试官 Agent。
func NewInterviewer(llm llm.Client) *Interviewer {
	return &Interviewer{llm: llm}
}

// JudgeAnswer 评估用户回答，返回结构化判定。
// qaHistory 为当前知识点下此前（题, 回答）对，用于避免重复与判定推进。
func (i *Interviewer) JudgeAnswer(ctx context.Context, q model.Question, answer string, qaHistory []QAPair) (InterviewJudge, error) {
	const system = `你是技术面试的面试官。评估候选人对一道题的作答，判断能否继续深入。
判定规则：
- continue：候选人回答表现出游刃有余（概念清晰、有展开、可深入），可继续追问；
- advance：候选人已充分掌握该知识点（回答准确完整，再问边际价值低），知识点过关；
- stop：候选人明显无力回答（回答错误、空白、含糊），应放弃追问避免为难。
评分：0-10，评估回答质量。

只输出 JSON：{"judge": "continue|advance|stop", "score": 0-10, "reason": "简短理由", "follow_up_hint": "（judge=continue 时）下一轮追问应针对回答中的哪个点"}，不要任何解释文字。`

	var b strings.Builder
	for _, pair := range qaHistory {
		fmt.Fprintf(&b, "- 题：%s\n  答：%s\n", pair.Question.Text, pair.Answer)
	}
	history := b.String()

	user := fmt.Sprintf(`本轮题目：%s
本轮回答：%s

此前问答记录：
%s`, q.Text, answer, history)

	var j InterviewJudge
	if err := i.llm.GenerateJSON(ctx, system, user, &j); err != nil {
		return j, err
	}
	switch j.Judge {
	case model.JudgeContinue, model.JudgeAdvance, model.JudgeStop:
	default:
		return j, fmt.Errorf("interviewer: invalid judge %q", j.Judge)
	}
	return j, nil
}

// QAPair 一问一答记录。
type QAPair struct {
	Question model.Question
	Answer   string
}
