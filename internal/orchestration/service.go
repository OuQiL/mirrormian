// 面试服务：编排层对交互层暴露的入口。
//
// 三阶段流程：准备（direction_planner）→ 面试（状态机 + questioner + interviewer）→
// 复盘（reviewer）→ 画像沉淀（薄弱点 + 掌握度 EMA）→ 会话持久化。
package orchestration

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"mirror-mian/internal/agents"
	"mirror-mian/internal/domain"
	"mirror-mian/internal/embedding"
	"mirror-mian/internal/llm"
	"mirror-mian/internal/model"
	"mirror-mian/internal/profile"
	"mirror-mian/internal/rag"
	"mirror-mian/internal/store"
	"mirror-mian/internal/vector"
)

// AskFunc 交互层注入的回答回调：展示题目、读取用户回答。
type AskFunc func(q model.Question) (string, error)

// Service 面试编排服务。
type Service struct {
	nodes *graphNodes
	// RAG：出题注入知识库上下文（Embedder 不可用时降级为空上下文）
	rag *rag.Service
}

// NewService 创建编排服务（Agent 协作与持久化由本层组合）。
// emb 可空：nil 或不可用时，知识库检索与向量去重自动降级。
// kbRoot 为领域内容目录（kb/<topic>/）；为空时高频题注入降级为无。
func NewService(llm llm.Client, st store.Store, emb embedding.Embedder, kbRoot string) *Service {
	ragSvc := rag.NewService(emb, st)
	// LLM Rerank：RAG_RERANKER=none 时禁用；默认 llm 可用时启用
	rerankEnabled := os.Getenv("RAG_RERANKER") != "none"
	ragSvc.SetReranker(rag.NewReranker(llm, rerankEnabled))

	return &Service{
		nodes: &graphNodes{
			planner:     agents.NewDirectionPlanner(llm),
			questioner:  agents.NewQuestioner(llm),
			interviewer: agents.NewInterviewer(llm),
			reviewer:    agents.NewReviewer(llm),
			store:       st,
			rag:         ragSvc,
			domain:      domain.NewService(kbRoot, st, emb, llm),
		},
		rag: ragSvc,
	}
}

// Embedder 返回编排层持有的 Embedder（供画像沉淀向量去重用）。
func (s *Service) Embedder() embedding.Embedder { return s.rag.Embedder() }

// SetMilvus 注入 Milvus：向量检索路与领域同步双写。
func (s *Service) SetMilvus(m *vector.Milvus) {
	if m == nil {
		return
	}
	s.rag.SetVectorStore(m)
	s.nodes.rag.SetVectorStore(m)
	s.nodes.domain.SetMilvus(m)
}

// RunSpecial 专项面试：主题驱动的三阶段流程（Eino Graph 编排）。
func (s *Service) RunSpecial(ctx context.Context, topic string, ask AskFunc) (*model.TrainingSession, error) {
	return s.runWithGraph(ctx, graphInput{mode: model.ModeSpecial, topic: topic, ask: ask})
}

// RunFull 综合面试：简历/JD 驱动的三阶段流程（Eino Graph 编排）。
func (s *Service) RunFull(ctx context.Context, resumeText, jdText string, ask AskFunc) (*model.TrainingSession, error) {
	return s.runWithGraph(ctx, graphInput{mode: model.ModeFull, resume: resumeText, jd: jdText, ask: ask})
}

// runInterview 面试阶段：状态机驱动逐题问答（Graph 的 interview 节点实现）。
func (ns *graphNodes) runInterview(ctx context.Context, direction model.Direction, ask AskFunc) (*model.TrainingSession, error) {
	sess := &model.TrainingSession{
		ID:        uuid.NewString(),
		Mode:      direction.Mode,
		Status:    model.SessionOngoing,
		Direction: direction,
		CreatedAt: time.Now(),
	}
	if err := ns.store.SaveSession(sess); err != nil {
		return nil, fmt.Errorf("persist session: %w", err)
	}

	machine := NewMachine(direction.KeyPoints)
	qa := make([]agents.QAPair, 0, 8)

	action := machine.Start()
	for action.Type != InterviewDone {
		if ctx.Err() != nil {
			return sess, fmt.Errorf("interview cancelled: %w", ctx.Err())
		}
		var q model.Question
		var err error
		switch action.Type {
		case AskInitial:
			kc := ns.knowledgeContext(ctx, direction.Topic, action.Knowledge)
			q, err = ns.questioner.GenerateInitial(ctx, action.Knowledge, kc)
			if err != nil {
				return sess, fmt.Errorf("ask initial: %w", err)
			}
		case AskFollowUp:
			q, err = ns.questioner.GenerateFollowUp(ctx, action.PrevQ, action.Answer, action.Round)
			if err != nil {
				return sess, fmt.Errorf("ask follow-up: %w", err)
			}
		}
		sess.Questions = append(sess.Questions, q)

		answer, err := ask(q)
		if err != nil {
			return sess, err
		}
		qa = append(qa, agents.QAPair{Question: q, Answer: answer})

		j, err := ns.interviewer.JudgeAnswer(ctx, q, answer, qa[:len(qa)-1])
		if err != nil {
			return sess, fmt.Errorf("judge: %w", err)
		}
		sess.Answers = append(sess.Answers, model.Answer{QuestionID: q.ID, Text: answer, Score: j.Score})
		_ = ns.store.SaveSession(sess)
		action = machine.OnAnswer(q, answer, j)
	}
	return sess, nil
}

// runReview 复盘阶段：彻底复盘（Markdown + 结构化 JSON 双份）+ 画像沉淀 + 最终持久化。
func (ns *graphNodes) runReview(ctx context.Context, sess *model.TrainingSession) (*model.TrainingSession, error) {
	sess.Status = model.SessionFinished
	sess.FinishedAt = time.Now()
	reviewMD, report, err := ns.reviewer.Review(ctx, sess)
	if err != nil {
		return sess, fmt.Errorf("review: %w", err)
	}
	sess.Review = reviewMD
	if err := ns.absorb(sess, report); err != nil {
		return sess, fmt.Errorf("absorb profile: %w", err)
	}
	if err := ns.store.SaveSession(sess); err != nil {
		return sess, fmt.Errorf("persist final session: %w", err)
	}
	return sess, nil
}

// absorb 将复盘结果沉淀进画像：薄弱点（含逐题低分）与掌握度 EMA。
func (ns *graphNodes) absorb(sess *model.TrainingSession, report model.ReviewReport) error {
	p, err := ns.store.GetProfile()
	if err != nil {
		return err
	}
	if p.WeakPoints == nil {
		p.WeakPoints = []model.WeakPoint{}
	}
	if p.Mastery == nil {
		p.Mastery = map[string]*model.Mastery{}
	}
	today := time.Now()

	// 1) 逐题低分沉淀为薄弱点（知识点为薄弱点文本）
	for _, a := range sess.Answers {
		if a.Score >= profile.WeaknessThreshold {
			continue
		}
		kp := knowledgePointOf(sess, a.QuestionID)
		profile.AbsorbAnswer(p, topicOf(sess), kp, a.Score, today, ns.rag.Embedder())
	}
	// 2) 复盘报告的薄弱点（统一以低分吸收）
	for _, wp := range report.WeakPoints {
		if wp == "" {
			continue
		}
		profile.AbsorbAnswer(p, topicOf(sess), wp, 4, today, ns.rag.Embedder())
	}
	// 3) 掌握度 EMA
	m := p.Mastery[topicOf(sess)]
	if m == nil {
		m = &model.Mastery{Topic: topicOf(sess)}
		p.Mastery[topicOf(sess)] = m
	}
	profile.UpdateMastery(m, profile.SessionScore(sess.Answers), today)

	return ns.store.SaveProfile(p)
}

func knowledgePointOf(sess *model.TrainingSession, qid string) string {
	for _, q := range sess.Questions {
		if q.ID == qid {
			return q.KnowledgePt
		}
	}
	return "未归类"
}

func topicOf(sess *model.TrainingSession) string {
	if sess.Direction.Topic != "" {
		return sess.Direction.Topic
	}
	return "综合面试"
}
