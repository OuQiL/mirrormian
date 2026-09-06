// 个人账户认证：注册/登录/登出/当前用户 + 鉴权中间件。
// 登录态用 HttpOnly Cookie 持久化；各请求经鉴权解析出 userID 后访问该用户的隔离服务。
package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mirror-mian/internal/model"
)

const (
	authCookie = "mj_token"
	ctxKey     = ctxKeyID("userID")
)

type ctxKeyID string

// userIDFrom 从请求上下文取已鉴权 userID。
func userIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKey).(string); ok {
		return v
	}
	return ""
}

// ucFor 返回当前请求所属用户的隔离服务包；未登录时写 401 并返回 nil。
func (s *Server) ucFor(w http.ResponseWriter, r *http.Request) *userCtx {
	uid := userIDFrom(r.Context())
	if uid == "" {
		writeErr(w, http.StatusUnauthorized, "未登录或登录已过期")
		return nil
	}
	return s.userFor(uid)
}

// requireAuth 鉴权中间件：校验 Cookie/Header 中的登录态，将 userID 写入上下文。
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if c, err := r.Cookie(authCookie); err == nil {
			token = c.Value
		}
		if token == "" {
			if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
				token = strings.TrimPrefix(h, "Bearer ")
			}
		}
		if token == "" {
			writeErr(w, http.StatusUnauthorized, "未登录或登录已过期")
			return
		}
		uid, ok := s.tokens.get(token)
		if !ok {
			writeErr(w, http.StatusUnauthorized, "未登录或登录已过期")
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey, uid)
		next(w, r.WithContext(ctx))
	}
}

// (tokenStore) get/set/delete with mutex. 登录态持久化到磁盘，服务重启后仍有效。
func (t *tokenStore) get(token string) (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	uid, ok := t.m[token]
	return uid, ok
}

func (t *tokenStore) set(token, userID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.m[token] = userID
	t.persist()
}

func (t *tokenStore) delete(token string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.m, token)
	t.persist()
}

// load 启动时从磁盘恢复登录态；文件缺失或损坏时静默忽略。
func (t *tokenStore) load() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.path == "" {
		return
	}
	data, err := os.ReadFile(t.path)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &t.m); err != nil {
		t.m = map[string]string{}
		return
	}
	if t.m == nil {
		t.m = map[string]string{}
	}
}

// persist 将登录态原子写入磁盘（先写临时文件再重命名，避免写一半损坏）。
func (t *tokenStore) persist() {
	if t.path == "" {
		return
	}
	data, err := json.Marshal(t.m)
	if err != nil {
		return
	}
	tmp := t.path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(t.path), 0o755); err != nil {
		return
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, t.path)
}

// newToken 生成随机登录态 token。
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: authCookie, Value: token, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
		// 不设 MaxAge（会话 Cookie），关闭浏览器即失效
	})
}

func clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: authCookie, Value: "", Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

// currentUser 返回当前登录用户（未登录返回 nil）。
func (s *Server) currentUser(r *http.Request) (*userView, error) {
	uid := userIDFrom(r.Context())
	if uid == "" {
		return nil, nil
	}
	u, err := s.base.GetUserByID(uid)
	if err != nil {
		return nil, err
	}
	return &userView{ID: u.ID, Email: u.Email, Name: u.Name}, nil
}

// userView 前端可见的用户信息（不含密码/盐）。
type userView struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// --- handlers ---

// handleRegister 注册。首个注册者将接管升级前遗留（无归属）的本地历史数据。
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		writeErr(w, http.StatusBadRequest, "请输入有效的邮箱地址")
		return
	}
	if len(strings.TrimSpace(req.Password)) < 6 {
		writeErr(w, http.StatusBadRequest, "密码至少 6 位")
		return
	}

	first, err := s.base.CountUsers()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.base.CreateUser(&model.User{Email: req.Email, PasswordHash: req.Password, Name: req.Name}); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	u, err := s.base.GetUserByEmail(req.Email)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if first == 0 {
		// 首个用户接管旧库数据 + 迁移旧文件目录
		if err := s.base.AdoptLegacyData(u.ID); err != nil {
			writeErr(w, http.StatusInternalServerError, "交接历史数据失败: "+err.Error())
			return
		}
		s.adoptLegacyFiles(u.ID)
	}

	token, err := newToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.tokens.set(token, u.ID)
	setAuthCookie(w, token)
	writeJSON(w, map[string]any{"ok": true, "user": userView{ID: u.ID, Email: u.Email, Name: u.Name}})
}

// handleLogin 登录：校验邮箱密码，写入登录态。
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	u, err := s.base.GetUserByEmail(req.Email)
	if err != nil || !s.base.VerifyPassword(u, req.Password) {
		writeErr(w, http.StatusUnauthorized, "邮箱或密码错误")
		return
	}
	token, err := newToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.tokens.set(token, u.ID)
	setAuthCookie(w, token)
	writeJSON(w, map[string]any{"ok": true, "user": userView{ID: u.ID, Email: u.Email, Name: u.Name}})
}

// handleLogout 登出：清除登录态。
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(authCookie); err == nil {
		s.tokens.delete(c.Value)
	}
	clearAuthCookie(w)
	writeJSON(w, map[string]any{"ok": true})
}

// handleMe 当前登录用户。
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.currentUser(r)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "未登录")
		return
	}
	writeJSON(w, map[string]any{"user": u})
}

// adoptLegacyFiles 将旧版顶层目录（kb/<topic>）迁移到首个用户的子目录。
// 源目录不存在时静默返回。
func (s *Server) adoptLegacyFiles(userID string) {
	moveDirContents(s.cfg.KBPath, filepath.Join(s.cfg.KBPath, userID))
}

// moveDirContents 把 src 下的条目（非目标目录自身）移入 dst，跳过隐藏/系统文件。
func moveDirContents(src, dst string) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Base(e.Name()) == filepath.Base(src) {
			continue
		}
		old := filepath.Join(src, e.Name())
		// 跳过用户自身目录，避免嵌套
		if old == dst || e.Name() == filepath.Base(dst) {
			continue
		}
		if err := os.MkdirAll(dst, 0o755); err != nil {
			continue
		}
		if err := os.Rename(old, filepath.Join(dst, e.Name())); err != nil {
			fmt.Fprintln(os.Stderr, "⚠ 迁移历史文件失败:", err)
		}
	}
}