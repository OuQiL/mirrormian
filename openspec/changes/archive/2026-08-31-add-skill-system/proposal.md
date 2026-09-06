## Why

系统目前只有一种固定玩法（面试三阶段写死在编排层）。加新交互玩法（快速测验、概念教学、复习回顾）需要改编排代码。按用户要求引入 **Skill 技能系统**：有状态多轮交互的 Skill 接口（区别于无状态 Tool）、统一注册的 SkillRegistry（优先级匹配分发）、4 个内置 Skill，让玩法可插拔扩展。

## What Changes

- **Skill 接口**（internal/skill）：`Skill`（Name/Description/MatchScore）+ `Session`（有状态多轮，持久化到 SQLite `skill_sessions` 表）+ `TurnResult`
- **SkillRegistry**：注册/列表/按输入优先级匹配（`Match(ctx, input) → 最高分 Skill`），统一分发入口
- **4 个内置 Skill**：
  - `quick-quiz` 快速测验：领域主题 → 逐题 → 即时判定 + 讲解 → 计分总结
  - `concept-tutor` 概念教学：知识点 → 讲解 → 提问确认理解 → 未懂换角度再讲 → 深入
  - `interview` 专项面试：包装现有 orchestration（方向→逐题→追问→复盘，复用编排层）
  - `review` 复习回顾：画像薄弱点逐个过 → 复习反馈 → 更新 SM-2
- **入口**：CLI `skill list` / `skill run <请求>`（按意图匹配）/ `skill <name>`（指定）；Web 技能页（列表 + 多轮聊天交互，复用聊天 UI 样式）

## Capabilities

### New Capabilities

- `skill-system`: Skill 技能系统——有状态多轮 Skill 接口、SkillRegistry 优先级匹配、会话持久化、内置技能注册与入口

### Modified Capabilities

（无——现有面试流程被 Skill 包装，行为不变）

## Impact

- **新增**：`internal/skill/`（接口/Registry/会话存储/4 个内置 Skill）、`skill` 命令、Web 技能页与接口、`skill_sessions` 表
- **修改**：`internal/store`（skill_sessions 表）、`cmd`、`internal/web`（路由/页面）
- **风险**：低——纯新增；interview Skill 包装现有链路不改变其行为
