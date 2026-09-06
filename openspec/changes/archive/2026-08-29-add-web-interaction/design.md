## Context

mirror-mian 已有 CLI 交互层 + Eino Graph 三阶段编排 + 数据飞轮（见 create-mirror-mian）。现有编排层 `runInterview` 是一次性循环（出题→等回答→判定→…），需要拆成可断点的控制器才能服务 Web 每轮一问一答。状态（会话）已持久化在 SQLite，状态机规则不变。

## Goals / Non-Goals

**Goals:**
- `web` 子命令 + 静态页面，浏览器可直接试用完整面试
- 面试控制器断点化：start / answer 两步驱动，会话可恢复
- 全部用 Go 标准库（net/http + html/template），零新依赖
- CLI 行为不变（现有 29 个测试保持通过）

**Non-Goals:**
- 不做实时流式（SSE/WebSocket）——每轮请求/响应足够试用
- 不做用户账号/鉴权——单机本地试用
- 不做前端框架——原生 JS + 单页

## Decisions

### 1. 控制器断点化（interviewController）
在 `orchestration.Service` 上增加：
- `StartSpecial/StartFull(ctx, ...) (*model.TrainingSession, error)`：跑准备阶段（方向规划），出第一题（内部执行状态机 start + questioner），返回 ongoing 会话
- `SubmitAnswer(ctx, sessionID, answer) (StepResult, error)`：读会话 → 追加回答 → 面试官判定 → 状态机推进 → 若继续：出下一题并保存；若结束：执行复盘（复用 `runReview`）并返回复盘
- `StepResult{ NextQuestion *model.Question; Review *model.TrainingSession }`：一轮的产物
**理由**：复用现有 Agent 与状态机，改动最小；CLI 链路保持调用原 `runWithGraph` 不变。

### 2. 状态机恢复
控制器每步从 SQLite 会话重建状态机：`kpIdx` = 最后一题知识点在方向中的位置，`round` = 最后一题的 Round。
**备选**：状态机状态也持久化——会话 JSON 已含全部题目，推导即可，不新增字段。

### 3. Web 层结构
```
internal/web/
├── server.go        # HTTP mux：路由 + 静态文件
├── handlers.go      # start/answer/profile/review handler（只做解析/调用/渲染）
└── static/          # index.html / app.js / style.css（embed 打包）
```
接口：
- `POST /api/interview/start` `{mode, topic?, jd?, resume?}` → `{session_id, question}`
- `POST /api/interview/answer` `{session_id, answer}` → `{next_question?} | {review_md, summary}`
- `GET /api/profile` / `GET /api/review`：画像与复习
**理由**：单页 fetch 足够；`embed` 打包静态文件便于单二进制分发。

### 4. 端口配置
`web` 子命令支持 `-addr`（默认 `:8080`），配置走既有 config 加载。

## Risks / Trade-offs

- [控制器与 CLI 循环逻辑重复] → 控制器复用同一 Machine/Agent；CLI 保留 runWithGraph（互不干扰，测试双覆盖）
- [无鉴权，仅限本机] → README 明示仅本地试用；远程部署为后续 change
- [多会话并发] → 会话按 ID 隔离，SQLite 单写者，本地试用足够

## Migration Plan

新增路径，无迁移。CLI 行为不变。

## Open Questions

无——范围已确认（简单 Web 试用，非流式、无鉴权）。
