## Purpose

定义 mirror-mian 的 Skill 技能系统：有状态多轮交互的 Skill 接口（区别于无状态 Tool）、SkillRegistry 统一注册与优先级匹配、会话持久化，以及 4 个内置 Skill（快速测验/概念教学/专项面试/复习回顾）的注册与入口。

## ADDED Requirements

### Requirement: Skill 接口与会话

系统 SHALL 提供有状态多轮交互的 Skill 接口：Skill 声明名称/描述/匹配评分；会话（Session）SHALL 持久化（skill_sessions 表），每轮交互通过 Turn 推进并保存状态；Finish 结束会话并返回结果。

#### Scenario: 多轮交互

- **WHEN** 对 Skill 会话提交多轮输入
- **THEN** 每轮返回该轮结果且会话状态持久化，中断后可恢复继续

#### Scenario: 会话恢复

- **WHEN** 按会话 ID 恢复
- **THEN** 从持久化状态继续多轮交互

### Requirement: SkillRegistry 优先级匹配

系统 SHALL 提供 SkillRegistry：注册技能、按输入文本计算各技能匹配分、返回最高分技能（低于阈值则报无可匹配技能）。

#### Scenario: 按意图匹配

- **WHEN** 输入「考考我 Redis 概念」
- **THEN** 匹配到快速测验技能（匹配分最高），可启动对应会话

#### Scenario: 无可匹配

- **WHEN** 输入与所有技能匹配分均低于阈值
- **THEN** 返回可读提示并列出可用技能

### Requirement: 内置技能

系统 SHALL 内置 4 个技能：快速测验（逐题即时判定+讲解+计分）、概念教学（讲解→提问确认→未懂换角度）、专项面试（包装现有面试编排，行为不变）、复习回顾（画像薄弱点逐个过，复习反馈更新 SM-2）。

#### Scenario: 快速测验

- **WHEN** 选择主题开始快速测验
- **THEN** 逐题出题，每题即时判定并讲解，结束输出成绩与薄弱点

#### Scenario: 概念教学

- **WHEN** 选择知识点开始概念教学
- **THEN** 先讲解，再提问确认理解；答错换角度重讲，确认掌握后深入下一层

#### Scenario: 专项面试技能

- **WHEN** 通过技能入口启动专项面试
- **THEN** 行为与现有面试流程一致（准备→面试→复盘→画像沉淀）

#### Scenario: 复习回顾

- **WHEN** 启动复习回顾
- **THEN** 按画像薄弱点逐个提问复习，反馈评分更新 SM-2 状态

### Requirement: 入口与分发

系统 SHALL 提供统一入口：CLI `skill run <请求>`（按意图匹配启动）、`skill <name>`（指定技能）、`skill list`；Web 技能页支持列表与多轮聊天交互。

#### Scenario: 智能分发

- **WHEN** 执行 `skill run "帮我复习 MySQL 索引"`
- **THEN** 匹配复习回顾技能并开始会话

#### Scenario: Web 交互

- **WHEN** 在 Web 技能页选择技能并对话
- **THEN** 多轮聊天正常流转，会话可随时结束
