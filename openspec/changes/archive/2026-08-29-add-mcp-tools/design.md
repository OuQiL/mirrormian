## Context

蓝本 interview-agent 已有成熟的 MCP 实现（internal/mcp：GitHubSearcher + WebScraper，基于 mark3labs/mcp-go，stdio 方式启动 MCP Server）。mirror-mian 数据飞轮闭环已就位，缺外部信息源。本 change 移植两个工具并接两个应用点（复习资源、JD URL）。

## Goals / Non-Goals

**Goals:**
- internal/mcp：GitHubSearcher（搜索仓库）+ WebScraper（JS 渲染抓取）
- 应用：`resources <topic>`（CLI + Web 复习页按钮）、`train full --jd-url <url>`
- MCP 为可选能力：token/工具链缺失时清晰降级

**Non-Goals:**
- 不做 MCP Server 常驻/热重启（每次命令按需启动，用完关闭）
- 不做浏览器自动化配置（Playwright 全局安装优先，npx 回退，对齐蓝本）
- 不做搜索缓存（结果即时返回）

## Decisions

### 1. 移植蓝本实现（internal/mcp）
`GitHubSearcher`：`client.NewStdioMCPClient("npx", env, "@modelcontextprotocol/server-github")`，env 带 `GITHUB_PERSONAL_ACCESS_TOKEN`；`SearchRepos(ctx, query, limit)` 调 `search_repositories` 工具，返回 RepoInfo{Name, URL, Stars, Desc}。
`WebScraper`：`resolveMCPCommand()` 优先 `npm root -g` 定位 `@playwright/mcp/cli.js`（离线秒起），否则 `npx @playwright/mcp@latest --headless`；`Fetch(ctx, url)` 调 `browser_navigate` 抓取文本。
**理由**：蓝本已验证可用（含被墙场景的 npx 回退策略），直接移植降低风险。

### 2. 可选能力装配（cmd/web 层）
```
GITHUB_TOKEN=xxx mian resources Redis 5
mian train full --jd-url https://.../job
```
- cmdTrain 增加 `--jd-url`：调用 WebScraper.Fetch → 文本当 JD
- `resources` 子命令：topic → 拼查询（`"<topic> interview questions"` / `"<topic> stars:>100"`）→ SearchRepos → 打印 top N
- Web `/api/resources?topic=X` + 复习页按钮（加载中动画复用 loading）
**理由**：应用点都在交互层，编排层无感知；MCP 失败 = 命令报错，不影响核心闭环。

### 3. 生命周期
每次调用创建 → 用完 `Close()`（杀掉 stdio 子进程）。首启 npx 下载 MCP Server 可能 30s+，超时 60s 并提示。
**理由**：命令式使用场景无需常驻；避免泄漏子进程。

### 4. 查询构造
资源推荐查询模板：`"<topic> interview questions stars:>50"`（稳定出高质量结果）；Web 端可自定义 query。

## Risks / Trade-offs

- [npx 下载被墙/慢] → 对齐蓝本：优先全局 npm 包；提示安装 `npm i -g @playwright/mcp` 与 `@modelcontextprotocol/server-github`
- [GitHub token 缺失] → GITHUB_TOKEN 未配置时 GitHub 工具直接报可读错误；Web 抓取不受影响
- [页面抓取内容含噪音] → 抓取为文本后交给 JD 分析 Agent 提炼（现有流程天然处理）

## Migration Plan

纯新增。MCP 工具为可选依赖，不影响既有功能。

## Open Questions

无——范围已确认（两工具 + 两应用点，可选降级）。
