package rag

import (
	"context"
	"testing"

	"mirror-mian/internal/store"
)

func mkChunks(contents ...string) []store.KnowledgeChunk {
	out := make([]store.KnowledgeChunk, len(contents))
	for i, c := range contents {
		out[i] = store.KnowledgeChunk{ID: int64(i + 1), Topic: "T", Content: c, Index: i}
	}
	return out
}

func TestTokenize(t *testing.T) {
	// 中文 bigram + 英文词
	terms := tokenize("AOF重写机制 Redis持久化")
	found := map[string]bool{}
	for _, tm := range terms {
		found[tm] = true
	}
	// 英文按词；中文按 bigram（相邻字符对）
	if !found["aof"] || !found["redis"] || !found["持久"] || !found["久化"] || !found["机制"] {
		t.Fatalf("terms = %v", terms)
	}
	// bigram："重写" = "重"+"写"
	if !found["重写"] {
		t.Fatalf("bigram missing: %v", terms)
	}
}

func TestBM25Search(t *testing.T) {
	chunks := mkChunks(
		"Redis AOF 持久化机制：appendonly 配置与 everysec 策略",
		"MySQL 索引优化：B+Tree 与回表",
		"Redis 内存淘汰策略与过期删除",
		"Go 并发编程：goroutine 与 channel",
	)
	idx := BuildBM25(chunks)
	// 关键词「AOF」精确命中块 0
	ids := idx.Search("AOF 持久化", 3)
	if len(ids) == 0 || ids[0] != 0 {
		t.Fatalf("AOF search = %v", ids)
	}
	// 关键词「索引」命中块 1
	ids = idx.Search("MySQL 索引", 3)
	if len(ids) == 0 || ids[0] != 1 {
		t.Fatalf("index search = %v", ids)
	}
	// 空索引
	empty := BuildBM25(nil)
	if got := empty.Search("x", 5); len(got) != 0 {
		t.Fatalf("empty search = %v", got)
	}
}

func TestRRFMerge(t *testing.T) {
	// 两路：路 A 排名 [doc1, doc2, doc3]，路 B 排名 [doc2, doc4]
	listA := []Result{
		{Content: "doc1 content one"}, {Content: "doc2 content two"}, {Content: "doc3 content three"},
	}
	listB := []Result{
		{Content: "doc2 content two"}, {Content: "doc4 content four"},
	}
	merged := rrfMerge(listA, listB)
	if len(merged) != 4 {
		t.Fatalf("merged = %d", len(merged))
	}
	// doc2 两路都在前列 → 应排第一
	if merged[0].Content != "doc2 content two" {
		t.Fatalf("doc2 should rank first: %+v", merged[0])
	}
	// 单路可用
	only := rrfMerge(listA)
	if len(only) != 3 || only[0].Content != "doc1 content one" {
		t.Fatalf("single list = %+v", only)
	}
}

func TestLLMRerank(t *testing.T) {
	// mock LLM：按内容关键词打分（含"相关"的块给高分）
	lm := &rerankLLM{}
	rr := NewReranker(lm, true)
	cands := []Result{
		{Content: "不相关内容甲"}, {Content: "相关内容乙"},
	}
	reordered, err := rr.Rerank(context.Background(), "q", cands)
	if err != nil {
		t.Fatalf("Rerank: %v", err)
	}
	if reordered[0].Content != "相关内容乙" {
		t.Fatalf("rerank order wrong: %+v", reordered)
	}
	// none 原序
	none := NewReranker(lm, false)
	out, _ := none.Rerank(context.Background(), "q", cands)
	if out[0].Content != cands[0].Content {
		t.Fatalf("none should keep order")
	}
	// 失败回退原序
	lm.fail = true
	out, err = NewReranker(lm, true).Rerank(context.Background(), "q", cands)
	if err == nil || out[0].Content != cands[0].Content {
		t.Fatalf("fail should fallback: err=%v", err)
	}
}

// rerankLLM 测试桩：按内容是否含「相关」打分。
type rerankLLM struct {
	fail bool
}

func (l *rerankLLM) ModelName() string { return "fake" }

func (l *rerankLLM) Generate(_ context.Context, _, _ string) (string, error) {
	return `{"scores": [1, 9]}`, nil
}

func (l *rerankLLM) GenerateJSON(_ context.Context, _, _ string, out any) error {
	if l.fail {
		return context.Canceled
	}
	// 固定两块：块 0 低分、块 1 高分（块 1 是「相关内容」）
	*out.(*struct {
		Scores []int `json:"scores"`
	}) = struct {
		Scores []int `json:"scores"`
	}{Scores: []int{1, 9}}
	return nil
}
