// 复习资源推荐：按主题搜索 GitHub 项目（交互层，无业务逻辑）。
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"mirror-mian/internal/config"
	"mirror-mian/internal/mcp"
)

// cmdResources 按主题推荐 GitHub 学习资源。
func cmdResources(_ *config.Config, args []string) error {
	if len(args) < 1 || args[0] == "" {
		return fmt.Errorf("resources 需要主题参数：resources <topic> [count]")
	}
	topic := args[0]
	limit := 5
	if len(args) >= 2 {
		if n, err := strconv.Atoi(args[1]); err == nil && n > 0 {
			limit = n
		}
	}
	token := os.Getenv("GITHUB_TOKEN")
	searcher, err := mcp.NewGitHubSearcher(token)
	if err != nil {
		return err
	}
	defer searcher.Close()

	query := fmt.Sprintf("%q interview questions stars:>50", topic)
	fmt.Printf("搜索 GitHub：%s（%d 条）...\n", query, limit)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	repos, err := searcher.SearchRepos(ctx, query, limit)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Println("没有找到相关项目，试试更宽泛的主题词")
		return nil
	}
	fmt.Printf("\n「%s」推荐资源（按 star 排序）：\n", topic)
	for i, r := range repos {
		fmt.Printf("  %d. ★%d  %s\n     %s\n     %s\n", i+1, r.Stars, r.Name, r.URL, firstLine(r.Desc))
	}
	return nil
}

func firstLine(s string) string {
	if s == "" {
		return "（无描述）"
	}
	// 取第一行，截断到 80 字符
	end := min(len(s), 80)
	for i := 0; i < end; i++ {
		if s[i] == '\n' {
			end = i
			break
		}
	}
	return s[:end]
}
