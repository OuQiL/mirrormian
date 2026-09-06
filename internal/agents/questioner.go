// 出题 Agent（questioner）：按出题方向生成最小单位题。
//
// 每题只考察一个知识点（最小单位）；初始题为方向下的知识点命题，
// 追问由面试官 Agent 基于回答生成（见 interviewer.go）。
package agents

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"mirror-mian/internal/llm"
	"mirror-mian/internal/model"
)

// Questioner 出题 Agent。
type Questioner struct {
	llm llm.Client
}

// NewQuestioner 创建出题 Agent。
func NewQuestioner(llm llm.Client) *Questioner {
	return &Questioner{llm: llm}
}

// GenerateInitial 为单个知识点生成初始题（最小单位题，只考察该知识点）。
// knowledgeContext 为 RAG 检索到的知识库参考（可空，空则不注入）。
func (q *Questioner) GenerateInitial(ctx context.Context, knowledgePoint, knowledgeContext string) (model.Question, error) {
	const system = `你是技术面试的出题官。给定一个知识点，出一道最小单位题。
要求：
- 题目必须只考察这一个知识点，不得混入其他概念；
- 题目应能区分候选人是否真正理解该知识点（可含具体场景）；
- 若有「知识库参考」，可结合其中内容出题（术语、边界、常见陷阱优先）；
- 只输出 JSON，不要任何解释文字。`

	user := fmt.Sprintf(`知识点：%s
请输出：{"question": "题目文本", "knowledge_point": "%s"}`, knowledgePoint, knowledgePoint)
	if knowledgeContext != "" {
		user = fmt.Sprintf(`知识点：%s

%s

请输出：{"question": "题目文本", "knowledge_point": "%s"}`, knowledgePoint, knowledgeContext, knowledgePoint)
	}

	var out struct {
		Question      string `json:"question"`
		KnowledgePoint string `json:"knowledge_point"`
	}
	if err := q.llm.GenerateJSON(ctx, system, user, &out); err != nil {
		return model.Question{}, err
	}
	if out.Question == "" {
		return model.Question{}, fmt.Errorf("questioner: empty question for %q", knowledgePoint)
	}
	return model.Question{
		ID:          uuid.NewString(),
		Text:        out.Question,
		KnowledgePt: knowledgePoint,
		Round:       0,
	}, nil
}

// GenerateFollowUp 基于回答生成追问（由面试官 Agent 判定继续后调用）。
// 追问必须针对该回答的具体内容，不得重复已问内容。
func (q *Questioner) GenerateFollowUp(ctx context.Context, prev model.Question, answer string, round int) (model.Question, error) {
	const system = `你是技术面试的面试官。基于候选人对上一题的回答，生成一道追问。
要求：
- 追问必须紧扣候选人的回答内容（针对其提到的概念、遗漏或错误）；
- 仍只考察同一知识点，不引入新知识点；
- 不得重复已经问过的问题；
- 只输出 JSON，不要任何解释文字。`

	user := fmt.Sprintf(`知识点：%s
上一题：%s
候选人回答：%s

请输出：{"question": "追问文本", "knowledge_point": "%s"}`, prev.KnowledgePt, prev.Text, answer, prev.KnowledgePt)

	var out struct {
		Question       string `json:"question"`
		KnowledgePoint string `json:"knowledge_point"`
	}
	if err := q.llm.GenerateJSON(ctx, system, user, &out); err != nil {
		return model.Question{}, err
	}
	if out.Question == "" {
		return model.Question{}, fmt.Errorf("questioner: empty follow-up")
	}
	return model.Question{
		ID:          uuid.NewString(),
		Text:        out.Question,
		KnowledgePt: prev.KnowledgePt,
		Round:       round,
		ParentID:    prev.ID,
	}, nil
}
