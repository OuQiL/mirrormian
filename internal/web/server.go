// Web 交互层：HTTP 服务与页面。
// 只做参数解析、调用编排层、展示输出——不含业务逻辑（spec「无业务逻辑的交互层」）。
//
// 多用户隔离：每个请求经鉴权解析出 userID，懒加载该用户的隔离服务包
// （按用户 Store + 按用户 kb/与 resumes/ 目录），确保各账户数据与上传资料互不可见。
package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"mirror-mian/internal/config"
	"mirror-mian/internal/domain"
	"mirror-mian/internal/embedding"
	"mirror-mian/internal/llm"
	"mirror-mian/internal/model"
	"mirror-mian/internal/orchestration"
	"mirror-mian/internal/profile"
	"mirror-mian/internal/resume"
	"mirror-mian/internal/skill"
	"mirror-mian/internal/store"
	"mirror-mian/internal/vector"
)

//go:embed static
var staticFS embed.FS

// warmupPending 综合面试暖场：后台准备结果暂存（token → 结果通道）。
type warmupPending struct {
	mu      sync.Mutex
	pending map[string]chan warmupResult
}

type warmupResult struct {
	sess *model.TrainingSession
	err  error
}

// userCtx 某用户专属的服务包（懒加载并缓存，按用户隔离）。
type userCtx struct {
	store  store.Store
	svc    *orchestration.Service
	domain *domain.Service
	resume *resume.Service
	skills *skill.Registry
}

// tokenStore 登录态：token → userID（持久化到磁盘，服务重启后仍有效）。
type tokenStore struct {
	mu   sync.Mutex
	m    map[string]string
	path string // 持久化文件路径，空则不持久化
}

// Server Web 服务。
type Server struct {
	cfg    *config.Config
	base   store.Store       // 共享库：账户管理 + ForUser 派生
	llm    llm.Client
	emb    embedding.Embedder
	milvus *vector.Milvus
	// 每个用户的隔离服务包
	mu      sync.Mutex
	users   map[string]*userCtx
	tokens  *tokenStore
	warmups warmupPending
	timeout time.Duration
}

// NewServer 创建 Web 服务。st 传入共享库（账户表 + 派生按用户 Store）。
func NewServer(cfg *config.Config, st store.Store) *Server {
	llmClient, _ := llm.New(cfg) // 生成核心梳理用；未配置时为 nil
	tokens := &tokenStore{
		m:    map[string]string{},
		path: filepath.Join(filepath.Dir(cfg.DBPath), "tokens.json"),
	}
	tokens.load() // 恢复上次登录态，服务重启不失效
	return &Server{
		cfg: cfg, base: st, llm: llmClient,
		emb:    embedding.New(cfg),
		milvus: vector.New(cfg.MilvusAddr),
		users:  map[string]*userCtx{},
		tokens: tokens,
		warmups: warmupPending{pending: map[string]chan warmupResult{}},
		timeout: 3 * time.Minute,
	}
}

// userFor 获取某用户的隔离服务包，未加载则懒构建并缓存。
func (s *Server) userFor(userID string) *userCtx {
	if userID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if uc, ok := s.users[userID]; ok {
		return uc
	}
	userStore := s.base.ForUser(userID)
	kbRoot := filepath.Join(s.cfg.KBPath, userID)
	res := resume.NewService(userStore)
	svc := orchestration.NewService(s.llm, userStore, s.emb, kbRoot)
	svc.SetMilvus(s.milvus)

	dom := domain.NewService(kbRoot, userStore, s.emb, s.llm)
	dom.SetMilvus(s.milvus)

	reg := skill.NewRegistry(userStore)
	if s.llm != nil {
		reg.Register(
			skill.NewQuickQuiz(s.llm, userStore),
			skill.NewConceptTutor(s.llm),
			skill.NewReviewSkill(s.llm, userStore),
		)
		// 未命中任何技能时兜底为大模型调用（普通对话，不注册为技能）
		reg.SetFallback(skill.NewChat(s.llm))
	}

	uc := &userCtx{store: userStore, svc: svc, domain: dom, resume: res, skills: reg}
	s.users[userID] = uc
	return uc
}

// Handler 返回根 mux。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// 静态页面（embed）
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	// 账户（公开）
	mux.HandleFunc("/api/auth/register", s.handleRegister)
	mux.HandleFunc("/api/auth/login", s.handleLogin)
	mux.HandleFunc("/api/auth/logout", s.handleLogout)
	mux.HandleFunc("/api/auth/me", s.requireAuth(s.handleMe))

	// 设置（全局机器配置，公开）
	mux.HandleFunc("/api/settings", s.handleSettingsGet)
	mux.HandleFunc("/api/settings/save", s.handleSettingsSave)
	mux.HandleFunc("/api/settings/test", s.handleSettingsTest)

	// 受保护接口（用户数据，需登录）
	p := s.requireAuth
	mux.HandleFunc("/api/interview/start", p(s.handleStart))
	mux.HandleFunc("/api/interview/answer", p(s.handleAnswer))
	mux.HandleFunc("/api/interview/finish", p(s.handleFinish))
	mux.HandleFunc("/api/interview/warmup-complete", p(s.handleWarmupComplete))
	mux.HandleFunc("/api/profile", p(s.handleProfile))
	mux.HandleFunc("/api/review", p(s.handleReview))
	mux.HandleFunc("/api/kb", p(s.handleKB))

	// 领域管理
	mux.HandleFunc("/api/domain", p(s.handleDomainList))
	mux.HandleFunc("/api/domain/create", p(s.handleDomainCreate))
	mux.HandleFunc("/api/domain/delete", p(s.handleDomainDelete))
	mux.HandleFunc("/api/domain/rename", p(s.handleDomainRename))
	mux.HandleFunc("/api/domain/detail", p(s.handleDomainDetail))
	mux.HandleFunc("/api/domain/save-file", p(s.handleDomainSaveFile))
	mux.HandleFunc("/api/domain/sync", p(s.handleDomainSync))
	mux.HandleFunc("/api/domain/generate-core", p(s.handleDomainGenerateCore))
	mux.HandleFunc("/api/resources", p(s.handleResources))
	mux.HandleFunc("/api/plan", p(s.handlePlan))
	// 历史面试记录
	mux.HandleFunc("/api/history", p(s.handleHistoryList))
	mux.HandleFunc("/api/history/detail", p(s.handleHistoryDetail))

	// 简历管理
	mux.HandleFunc("/api/resume", p(s.handleResumeList))
	mux.HandleFunc("/api/resume/", p(s.handleResumeDetail))
	mux.HandleFunc("/api/resume/upload", p(s.handleResumeUpload))
	mux.HandleFunc("/api/resume/text", p(s.handleResumeText))
	mux.HandleFunc("/api/resume/delete", p(s.handleResumeDelete))

	// 技能系统
	mux.HandleFunc("/api/skill", p(s.handleSkillList))
	mux.HandleFunc("/api/skill/start", p(s.handleSkillStart))
	mux.HandleFunc("/api/skill/turn", p(s.handleSkillTurn))
	mux.HandleFunc("/api/skill/history", p(s.handleSkillHistory))
	return logRequests(mux)
}

// Start 启动 HTTP 服务（阻塞）。
func (s *Server) Start(addr string) error {
	log.Printf("mirror-mian web: http://localhost%s", addr)
	return http.ListenAndServe(addr, s.Handler())
}

// --- handlers ---

type startReq struct {
	Mode     string `json:"mode"` // special / full
	Topic    string `json:"topic"`
	JD       string `json:"jd"`
	Resume   string `json:"resume"`
	ResumeID string `json:"resume_id"`
	Warmup   bool   `json:"warmup"` // 综合面试暖场模式
}

type questionDTO struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	KnowledgePt string `json:"knowledge_point"`
	Round       int    `json:"round"`
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	// LLM 未配置时给出明确提示（配置在设置页完成）
	if s.cfg.LLMAPIKey == "" {
		writeErr(w, http.StatusBadRequest, "LLM 未配置——请先到「设置」页填写 API Key 并保存（保存后需重启服务生效）")
		return
	}
	var req startReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	var sessID string
	var first model.Question
	var direction model.Direction
	switch req.Mode {
	case "special":
		if req.Topic == "" {
			writeErr(w, http.StatusBadRequest, "专项面试需要 topic 参数")
			return
		}
		sess, err := uc.svc.StartSpecial(ctx, req.Topic)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		sessID, first, direction = sess.ID, sess.Questions[len(sess.Questions)-1], sess.Direction
	case "full":
		if req.JD == "" {
			writeErr(w, http.StatusBadRequest, "综合面试需要 jd 参数")
			return
		}
		resumeText := req.Resume
		if req.ResumeID != "" {
			// 从当前用户的简历库取解析文本
			r, err := uc.resume.Get(req.ResumeID)
			if err != nil {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
			resumeText = r.Text
		}
		if req.Warmup {
			// 暖场模式：后台并行准备（匹配分析+方向+第一题），返回 token
			token, err := s.warmupStart(uc, resumeText, req.JD)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, map[string]any{
				"warmup":   true,
				"token":    token,
				"question": "你想要什么样的面试？",
			})
			return
		}
		sess, err := uc.svc.StartFull(ctx, resumeText, req.JD)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		sessID, first, direction = sess.ID, sess.Questions[len(sess.Questions)-1], sess.Direction
	default:
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("未知 mode %q（支持 special / full）", req.Mode))
		return
	}

	writeJSON(w, map[string]any{
		"session_id": sessID,
		"question":   questionDTO{ID: first.ID, Text: first.Text, KnowledgePt: first.KnowledgePt, Round: first.Round},
		"mode":       req.Mode,
		"topic":      req.Topic,
		"direction": directionDTO{
			KeyPoints:     direction.KeyPoints,
			JDSummary:     direction.JDSummary,
			ResumeSummary: direction.ResumeSummary,
		},
	})
}

// directionDTO 出题方向（准备阶段产物，用于前端状态栏展示）。
type directionDTO struct {
	KeyPoints     []string `json:"key_points"`
	JDSummary     string   `json:"jd_summary,omitempty"`
	ResumeSummary string   `json:"resume_summary,omitempty"`
}

// finishReq 提前结束面试请求。
type finishReq struct {
	SessionID string `json:"session_id"`
}

func (s *Server) handleFinish(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	var req finishReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.SessionID == "" {
		writeErr(w, http.StatusBadRequest, "session_id 必填")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	sess, err := uc.svc.Finish(ctx, req.SessionID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{
		"done":      true,
		"review_md": sess.Review,
		"session":   sess.ID,
	})
}

type answerReq struct {
	SessionID string `json:"session_id"`
	Answer    string `json:"answer"`
}

func (s *Server) handleAnswer(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	var req answerReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.SessionID == "" || req.Answer == "" {
		writeErr(w, http.StatusBadRequest, "session_id 与 answer 必填")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	res, err := uc.svc.SubmitAnswer(ctx, req.SessionID, req.Answer)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if res.NextQuestion != nil {
		writeJSON(w, map[string]any{
			"next_question": questionDTO{
				ID: res.NextQuestion.ID, Text: res.NextQuestion.Text,
				KnowledgePt: res.NextQuestion.KnowledgePt, Round: res.NextQuestion.Round,
			},
		})
		return
	}
	// 面试结束：复盘
	writeJSON(w, map[string]any{
		"done":      true,
		"review_md": res.Review.Review,
		"session":   res.Review.ID,
	})
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	p, err := uc.store.GetProfile()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, p)
}

func (s *Server) handleReview(w http.ResponseWriter, r *http.Request) {
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
	type dueItem struct {
		ID         string  `json:"id"`
		Topic      string  `json:"topic"`
		Point      string  `json:"point"`
		TimesSeen  int     `json:"times_seen"`
		EaseFactor float64 `json:"ease_factor"`
		NextReview string  `json:"next_review"`
	}
	items := make([]dueItem, 0, len(due))
	for _, w := range due {
		items = append(items, dueItem{
			ID: w.ID, Topic: w.Topic, Point: w.Point,
			TimesSeen: w.TimesSeen, EaseFactor: w.SR.EaseFactor, NextReview: w.SR.NextReview,
		})
	}
	writeJSON(w, map[string]any{"due": items})
}

// handleKB 知识库状态（各主题块数）。
func (s *Server) handleKB(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	topics, err := uc.store.ListAllTopics()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"topics": topics})
}

// --- 历史面试记录 ---

// historyItem 历史面试列表项摘要。
type historyItem struct {
	ID           string   `json:"id"`
	Mode         string   `json:"mode"`
	Status       string   `json:"status"`
	Topic        string   `json:"topic"`
	KeyPoints    []string `json:"key_points"`
	QuestionCnt  int      `json:"question_count"`
	AvgScore     *float64 `json:"avg_score"`
	Reviewed     bool     `json:"reviewed"`
	CreatedAt    string   `json:"created_at"`
	FinishedAt   string   `json:"finished_at"`
}

func (s *Server) handleHistoryList(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	sessions, err := uc.store.ListSessions()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]historyItem, 0, len(sessions))
	for i := range sessions {
		sess := &sessions[i]
		item := historyItem{
			ID:          sess.ID,
			Mode:        sess.Mode,
			Status:      sess.Status,
			Topic:       sess.Direction.Topic,
			KeyPoints:   sess.Direction.KeyPoints,
			QuestionCnt: len(sess.Questions),
			Reviewed:    sess.Review != "",
			CreatedAt:   sess.CreatedAt.Format(time.RFC3339),
		}
		if !sess.FinishedAt.IsZero() {
			item.FinishedAt = sess.FinishedAt.Format(time.RFC3339)
		}
		// 平均得分（已评分的回答，无则略过）
		if len(sess.Answers) > 0 {
			var sum float64
			var n int
			for _, a := range sess.Answers {
				if a.Score > 0 {
					sum += a.Score
					n++
				}
			}
			if n > 0 {
				v := sum / float64(n)
				avg := math.Round(v*10) / 10
				item.AvgScore = &avg
			}
		}
		items = append(items, item)
	}
	writeJSON(w, map[string]any{"items": items})
}

type historyDetailReq struct {
	ID string `json:"id"`
}

func (s *Server) handleHistoryDetail(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	var req historyDetailReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ID == "" {
		writeErr(w, http.StatusBadRequest, "id 必填")
		return
	}
	sess, err := uc.store.GetSession(req.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{
		"id":            sess.ID,
		"mode":          sess.Mode,
		"status":        sess.Status,
		"direction":     sess.Direction,
		"questions":     sess.Questions,
		"answers":       sess.Answers,
		"review":        sess.Review,
		"created_at":    sess.CreatedAt,
		"finished_at":   sess.FinishedAt,
	})
}

// --- 工具 ---

func decodeBody(r *http.Request, out any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("无效的 JSON 请求: %v", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start))
	})
}