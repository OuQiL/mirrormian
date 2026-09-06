## Purpose

定义 mirror-mian 的 RAG 多路召回能力：BM25 关键词检索与向量检索双路并行召回，RRF(k=60) 融合排序，LLM Rerank 重排序输出最终候选题；外部检索接口行为不变。

## ADDED Requirements

### Requirement: BM25 关键词检索

系统 SHALL 维护知识库块的内存 BM25 索引（中文 bigram + 英文词分词），按查询关键词打分返回 topK；索引 SHALL 在知识库同步（domain sync / kb import）时构建或重建。

#### Scenario: 关键词检索

- **WHEN** 对知识库执行关键词查询（如「AOF 重写」）
- **THEN** 返回按 BM25 打分排序的块（精确包含关键词的块优先）

#### Scenario: 索引重建

- **WHEN** 领域内容同步后
- **THEN** BM25 索引随之重建，检索使用新内容

### Requirement: RRF 融合排序

系统 SHALL 将向量路与 BM25 路的召回结果按 RRF 融合：`score = Σ 1/(k + rank)`，k=60；融合后按分数降序输出。

#### Scenario: 融合排序

- **WHEN** 向量路与 BM25 路均返回结果
- **THEN** 输出按 RRF 分数降序的融合列表（两路排名都靠前的块优先）

#### Scenario: 单路可用

- **WHEN** 某一路无结果（如 Embedding 未配置）
- **THEN** 用另一路结果直接输出，不报错

### Requirement: LLM Rerank

系统 SHALL 提供 Reranker：对融合后的 topN 候选由 LLM 按查询相关性打分并重排；Reranker 类型可配置（llm / none，默认 llm 可用时启用、否则 none）。

#### Scenario: LLM 重排

- **WHEN** 融合结果交 LLM Rerank
- **THEN** 输出按 LLM 相关性分数降序的最终结果

#### Scenario: 降级

- **WHEN** LLM 不可用或 Rerank 失败
- **THEN** 回退使用融合排序结果，不报错
