package rag

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"mirror-mian/internal/embedding"
	"mirror-mian/internal/store"
)

// fakeEmbedder 确定性向量：按文本特征生成，便于测试相似度排序。
type fakeEmbedder struct{}

func (fakeEmbedder) Available() bool { return true }

func (fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = hashVector(t)
	}
	return out, nil
}

// hashVector 字符袋向量：共享字符越多，余弦相似度越高。
func hashVector(s string) []float32 {
	v := make([]float32, 32)
	for _, r := range []rune(s) {
		v[int(r)%32] += 1
	}
	return v
}

func newTestRAG(t *testing.T) (*Service, store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "rag.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return NewService(fakeEmbedder{}, st), st
}

func TestRetrieveTopKAndDedup(t *testing.T) {
	svc, st := newTestRAG(t)
	// 两条内容前缀相同（去重只留一条），一条不同
	err := st.SaveTopicChunks("Redis", []store.KnowledgeChunk{
		{Content: "Redis RDB 持久化机制详解 第一部分", Embedding: hashVector("RDB 持久化"), Index: 0},
		{Content: "Redis RDB 持久化机制详解 第二部分", Embedding: hashVector("RDB 持久化"), Index: 1},
		{Content: "Redis 哨兵高可用方案", Embedding: hashVector("Sentinel 高可用"), Index: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	results, err := svc.Retrieve(context.Background(), "Redis", "RDB 持久化怎么做", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 { // 3 条 → 前缀去重剩 2 条
		t.Fatalf("results = %d, want 2 (dedup)", len(results))
	}
	if !strings.Contains(results[0].Content, "RDB") {
		t.Fatalf("should rank RDB chunk first: %+v", results[0])
	}
}

func TestRetrieveEmptyTopic(t *testing.T) {
	svc, _ := newTestRAG(t)
	results, err := svc.Retrieve(context.Background(), "MySQL", "anything", 5)
	if err != nil || len(results) != 0 {
		t.Fatalf("empty topic: %v %d", err, len(results))
	}
}

func TestContextBudget(t *testing.T) {
	svc, st := newTestRAG(t)
	_ = st.SaveTopicChunks("Redis", []store.KnowledgeChunk{
		{Content: "AAA " + strings.Repeat("x", 6000), Embedding: hashVector("A"), Index: 0},
		{Content: "BBB " + strings.Repeat("y", 6000), Embedding: hashVector("B"), Index: 1},
	})
	ctxStr, err := svc.Context(context.Background(), "Redis", "A", 8000)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ctxStr, "知识库参考") {
		t.Fatalf("context should have header: %q", ctxStr[:min(20, len(ctxStr))])
	}
	if len(ctxStr) > 8000 {
		t.Fatalf("context over budget: %d", len(ctxStr))
	}
}

func TestCosine(t *testing.T) {
	a := []float32{1, 0, 0}
	if embedding.Cosine(a, []float32{1, 0, 0}) < 0.999 {
		t.Fatal("identical should be ~1")
	}
	if embedding.Cosine(a, []float32{0, 1, 0}) > 0.01 {
		t.Fatal("orthogonal should be ~0")
	}
	if embedding.Cosine(a, []float32{1}) != 0 {
		t.Fatal("dim mismatch should be 0")
	}
}

func TestChunkEmptyAndList(t *testing.T) {
	svc, st := newTestRAG(t)
	if got := ChunkText("", 1000, 150); len(got) != 0 {
		t.Fatalf("empty chunk: %+v", got)
	}
	topics, err := st.ListAllTopics()
	if err != nil || len(topics) != 0 {
		t.Fatalf("list topics: %v %+v", err, topics)
	}
	_ = svc
}
