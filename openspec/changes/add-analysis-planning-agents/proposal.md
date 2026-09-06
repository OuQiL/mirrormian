## Why

综合面试目前由 DirectionPlanner 顺带完成 JD/简历分析，但缺少**独立的匹配度分析**：看不到"简历与 JD 的差距在哪、哪些要求不满足"，出题方向无法精准聚焦差距。另外数据飞轮积累的画像（薄弱点/掌握度/SM-2）缺少**复习规划**出口——到期复习只是列表，没有"今天练什么、按什么顺序"的行动计划。

## What Changes

- **新增 JD/简历分析 Agent（JDResumeAnalyzer）**：
  - 解析 JD（职责/任职要求/加分项/技术栈）
  - 解析简历（技能/项目/经验）
  - 输出**匹配度分析**：逐项要求对位（满足/部分/缺失）、匹配评分、差距清单
  - 综合面试准备阶段改为「先分析匹配 → 基于差距生成出题方向」（差距优先考察）
  - `analyze --jd <file> --resume <file>` 命令查看完整匹配报告
- **新增复习规划 Agent（ReviewPlanner）**：
  - 输入画像（薄弱点 + SM-2 状态 + 掌握度 + 到期复习）
  - 输出复习计划：今日行动项（按优先级/主题）、每项练什么/预计耗时、顺序建议
  - `plan` 命令 + Web 复习页「生成复习计划」按钮

## Capabilities

### New Capabilities

- `jd-resume-analysis`: JD/简历解析与匹配度分析——要求拆解、对位判断、差距清单、匹配评分，驱动综合面试出题方向
- `review-planning`: 复习规划——基于画像生成行动化复习计划（优先级/顺序/耗时）

### Modified Capabilities

（无）

## Impact

- **新增**：`internal/agents/jd_resume_analyzer.go`、`internal/agents/review_planner.go`、`analyze` 命令、`plan` 命令、Web 复习页计划按钮
- **修改**：`internal/agents/planner.go`（PlanFull 改为基于匹配分析）、`internal/orchestration`（综合面试接线）
- **风险**：低——纯新增 Agent；PlanFull 改造保持输出结构兼容（Direction.KeyPoints/JDSummary/ResumeSummary）
