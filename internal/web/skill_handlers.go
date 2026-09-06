// 技能接口（Web 交互层，无业务逻辑）。
package web

import (
	"context"
	"net/http"
	"time"

	"mirror-mian/internal/skill"
)

// handleSkillList 技能列表。
func (s *Server) handleSkillList(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	type item struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	out := make([]item, 0)
	for _, sk := range uc.skills.List() {
		out = append(out, item{Name: sk.Name(), Description: sk.Description()})
	}
	writeJSON(w, map[string]any{"skills": out})
}

// handleSkillHistory 当前用户的聊天历史（按最近更新倒序，含完整对话转写）。
func (s *Server) handleSkillHistory(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	list, err := uc.store.ListSkillSessions()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	type item struct {
		ID        string   `json:"id"`
		SkillName string   `json:"skill_name"`
		Messages  []string `json:"messages"`
		Finished  bool     `json:"finished"`
		UpdatedAt string   `json:"updated_at"`
	}
	out := make([]item, 0, len(list))
	for _, sess := range list {
		out = append(out, item{
			ID: sess.ID, SkillName: sess.SkillName,
			Messages: sess.Messages, Finished: sess.Finished,
			UpdatedAt: sess.UpdatedAt.Format("2006-01-02 15:04"),
		})
	}
	writeJSON(w, map[string]any{"sessions": out})
}

// handleSkillStart 启动技能（name 指定 或 input 匹配）。
func (s *Server) handleSkillStart(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	var req struct {
		Name  string `json:"name"`
		Input string `json:"input"`
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	var sess *skill.Session
	var res *skill.TurnResult
	var err error
	if req.Name != "" {
		sess, res, err = uc.skills.StartSkill(ctx, req.Name, req.Input)
	} else {
		sess, res, err = uc.skills.StartMatched(ctx, req.Input)
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"session_id": sess.ID, "skill": sess.SkillName, "reply": res.Reply, "finished": res.Finished})
}

// handleSkillTurn 推进技能会话。
func (s *Server) handleSkillTurn(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	var req struct {
		SessionID string `json:"session_id"`
		Input     string `json:"input"`
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.SessionID == "" || req.Input == "" {
		writeErr(w, http.StatusBadRequest, "session_id 与 input 必填")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	res, err := uc.skills.Turn(ctx, req.SessionID, req.Input)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"reply": res.Reply, "finished": res.Finished})
}
