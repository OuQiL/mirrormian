## Context

现有 4 个 Agent（方向规划/出题/面试官/复盘）。综合面试的方向由 DirectionPlanner.PlanFull 一次性完成（JD 分析+简历匹配+方向），无独立匹配报告；复习只有到期列表（DueReviews），无行动计划。本 change 加 2 个 Agent，并让综合面试走「分析 → 差距 → 方向」链路。

## Goals / Non-Goals

**Goals:**
- JDResumeAnalyzer：解析 + 匹配度分析（对位/评分/差距）
- ReviewPlanner：画像 → 行动化复习计划
- PlanFull 改造：基于匹配分析生成方向（输出结构兼容）
- analyze / plan 命令 + Web 复习页计划按钮

**Non-Goals:**
- 不做简历/JD 文件解析（PDF/DOCX 为后续 change，本 change 输入为文本）
- 不做复习计划持久化（每次生成即时输出）
- 不改复盘 Agent（Reviewer 保持）

## Decisions

### 1. JDResumeAnalyzer（internal/agents/jd_resume_analyzer.go）
```go
type JDAnalysis struct {
    Responsibilities []string  `json:"responsibilities"`   // 岗位职责
    Requirements     []Requirement `json:"requirements"`   // 任职要求（逐项）
    TechStack        []string  `json:"tech_stack"`         // 技术栈
    MatchScore       int       `json:"match_score"`        // 0-100
    Gaps             []Gap     `json:"gaps"`               // 差距（严重度降序）
    Summary          string    `json:"summary"`            // 总评
}
type Requirement struct {
    Item    string `json:"item"`
    Level   string `json:"level"`    // hard / plus
    Status  string `json:"status"`   // met / partial / missing
    Evidence string `json:"evidence"` // 简历中的支撑
}
type Gap struct {
    Item     string `json:"item"`
    Severity string `json:"severity"` // high / medium / low
    Suggestion string `json:"suggestion"`
}
```
一次 LLM 调用输出完整 JSON。prompt 强调：逐项对位必须引用简历证据；差距按严重度排序；评分基于满足率加权。

### 2. 综合面试接线（planner.go PlanFull 改造）
`PlanFull` 内部：`analyzer.Analyze(ctx, resume, jd)` → 基于分析结果生成方向：
- `KeyPoints`：差距项（high/medium）优先 + JD 核心要求的技术栈知识点
- `JDSummary`：分析摘要；`ResumeSummary`：匹配总评
输出结构不变（Direction 字段兼容），orchestration 与 Web 无需改动。
**理由**：保持对外结构稳定；出题方向天然聚焦差距（spec「差距优先」）。

### 3. ReviewPlanner（internal/agents/review_planner.go）
```go
type ReviewPlan struct {
    Summary   string      `json:"summary"`    // 今日计划概述
    Items     []PlanItem  `json:"items"`      // 按优先级排序
}
type PlanItem struct {
    Topic     string `json:"topic"`
    Action    string `json:"action"`     // 练什么（到期复习/专项训练/补知识点）
    Priority  string `json:"priority"`   // high/medium/low
    Suggestion string `json:"suggestion"` // 练习方式建议
    Minutes   int    `json:"minutes"`    // 预计耗时
}
```
输入：画像 JSON（薄弱点含 SM-2、掌握度、到期项）拼进 prompt；空画像返回提示（不调 LLM 也可——直接判断）。到期复习项自动排 high。
**理由**：把到期复习转化为行动化计划，连接画像与每日练习。

### 4. 入口
- `analyze --jd <file> --resume <file>`：读文件 → Analyzer → 打印报告（CLI）
- `plan`：读画像 → Planner → 打印计划
- Web：`POST /api/analyze`（可选，后续）、`POST /api/plan` + 复习页「生成复习计划」按钮
**范围**：本 change Web 只加 plan 按钮（analyze 走 CLI，Web 后续）。

## Risks / Trade-offs

- [LLM 匹配评分不精准] → prompt 要求引用证据 + 满足率加权；评分仅参考，差距清单是主要产物
- [PlanFull 改造影响综合面试] → 输出结构兼容 + 既有 E2E 测试守护；分析失败时回退原 PlanFull 逻辑
- [复习计划无持久化] → 即时生成够用；后续如需历史计划再加表

## Migration Plan

纯新增。PlanFull 改造保持 Direction 结构不变，无需数据迁移。

## Open Questions

无——范围已确认（两个 Agent + 两个命令 + Web plan 按钮）。
