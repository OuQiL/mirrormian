package mcp

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestParseReposSearchFormat(t *testing.T) {
	// GitHub API 搜索格式：{"items": [...]}
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Text: `{"items": [
				{"full_name": "owner/repo1", "html_url": "https://github.com/owner/repo1", "description": "desc1", "stargazers_count": 123},
				{"full_name": "owner/repo2", "html_url": "https://github.com/owner/repo2", "description": "desc2", "stargazers_count": 45}
			]}`},
		},
	}
	repos, err := parseRepos(result)
	if err != nil {
		t.Fatalf("parseRepos: %v", err)
	}
	if len(repos) != 2 || repos[0].Name != "owner/repo1" || repos[0].Stars != 123 {
		t.Fatalf("repos = %+v", repos)
	}
}

func TestParseReposArrayFormat(t *testing.T) {
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Text: `[{"full_name": "a/b", "html_url": "https://github.com/a/b", "description": "d", "stargazers_count": 7}]`},
		},
	}
	repos, err := parseRepos(result)
	if err != nil || len(repos) != 1 || repos[0].URL != "https://github.com/a/b" {
		t.Fatalf("repos = %+v err=%v", repos, err)
	}
}

func TestParseReposInvalid(t *testing.T) {
	result := &mcp.CallToolResult{
		Content: []mcp.Content{mcp.TextContent{Text: "not json at all"}},
	}
	if _, err := parseRepos(result); err == nil {
		t.Fatal("expected error for invalid content")
	}
}

func TestNewGitHubSearcherNoToken(t *testing.T) {
	_, err := NewGitHubSearcher("")
	if err == nil {
		t.Fatal("expected error without token")
	}
}
