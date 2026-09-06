// 知识库检索：余弦相似度 topK + 内容去重 + 上下文拼装。
package rag

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"mirror-mian/internal/embedding"
	"mirror-mian/internal/store"
	"mirror-mian/internal/vector"
)

// vectorStore Milvus 向量检索接口（可 nil：回退 SQLite 暴力余弦）。
type vectorStore interface {
	Search(ctx context.Context, topic string, queryVec []float32, topK int) ([]vector.Hit, error)
	Available() bool
}

const (
	defaultTopK   = 5
	dedupPrefix   = 100 // 按首 100 字符去重
	defaultBudget = 8000
)

// DefaultBudget 知识上下文默认字符预算。
func DefaultBudget() int { return defaultBudget }

// Service 知识库检索服务（基础能力层）。
type Service struct {
	embedder embedding.Embedder
	store    store.Store
	reranker Reranker
	milvus   vectorStore

	mu        sync.Mutex
	bm25ByTopic map[string]*BM25Index // BM25 索引（按主题惰性构建）
}

// NewService 创建知识库服务。
func NewService(e embedding.Embedder, st store.Store) *Service {
	return &Service{
		embedder:    e,
		store:       st,
		reranker:    NoneReranker{},
		bm25ByTopic: map[string]*BM25Index{},
	}
}

// SetReranker 设置重排序器（默认 none；llm 可用时调用方注入）。
func (s *Service) SetReranker(r Reranker) {
	if r != nil {
		s.reranker = r
	}
}

// SetVectorStore 设置 Milvus 向量检索（nil 时向量路回退 SQLite 暴力余弦）。
func (s *Service) SetVectorStore(v vectorStore) {
	s.milvus = v
}

// RebuildIndex 重建某主题的 BM25 索引（领域同步/导入后调用）。
func (s *Service) RebuildIndex(topic string) error {
	chunks, err := s.store.ListTopicChunks(topic)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.bm25ByTopic[topic] = BuildBM25(chunks)
	s.mu.Unlock()
	return nil
}

// Available embedding 是否可用。
func (s *Service) Available() bool { return s.embedder != nil && s.embedder.Available() }

// Embedder 返回持有的向量化器（供画像去重等复用；nil 表示降级）。
func (s *Service) Embedder() embedding.Embedder { return s.embedder }

// Result 一条检索结果。
type Result struct {
	Topic   string
	Content string
	Score   float64
}

// Retrieve 检索某主题知识库：向量 + BM25 双路并行召回 → RRF(k=60) 融合 → LLM Rerank → topK。
// 单路可用时（Embedding 未配置 / BM25 无命中）自动降级为单路，不报错。
func (s *Service) Retrieve(ctx context.Context, topic, query string, topK int) ([]Result, error) {
	if topK <= 0 {
		topK = defaultTopK
	}
	chunks, err := s.store.ListTopicChunks(topic)
	if err != nil {
		return nil, fmt.Errorf("rag: list chunks: %w", err)
	}
	if len(chunks) == 0 {
		return nil, nil
	}

	var lists [][]Result

	// 路 1：向量检索（Milvus 优先，否则 SQLite 暴力余弦；embedding 可用时）
	if s.Available() {
		vecs, err := s.embedder.Embed(ctx, []string{query})
		if err == nil {
			q := vecs[0]
			var scored []Result
			if s.milvus != nil && s.milvus.Available() {
				// Milvus 检索：按 chunk_index 映射回块内容
				hits, herr := s.milvus.Search(ctx, topic, q, topK*2)
				if herr == nil {
					byIndex := map[int64]store.KnowledgeChunk{}
					for _, c := range chunks {
						byIndex[int64(c.Index)] = c
					}
					for _, h := range hits {
						c, ok := byIndex[h.ChunkIndex]
						if !ok {
							continue
						}
						scored = append(scored, Result{
							Topic: c.Topic, Content: c.Content,
							Score: float64(h.Score),
						})
					}
				}
			}
			if len(scored) == 0 {
				// 回退：SQLite 暴力余弦
				seen := map[string]bool{}
				for _, c := range chunks {
					key := prefixKey(c.Content)
					if seen[key] {
						continue
					}
					seen[key] = true
					scored = append(scored, Result{
						Topic: c.Topic, Content: c.Content,
						Score: embedding.Cosine(q, c.Embedding),
					})
				}
				sortResults(scored)
			}
			if len(scored) > topK*2 {
				scored = scored[:topK*2]
			}
			lists = append(lists, scored)
		}
	}

	// 路 2：BM25 关键词检索（索引惰性构建）
	bm25 := s.bm25For(topic, chunks)
	if bm25 != nil {
		if ids := bm25.Search(query, topK*2); len(ids) > 0 {
			bm25Results := make([]Result, 0, len(ids))
			for _, id := range ids {
				c := chunks[id]
				bm25Results = append(bm25Results, Result{
					Topic: c.Topic, Content: c.Content, Score: 0,
				})
			}
			lists = append(lists, bm25Results)
		}
	}

	// 无任何路可用
	if len(lists) == 0 {
		return nil, nil
	}

	// RRF 融合（k=60）
	merged := rrfMerge(lists...)
	if len(merged) > topK*2 {
		merged = merged[:topK*2]
	}

	// LLM Rerank（失败回退融合序）
	reordered, err := s.reranker.Rerank(ctx, query, merged)
	if err != nil {
		reordered = merged // 降级
	}
	if len(reordered) > topK {
		reordered = reordered[:topK]
	}
	return reordered, nil
}

// bm25For 获取（或构建）主题 BM25 索引。
func (s *Service) bm25For(topic string, chunks []store.KnowledgeChunk) *BM25Index {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idx, ok := s.bm25ByTopic[topic]; ok {
		return idx
	}
	idx := BuildBM25(chunks)
	s.bm25ByTopic[topic] = idx
	return idx
}

// sortResults 按 Score 降序。
func sortResults(rs []Result) {
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0 && rs[j].Score > rs[j-1].Score; j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}
}

// Context 检索并拼装为知识上下文段落（供出题 prompt 注入）。
// 无检索结果时返回空串（调用方不注入）。
func (s *Service) Context(ctx context.Context, topic, query string, budget int) (string, error) {
	if budget <= 0 {
		budget = defaultBudget
	}
	results, err := s.Retrieve(ctx, topic, query, defaultTopK)
	if err != nil || len(results) == 0 {
		return "", err
	}
	var b strings.Builder
	b.WriteString("[知识库参考]\n")
	for _, r := range results {
		if b.Len()+len(r.Content)+2 > budget {
			break
		}
		b.WriteString(r.Content)
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String()), nil
}

// prefixKey 内容去重键：前 dedupPrefix 字符。
func prefixKey(s string) string {
	if len(s) <= dedupPrefix {
		return s
	}
	return s[:dedupPrefix]
}
