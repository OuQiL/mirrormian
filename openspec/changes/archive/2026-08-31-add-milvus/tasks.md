## 1. Milvus 客户端

- [x] 1.1 `internal/vector`：New/Available/Upsert/Delete/Search（milvus-sdk-go，集合自动建+HNSW 索引，topic 过滤），验证单测 mock 或降级路径
- [x] 1.2 config 增加 MILVUS_ADDR；web/cmd 启动时连接（失败不 panic）

## 2. 检索与同步接线

- [x] 2.1 rag.Service 向量路：Milvus 优先 / SQLite 回退（接口隔离），验证单测回退路径与既有测试回归
- [x] 2.2 domain.Sync 双写：Milvus Upsert 失败不阻断 + 降级提示；kb import 同步，验证单测

## 3. 部署与验证

- [x] 3.1 docker-compose.yml 增加 Milvus 服务（etcd+minio+milvus），验证 `docker compose config` 与 `up -d milvus` 启动
- [x] 3.2 真实链路：起 Milvus → sync 双写 → kb search（向量路走 Milvus）→ 停 Milvus → 检索回退 SQLite 可用

## 4. 验证与收尾

- [x] 4.1 `go test ./...`、`go vet`、`go build` 全部通过
- [x] 4.2 openspec validate 通过；改动范围仅限 mirror-mian 内新增/修改文件
