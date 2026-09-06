// JD/简历匹配度分析命令（交互层，无业务逻辑）。
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"mirror-mian/internal/agents"
	"mirror-mian/internal/config"
	"mirror-mian/internal/llm"
)

// cmdAnalyze 解析 JD 与简历，输出匹配度分析报告。
func cmdAnalyze(cfg *config.Config, args []string) error {
	var jdPath, resumePath string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--jd":
			if i+1 >= len(args) {
				return fmt.Errorf("--jd 缺少文件路径")
			}
			jdPath = args[i+1]
			i++
		case "--resume":
			if i+1 >= len(args) {
				return fmt.Errorf("--resume 缺少文件路径")
			}
			resumePath = args[i+1]
			i++
		default:
			return fmt.Errorf("未知参数 %q", args[i])
		}
	}
	if jdPath == "" || resumePath == "" {
		return fmt.Errorf("analyze 需要 --jd <file> 与 --resume <file>")
	}
	if cfg.LLMAPIKey == "" {
		return fmt.Errorf("LLM_API_KEY 未配置，无法分析")
	}
	jdText, err := os.ReadFile(jdPath)
	if err != nil {
		return fmt.Errorf("读取 JD: %w", err)
	}
	resumeText, err := os.ReadFile(resumePath)
	if err != nil {
		return fmt.Errorf("读取简历: %w", err)
	}
	client, err := llm.New(cfg)
	if err != nil {
		return err
	}
	analyzer := agents.NewJDResumeAnalyzer(client)

	fmt.Println("分析中...")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	analysis, err := analyzer.Analyze(ctx, string(resumeText), string(jdText))
	if err != nil {
		return fmt.Errorf("分析失败: %w", err)
	}

	fmt.Printf("\n=== 匹配度分析：%d/100 ===\n", analysis.MatchScore)
	fmt.Printf("总评：%s\n\n", analysis.Summary)

	fmt.Println("【岗位职责】")
	for _, r := range analysis.Responsibilities {
		fmt.Printf("  • %s\n", r)
	}
	fmt.Println("\n【任职要求逐项对位】")
	for _, r := range analysis.Requirements {
		mark := map[string]string{"met": "✓ 满足", "partial": "◐ 部分", "missing": "✗ 缺失"}[r.Status]
		level := "硬性"
		if r.Level == "plus" {
			level = "加分"
		}
		evidence := ""
		if r.Evidence != "" {
			evidence = fmt.Sprintf("（简历：%s）", truncateStr(r.Evidence, 60))
		}
		fmt.Printf("  %s [%s] %s %s\n", mark, level, r.Item, evidence)
	}
	fmt.Println("\n【技术栈】")
	fmt.Printf("  %s\n", joinAll(analysis.TechStack))
	fmt.Println("\n【差距清单（按严重度）】")
	if len(analysis.Gaps) == 0 {
		fmt.Println("  （无差距）")
	}
	for _, g := range analysis.Gaps {
		item := g.Item
		if item == "" {
			item = "未命名差距" // 模型偶发缺字段，容错
		}
		fmt.Printf("  [%s] %s\n     建议：%s\n", g.Severity, item, g.Suggestion)
	}
	return nil
}

func joinAll(items []string) string {
	var b strings.Builder
	for i, s := range items {
		if i > 0 {
			b.WriteString("、")
		}
		b.WriteString(s)
	}
	return b.String()
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
