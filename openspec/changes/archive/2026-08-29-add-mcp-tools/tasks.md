## 1. MCP 工具移植

- [x] 1.1 创建 `internal/mcp`：GitHubSearcher（stdio 启动 server-github，SearchRepos 返回 RepoInfo），验证启动失败/无 token 时返回可读错误
- [x] 1.2 创建 `internal/mcp`：WebScraper（resolveMCPCommand 全局优先/npx 回退，Fetch 返回页面文本），验证抓取失败时返回可读错误

## 2. 应用点

- [x] 2.1 `resources <topic> [count]` 子命令：拼查询 → 搜索 → 打印 top N，验证无 token 时提示清晰
- [x] 2.2 `train full --jd-url <url>`：抓取页面文本作为 JD，验证抓取失败时提示改用 --jd
- [x] 2.3 Web `/api/resources` 接口 + 复习页「推荐资源」按钮（loading 复用），验证接口与页面

## 3. 验证与收尾

- [x] 3.1 `go test ./...`、`go vet`、`go build` 全部通过（mcp 相关以错误路径单测覆盖，真实调用按环境可用性验证）
- [x] 3.2 openspec validate 通过；改动范围仅限 mirror-mian 内新增/修改文件
