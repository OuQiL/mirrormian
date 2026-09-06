// LLM Rerank：对融合后的候选按查询相关性重排（失败回退原序）。
package rag

import (
	"context"
	"fmt"
	"strings"

	"mirror-mian/internal/llm"
)

// Reranker 重排序接口。
type Reranker interface {
	// Rerank 按查询相关性对候选重排（返回新顺序）。
	Rerank(ctx context.Context, query string, candidates []Result) ([]Result, error)
}

// NewReranker 按配置创建 Reranker：llm（默认，可用时）或 none。
// LLM 不可用时返回 NoneReranker（原序）。
func NewReranker(lm llm.Client, enabled bool) Reranker {
	if !enabled || lm == nil {
		return NoneReranker{}
	}
	return &LLMReranker{llm: lm}
}

// LLMReranker 用 LLM 对候选块按查询相关性打分（0-10）并降序重排。
type LLMReranker struct {
	llm llm.Client
}

// Rerank 实现 Reranker：一次 LLM 调用打分，失败回退原序。
func (r *LLMReranker) Rerank(ctx context.Context, query string, candidates []Result) ([]Result, error) {
	if len(candidates) <= 1 {
		return candidates, nil
	}
	const system = `你是信息检索相关性评估器。给定查询与候选知识块列表，为每个块输出与查询的相关性分数（0-10 整数，10=高度相关）。
只输出 JSON：{"scores": [3, 8, 5, ...]}（顺序与候选列表一致），不要任何解释文字。`

	var b strings.Builder
	fmt.Fprintf(&b, "查询：%s\n\n候选块：\n", query)
	for i, c := range candidates {
		fmt.Fprintf(&b, "[%d] %s\n", i+1, truncateContent(c.Content))
	}
	var out struct {
		Scores []int `json:"scores"`
	}
	if err := r.llm.GenerateJSON(ctx, system, b.String(), &out); err != nil {
		return candidates, fmt.Errorf("rerank: %w", err) // 调用方回退
	}
	if len(out.Scores) != len(candidates) {
		return candidates, fmt.Errorf("rerank: 分数数量 %d != 候选 %d", len(out.Scores), len(candidates))
	}
	// 按分数降序重排（稳定）
	idx := make([]int, len(candidates))
	for i := range idx {
		idx[i] = i
	}
	for i := 1; i < len(idx); i++ {
		for j := i; j > 0 && out.Scores[idx[j]] > out.Scores[idx[j-1]]; j-- {
			idx[j], idx[j-1] = idx[j-1], idx[j]
		}
	}
	reordered := make([]Result, len(candidates))
	for i, j := range idx {
		reordered[i] = candidates[j]
	}
	return reordered, nil
}

// NoneReranker 原序返回（降级）。
type NoneReranker struct{}

// Rerank 实现 Reranker：原序。
func (NoneReranker) Rerank(_ context.Context, _ string, candidates []Result) ([]Result, error) {
	return candidates, nil
}

// truncateContent 截断候选内容（省 token）。
func truncateContent(s string) string {
	const max = 300
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
