## 1. JD/简历分析 Agent

- [x] 1.1 创建 `internal/agents/jd_resume_analyzer.go`：Analyze 输出结构化 JDAnalysis（职责/要求/技术栈/评分/差距），验证单测 prompt 注入与 JSON 解析
- [x] 1.2 PlanFull 改造：先分析匹配 → 差距优先生成方向（输出结构兼容），验证既有综合面试 E2E 测试通过
- [x] 1.3 `analyze --jd <file> --resume <file>` 命令输出完整报告，验证命令行为

## 2. 复习规划 Agent

- [x] 2.1 创建 `internal/agents/review_planner.go`：ReviewPlanner 输入画像输出计划（排序/建议/耗时），验证单测空画像返回提示
- [x] 2.2 `plan` 命令：读画像 → 生成并打印计划，验证命令行为
- [x] 2.3 Web `POST /api/plan` + 复习页「生成复习计划」按钮展示，验证页面

## 3. 验证与收尾

- [x] 3.1 `go test ./...`、`go vet`、`go build` 全部通过；真实 LLM 跑通 analyze 与 plan
- [x] 3.2 openspec validate 通过；改动范围仅限 mirror-mian 内新增/修改文件
