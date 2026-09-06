## Purpose

定义 mirror-mian 的 Web 交互层行为：浏览器驱动的面试会话（一问一答、追问流转、复盘展示、画像查看），交互层不包含业务逻辑，全部经编排层控制器执行。

## ADDED Requirements

### Requirement: Web 服务启动

系统 SHALL 提供 `web` 子命令启动 HTTP 服务，浏览器访问首页即可使用；端口可配置。

#### Scenario: 启动服务

- **WHEN** 运行 `mian web`
- **THEN** HTTP 服务在配置端口监听，`GET /` 返回面试页面

### Requirement: 会话断点驱动

编排层 SHALL 提供可断点的面试控制器：`start` 创建会话并出第一题，`answer` 提交回答并推进一轮（判定 → 追问或进入复盘），每步状态持久化；CLI 链路 SHALL 保持原有行为不变。

#### Scenario: 创建会话

- **WHEN** 调用 start（专项：主题；综合：JD/简历）
- **THEN** 返回会话 ID 与第一道题

#### Scenario: 逐轮推进

- **WHEN** 对会话提交一次回答
- **THEN** 返回下一道追问（或知识点过关后的下一题），最后一轮后返回复盘结果并沉淀画像

#### Scenario: CLI 兼容

- **WHEN** 执行 CLI 面试命令
- **THEN** 行为与控制器拆分前一致（同一状态机规则）

### Requirement: 面试页面

Web 页面 SHALL 支持：选择专项/综合模式并开始面试、逐题展示与回答、追问流转、结束后的复盘展示；页面 SHALL 提供画像与到期复习的查看入口。

#### Scenario: 完整面试流程

- **WHEN** 用户在页面选择主题开始专项面试并逐题作答
- **THEN** 页面展示题目与追问流转，面试结束后展示复盘报告

#### Scenario: 画像与复习查看

- **WHEN** 用户打开画像/复习页面
- **THEN** 展示掌握度、薄弱点与到期复习项

### Requirement: 无业务逻辑的交互层

Web 层代码 SHALL NOT 包含业务逻辑（出题、判定、画像计算等），全部委托编排层控制器。

#### Scenario: 分层校验

- **WHEN** 检查 internal/web 包
- **THEN** 仅包含 HTTP 收发、参数校验与展示逻辑，不含状态机或算法实现
