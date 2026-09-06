// MCP 工具结果解析（对齐蓝本：兼容 GitHub API search 与裸数组两种格式）。
package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// repoItem GitHub 仓库条目的最小字段集。
type repoItem struct {
	FullName        string `json:"full_name"`
	HTMLURL         string `json:"html_url"`
	Description     string `json:"description"`
	StargazersCount int    `json:"stargazers_count"`
}

// parseRepos 解析 CallTool 返回的文本内容为仓库列表。
func parseRepos(result *mcp.CallToolResult) ([]RepoInfo, error) {
	var repos []RepoInfo
	for _, c := range result.Content {
		tc, ok := c.(mcp.TextContent)
		if !ok {
			continue
		}
		// 标准 GitHub API 搜索格式：{"items": [...]}
		var searchResult struct {
			Items []repoItem `json:"items"`
		}
		if err := json.Unmarshal([]byte(tc.Text), &searchResult); err == nil && len(searchResult.Items) > 0 {
			repos = append(repos, toRepos(searchResult.Items)...)
			continue
		}
		// 裸数组格式：[{...}]
		var items []repoItem
		if err := json.Unmarshal([]byte(tc.Text), &items); err == nil {
			repos = append(repos, toRepos(items)...)
			continue
		}
		return nil, fmt.Errorf("无法解析 MCP 返回内容: %.200s", tc.Text)
	}
	return repos, nil
}

func toRepos(items []repoItem) []RepoInfo {
	out := make([]RepoInfo, 0, len(items))
	for _, it := range items {
		out = append(out, RepoInfo{
			Name:  it.FullName,
			URL:   it.HTMLURL,
			Stars: it.StargazersCount,
			Desc:  it.Description,
		})
	}
	return out
}
