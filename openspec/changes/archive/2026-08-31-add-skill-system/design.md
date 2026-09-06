## Context

系统现有固定面试流程（编排层 6 Agent + Eino Graph），无状态工具（kb search/resources/抓取）已就位。本 change 引入 Skill 层：面向用户的可插拔有状态玩法，InterviewSkill 包装现有编排，其余 3 个为新实现。

## Goals / Non-Goals

**Goals:**
- Skill 接口 + Session 持久化（skill_sessions 表）
- SkillRegistry 优先级匹配分发
- 4 个内置 Skill（快速测验/概念教学/专项面试包装/复习回顾）
- CLI + Web 入口

**Non-Goals:**
- 不做 Skill 热加载（编译期注册即可，后续可扩展）
- 不做 Skill 组合编排（技能内触发其他技能，后续）
- 不改现有面试编排逻辑（interview Skill 仅包装）

## Decisions

### 1. Skill 接口（internal/skill/skill.go）
```go
type Skill interface {
    Name() string
    Description() string
    // MatchScore 输入意图匹配分（0-100；<threshold 视为不匹配）
    MatchScore(ctx context.Context, input string) int
    // Start 创建会话（返回初始提示）
    Start(ctx context.Context, input string) (*TurnResult, error)
    // Turn 多轮推进（返回该轮输出）
    Turn(ctx context.Context, sess *Session, userInput string) (*TurnResult, error)
    // Done 判断会话是否结束
}
type Session struct {
    ID        string
    SkillName string
    State     json.RawMessage // 技能私有状态
    Messages  []string        // 交互历史（展示用）
    Finished  bool
    CreatedAt time.Time
}
type TurnResult struct {
    Reply    string // 展示文本
    Finished bool
    Extra    any    // 结构化附加（可选）
}
```
状态持久化：skill_sessions 表（id, skill_name, state JSON, messages JSON, finished, created_at, updated_at），store 接口 `SaveSkillSession/GetSkillSession`。

### 2. SkillRegistry（internal/skill/registry.go）
```go
type Registry struct { skills []Skill }
func (r *Registry) Register(s ...Skill)
func (r *Registry) List() []Skill
func (r *Registry) Match(ctx, input string) (Skill, int) // 最高分（阈值 30）
func (r *Registry) NewSession(ctx, skillName, input) (*Session, *TurnResult, error)
func (r *Registry) Turn(ctx, sessionID, input) (*TurnResult, error) // 自动恢复会话
```
**理由**：Registry 只管注册/匹配/会话存取，业务在 Skill 内部。

### 3. 内置 Skill

**quick-quiz（快速测验）**：状态 `{topic, questions[]?, idx, score}`。Start：方向规划（复用 DirectionPlanner）→ 出第一题；Turn：判定回答（复用 Interviewer.JudgeAnswer）+ 讲解（LLM）+ 记分 → 下一题（复用 Questioner）；结束输出成绩 + 薄弱点（沉淀画像，复用 profile.AbsorbAnswer）。

**concept-tutor（概念教学）**：状态 `{topic, concepts[]?, idx, mode}`。讲解（LLM 生成概念讲解）→ 提问确认（Questioner 出题）→ 判定：懂 → 下一概念/深入；不懂 → 换角度重讲（LLM）。

**interview（专项面试）**：包装现有 `orchestration.Service.RunSpecial`——但 RunSpecial 是一次性循环（ask 回调），Skill 需要多轮。改为用 `StartSpecial`/`SubmitAnswer`（断点控制器，已有）包装：Start 调 StartSpecial，Turn 调 SubmitAnswer。状态持久化天然在编排层会话。

**review（复习回顾）**：状态 `{weakPoints[]?, idx}`。Start：DueReviews 取到期薄弱点（无则提示）；Turn：逐项提问复习（LLM 生成复习题）→ 用户回答 → ReviewWeakPoint 更新 SM-2 → 下一项；结束总结。

**理由**：复用既有 Agent/画像/控制器，Skill 层只做状态编排，避免重复实现。

### 4. 入口
CLI：
- `skill list`：技能列表（名称/描述/匹配关键词）
- `skill run "<请求>"`：Registry.Match → NewSession → 打印初始提示 → 交互循环（复用 askQuestion 式 stdin）
- `skill <name>`：指定技能（跳过匹配）
Web：
- `GET /api/skill`：列表；`POST /api/skill/start`（{name|input}）；`POST /api/skill/turn`（{session_id, input}）；`POST /api/skill/finish`
- 前端技能页：技能列表卡片 + 聊天面板（复用现有 msg/loading 样式）

### 5. 会话生命周期
Turn 后保存状态（每次）；Finish（用户结束或技能 Done）标记 finished。Web/CLI 都能恢复进行中会话（列表展示未完成会话）。

## Risks / Trade-offs

- [InterviewSkill 与现有面试入口重复] → 技能入口是包装（StartSpecial/SubmitAnswer），行为一致；保留原 CLI/Web 面试入口
- [Skill 状态 JSON 脆弱] → 每技能私有结构 + 版本字段（state_version）；解析失败重建会话
- [匹配分误判] → 阈值 30 + 关键词命中加权；skill run 支持显式指定技能绕过匹配

## Migration Plan

纯新增。skill_sessions 表随 store migrate 创建。

## Open Questions

无——范围已确认（接口 + Registry + 4 技能 + CLI/Web 入口）。
