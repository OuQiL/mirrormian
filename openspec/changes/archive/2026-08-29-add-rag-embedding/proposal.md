## Why

roadmap 第一项：数据飞轮缺 Embedding/RAG 两环。当前薄弱点用「主题 + 规范化文本」精确去重（表述稍变就重复沉淀，是已声明的技术债）；出题 Agent 只凭知识点名命题，没有知识库上下文（题目深度受限，且无法利用用户沉淀的学习材料）。

## What Changes

- 新增 **Embedding 客户端**（OpenAI 兼容 `/embeddings` 端点，base_url/api_key/model 独立配置，默认复用 LLM 配置）
- 新增 **知识库**：按主题维护 Markdown 材料 → 分块（1000 字符 + 150 重叠）→ 向量化 → SQLite 存储；CLI 提供 `kb import <topic> <file>` 与 `kb list`
- **RAG 参与出题**：专项面试出题时检索知识库 topK（5）拼入出题上下文，题目可结合用户材料（仿 TechSpar 四路输入中的知识库一路）
- **薄弱点向量去重**：`AbsorbAnswer` 升级为 embedding 余弦相似度（阈值 0.75，TechSpar 同参数）合并既有薄弱点；Embedding 未配置时自动回退精确匹配，不崩溃
- Web 端画像页展示知识库状态（可选，简单展示主题与块数）

## Capabilities

### New Capabilities

- `rag-embedding`: mirror-mian 的 RAG 与 Embedding 能力——知识库（分块/向量化/检索）、出题注入、薄弱点向量去重、Embedding 云端接入（可配置、可降级）

### Modified Capabilities

（无——web-interaction 能力不受影响）

## Impact

- **新增**：`internal/embedding/`（Embedder 接口 + OpenAI 兼容实现）、`internal/rag/`（分块/向量存储/检索）
- **修改**：`internal/store`（knowledge_chunks 表）、`internal/profile`（向量去重）、`internal/agents/questioner`（知识上下文注入）、`internal/orchestration`（出题接线）、`cmd`（kb 子命令）、`internal/config`（embedding 配置项）
- **依赖**：无新增第三方库（embedding 走 HTTP，JSON Float32）
- **风险**：中——Embedding API 为外部依赖；未配置时全链路降级可用（精确匹配 + 无知识上下文），spec 中约束降级行为
