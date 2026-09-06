## Why

mirror-mian 目前只有 CLI 交互层，真实面试体验依赖终端交互（且非交互环境无法运行）。用户需要一个简单可用的 Web 端来试用完整面试流程，降低体验门槛。

## What Changes

- 新增 `web` 子命令：启动 HTTP 服务，浏览器访问即可开始面试
- 新增面试控制器（Controller）：把现有一次性面试循环拆成可断点的 start/answer 两步驱动，供 HTTP 每轮一问一答调用（会话状态继续外置持久化）
- 新增 Web 交互层：出题展示、回答提交、追问流转、复盘展示、画像/复习查看
- 交互层仍不含业务逻辑：HTTP handler 只做参数解析、调用编排层、渲染输出

## Capabilities

### New Capabilities

- `web-interaction`: mirror-mian 的 Web 交互层——HTTP 接口、面试会话断点驱动（start/answer）、页面交互与展示

### Modified Capabilities

（无——mirror-mian 项目 openspec/specs/ 为空，本次建立首个能力）

## Impact

- **新增**：`internal/web/`（HTTP handlers）、`cmd/mian.go` 的 `web` 子命令、静态页面（index.html/app.js/style.css）
- **修改**：`internal/orchestration/service.go`——面试循环拆分为可断点控制器（CLI 行为不变）
- **依赖**：仅 Go 标准库 net/http + html/template，不引入 Web 框架
- **风险**：低——CLI 链路保持兼容，新增路径不影响既有测试
