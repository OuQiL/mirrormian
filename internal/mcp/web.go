// 网页抓取：基于 Playwright MCP Server（JS 渲染页面），移植自 interview-agent 蓝本。
package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// WebScraper 网页抓取器。
type WebScraper struct {
	client *client.Client
	mu     sync.Mutex
}

// resolveMCPCommand 决定如何启动 Playwright MCP Server：
// 优先用 `npm root -g` 定位全局安装的 @playwright/mcp 直接 node 启动（离线秒起），
// 全局未安装时回退 `npx @playwright/mcp@latest` 自动下载。
func resolveMCPCommand() (string, []string) {
	if out, err := exec.Command("npm", "root", "-g").Output(); err == nil {
		cliPath := filepath.Join(strings.TrimSpace(string(out)), "@playwright", "mcp", "cli.js")
		if _, err := os.Stat(cliPath); err == nil {
			return "node", []string{cliPath, "--headless"}
		}
	}
	return "npx", []string{"@playwright/mcp@latest", "--headless"}
}

// NewWebScraper 创建网页抓取器。
func NewWebScraper() (*WebScraper, error) {
	cmd, args := resolveMCPCommand()
	cli, err := client.NewStdioMCPClient(cmd, nil, args...)
	if err != nil {
		return nil, fmt.Errorf("mcp/web: 启动 Playwright MCP Server 失败（请确认已安装 node，或 npm i -g @playwright/mcp）：%w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "mirror-mian", Version: "1.0.0"}
	if _, err := cli.Initialize(ctx, initReq); err != nil {
		cli.Close()
		return nil, fmt.Errorf("mcp/web: MCP 初始化失败（首次启动需下载，可能较慢）：%w", err)
	}
	return &WebScraper{client: cli}, nil
}

// Fetch 抓取页面文本（JS 渲染后）。
func (ws *WebScraper) Fetch(ctx context.Context, url string) (string, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	navReq := mcp.CallToolRequest{}
	navReq.Params.Name = "browser_navigate"
	navReq.Params.Arguments = map[string]any{"url": url}
	if _, err := ws.client.CallTool(ctx, navReq); err != nil {
		return "", fmt.Errorf("mcp/web: 导航失败: %w", err)
	}

	textReq := mcp.CallToolRequest{}
	textReq.Params.Name = "browser_get_text"
	if result, err := ws.client.CallTool(ctx, textReq); err != nil {
		return "", fmt.Errorf("mcp/web: 读取文本失败: %w", err)
	} else {
		var b strings.Builder
		for _, c := range result.Content {
			if tc, ok := c.(mcp.TextContent); ok {
				b.WriteString(tc.Text)
				b.WriteString("\n")
			}
		}
		text := strings.TrimSpace(b.String())
		if text == "" {
			return "", fmt.Errorf("mcp/web: 页面无文本内容（可能被反爬或需登录）")
		}
		return text, nil
	}
}

// Close 关闭 MCP Server。
func (ws *WebScraper) Close() error {
	if ws.client != nil {
		return ws.client.Close()
	}
	return nil
}
