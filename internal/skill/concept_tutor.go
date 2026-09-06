// 知识学习 Skill：讲解概念 → 提问确认理解 → 未懂换角度再讲 → 深入。
package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mirror-mian/internal/agents"
	"mirror-mian/internal/llm"
	"mirror-mian/internal/model"
)

// TutorState 教学会话状态。
type TutorState struct {
	Topic    string   `json:"topic"`
	Concepts []string `json:"concepts"`
	Idx      int      `json:"idx"`
	Mode     string   `json:"mode"` // explain / question
	Attempts int      `json:"attempts"` // 当前概念讲解次数
	Done     bool     `json:"done"`
}

// ConceptTutor 知识学习技能。
type ConceptTutor struct {
	llm         llm.Client
	planner     *agents.DirectionPlanner
	questioner  *agents.Questioner
	interviewer *agents.Interviewer
}

// NewConceptTutor 创建知识学习技能。
func NewConceptTutor(lm llm.Client) *ConceptTutor {
	return &ConceptTutor{
		llm:         lm,
		planner:     agents.NewDirectionPlanner(lm),
		questioner:  agents.NewQuestioner(lm),
		interviewer: agents.NewInterviewer(lm),
	}
}

// Name 技能名。
func (t *ConceptTutor) Name() string { return "知识学习" }

// Description 描述。
func (t *ConceptTutor) Description() string {
	return "知识学习：讲解一个概念，提问确认理解，没懂就换角度再讲，直到掌握"
}

// MatchScore 意图匹配。
func (t *ConceptTutor) MatchScore(_ context.Context, input string) int {
	score := 0
	for _, kw := range []string{"讲讲", "教我", "教学", "学习", "理解", "是什么", "知识"} {
		if strings.Contains(input, kw) {
			score += 30
		}
	}
	if strings.Contains(input, "考") || strings.Contains(input, "测") {
		score = 0 // 测验意图归 快问快答
	}
	return score
}

// Start 开始教学：方向规划 → 讲解第一个概念。
func (t *ConceptTutor) Start(ctx context.Context, input string) (*Session, *TurnResult, error) {
	topic := extractTopic(input)
	if topic == "" {
		return nil, nil, fmt.Errorf("请指定要学的主题，如：教我 Redis 持久化")
	}
	direction, err := t.planner.PlanSpecial(ctx, topic)
	if err != nil {
		return nil, nil, err
	}
	state := &TutorState{Topic: topic, Concepts: direction.KeyPoints, Idx: 0, Mode: "explain"}
	sess, err := newSession(t.Name(), state)
	if err != nil {
		return nil, nil, err
	}
	res, err := t.explainCurrent(ctx, state, sess)
	return sess, res, err
}

// Turn 推进：回答确认 → 判定掌握与否。
func (t *ConceptTutor) Turn(ctx context.Context, sess *Session, userInput string) (*TurnResult, error) {
	var state TutorState
	if err := json.Unmarshal(sess.State, &state); err != nil {
		return nil, err
	}
	if state.Done {
		return nil, fmt.Errorf("会话已结束")
	}
	if state.Mode != "question" {
		return nil, fmt.Errorf("当前在讲解模式，请输入 /next 进入下一概念")
	}

	// 判定理解程度（复用面试官判定：advance=懂了，continue=继续，stop=不懂）
	judge, err := t.interviewer.JudgeAnswer(ctx, model.Question{
		ID: "tutor-q", Text: "请用自己的话复述刚才讲解的概念", KnowledgePt: state.Concepts[state.Idx],
	}, userInput, nil)
	if err != nil {
		return nil, err
	}
	switch judge.Judge {
	case model.JudgeAdvance:
		// 掌握 → 下一概念
		state.Idx++
		state.Attempts = 0
		if state.Idx >= len(state.Concepts) {
			state.Done = true
			return &TurnResult{
				Reply:    fmt.Sprintf("🎓 完成！%s 的 %d 个核心概念已全部掌握。", state.Topic, len(state.Concepts)),
				Finished: true,
			}, saveState(sess, &state, nil)
		}
		return t.explainCurrent(ctx, &state, sess)
	case model.JudgeStop:
		// 不懂 → 换角度重讲
		state.Attempts++
		return t.explainCurrent(ctx, &state, sess)
	default:
		// 部分理解 → 再追问一次确认
		state.Attempts++
		return &TurnResult{Reply: "理解得不错，再深入一点：这个概念的**核心原理**是什么？"}, saveState(sess, &state, nil)
	}
}

// explainCurrent 讲解当前概念（首次/换角度）。
func (t *ConceptTutor) explainCurrent(ctx context.Context, state *TutorState, sess *Session) (*TurnResult, error) {
	concept := state.Concepts[state.Idx]
	angle := "用通俗的语言讲解"
	if state.Attempts > 0 {
		angle = fmt.Sprintf("上次没讲明白，换一个角度（比喻/例子）重新讲解，第 %d 次尝试", state.Attempts+1)
	}
	content, err := t.llm.Generate(ctx,
		"你是耐心的技术讲师。用通俗语言讲解概念，含 1 个生活化比喻与关键要点。",
		fmt.Sprintf("概念：%s\n要求：%s", concept, angle))
	if err != nil {
		return nil, err
	}
	state.Mode = "question"
	reply := fmt.Sprintf("### 📖 %s\n\n%s\n\n**请用自己的话复述，确认理解了吗？**", concept, content)
	return &TurnResult{Reply: reply}, saveState(sess, state, nil)
}
