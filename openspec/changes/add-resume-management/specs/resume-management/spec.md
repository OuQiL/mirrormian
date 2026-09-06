## Purpose

定义 mirror-mian 的简历管理能力：多份简历的上传与解析（PDF/DOCX/TXT/MD）、列表与删除、解析文本查看，以及综合面试时从已上传简历中选择作为输入。

## ADDED Requirements

### Requirement: 简历上传与解析

系统 SHALL 支持上传简历文件（PDF/DOCX/TXT/MD），解析结果 SHALL 统一转为 Markdown 文本并持久化（原文件存 `resumes/`，元数据与 Markdown 文本存 SQLite）；解析失败 SHALL 报明确错误且不入库。

#### Scenario: 上传 PDF 简历

- **WHEN** 上传 PDF 简历
- **THEN** 解析为 Markdown 文本成功入库（ext 记 md），列表可见该简历

#### Scenario: 解析失败

- **WHEN** 上传无法解析的文件（如损坏 PDF）
- **THEN** 返回可读错误，列表不出现该简历

### Requirement: 粘贴上传简历

系统 SHALL 支持粘贴文本直接保存为简历（不经文件上传），文本同样按 Markdown 入库。

#### Scenario: 粘贴保存

- **WHEN** 在简历页粘贴文本并保存
- **THEN** 生成一条 Markdown 简历记录，可被综合面试选择

### Requirement: 简历列表与删除

系统 SHALL 提供简历列表（ID/文件名/格式/上传时间/文本长度）与删除；删除 SHALL 移除文件与记录。

#### Scenario: 列出简历

- **WHEN** 请求简历列表
- **THEN** 返回全部简历的元数据

#### Scenario: 删除简历

- **WHEN** 删除指定简历
- **THEN** 文件与数据库记录一并移除

### Requirement: 简历文本查看

系统 SHALL 提供简历解析文本的查看接口（供用户在综合面试前确认内容）。

#### Scenario: 查看文本

- **WHEN** 请求某简历详情
- **THEN** 返回解析后的完整文本

### Requirement: 综合面试选择简历

综合面试 SHALL 仅允许通过简历 ID 使用已上传简历（Web 下拉选择必选 / CLI `--resume-id`）；未选择简历时 SHALL 提示前往简历页添加（`--resume <file>` 保持兼容）。

#### Scenario: Web 选择简历

- **WHEN** 综合面试表单选择某简历
- **THEN** 面试使用该简历的 Markdown 文本进行匹配分析与出题

#### Scenario: 未选择简历

- **WHEN** 综合面试表单未选择简历
- **THEN** 提示「请选择简历（可在简历页上传或粘贴）」，不进入面试

#### Scenario: CLI 指定简历

- **WHEN** 执行 `train full --jd <file> --resume-id <id>`
- **THEN** 使用该简历文本，行为与 `--resume <file>` 一致
