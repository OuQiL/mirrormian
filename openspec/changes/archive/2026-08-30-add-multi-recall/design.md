## Context

现有检索为单路向量余弦（SQLite 暴力扫描），`rag.Service.Retrieve` 返回 topK。本 change 在其内部扩展为多路：BM25 + 向量 → RRF → Rerank，外部接口（Retrieve/Context）不变。蓝本 interview-agent 有 RerankStrategy 接口可参考（llm / cross-encoder / none），本 change 用 LLM Rerank（cross-encoder 需独立模型服务，暂不做）。

## Goals / Non-Goals

**Goals:**
- BM25 内存倒排索引（中文 bigram + 英文词），随 sync 重建
- RRF(k=60) 融合 + LLM Rerank（可切换 none）
- Retrieve/Context 外部行为不变

**Non-Goals:**
- 不做 cross-encoder Rerank（需要专用模型服务，后续）
- 不做 BM25 持久化（每次启动从块重建，量小）
- 不改出题 Agent 接口

## Decisions

### 1. BM25 实现（internal/rag/bm25.go）
```go
type BM25Index struct {
    df map[string]int      // 文档频率
    postings map[string][]int // 词 → 文档 id 列表
    docs []bm25Doc         // 块内容 + 长度
    avgDocLen float64
}
func BuildBM25(chunks []store.KnowledgeChunk) *BM25Index
func (b *BM25Index) Search(query string, topK int) []int  // 返回块下标，按 BM25 分数
```
- 分词：`tokenize(s)`——中文连续段按 bigram（相邻字符对）切分 + 英文/数字按词切分（小写）
- BM25 打分：标准公式，k1=1.5, b=0.75；查询词 IDF 加权
- 索引构建时机：`rag.Service` 增加 `RebuildIndex(topic)`，由 `domain.Sync` 与 `kb import` 在 SaveTopicChunks 后调用

### 2. RRF 融合（internal/rag/fusion.go）
```go
func rrfMerge(vectorRanks, bm25Ranks []Result, k int) []Result
// score = Σ 1/(k + rank)，k=60；两路合并去重，按分数降序
```
`Result` 以内容前缀（前 100 字符）为去重键（与现有 dedup 一致）。

### 3. LLM Rerank（internal/rag/rerank.go）
```go
type Reranker interface { Rerank(ctx, query string, candidates []Result) ([]Result, error) }
type LLMReranker struct { llm llm.Client }
type NoneReranker struct{}
```
LLM prompt：给定查询与候选块列表，输出每块相关性分数（0-10）JSON 数组（对齐块序），按分数降序。失败回退原序。
配置：`RAG_RERANKER`（llm/none，默认 llm；Embedding 或 LLM 不可用自动 none）。

### 4. Retrieve 改造
```go
func (s *Service) Retrieve(ctx, topic, query, topK) ([]Result, error) {
    // 1) 并行双路：向量（现有）+ BM25（索引）
    // 2) RRF 融合 topK*2
    // 3) LLM Rerank topK
    // 4) 截断 topK
}
```
Context 不变（内部调 Retrieve）。

## Risks / Trade-offs

- [BM25 中文 bigram 不如专业分词] → 术语/代码检索效果足够；词典分词为后续优化
- [LLM Rerank 延迟（一次调用）] → 仅对融合后 topN（默认 10）打分；失败回退融合序
- [索引与向量一致性] → 同一同步入口触发（Sync/RebuildIndex 同时重建）

## Migration Plan

纯新增。`rag.Service` 持有 BM25 索引（按 topic 惰性构建），首次 Retrieve 自动构建。

## Open Questions

无——范围已确认（BM25 + RRF + LLM Rerank，外部接口不变）。
