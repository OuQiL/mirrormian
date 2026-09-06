## 1. BM25 检索

- [x] 1.1 `internal/rag/bm25.go`：tokenize（中文 bigram + 英文词）、BuildBM25、Search（k1=1.5/b=0.75），验证单测打分排序与关键词命中
- [x] 1.2 `rag.Service` 持 BM25 索引（按 topic 惰性构建），RebuildIndex 接口；domain.Sync/kb import 后重建，验证单测索引重建后检索新内容

## 2. RRF 融合与 Rerank

- [x] 2.1 `internal/rag/fusion.go`：rrfMerge（k=60，内容去重键，单路可用），验证单测两路/单路
- [x] 2.2 `internal/rag/rerank.go`：Reranker 接口 + LLMReranker（分数 JSON 解析、失败回退）+ NoneReranker；配置 RAG_RERANKER，验证单测 mock 打分与降级

## 3. Retrieve 改造

- [x] 3.1 Retrieve 双路并行 → RRF → Rerank → topK，外部行为不变；验证单测：关键词路补充向量路遗漏的块、融合排序正确
- [x] 3.2 既有测试（rag/domain/orchestration E2E）全部通过（回归）

## 4. 验证与收尾

- [x] 4.1 `go test ./...`、`go vet`、`go build` 全部通过；真实链路：sync 后检索对比单路/多路结果
- [x] 4.2 openspec validate 通过；改动范围仅限 mirror-mian 内新增/修改文件
