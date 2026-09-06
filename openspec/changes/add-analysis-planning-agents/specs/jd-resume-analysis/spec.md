## Purpose

定义 mirror-mian 的 JD/简历分析能力：解析 JD 与简历文本，输出结构化匹配度分析（要求拆解、逐项对位、差距清单、匹配评分），并驱动综合面试的出题方向（差距优先）。

## ADDED Requirements

### Requirement: JD 解析

系统 SHALL 解析 JD 文本，拆解出岗位职责、任职要求、加分项与技术栈。

#### Scenario: 解析 JD

- **WHEN** 提供 JD 文本执行分析
- **THEN** 输出结构化的岗位职责、任职要求（硬性/加分）、技术栈清单

### Requirement: 简历解析

系统 SHALL 解析简历文本，提取技能、项目经历与经验概况。

#### Scenario: 解析简历

- **WHEN** 提供简历文本执行分析
- **THEN** 输出技能清单、项目经历要点与经验概况

### Requirement: 匹配度分析

系统 SHALL 输出简历与 JD 的匹配度分析：逐项任职要求对位（满足/部分满足/缺失）、总体匹配评分（0-100）与差距清单。

#### Scenario: 生成匹配报告

- **WHEN** 完成 JD 与简历解析
- **THEN** 输出逐项对位、匹配评分与差距清单（差距按严重度排序）

### Requirement: 差距驱动的综合面试

综合面试准备阶段 SHALL 先执行匹配度分析，再基于差距生成出题方向（差距项优先）。

#### Scenario: 出题方向聚焦差距

- **WHEN** 综合面试使用简历与 JD
- **THEN** 出题方向包含匹配差距对应的知识点，且差距项在方向中优先

### Requirement: 独立分析命令

系统 SHALL 提供 `analyze --jd <file> --resume <file>` 命令，直接输出完整匹配报告（不进入面试）。

#### Scenario: 独立分析

- **WHEN** 执行 analyze 命令
- **THEN** 输出匹配报告（要求对位/评分/差距），不创建面试会话
