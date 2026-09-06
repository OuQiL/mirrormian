// 面试阶段状态机：追问规则的纯逻辑实现（无 IO，可独立测试）。
//
// 规则（spec「最小单位题与追问规则」）：
//   - 每轮仅一道题；LLM 判定 advance（过关）或 stop（无力回答）→ 结束当前知识点；
//   - judge=continue 且追问轮数 < 3 → 生成下一轮追问（轮数+1）；
//   - judge=continue 且追问轮数已达 3 → 强制结束当前知识点（保证只有 3 轮）。
package orchestration

import (
	"mirror-mian/internal/agents"
	"mirror-mian/internal/model"
)

// ActionType 状态机输出的下一步动作。
type ActionType int

const (
	// AskInitial 对下一知识点出初始题（携带知识点名）。
	AskInitial ActionType = iota
	// AskFollowUp 基于上一题与回答生成追问（round = 上一轮+1）。
	AskFollowUp
	// InterviewDone 全部知识点结束，进入复盘阶段。
	InterviewDone
)

// Action 状态机推进结果。
type Action struct {
	Type      ActionType
	Knowledge string        // AskInitial：下一知识点
	PrevQ     model.Question // AskFollowUp：上一题
	Answer    string         // AskFollowUp：上轮回答
	FollowUpHint string      // AskFollowUp：面试官建议的追问方向
	Round     int            // AskFollowUp：本追问的轮数（1..3）
}

// Machine 面试状态机：按知识点推进，每个知识点最多 3 轮追问。
type Machine struct {
	keyPoints []string
	kpIdx     int
	round     int // 当前知识点已追问轮数
}

// NewMachine 创建状态机。
func NewMachine(keyPoints []string) *Machine {
	return &Machine{keyPoints: keyPoints, kpIdx: -1, round: 0}
}

// Start 开始面试：返回第一个知识点（AskInitial 动作），无知识点时返回 InterviewDone。
func (m *Machine) Start() Action {
	return m.advanceToNext()
}

// OnAnswer 提交一轮（题、回答、面试官判定），推进状态机。
func (m *Machine) OnAnswer(q model.Question, answer string, j agents.InterviewJudge) Action {
	switch j.Judge {
	case model.JudgeStop:
		// 无力回答：提前结束当前知识点（不保证 3 轮）
		return m.advanceToNext()
	case model.JudgeAdvance:
		// 知识点已过关
		return m.advanceToNext()
	default: // continue
		if m.round >= model.MaxFollowUpRounds {
			// 已追问 3 轮，强制结束（只有 3 轮）
			return m.advanceToNext()
		}
		m.round++
		return Action{
			Type:         AskFollowUp,
			PrevQ:        q,
			Answer:       answer,
			FollowUpHint: j.FollowUpHint,
			Round:        m.round,
		}
	}
}

// advanceToNext 进入下一知识点（重置追问轮数）；全部完成返回 InterviewDone。
func (m *Machine) advanceToNext() Action {
	m.round = 0
	m.kpIdx++
	if m.kpIdx >= len(m.keyPoints) {
		return Action{Type: InterviewDone}
	}
	return Action{Type: AskInitial, Knowledge: m.keyPoints[m.kpIdx]}
}
