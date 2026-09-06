# mcp-tools Specification

## Purpose
定义 mirror-mian 的 MCP 工具能力：GitHub 项目搜索与网页抓取两个外部信息工具，以及它们在复习资源推荐与综合面试 JD 抓取上的应用；工具为可选能力，环境不满足时降级提示明确。

## Requirements

### Requirement: GitHub 项目搜索

系统 SHALL 提供 GitHub 项目搜索工具（基于 GitHub MCP Server）：按查询语法搜索仓库，返回名称、URL、star 数与描述；GitHub token 未配置或 MCP Server 启动失败时 SHALL 返回明确错误提示而非崩溃。

#### Scenario: 搜索仓库

- **WHEN** 调用搜索（如 query="go interview questions", limit=5）
- **THEN** 返回仓库列表（名称/URL/star/描述），可按 star 排序

#### Scenario: 环境缺失降级

- **WHEN** GITHUB_TOKEN 未配置或 npx 不可用
- **THEN** 返回可读错误（说明需配置 token / 安装 node），进程不崩溃

### Requirement: 网页抓取

系统 SHALL 提供网页抓取工具（基于 Playwright MCP Server）：输入 URL 返回页面文本内容；抓取失败（网络/页面不可访问）SHALL 返回明确错误。

#### Scenario: 抓取页面

- **WHEN** 提供可访问的 URL
- **THEN** 返回页面提取后的文本（JS 渲染后内容）

#### Scenario: 抓取失败

- **WHEN** URL 不可访问或抓取超时
- **THEN** 返回可读错误，不影响其他功能

### Requirement: 复习资源推荐

系统 SHALL 按主题/薄弱点推荐 GitHub 学习资源：`resources <topic>` 命令与 Web 复习页「推荐资源」按钮均可触发，返回 top N 项目（名称/star/描述/URL）。

#### Scenario: 按主题推荐

- **WHEN** 对主题（如 Redis）执行资源推荐
- **THEN** 返回该主题相关的高 star 开源项目，供复习参考

### Requirement: JD URL 抓取备面

综合面试 SHALL 支持 `--jd-url <url>`：抓取招聘页面文本作为 JD 输入（与粘贴 JD 行为一致，进入同一方向分析流程）。

#### Scenario: URL 驱动综合面试

- **WHEN** 执行 `train full --jd-url <url>` 且抓取成功
- **THEN** 以抓取文本为 JD 完成方向分析并开始面试

#### Scenario: URL 抓取失败

- **WHEN** 页面抓取失败
- **THEN** 报错并提示可改用 `--jd <file>` 或粘贴方式
