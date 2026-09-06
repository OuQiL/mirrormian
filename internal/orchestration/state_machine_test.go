package orchestration

import (
	"testing"

	"mirror-mian/internal/agents"
	"mirror-mian/internal/model"
)

func judge(t string) agents.InterviewJudge {
	return agents.InterviewJudge{Judge: t}
}

// 路径 1：连续 continue 直到满 3 轮 → 强制结束进入下一知识点
func TestMachineThreeRoundsThenForceEnd(t *testing.T) {
	m := NewMachine([]string{"KP1", "KP2"})
	a := m.Start()
	if a.Type != AskInitial || a.Knowledge != "KP1" {
		t.Fatalf("start = %+v", a)
	}

	q := model.Question{ID: "q0", Text: "初始", KnowledgePt: "KP1", Round: 0}
	// 三轮 continue
	a = m.OnAnswer(q, "答0", judge(model.JudgeContinue))
	if a.Type != AskFollowUp || a.Round != 1 {
		t.Fatalf("round1 = %+v", a)
	}
	q1 := model.Question{ID: "q1", Round: 1}
	a = m.OnAnswer(q1, "答1", judge(model.JudgeContinue))
	if a.Type != AskFollowUp || a.Round != 2 {
		t.Fatalf("round2 = %+v", a)
	}
	a = m.OnAnswer(model.Question{ID: "q2", Round: 2}, "答2", judge(model.JudgeContinue))
	if a.Type != AskFollowUp || a.Round != 3 {
		t.Fatalf("round3 = %+v", a)
	}

	// 第 3 轮追问后继续 continue → 强制结束，进入下一知识点
	a = m.OnAnswer(model.Question{ID: "q3", Round: 3}, "答3", judge(model.JudgeContinue))
	if a.Type != AskInitial || a.Knowledge != "KP2" {
		t.Fatalf("force end should move to KP2: %+v", a)
	}
}

// 路径 2：advance（过关）→ 提前结束当前知识点
func TestMachineAdvanceStops(t *testing.T) {
	m := NewMachine([]string{"KP1", "KP2"})
	m.Start()
	a := m.OnAnswer(model.Question{ID: "q", KnowledgePt: "KP1"}, "答", judge(model.JudgeAdvance))
	if a.Type != AskInitial || a.Knowledge != "KP2" {
		t.Fatalf("advance should move to next: %+v", a)
	}
}

// 路径 3：stop（无力回答）→ 提前放弃当前知识点
func TestMachineStopGivesUp(t *testing.T) {
	m := NewMachine([]string{"KP1", "KP2"})
	m.Start()
	a := m.OnAnswer(model.Question{ID: "q", KnowledgePt: "KP1"}, "答", judge(model.JudgeStop))
	if a.Type != AskInitial || a.Knowledge != "KP2" {
		t.Fatalf("stop should move to next: %+v", a)
	}
}

// 路径 4：全部知识点完成 → InterviewDone
func TestMachineDone(t *testing.T) {
	m := NewMachine([]string{"KP1"})
	m.Start()
	a := m.OnAnswer(model.Question{ID: "q"}, "答", judge(model.JudgeAdvance))
	if a.Type != InterviewDone {
		t.Fatalf("should be done: %+v", a)
	}
}

// 空方向 → 直接结束
func TestMachineEmpty(t *testing.T) {
	m := NewMachine(nil)
	if a := m.Start(); a.Type != InterviewDone {
		t.Fatalf("empty should done: %+v", a)
	}
}

// 追问携带上下文：PrevQ / Answer / FollowUpHint / Round
func TestFollowUpContext(t *testing.T) {
	m := NewMachine([]string{"KP1"})
	m.Start()
	a := m.OnAnswer(model.Question{ID: "q", Text: "题目", KnowledgePt: "KP1"}, "我的回答",
		agents.InterviewJudge{Judge: model.JudgeContinue, FollowUpHint: "追问 X"})
	if a.PrevQ.ID != "q" || a.Answer != "我的回答" || a.FollowUpHint != "追问 X" || a.Round != 1 {
		t.Fatalf("follow-up context wrong: %+v", a)
	}
}
