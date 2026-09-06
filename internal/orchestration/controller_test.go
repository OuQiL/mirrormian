package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"mirror-mian/internal/model"
	"mirror-mian/internal/store"
)

// scriptedLLM 面试官判定可按脚本编排（依次消费），其余逻辑复用 fakeLLM。
type scriptedLLM struct {
	fakeLLM
	judges []string // 如 "continue" "advance" "stop"
}

func (s *scriptedLLM) Generate(ctx context.Context, system, user string) (string, error) {
	if strings.Contains(system, "评估候选人对一道题的作答") {
		j := "advance"
		if len(s.judges) > 0 {
			j = s.judges[0]
			s.judges = s.judges[1:]
		}
		return fmt.Sprintf(`{"judge": "%s", "score": 6, "reason": "脚本判定"}`, j), nil
	}
	return s.fakeLLM.Generate(ctx, system, user)
}

// GenerateJSON 必须覆盖：否则继承的 fakeLLM.GenerateJSON 会绕过本类型
// 的 Generate（走 fakeLLM 的默认 advance），judges 脚本失效。
func (s *scriptedLLM) GenerateJSON(ctx context.Context, system, user string, out any) error {
	text, err := s.Generate(ctx, system, user)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(text), out)
}

func newControllerService(t *testing.T, judges ...string) (*Service, store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "ctrl.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return NewService(&scriptedLLM{fakeLLM: fakeLLM{}, judges: judges}, st, nil, ""), st
}

// 任务 1.1：start 创建会话并出第一题
func TestControllerStart(t *testing.T) {
	svc, st := newControllerService(t)
	sess, err := svc.StartSpecial(context.Background(), "Redis")
	if err != nil {
		t.Fatalf("StartSpecial: %v", err)
	}
	if sess.Status != model.SessionOngoing {
		t.Fatalf("status = %s", sess.Status)
	}
	if len(sess.Questions) != 1 || sess.Questions[0].Round != 0 {
		t.Fatalf("should have 1 initial question: %+v", sess.Questions)
	}
	// 已持久化
	got, err := st.GetSession(sess.ID)
	if err != nil || len(got.Questions) != 1 {
		t.Fatalf("session not persisted: %v", err)
	}
}

// 任务 1.2：answer 推进——advance 过关 → 下一知识点初始题
func TestControllerAnswerAdvance(t *testing.T) {
	svc, _ := newControllerService(t, "advance", "advance")
	sess, _ := svc.StartSpecial(context.Background(), "Redis")
	kp0 := sess.Questions[0].KnowledgePt

	res, err := svc.SubmitAnswer(context.Background(), sess.ID, "回答1")
	if err != nil {
		t.Fatalf("SubmitAnswer: %v", err)
	}
	if res.NextQuestion == nil || res.Review != nil {
		t.Fatalf("expect next question: %+v", res)
	}
	if res.NextQuestion.KnowledgePt == kp0 {
		t.Fatalf("advance should move to next knowledge point: %+v", res.NextQuestion)
	}
	if res.NextQuestion.Round != 0 {
		t.Fatalf("next should be initial round: %+v", res.NextQuestion)
	}
}

// 任务 1.2：continue → 追问
func TestControllerAnswerContinue(t *testing.T) {
	svc, _ := newControllerService(t, "continue", "advance")
	sess, _ := svc.StartSpecial(context.Background(), "Redis")
	kp0 := sess.Questions[0].KnowledgePt

	res, err := svc.SubmitAnswer(context.Background(), sess.ID, "回答1")
	if err != nil {
		t.Fatalf("SubmitAnswer: %v", err)
	}
	if res.NextQuestion == nil {
		t.Fatal("expect follow-up")
	}
	if res.NextQuestion.Round != 1 || res.NextQuestion.KnowledgePt != kp0 {
		t.Fatalf("expect follow-up round 1 on same kp: %+v", res.NextQuestion)
	}
}

// 任务 1.2：stop 无力 → 直接下一知识点（不保证 3 轮）
func TestControllerAnswerStop(t *testing.T) {
	svc, _ := newControllerService(t, "stop", "advance")
	sess, _ := svc.StartSpecial(context.Background(), "Redis")
	kp0 := sess.Questions[0].KnowledgePt

	res, err := svc.SubmitAnswer(context.Background(), sess.ID, "答不上")
	if err != nil {
		t.Fatalf("SubmitAnswer: %v", err)
	}
	if res.NextQuestion == nil || res.NextQuestion.KnowledgePt == kp0 {
		t.Fatalf("stop should move to next kp: %+v", res.NextQuestion)
	}
}

// 任务 1.2：最后一知识点完成后 → 复盘 + 画像沉淀
func TestControllerFinishWithReview(t *testing.T) {
	svc, st := newControllerService(t) // 默认全部 advance
	sess, _ := svc.StartSpecial(context.Background(), "Redis")

	var res StepResult
	for {
		if res.Review != nil {
			break
		}
		r, err := svc.SubmitAnswer(context.Background(), sess.ID, "答")
		if err != nil {
			t.Fatalf("SubmitAnswer: %v", err)
		}
		res = r
	}

	if res.Review.Status != model.SessionFinished || res.Review.Review == "" {
		t.Fatalf("review incomplete: %+v", res.Review)
	}
	// 画像沉淀
	p, err := st.GetProfile()
	if err != nil || len(p.WeakPoints) == 0 {
		t.Fatalf("profile not absorbed: %+v", p)
	}
	// 会话已结束
	got, _ := st.GetSession(sess.ID)
	if got.Status != model.SessionFinished {
		t.Fatalf("session should be finished: %s", got.Status)
	}
}

// 结束面试：未答题目被丢弃，已答题目生成复盘并沉淀画像
func TestControllerFinish(t *testing.T) {
	svc, st := newControllerService(t, "advance")
	sess, _ := svc.StartSpecial(context.Background(), "Redis")
	// 答一题后结束（最后一题未答会被丢弃）
	if _, err := svc.SubmitAnswer(context.Background(), sess.ID, "第一答"); err != nil {
		t.Fatalf("SubmitAnswer: %v", err)
	}
	finished, err := svc.Finish(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if finished.Status != model.SessionFinished || finished.Review == "" {
		t.Fatalf("finish review incomplete: status=%s review=%q", finished.Status, finished.Review)
	}
	// 未答题目被丢弃：questions == answers
	got, _ := st.GetSession(sess.ID)
	if len(got.Questions) != len(got.Answers) || len(got.Questions) != 1 {
		t.Fatalf("unanswered question not dropped: q=%d a=%d", len(got.Questions), len(got.Answers))
	}
	// 画像沉淀
	p, _ := st.GetProfile()
	if len(p.WeakPoints) == 0 {
		t.Fatal("weak points should be absorbed")
	}
}

// 未答任何题就结束 → 明确错误
func TestControllerFinishNoAnswers(t *testing.T) {
	svc, _ := newControllerService(t)
	sess, _ := svc.StartSpecial(context.Background(), "Redis")
	if _, err := svc.Finish(context.Background(), sess.ID); err == nil {
		t.Fatal("finish with no answers should error")
	}
}

// 任务 1.3：状态机恢复——两轮问答后从持久化恢复继续
func TestControllerResume(t *testing.T) {
	svc, st := newControllerService(t, "continue", "advance", "advance")
	sess, _ := svc.StartSpecial(context.Background(), "Redis")

	res, err := svc.SubmitAnswer(context.Background(), sess.ID, "第一答")
	if err != nil || res.NextQuestion == nil {
		t.Fatalf("first submit: %v", err)
	}

	// 模拟 Web 重启：重新加载服务（同一 store），从持久化恢复
	svc2, _ := newControllerServiceWithStore(t, st)
	res2, err := svc2.SubmitAnswer(context.Background(), sess.ID, "第二答")
	if err != nil {
		t.Fatalf("resumed submit: %v", err)
	}
	if res2.NextQuestion == nil {
		t.Fatal("resumed session should continue")
	}
	if res2.NextQuestion.Round != 0 {
		t.Fatalf("after advance should be initial on new kp: %+v", res2.NextQuestion)
	}
	// 会话题目数递增
	got, _ := st.GetSession(sess.ID)
	if len(got.Questions) != 3 {
		t.Fatalf("questions = %d, want 3", len(got.Questions))
	}
}

func newControllerServiceWithStore(t *testing.T, st store.Store) (*Service, store.Store) {
	t.Helper()
	return NewService(&scriptedLLM{fakeLLM: fakeLLM{}, judges: []string{"advance"}}, st, nil, ""), st
}
