// 综合面试暖场：开始后先问期望 → 引导自我介绍，期间后台并行执行准备阶段
// （匹配分析 + 方向规划 + 出题），自我介绍完成后取回第一题，提升体验感。
package web

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// handleWarmupStart 综合面试暖场启动：后台准备 + 返回暖场提示。
// 由 handleStart 的 full 分支调用（warmup=true 时）。
func (s *Server) warmupStart(uc *userCtx, resumeText, jdText string) (string, error) {
	token := uuid.NewString()
	ch := make(chan warmupResult, 1)

	go func() {
		// 后台并行执行完整准备：匹配分析 + 方向规划 + 第一题。
		// 使用独立 context（请求 ctx 在 start 返回后即被取消）。
		ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
		defer cancel()
		sess, err := uc.svc.StartFull(ctx, resumeText, jdText)
		ch <- warmupResult{sess: sess, err: err}
	}()

	s.warmups.mu.Lock()
	s.warmups.pending[token] = ch
	s.warmups.mu.Unlock()
	return token, nil
}

// handleWarmupComplete 自我介绍完成后取回准备结果（等待后台准备完成）。
func (s *Server) handleWarmupComplete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		Expectation string `json:"expectation"` // 用户期望（体验用途，不注入方向）
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Token == "" {
		writeErr(w, http.StatusBadRequest, "缺少 token")
		return
	}

	s.warmups.mu.Lock()
	ch, ok := s.warmups.pending[req.Token]
	if ok {
		delete(s.warmups.pending, req.Token)
	}
	s.warmups.mu.Unlock()
	if !ok {
		writeErr(w, http.StatusBadRequest, "暖场 token 无效或已过期")
		return
	}

	select {
	case res := <-ch:
		if res.err != nil {
			writeErr(w, http.StatusInternalServerError, res.err.Error())
			return
		}
		sess := res.sess
		if len(sess.Questions) == 0 {
			writeErr(w, http.StatusInternalServerError, "准备未产出题目")
			return
		}
		first := sess.Questions[len(sess.Questions)-1]
		writeJSON(w, map[string]any{
			"session_id": sess.ID,
			"question": questionDTO{
				ID: first.ID, Text: first.Text,
				KnowledgePt: first.KnowledgePt, Round: first.Round,
			},
			"mode": sess.Mode,
		})
	case <-time.After(3 * time.Minute):
		writeErr(w, http.StatusGatewayTimeout, "准备工作超时，请重试")
	}
}
