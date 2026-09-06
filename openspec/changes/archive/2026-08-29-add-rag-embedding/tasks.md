## 1. Embedding 客户端

- [x] 1.1 创建 `internal/embedding`：Embedder 接口（Embed/Available）+ OpenAIEmbedder（`/embeddings` 端点，base_url/api_key/model 可独立配置、回退复用 LLM 配置），验证单测 mock 响应解析正确、未配置时 Available()=false
- [x] 1.2 `internal/config` 增加 embedding 三项配置与校验，验证缺省回退逻辑

## 2. 知识库

- [x] 2.1 `internal/rag`：ChunkText（1000 字符/150 重叠），验证单测覆盖长文切分、重叠、空文本
- [x] 2.2 `internal/store` 增加 knowledge_chunks 表与 SaveChunks/DeleteTopic/ListChunks，验证建库/写入/重建/读取往返
- [x] 2.3 rag.Retrieve：余弦相似度 topK + 首 100 字符去重，验证单测排序与去重正确
- [x] 2.4 rag.Context：检索拼接为知识上下文段落（charBudget 截断），验证单测

## 3. 薄弱点向量去重

- [x] 3.1 profile.AbsorbAnswer 增加可选 Embedder：相似度 ≥0.75 命中合并、<0.75 新建、embedder 为 nil 回退精确匹配，验证单测覆盖合并/不合并/降级三路径

## 4. 出题注入

- [x] 4.1 questioner.GenerateInitial 增加 knowledgeContext 参数（空时行为不变），验证注入后 prompt 含知识块
- [x] 4.2 orchestration 专项模式出题前调用 rag.Context（topK 5、8000 字符预算），知识库空时正常出题，验证单测

## 5. CLI 与 Web

- [x] 5.1 实现 `kb import <topic> <file>`（重建主题）与 `kb list`、`kb search`，验证导入后 list 块数正确、search 返回相关块
- [x] 5.2 Web `/api/kb` 接口 + 画像页知识库状态展示，验证页面显示主题与块数

## 6. 验证与收尾

- [x] 6.1 `go test ./...`、`go vet`、`go build` 全部通过；真实 LLM+Embedding 跑通 kb import → 专项面试（出题含知识上下文）→ 复盘全链路
- [x] 6.2 openspec validate 通过；改动范围仅限 mirror-mian 内新增/修改文件
