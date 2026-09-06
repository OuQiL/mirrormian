## 1. Skill 接口与存储

- [x] 1.1 `internal/skill`：Skill 接口 + Session + TurnResult；store 增加 skill_sessions 表与 Save/Get，验证单测会话往返与恢复
- [x] 1.2 SkillRegistry：Register/List/Match（阈值 30）/NewSession/Turn 自动恢复，验证单测匹配与无可匹配提示

## 2. 内置 Skill

- [x] 2.1 quick-quiz：逐题即时判定+讲解+计分，结束时沉淀薄弱点，验证单测多轮流转与计分
- [x] 2.2 concept-tutor：讲解→提问确认→未懂换角度，验证单测流转
- [x] 2.3 interview：包装 StartSpecial/SubmitAnswer，验证与现有面试行为一致（E2E）
- [x] 2.4 review：到期薄弱点逐个复习并更新 SM-2，验证单测复习反馈

## 3. 入口

- [x] 3.1 CLI `skill list / skill run "<请求>" / skill <name>`（交互循环），验证命令行为
- [x] 3.2 Web `/api/skill` 系列接口 + 技能页（列表 + 聊天面板），验证浏览器端到端

## 4. 验证与收尾

- [x] 4.1 `go test ./...`、`go vet`、`go build` 全部通过；真实 LLM 跑通 quick-quiz 与 review 技能
- [x] 4.2 openspec validate 通过；改动范围仅限 mirror-mian 内新增/修改文件
