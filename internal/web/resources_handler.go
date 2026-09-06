// 复习资源推荐接口（Web 交互层，无业务逻辑）。
package web

import (
	"context"
	"net/http"
	"os"
	"time"

	"mirror-mian/internal/mcp"
)

// handleResources 按主题推荐 GitHub 学习资源。
func (s *Server) handleResources(w http.ResponseWriter, r *http.Request) {
	topic := r.URL.Query().Get("topic")
	if topic == "" {
		writeErr(w, http.StatusBadRequest, "缺少 topic 参数")
		return
	}
	token := os.Getenv("GITHUB_TOKEN")
	searcher, err := mcp.NewGitHubSearcher(token)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	defer searcher.Close()

	query := `"` + topic + `" interview questions stars:>50`
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	repos, err := searcher.SearchRepos(ctx, query, 5)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"topic": topic, "repos": repos})
}
