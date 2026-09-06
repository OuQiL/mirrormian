## Why

数据飞轮闭环已就位（训练 → 画像 → 复习），但复习和备面缺少外部信息源：复习薄弱点时没有可参考的优质资源，综合面试只能粘贴 JD 文本（无法直接抓取招聘页面）。MCP 工具接入（GitHub 搜索 + 网页抓取）补上这一环，蓝本 interview-agent 已有成熟实现可直接移植。

## What Changes

- **internal/mcp 包**（移植蓝本）：`GitHubSearcher`（@modelcontextprotocol/server-github，按查询搜仓库/返回 star/描述）+ `WebScraper`（Playwright MCP，JS 渲染页面抓取，优先全局 npm 安装、npx 回退）
- **复习资源推荐**：`resources <topic> <count>` 命令 + Web 复习页「推荐资源」按钮——按主题/薄弱点搜 GitHub 项目（如 "go interview questions"）
- **综合面试 JD URL**：`train full --jd-url <url>`——抓取招聘页面文本作为 JD（与粘贴 JD 等价）
- 依赖 `github.com/mark3labs/mcp-go`；GitHub token（GITHUB_TOKEN）与 npx 为可选前提，未配置/启动失败时命令给出明确降级提示

## Capabilities

### New Capabilities

- `mcp-tools`: mirror-mian 的 MCP 工具能力——GitHub 项目搜索、网页抓取，以及两个应用点（复习资源推荐、JD URL 抓取）

### Modified Capabilities

（无）

## Impact

- **新增**：`internal/mcp/`（GitHubSearcher/WebScraper）、`resources` 命令、`--jd-url` 参数、Web 复习页推荐按钮
- **依赖**：`github.com/mark3labs/mcp-go`（+ 运行时 npx/node 与 MCP Server）
- **风险**：中——MCP Server 依赖外部工具链（npx 首次下载慢/可能被墙），全部为可选能力，降级提示清晰
