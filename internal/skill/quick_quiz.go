// 快问快答 Skill：领域主题 → 逐题 → 即时判定 + 讲解 → 计分总结。
package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"mirror-mian/internal/agents"
	"mirror-mian/internal/llm"
	"mirror-mian/internal/model"
	"mirror-mian/internal/profile"
	"mirror-mian/internal/store"
)

// QuizState 测验会话状态。
type QuizState struct {
	Topic     string   `json:"topic"`
	KeyPoints []string `json:"key_points"`
	Idx       int      `json:"idx"`       // 当前知识点下标
	Round     int      `json:"round"`     // 当前知识点追问轮数
	Score     float64  `json:"score"`     // 累计分数
	Count     int      `json:"count"`     // 已答题数
	LastQ     *model.Question `json:"last_q,omitempty"`
	Done      bool     `json:"done"`
}

// QuickQuiz 快问快答技能。
type QuickQuiz struct {
	llm        llm.Client
	planner    *agents.DirectionPlanner
	questioner *agents.Questioner
	interviewer *agents.Interviewer
	store      store.Store
}

// NewQuickQuiz 创建快问快答技能。
func NewQuickQuiz(lm llm.Client, st store.Store) *QuickQuiz {
	return &QuickQuiz{
		llm: lm, store: st,
		planner: agents.NewDirectionPlanner(lm),
		questioner: agents.NewQuestioner(lm),
		interviewer: agents.NewInterviewer(lm),
	}
}

// Name 技能名。
func (q *QuickQuiz) Name() string { return "快问快答" }

// Description 描述。
func (q *QuickQuiz) Description() string {
	return "快问快答：选一个主题，逐题快答，每题即时判定并讲解，结束给出成绩与薄弱点"
}

// MatchScore 意图匹配。
func (q *QuickQuiz) MatchScore(_ context.Context, input string) int {
	score := 0
	for _, kw := range []string{"考考", "测验", "quiz", "测测", "抽查", "快问", "快答"} {
		if strings.Contains(input, kw) {
			score += 30
		}
	}
	return score
}

// Start 开始测验：方向规划 → 出第一题。
func (q *QuickQuiz) Start(ctx context.Context, input string) (*Session, *TurnResult, error) {
	topic := extractTopic(input)
	if topic == "" {
		return nil, nil, fmt.Errorf("请指定主题，如：考考我 Redis 概念")
	}
	direction, err := q.planner.PlanSpecial(ctx, topic)
	if err != nil {
		return nil, nil, err
	}
	state := &QuizState{Topic: topic, KeyPoints: direction.KeyPoints, Idx: 0}
	sess, err := newSession(q.Name(), state)
	if err != nil {
		return nil, nil, err
	}
	res, err := q.askCurrent(ctx, state, sess)
	if err != nil {
		return nil, nil, err
	}
	if err := saveState(sess, state, nil); err != nil {
		return nil, nil, err
	}
	return sess, res, nil
}

// Turn 推进一轮：判定 → 讲解 → 下一题或结束。
func (q *QuickQuiz) Turn(ctx context.Context, sess *Session, userInput string) (*TurnResult, error) {
	var state QuizState
	if err := json.Unmarshal(sess.State, &state); err != nil {
		return nil, err
	}
	if state.Done || state.LastQ == nil {
		return nil, fmt.Errorf("会话状态异常")
	}
	// 判定回答
	judge, err := q.interviewer.JudgeAnswer(ctx, *state.LastQ, userInput, nil)
	if err != nil {
		return nil, err
	}
	state.Score += judge.Score
	state.Count++

	// 讲解（LLM 生成简短解析）
	explain, err := q.llm.Generate(ctx,
		"你是面试讲解员。基于题目与回答，用 2-3 句讲解正确答案要点（指出对错）。",
		fmt.Sprintf("题目：%s\n回答：%s\n评分：%.0f/10", state.LastQ.Text, userInput, judge.Score))
	if err != nil {
		explain = ""
	}
	// 低分沉淀薄弱点（embedder 暂不注入，精确匹配降级）
	if judge.Score < profile.WeaknessThreshold {
		p, _ := q.store.GetProfile()
		profile.AbsorbAnswer(p, state.Topic, state.LastQ.KnowledgePt, judge.Score, time.Now(), nil)
		_ = q.store.SaveProfile(p)
	}

	// 下一题
	state.LastQ = nil
	state.Round = 0
	next, err := q.askCurrent(ctx, &state, sess)
	if err != nil {
		return nil, err
	}
	reply := fmt.Sprintf("【第 %d 题】得分 %.0f/10\n📖 %s\n\n%s", state.Count, judge.Score, explain, next.Reply)
	next.Reply = reply
	return next, saveState(sess, &state, q.store)
}

// askCurrent 出当前知识点题目；全部完成输出总结。
func (q *QuickQuiz) askCurrent(ctx context.Context, state *QuizState, sess *Session) (*TurnResult, error) {
	if state.Idx >= len(state.KeyPoints) {
		state.Done = true
		avg := 0.0
		if state.Count > 0 {
			avg = state.Score / float64(state.Count)
		}
		return &TurnResult{
			Reply:    fmt.Sprintf("✅ 测验完成！共 %d 题，平均 %.1f/10。薄弱点已沉淀到画像，可在「复习」页查看。", state.Count, avg),
			Finished: true,
		}, nil
	}
	qn, err := q.questioner.GenerateInitial(ctx, state.KeyPoints[state.Idx], "")
	if err != nil {
		return nil, err
	}
	state.LastQ = &qn
	state.Round = 0
	return &TurnResult{
		Reply: fmt.Sprintf("【%s】%s", qn.KnowledgePt, qn.Text),
	}, nil
}

// extractTopic 从输入提取主题（简单启发式：去掉动作词）。
func extractTopic(input string) string {
	for _, kw := range []string{"考考我", "考考", "测验", "测测", "抽查", "quiz"} {
		input = strings.ReplaceAll(input, kw, "")
	}
	input = strings.TrimSpace(strings.Trim(input, "：:，,。 "))
	return input
}
