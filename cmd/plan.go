// 复习计划命令（交互层，无业务逻辑）。
package main

import (
	"context"
	"fmt"
	"time"

	"mirror-mian/internal/agents"
	"mirror-mian/internal/config"
	"mirror-mian/internal/llm"
	"mirror-mian/internal/profile"
	"mirror-mian/internal/store"
)

// cmdPlan 基于画像生成复习计划。
func cmdPlan(cfg *config.Config) error {
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()
	p, err := st.GetProfile()
	if err != nil {
		return err
	}
	due := profile.DueReviews(p.WeakPoints, time.Now())
	if len(p.WeakPoints) == 0 && len(p.Mastery) == 0 {
		fmt.Println("画像为空——先完成几场面试积累数据，再生成复习计划")
		return nil
	}
	if cfg.LLMAPIKey == "" {
		return fmt.Errorf("LLM_API_KEY 未配置，无法生成复习计划")
	}
	client, err := llm.New(cfg)
	if err != nil {
		return err
	}
	planner := agents.NewReviewPlanner(client)

	fmt.Println("生成复习计划中...")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	plan, err := planner.Plan(ctx, p, len(due))
	if err != nil {
		return fmt.Errorf("生成计划失败: %w", err)
	}

	fmt.Printf("\n=== 今日复习计划 ===\n%s\n", plan.Summary)
	if len(plan.Items) == 0 {
		fmt.Println("（暂无行动项）")
		return nil
	}
	for i, item := range plan.Items {
		prio := item.Priority
		if prio == "" {
			prio = "medium" // 模型偶发缺字段，容错
		}
		mark := map[string]string{"high": "🔴", "medium": "🟡", "low": "🟢"}[prio]
		action := item.Action
		if action == "" {
			action = item.Topic // 容错：Action 缺失时用主题
		}
		fmt.Printf("\n%d. %s [%s] %s\n", i+1, mark, prio, action)
		fmt.Printf("   主题：%s\n", item.Topic)
		if item.Suggestion != "" {
			fmt.Printf("   建议：%s\n", item.Suggestion)
		}
		if item.Minutes > 0 {
			fmt.Printf("   预计：%d 分钟\n", item.Minutes)
		}
	}
	return nil
}
