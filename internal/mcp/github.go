// GitHub 项目搜索：基于 GitHub MCP Server（stdio），移植自 interview-agent 蓝本。
package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// RepoInfo GitHub 仓库信息。
type RepoInfo struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Stars int    `json:"stars"`
	Desc  string `json:"desc"`
}

// GitHubSearcher GitHub 项目搜索器。
type GitHubSearcher struct {
	client *client.Client
	mu     sync.Mutex
}

// NewGitHubSearcher 启动 GitHub MCP Server（stdio）。token 为空时返回可读错误。
func NewGitHubSearcher(token string) (*GitHubSearcher, error) {
	if token == "" {
		return nil, fmt.Errorf("mcp/github: GITHUB_TOKEN 未配置（GitHub 资源推荐不可用）")
	}
	env := []string{fmt.Sprintf("GITHUB_PERSONAL_ACCESS_TOKEN=%s", token)}
	cli, err := client.NewStdioMCPClient("npx", env, "@modelcontextprotocol/server-github")
	if err != nil {
		return nil, fmt.Errorf("mcp/github: 启动 GitHub MCP Server 失败（请确认已安装 node/npx）：%w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "mirror-mian", Version: "1.0.0"}
	if _, err := cli.Initialize(ctx, initReq); err != nil {
		cli.Close()
		return nil, fmt.Errorf("mcp/github: MCP 初始化失败（首次启动需 npx 下载，可能较慢）：%w", err)
	}
	return &GitHubSearcher{client: cli}, nil
}

// SearchRepos 搜索 GitHub 开源项目。
// query: GitHub 搜索语法，如 "go interview questions stars:>100"；limit: 结果上限。
func (gs *GitHubSearcher) SearchRepos(ctx context.Context, query string, limit int) ([]RepoInfo, error) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	if limit <= 0 {
		limit = 5
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = "search_repositories"
	req.Params.Arguments = map[string]any{"query": query, "perPage": limit}

	result, err := gs.client.CallTool(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("mcp/github: 搜索失败: %w", err)
	}
	if result.IsError {
		return nil, fmt.Errorf("mcp/github: 搜索错误: %v", result.Content)
	}
	repos, err := parseRepos(result)
	if err != nil {
		return nil, fmt.Errorf("mcp/github: 解析结果: %w", err)
	}
	return repos, nil
}

// Close 关闭 MCP Server（杀掉子进程）。
func (gs *GitHubSearcher) Close() error {
	if gs.client != nil {
		return gs.client.Close()
	}
	return nil
}
