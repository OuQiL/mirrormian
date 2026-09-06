## Why

用户要求 Milvus 向量检索（1024 维）。当前向量存 SQLite 暴力余弦，块数增长后性能下降；Milvus 提供规模化向量检索。本 change 接入 Milvus（Docker Compose standalone：etcd + MinIO + Milvus），向量路检索迁入，保留 SQLite 暴力扫描作为降级路径。

## What Changes

- docker-compose.yml 增加 Milvus 服务（etcd + minio + milvus standalone）
- `internal/vector`：Milvus 客户端（milvus-sdk-go），集合 `knowledge_chunks`（dim=1024，COSINE），按 topic 过滤
- rag.Service 向量路：Milvus 优先，不可用/失败时回退 SQLite 暴力余弦（现有逻辑保留）
- domain sync / kb import：向量写入 Milvus + SQLite 双写
- 配置 `MILVUS_ADDR`（默认 localhost:19530）；启动时自动建集合

## Capabilities

### New Capabilities

- `milvus-vector`: Milvus 向量库接入——向量存储/检索迁移、双写与降级

### Modified Capabilities

（无——multi-recall 的检索链路不变，仅向量路实现替换）

## Impact

- **新增**：`internal/vector/`（Milvus 客户端 + 集合管理）
- **修改**：`internal/rag/retrieve.go`（向量路接 Milvus）、`internal/domain/domain.go`（双写）、docker-compose.yml、`internal/config`（MILVUS_ADDR）
- **依赖**：`github.com/milvus-io/milvus-sdk-go/v2`（蓝本同款 v2.4.x）
- **风险**：中——Milvus 为外部服务；降级路径保证不可用时功能不受影响
