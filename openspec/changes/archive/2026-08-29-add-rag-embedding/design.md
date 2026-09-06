## Context

mirror-mian 已有：三层架构、Eino 多 Agent 编排（questioner/interviewer/…）、SQLite store（会话/画像）、薄弱点精确去重（技术债）、CLI + Web 交互层。本 change 补上数据飞轮的 Embedding/RAG 两环，设计参数对齐 TechSpar（chunk 1000/150、topK 5、去重阈值 0.75）。

## Goals / Non-Goals

**Goals:**
- Embedding 客户端可配置可降级；知识库分块/向量化/检索；出题注入知识上下文；薄弱点向量去重
- 全部在既有三层架构内：rag/embedding 属基础能力层，questioner 属编排层，CLI 属交互层
- 零新第三方依赖（embedding 走标准库 HTTP）

**Non-Goals:**
- 不做向量数据库（Milvus 等）——单机数据量小，SQLite 存 BLOB + 暴力余弦（TechSpar 同方案），后续量级上来再迁移
- 不做 Web 知识库编辑器——CLI 导入即可，Web 只展示状态
- 不做文档多格式解析（PDF/DOCX）——本 change 仅 Markdown/文本，多格式为后续 change

## Decisions

### 1. Embedding 客户端（internal/embedding）
```go
type Embedder interface {
    Embed(ctx, texts []string) ([][]float32, error)
    Available() bool   // 配置完整且已连通
}
```
实现 `OpenAIEmbedder`：POST `{base_url}/embeddings`，body `{"model": model, "input": texts}`。配置项：
`LLM_EMBEDDING_BASE_URL / LLM_EMBEDDING_API_KEY / LLM_EMBEDDING_MODEL`，三者均未设置时回退复用 LLM 配置；仍无 key 则 `Available()=false`。
**理由**：OpenAI 兼容事实标准，小米 MiMo 若提供 embeddings 端点可直接用，否则用户换任意兼容服务零代码改动。

### 2. 知识库分块与检索（internal/rag）
- `ChunkText(text, 1000, 150)`：按空行分段落累积，块 ≤1000 字符、块间 150 重叠（TechSpar `chunkText` 移植）
- 存储：SQLite 新表 `knowledge_chunks(id, topic, content, embedding BLOB Float32LE, chunk_index)`，按主题重建（重复导入先删该主题）
- `Retrieve(ctx, topic, query, topK=5)`：query embedding → 全量余弦 → topK，内容按首 100 字符去重
- `Context(ctx, topic, query, charBudget)`：检索拼接为 `[知识库]` 段落供 prompt 注入
**理由**：块数少（单主题几十块）暴力扫描足够；重建策略保证一致性（TechSpar 同策略）。

### 3. 出题注入（编排层接线）
`questioner.GenerateInitial` 增加可选参数 `knowledgeContext string`；`orchestration` 在专项模式出题前调用 `rag.Context(topic, topic名+知识点, 8000字符)`，非空则注入 user prompt 前缀。综合模式暂不注入（JD/简历本身已是上下文，后续 change 再接入）。
**理由**：四路输入之一的「知识库」一路就位；保持 questioner 无感知可空上下文。

### 4. 薄弱点向量去重（internal/profile）
`AbsorbAnswer` 增加可选 `Embedder`（nil 时回退精确匹配）：新弱点先与既有弱点逐个余弦比较，`≥0.75`（TechSpar WEAK_POINT_SIMILARITY）命中合并；`<0.75` 全部不命中才新建。调用方（orchestration absorb）在 embedder 可用时传入。
**理由**：直接消掉已声明的技术债；接口可空保证降级。

### 5. CLI（交互层）
- `kb import <topic> <file>`：读文件 → 分块 → embed → 入库（重建主题）
- `kb list`：主题与块数
- `kb search <topic> <query>`：检索 topK 打印（调试用）
Web 画像页下方附知识库状态（主题/块数），复用 `kb list` 数据。

### 6. Web 集成
`/api/kb` 接口返回各主题块数；画像页展示。出题注入对 Web 与 CLI 一致（都在 orchestration 层，天然共享）。

## Risks / Trade-offs

- [小米 MiMo 平台可能无 embeddings 端点] → 配置化 + 降级：首次 `kb import` 失败给出清晰错误提示换服务；无 embedding 时全功能可用
- [暴力余弦随块数增长变慢] → 单主题数百块内可接受；后续 change 换 ANN 或向量库
- [向量去重误合并（0.75 阈值误判）] → 阈值对齐 TechSpar 已验证参数；文本过短（<5 字）回退精确匹配

## Migration Plan

纯新增。`knowledge_chunks` 表由 store 迁移自动建表；画像表结构不变（去重逻辑在应用层）。

## Open Questions

无——范围已确认（CLI 导入、专项出题注入、薄弱点向量去重、可配置降级）。
