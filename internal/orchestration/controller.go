// 断点面试控制器：供 Web 等交互层每轮一问一答驱动。
//
// Start 创建会话并出第一题；SubmitAnswer 提交回答推进一轮
// （判定 → 追问/过关/无力 → 出下一题或触发复盘），状态每步持久化。
package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"mirror-mian/internal/agents"
	"mirror-mian/internal/model"
)

// StepResult 一轮推进的产物。
type StepResult struct {
	// NextQuestion 非空：继续面试，下一道题（初始题或追问）
	NextQuestion *model.Question
	// Review 非空：面试结束，已生成复盘并沉淀画像
	Review *model.TrainingSession
}

// StartSpecial 专项面试：准备阶段 + 出第一题。
func (s *Service) StartSpecial(ctx context.Context, topic string) (*model.TrainingSession, error) {
	direction, err := s.nodes.planner.PlanSpecial(ctx, topic)
	if err != nil {
		return nil, fmt.Errorf("prepare: %w", err)
	}
	return s.start(ctx, direction)
}

// StartFull 综合面试：准备阶段（JD/简历分析）+ 出第一题。
func (s *Service) StartFull(ctx context.Context, resumeText, jdText string) (*model.TrainingSession, error) {
	direction, err := s.nodes.planner.PlanFull(ctx, resumeText, jdText)
	if err != nil {
		return nil, fmt.Errorf("prepare: %w", err)
	}
	return s.start(ctx, direction)
}

// start 创建会话并生成第一题。
func (s *Service) start(ctx context.Context, direction model.Direction) (*model.TrainingSession, error) {
	sess := &model.TrainingSession{
		ID:        uuid.NewString(),
		Mode:      direction.Mode,
		Status:    model.SessionOngoing,
		Direction: direction,
		CreatedAt: time.Now(),
	}
	if len(direction.KeyPoints) == 0 {
		return nil, fmt.Errorf("controller: no key points in direction")
	}
	machine := NewMachine(direction.KeyPoints)
	action := machine.Start()
	if action.Type != AskInitial {
		return nil, fmt.Errorf("controller: expected first question, got %v", action.Type)
	}
	kc := s.nodes.knowledgeContext(ctx, direction.Topic, action.Knowledge)
	q, err := s.nodes.questioner.GenerateInitial(ctx, action.Knowledge, kc)
	if err != nil {
		return nil, fmt.Errorf("ask initial: %w", err)
	}
	sess.Questions = append(sess.Questions, q)
	if err := s.nodes.store.SaveSession(sess); err != nil {
		return nil, fmt.Errorf("persist session: %w", err)
	}
	return sess, nil
}

// SubmitAnswer 提交一轮回答并推进：追加回答与评分、状态机流转、出下一题或触发复盘。
func (s *Service) SubmitAnswer(ctx context.Context, sessionID, answer string) (StepResult, error) {
	sess, err := s.nodes.store.GetSession(sessionID)
	if err != nil {
		return StepResult{}, fmt.Errorf("load session: %w", err)
	}
	if sess.Status != model.SessionOngoing {
		return StepResult{}, fmt.Errorf("session %s is not ongoing (status=%s)", sessionID, sess.Status)
	}
	if len(sess.Questions) == 0 {
		return StepResult{}, fmt.Errorf("session %s has no questions", sessionID)
	}

	// 状态机恢复：从会话推导当前位置
	machine := restoreMachine(sess.Direction.KeyPoints, sess.Questions)
	lastQ := sess.Questions[len(sess.Questions)-1]

	// 判定（qaHistory = 最后一道题之前的所有问答对）
	qa := rebuildQA(sess)
	var judge agents.InterviewJudge
	if len(qa) > 0 {
		judge, err = s.nodes.interviewer.JudgeAnswer(ctx, lastQ, answer, qa[:len(qa)-1])
	} else {
		judge, err = s.nodes.interviewer.JudgeAnswer(ctx, lastQ, answer, nil)
	}
	if err != nil {
		return StepResult{}, fmt.Errorf("judge: %w", err)
	}
	sess.Answers = append(sess.Answers, model.Answer{QuestionID: lastQ.ID, Text: answer, Score: judge.Score})
	if err := s.nodes.store.SaveSession(sess); err != nil {
		return StepResult{}, fmt.Errorf("persist answer: %w", err)
	}

	// 状态机推进
	action := machine.OnAnswer(lastQ, answer, judge)
	switch action.Type {
	case AskInitial:
		kc := s.nodes.knowledgeContext(ctx, sess.Direction.Topic, action.Knowledge)
		q, err := s.nodes.questioner.GenerateInitial(ctx, action.Knowledge, kc)
		if err != nil {
			return StepResult{}, fmt.Errorf("ask initial: %w", err)
		}
		sess.Questions = append(sess.Questions, q)
		if err := s.nodes.store.SaveSession(sess); err != nil {
			return StepResult{}, fmt.Errorf("persist next question: %w", err)
		}
		return StepResult{NextQuestion: &q}, nil

	case AskFollowUp:
		q, err := s.nodes.questioner.GenerateFollowUp(ctx, action.PrevQ, action.Answer, action.Round)
		if err != nil {
			return StepResult{}, fmt.Errorf("ask follow-up: %w", err)
		}
		sess.Questions = append(sess.Questions, q)
		if err := s.nodes.store.SaveSession(sess); err != nil {
			return StepResult{}, fmt.Errorf("persist follow-up: %w", err)
		}
		return StepResult{NextQuestion: &q}, nil

	default: // InterviewDone：进入复盘
		reviewed, err := s.nodes.runReview(ctx, sess)
		if err != nil {
			return StepResult{}, fmt.Errorf("review: %w", err)
		}
		return StepResult{Review: reviewed}, nil
	}
}

// Finish 提前结束面试：丢弃未答的最后一道题，对已答题目生成复盘并沉淀画像。
func (s *Service) Finish(ctx context.Context, sessionID string) (*model.TrainingSession, error) {
	sess, err := s.nodes.store.GetSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	if sess.Status != model.SessionOngoing {
		return nil, fmt.Errorf("session %s is not ongoing (status=%s)", sessionID, sess.Status)
	}
	// 出题但未回答的题不参与复盘
	for len(sess.Questions) > len(sess.Answers) {
		sess.Questions = sess.Questions[:len(sess.Questions)-1]
	}
	if len(sess.Answers) == 0 {
		return nil, fmt.Errorf("session %s 还没有任何已回答的题目，无法复盘", sessionID)
	}
	reviewed, err := s.nodes.runReview(ctx, sess)
	if err != nil {
		return nil, fmt.Errorf("review: %w", err)
	}
	return reviewed, nil
}

// restoreMachine 从会话题目序列恢复状态机位置。
// kpIdx = 最后一题知识点在方向中的位置；round = 最后一题的追问轮数。
func restoreMachine(keyPoints []string, qs []model.Question) *Machine {
	m := NewMachine(keyPoints)
	if len(qs) == 0 {
		return m
	}
	last := qs[len(qs)-1]
	for i, kp := range keyPoints {
		if kp == last.KnowledgePt {
			m.kpIdx = i
			break
		}
	}
	m.round = last.Round
	return m
}

// rebuildQA 从会话重建全部问答对（题 + 对应回答）。
func rebuildQA(sess *model.TrainingSession) []agents.QAPair {
	answerByQ := map[string]string{}
	for _, a := range sess.Answers {
		answerByQ[a.QuestionID] = a.Text
	}
	pairs := make([]agents.QAPair, 0, len(sess.Questions))
	for _, q := range sess.Questions {
		if ans, ok := answerByQ[q.ID]; ok {
			pairs = append(pairs, agents.QAPair{Question: q, Answer: ans})
		}
	}
	return pairs
}
