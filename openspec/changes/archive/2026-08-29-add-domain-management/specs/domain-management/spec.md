## Purpose

定义 mirror-mian 的领域管理能力：领域的创建、删除、重命名与列表，领域内容的文件化管理（核心知识库 Markdown 与高频题库），以及 Web 管理界面；领域是知识库、训练与画像的共同组织单元。

## ADDED Requirements

### Requirement: 领域 CRUD

系统 SHALL 支持领域的创建、删除、重命名与列表；领域内容 SHALL 存储于 `kb/<topic>/` 文件目录（Markdown），可被编辑器直接修改；删除领域 SHALL 清理该领域的知识库块与画像数据；重命名 SHALL 同步更新知识库、掌握度与薄弱点中的主题名。

#### Scenario: 创建领域

- **WHEN** 执行 `domain create Redis`
- **THEN** 创建 `kb/Redis/` 目录（含 README 模板），`domain list` 可见该领域

#### Scenario: 删除领域

- **WHEN** 执行 `domain delete Redis`
- **THEN** `kb/Redis/` 目录移除，该主题的知识库块、掌握度、薄弱点同步清除

#### Scenario: 重命名领域

- **WHEN** 执行 `domain rename Redis KVStore`
- **THEN** 目录改名为 `kb/KVStore/`，知识库块/掌握度/薄弱点的主题统一更新为 KVStore，后续训练按新名归集

#### Scenario: 列出领域

- **WHEN** 执行 `domain list`
- **THEN** 展示各领域的知识库块数、训练场数（掌握度）与文件数

### Requirement: 领域内容管理

每个领域 SHALL 包含两类内容：**核心知识库**（Markdown 文件，参与出题检索）与**高频题库**（`high_freq.md`，面试常见问题/易错点清单）；CLI 与 Web SHALL 支持查看与编辑；文件变更后 SHALL 提供同步命令（重建该领域向量）。

#### Scenario: 编辑知识库

- **WHEN** 通过 CLI 或 Web 编辑领域的核心知识库文件
- **THEN** 文件落盘到 `kb/<topic>/`，执行同步后该领域向量更新，检索/出题立即使用新内容

#### Scenario: 高频题库生效

- **WHEN** 领域存在 `high_freq.md` 且执行同步
- **THEN** 该领域专项面试出题时注入高频题上下文（与知识库检索并列）

### Requirement: Web 领域管理

Web 端 SHALL 提供「领域」页面：领域列表（块数/场数/掌握度）、领域详情（知识库文件浏览与编辑、高频题编辑、同步与统计）。

#### Scenario: 领域列表页

- **WHEN** 打开领域页
- **THEN** 展示全部领域及统计信息

#### Scenario: 领域详情页

- **WHEN** 进入某领域详情
- **THEN** 可查看/编辑知识库文件与高频题，变更后一键同步向量

### Requirement: 同步机制

系统 SHALL 提供领域内容到向量的同步：新增/修改/删除 `kb/<topic>/` 文件后，执行同步 SHALL 重建该领域全部向量；未同步的变更 SHALL NOT 影响出题（出题使用已同步的向量）。

#### Scenario: 同步后生效

- **WHEN** 修改知识库文件并执行同步
- **THEN** 检索返回新内容，出题使用新内容

#### Scenario: 未同步不影响

- **WHEN** 修改文件但未执行同步
- **THEN** 出题/检索仍基于旧向量（与文件内容暂时不一致属预期）
