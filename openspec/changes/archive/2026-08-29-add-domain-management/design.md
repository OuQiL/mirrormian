## Context

mirror-mian 已有：RAG 知识库（`kb import` 按主题入库，SQLite 向量表）、画像（掌握度/薄弱点按主题）、专项训练按主题。领域概念分散，无统一管理。本 change 引入文件目录作为领域内容的单一来源（对齐 TechSpar「题库按训练领域组织 + 核心知识库/高频题库」设计）。

## Goals / Non-Goals

**Goals:**
- 领域 CRUD + 内容文件化管理（`kb/<topic>/`）+ 高频题库（high_freq.md）
- 重命名/删除时多表主题一致性（事务）
- Web 领域管理页 + CLI domain 子命令
- 同步机制：文件 → 向量重建

**Non-Goals:**
- 不做领域画像详情页（领域掌握度/薄弱点汇总在画像页已按主题展示，本 change 只做列表统计）
- 不做浏览器内 Markdown 富文本编辑——textarea 编辑即可
- 不自动监听文件变更（手动同步，简单可靠）

## Decisions

### 1. 文件目录即领域（kb/<topic>/）
```
kb/<topic>/
├── README.md        # 领域总览（创建时自动生成模板）
├── *.md             # 核心知识库（可拆分主题）
└── high_freq.md     # 高频题库（可选，创建时为空模板）
```
`domain list` 扫描目录即得领域清单（单一来源）；`kb import <topic> <file>` 演进为「导入文件到该领域目录 + 同步」。
**理由**：git 可管理、编辑器可改、与现有 kb 命令兼容。
**备选**：SQLite 管理领域清单——文件系统已是事实来源，避免双份状态。

### 2. domain 服务（internal/domain）
```go
type Service struct {
    root string        // kb/ 根目录（配置 MIRROR_KB_PATH，默认 ./kb）
    store store.Store
    embedder embedding.Embedder
    rag     *rag.Service
}
```
- Create/Delete/Rename/List：文件系统操作 + store 一致性更新
- Rename 实现：文件系统移动目录 + 事务内更新 knowledge_chunks/mastery/weak_points 的 topic（SQL UPDATE）
- Delete 实现：删目录 + 事务内删除对应行
- Sync(topic)：扫描目录全部 .md（含 high_freq.md）→ ChunkText → embed → SaveTopicChunks（重建）
- Read/Write(topic, file)：文件读写 + 统计（块数来自 ListTopicChunks）

### 3. 高频题库注入
rag.Service 增加 `HighFreq(topic)`：读取 `kb/<topic>/high_freq.md`，同步时作为独立块类型（chunk 带来源标记 `high_freq`）或单独读取。出题时 questioner 的 knowledgeContext 追加高频题段（与知识库检索并列）。
**实现选择**：同步时把 high_freq.md 的内容作为单独一组合并进 Context 输出（`[高频考点]` 段落，全部注入不检索——文件通常短）。

### 4. 同步语义
`domain sync <topic>`（或 `domain sync --all`）：扫描文件 → 分块 → embed → 重建该领域向量。`kb import` 兼容为「写入文件 + sync」。
Web 详情页「同步」按钮调 `/api/domain/<topic>/sync`。

### 5. Web 接口与页面
```
GET  /api/domain              → 领域列表（统计）
POST /api/domain              → {name} 创建
DELETE /api/domain/<topic>    → 删除
POST /api/domain/<topic>/rename → {new_name}
GET  /api/domain/<topic>      → 文件列表 + 内容
PUT  /api/domain/<topic>/file → {file, content} 保存
POST /api/domain/<topic>/sync → 重建向量
```
页面：领域列表（卡片：名称、块数、场数、掌握度、操作）→ 详情（文件树 + 编辑区 + 高频题编辑 + 同步按钮）。

### 6. CLI
`domain create|delete|rename|list|sync|files <topic>`；`kb import/list/search` 保留兼容（import = 写入文件 + sync）。

## Risks / Trade-offs

- [文件系统与 SQLite 双状态可能漂移] → 文件是唯一事实来源，向量是派生物（可随时 sync 重建）；未同步时检索用旧向量（spec 明示预期）
- [重命名跨表更新遗漏] → 全部在事务内 SQL UPDATE；测试覆盖三表主题一致性
- [目录路径含中文/空格] → Go 的 filepath 天然支持；URL 编码处理 topic 参数

## Migration Plan

现有 `knowledge_chunks` 数据不受影响（kb/ 目录为空时以库内数据为准，首次 sync 后迁移到文件+向量双份）。

## Open Questions

无——范围已确认（CRUD + 内容管理 + Web，文件目录存储）。
