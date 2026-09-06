## 1. domain 服务

- [x] 1.1 创建 `internal/domain`：Service（root 配置 MIRROR_KB_PATH 默认 ./kb）+ Create/Delete/Rename/List，验证单测覆盖建/删/改名及目录结构
- [x] 1.2 Rename/Delete 的 store 一致性：事务内更新 knowledge_chunks/mastery/weak_points 主题，验证单测三表一致
- [x] 1.3 Sync(topic)：扫描 `kb/<topic>/*.md`（含 high_freq.md）→ 分块 → 向量化 → 重建，验证单测文件变更后同步生效
- [x] 1.4 high_freq.md 支持：rag 提供高频题上下文段落（与知识库检索并列注入出题），验证单测注入后 prompt 含高频段

## 2. CLI

- [x] 2.1 实现 `domain create/delete/rename/list/sync/files <topic>`，验证命令行为与错误提示
- [x] 2.2 `kb import` 兼容为「写入 kb/<topic>/ + sync」，验证既有导入流程不变

## 3. Web

- [x] 3.1 `/api/domain` 系列接口（列表/创建/删除/重命名/详情/保存文件/同步），验证接口行为
- [x] 3.2 领域页：列表（统计卡片）+ 详情（文件树/编辑区/高频题/同步按钮），验证浏览器端 CRUD 与同步全流程

## 4. 验证与收尾

- [x] 4.1 `go test ./...`、`go vet`、`go build` 全部通过；真实链路：建领域 → 写知识库 → 同步 → 专项面试（含高频题注入）
- [x] 4.2 openspec validate 通过；改动范围仅限 mirror-mian 内新增/修改文件
