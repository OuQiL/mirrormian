## Context

向量路当前是 SQLite BLOB + 暴力余弦。multi-recall 已就位（BM25 + RRF + LLM Rerank），本 change 仅替换向量路实现。蓝本 interview-agent 用 milvus-sdk-go v2.4.2 + MilvusStore（按用户过滤）可参考。

## Goals / Non-Goals

**Goals:**
- Milvus 集合管理（自动建集合/索引）、插入/删除/按 topic 检索
- 向量路 Milvus 优先、SQLite 回退
- sync 双写；docker-compose 一键起 Milvus

**Non-Goals:**
- 不做分区策略（单用户场景，topic 字段过滤足够）
- 不做 Milvus 高可用/集群（standalone 单机）
- 不改 BM25/RRF/Rerank 链路

## Decisions

### 1. Milvus 部署（docker-compose.yml）
官方 standalone 三件套（etcd + minio + milvus），端口 19530（gRPC）+ 9091（HTTP）；镜像较大（首启拉取数分钟）。`docker compose up -d milvus` 启动。

### 2. internal/vector（Milvus 客户端）
```go
type Milvus struct {
    client  client.Client
    addr    string
    ok      bool  // 连接成功标记
}
func New(addr string) *Milvus
func (m *Milvus) Available() bool
func (m *Milvus) Upsert(ctx, topic string, chunks []store.KnowledgeChunk) error // 先删该 topic 再插
func (m *Milvus) Delete(ctx, topic string) error
func (m *Milvus) Search(ctx, topic string, queryVec []float32, topK int) ([]vectorHit, error)
```
集合 schema：`id`(int64 PK, auto) + `topic`(varchar) + `chunk_id`(int64) + `embedding`(float_vector 1024)；索引 HNSW(COSINE)。查询 `expr: topic == "X"` 过滤。
**理由**：蓝本同款 SDK 与用法；维度固定 1024（bge-m3）。

### 3. rag.Service 向量路改造
```go
type vectorStore interface {
    Search(ctx, topic, queryVec, topK) ([]vectorHit, error)
    Available() bool
}
```
rag.Service 增加 `milvus vectorStore`（可 nil）：Retrieve 向量路：
- milvus != nil && Available → Milvus Search
- 否则 → SQLite 暴力余弦（现有逻辑）
**理由**：接口隔离，回退路径零改动。

### 4. domain.Sync 双写
SaveTopicChunks(SQLite) 后：`milvus.Upsert`（失败仅记录日志，不阻断）；RebuildIndex(BM25) 照旧。
Milvus 不可用时 Upsert 返回错误 → Sync 打印「Milvus 降级」提示但继续成功。

### 5. 配置与生命周期
`MILVUS_ADDR`（默认 `localhost:19530`）；web/cmd 启动时 `vector.New`（连接成功标记 ok，失败不 panic）。kb 导入/删除同步调用 Milvus。

## Risks / Trade-offs

- [Milvus 镜像大、首启慢] → compose 一键起；未启动时全链路降级可用（spec 约束）
- [Milvus 与 SQLite 数据不一致] → sync 双写同一入口；Milvus 失败只影响向量路质量（回退 SQLite）
- [SDK 版本兼容] → 用蓝本同款 v2.4.x，已验证

## Migration Plan

启动 Milvus 后，首次 sync 自动双写；既有 SQLite 向量保留（回退路径数据源）。

## Open Questions

无——范围已确认（Milvus 向量路 + 双写 + 降级 + compose）。
