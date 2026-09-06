## Purpose

定义 mirror-mian 的 Milvus 向量库接入：知识库向量存储与检索迁移到 Milvus（1024 维 COSINE），与 SQLite 双写；Milvus 不可用时向量路自动回退 SQLite 暴力余弦，检索行为不变。

## ADDED Requirements

### Requirement: Milvus 向量检索

系统 SHALL 将向量路检索迁移到 Milvus：集合 `knowledge_chunks`（dim=1024、COSINE 度量、按 topic 过滤），查询返回余弦相似度 topK；Milvus 地址可配置（MILVUS_ADDR，默认 localhost:19530）。

#### Scenario: 向量检索走 Milvus

- **WHEN** Milvus 可用且执行检索
- **THEN** 向量路查询 Milvus 返回 topK（按 topic 过滤）

#### Scenario: 降级回退

- **WHEN** Milvus 不可用（未启动/连接失败）
- **THEN** 向量路回退 SQLite 暴力余弦，检索结果可用

### Requirement: 双写与同步

系统 SHALL 在知识库同步（domain sync / kb import）时向量双写：Milvus + SQLite；同一同步操作 SHALL 保证两边一致（Milvus 失败不阻断 SQLite 写入，记录降级）。

#### Scenario: 同步双写

- **WHEN** 执行领域同步
- **THEN** 向量写入 Milvus 与 SQLite，BM25 索引重建

#### Scenario: Milvus 写入失败

- **WHEN** Milvus 不可用时同步
- **THEN** SQLite 正常写入并提示 Milvus 降级，检索走回退路径
