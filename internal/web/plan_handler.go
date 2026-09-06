// 复习计划接口（Web 交互层，无业务逻辑）。
package web

import (
	"context"
	"net/http"
	"time"

	"mirror-mian/internal/agents"
	"mirror-mian/internal/llm"
	"mirror-mian/internal/profile"
)

// handlePlan 基于画像生成复习计划。
func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	p, err := uc.store.GetProfile()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	due := profile.DueReviews(p.WeakPoints, time.Now())
	if len(p.WeakPoints) == 0 && len(p.Mastery) == 0 {
		writeJSON(w, map[string]any{"summary": "画像为空——先完成几场面试积累数据，再生成复习计划。", "items": []any{}})
		return
	}
	llmClient, err := llm.New(s.cfg)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	planner := agents.NewReviewPlanner(llmClient)
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	plan, err := planner.Plan(ctx, p, len(due))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"summary": plan.Summary, "items": plan.Items})
}
