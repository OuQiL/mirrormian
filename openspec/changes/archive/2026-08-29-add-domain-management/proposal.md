## Why

mirror-mian 的领域（主题）概念已分散存在：知识库按主题（kb import）、画像掌握度按主题、训练按主题，但**没有统一的管理层**——不能建/删/改名领域、看不到领域下有什么内容、知识库只能在 CLI 导入。仿 TechSpar 的「训练领域」设计，需要一个领域管理模块作为动态出题的底座。

## What Changes

- **领域 CRUD**：`domain create <name>` / `domain delete <name>` / `domain rename <old> <new>` / `domain list`（显示块数、训练场数、掌握度）
- **领域内容管理**：知识库 Markdown 文件查看/编辑/导入（`domain kb edit <topic> <file>` 等），新增**高频题库**（仿 TechSpar high_freq：每领域一份高频题/易错点清单，参与出题）
- **文件目录存储**：领域内容存 `kb/<topic>/*.md`（git 可管理、编辑器可改），CLI/Web 操作时同步重建向量
- **Web 界面**：8012 Web 端新增「领域」页——领域列表、领域详情（知识库文件编辑、高频题编辑、统计）

## Capabilities

### New Capabilities

- `domain-management`: mirror-mian 的领域管理能力——领域 CRUD、领域内容（知识库/高频题库）文件化管理、Web 管理界面

### Modified Capabilities

（无——rag-embedding 的检索接口不变，仅新增内容来源）

## Impact

- **新增**：`internal/domain/`（领域服务：CRUD + 文件管理 + 统计）、`kb/` 目录（领域内容文件）、Web 领域页、CLI domain 子命令
- **修改**：`internal/rag`（支持文件目录作为知识库来源、高频题注入）、`internal/web`（领域接口与页面）、`internal/store`（无表变更，topic 重命名需要更新 weak_points/mastery/knowledge_chunks）
- **风险**：低——纯新增能力；重命名领域涉及多表 topic 更新（事务内完成）
