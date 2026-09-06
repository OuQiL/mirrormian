## Why

当前 RAG 检索是单路向量余弦（SQLite 暴力扫描），召回质量受限于向量路：关键词精确匹配的块（术语、代码、专有名词）可能被漏掉。按用户要求补齐多路召回：**向量 + BM25 双路并行 → RRF(k=60) 融合 → LLM Rerank**，显著提升知识库检索质量（对齐 interview-agent 蓝本的 RAG 设计）。

## What Changes

- **BM25 关键词检索**（纯内存）：知识库块 tokenize（中文 bigram + 英文词）→ 倒排索引 → BM25 打分
- **RRF 融合**：向量路（余弦 topK）与 BM25 路（topK）按 `score = Σ 1/(k + rank)`（k=60）融合排序
- **LLM Rerank**：融合后 topN 交 LLM 相关性打分重排（Reranker 接口，LLM/None 可切换，对齐蓝本）
- 检索接口 `Retrieve` 内部改造（外部行为不变：Context 注入出题仍返回拼接文本）
- 索引构建：知识库同步（domain sync / kb import）时同时构建 BM25 索引

## Capabilities

### New Capabilities

- `multi-recall`: RAG 多路召回——BM25 关键词检索、RRF(k=60) 融合排序、LLM Rerank 重排序

### Modified Capabilities

（无——rag-embedding 的检索外部接口不变）

## Impact

- **新增**：`internal/rag/bm25.go`（倒排索引 + 打分）、`internal/rag/fusion.go`（RRF）、`internal/rag/rerank.go`（Reranker 接口 + LLM 实现）
- **修改**：`internal/rag/retrieve.go`（双路并行 + 融合 + rerank）、`internal/rag`（索引构建接线）
- **依赖**：无新增（LLM Rerank 复用现有 llm.Client）
- **风险**：低——外部行为不变；BM25 中文分词的 bigram 方案简单可靠
