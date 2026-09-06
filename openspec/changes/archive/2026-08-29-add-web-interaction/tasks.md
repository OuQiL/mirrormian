## 1. 控制器断点化

- [x] 1.1 在 orchestration.Service 增加 StartSpecial/StartFull：准备阶段 + 第一题，返回 ongoing 会话，验证返回会话含方向与第一题且已持久化
- [x] 1.2 实现 SubmitAnswer：判定 → 状态机推进 → 出下一题或触发复盘（复用 runReview），验证单测覆盖继续/过关/无力/满 3 轮与复盘路径
- [x] 1.3 状态机恢复：从会话推导 kpIdx/round，验证恢复后与原始状态一致（问答中途恢复继续推进）
- [x] 1.4 现有 CLI 链路与全部测试保持通过（回归验证）

## 2. Web 层

- [x] 2.1 创建 internal/web：server.go（路由 + embed 静态文件）与 handlers.go（start/answer/profile/review），验证 `mian web` 启动且 `GET /` 返回页面
- [x] 2.2 实现 start/answer handler：调用控制器，校验参数（topic 必填/评分范围），验证错误响应清晰
- [x] 2.3 创建静态页面 index.html + app.js + style.css：模式选择、逐题展示回答、追问流转、复盘展示、画像/复习页，验证浏览器端到端跑通一场面试

## 3. 验证与收尾

- [x] 3.1 `go test ./...`、`go vet`、`go build` 全部通过；真实 LLM 跑通一场 Web 面试
- [x] 3.2 openspec validate 通过；改动范围仅限 mirror-mian 内新增/修改文件

## 4. 体验迭代（用户试用反馈）

- [x] 4.1 start 返回方向清单（key_points），页面顶部渲染面试方向状态栏（当前知识点脉冲高亮、完成打勾），验证方向随进度更新
- [x] 4.2 新增 `/api/interview/finish`：结束面试时丢弃未答题目、对已答题目生成复盘并沉淀画像，验证无回答时明确报错
- [x] 4.3 前端等待动画（spinner + 「AI 正在思考…」），API 调用期间禁用输入，验证调用期间可见转圈
