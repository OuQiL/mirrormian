## Purpose

定义 mirror-mian 的 RAG 与 Embedding 能力：知识库的分块、向量化与检索，出题时的知识上下文注入，薄弱点的向量去重；Embedding 云端接入可配置，未配置时全链路可降级使用。

## ADDED Requirements

### Requirement: Embedding 接入与降级

系统 SHALL 通过 OpenAI 兼容 `/embeddings` 端点提供文本向量化（base_url/api_key/model 独立配置，未配置时默认复用 LLM 配置）；Embedding 不可用时，系统 SHALL 降级运行：薄弱点去重回退精确匹配、出题不注入知识上下文，均不崩溃。

#### Scenario: 配置存在时向量化

- **WHEN** 配置了 embedding 端点并提交文本
- **THEN** 返回该文本的向量（Float32 数组），可参与检索与相似度计算

#### Scenario: 未配置时降级

- **WHEN** 未配置 embedding 端点（或调用失败）时触发薄弱点沉淀与出题
- **THEN** 薄弱点按精确匹配去重、出题不含知识上下文，流程正常完成并记录降级提示

### Requirement: 知识库管理

系统 SHALL 支持按主题维护知识库：`kb import <topic> <file>` 导入 Markdown 材料（分块 1000 字符、块间 150 字符重叠，向量化后入库），`kb list` 查看各主题的块数；重复导入同一主题 SHALL 重建该主题向量。

#### Scenario: 导入材料

- **WHEN** 执行 `kb import Redis ./redis-notes.md`
- **THEN** 材料按分块规则切分、向量化并入库，`kb list` 可见 Redis 主题与块数

#### Scenario: 重复导入

- **WHEN** 再次导入同一主题的新材料
- **THEN** 该主题旧向量被替换，无重复块残留

### Requirement: RAG 参与出题

专项面试出题 SHALL 检索该主题知识库（topK 5），把检索到的知识块拼入出题上下文；知识库为空或检索失败时 SHALL 正常出题（仅凭知识点名）。

#### Scenario: 有知识库时出题

- **WHEN** 主题知识库存在且完成向量化，开始专项面试
- **THEN** 出题上下文包含检索到的知识块，题目可引用材料内容

#### Scenario: 知识库为空时出题

- **WHEN** 主题无知识库内容
- **THEN** 出题行为与未接入 RAG 前一致，不报错

### Requirement: 薄弱点向量去重

薄弱点沉淀 SHALL 使用向量相似度去重：新薄弱点与既有薄弱点余弦相似度 ≥0.75 时按命中处理（times_seen++、退化标记、SM-2 重置）；Embedding 不可用时回退精确匹配（见「Embedding 接入与降级」）。

#### Scenario: 相似表述合并

- **WHEN** 新薄弱点「Redis 持久化机制理解不足」与既有「Redis 持久化理解不深」的向量相似度 ≥0.75
- **THEN** 不新建薄弱点，按既有薄弱点重复触发处理

#### Scenario: 不相似不合并

- **WHEN** 新薄弱点与所有既有薄弱点相似度 <0.75
- **THEN** 创建新薄弱点
