## 1. 配置读写与接口

- [x] 1.1 `internal/config` 增加 ReadEnvFile/WriteEnvFile（合并写保留既有内容），验证单测：读/写/空值删键/保留其他 key
- [x] 1.2 `GET /api/settings`：返回 LLM/Embedding 配置（key 脱敏），验证单测 key 不泄露完整值
- [x] 1.3 `POST /api/settings`：校验必填 → 合并写 .env → need_restart 提示，验证单测校验失败不写入
- [x] 1.4 `POST /api/settings/test`：LLM chat + Embedding 最小请求（不落盘），验证单测 mock 成功/失败两路径

## 2. 前端设置页

- [x] 2.1 设置页：导航入口 + 两组表单（base_url/key/model + key 显隐切换），打开时 GET 回填（key 留空=保持）
- [x] 2.2 「测试连接」按钮：逐组调用 test 接口显示 ✓/✗ 与错误；「保存配置」：校验 + POST + 「重启生效」提示
- [x] 2.3 验证浏览器端：读回填 → 改配置 → 测试 → 保存 → 重启后新配置生效（真实服务验证）

## 3. 验证与收尾

- [x] 3.1 `go test ./...`、`go vet`、`go build` 全部通过
- [x] 3.2 openspec validate 通过；改动范围仅限 mirror-mian 内新增/修改文件
