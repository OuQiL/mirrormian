## Why

LLM 与 Embedding 配置目前只能通过环境变量/.env 设置，改配置需要编辑文件、重启服务，门槛高。用户需要一个 Web 设置页：表单化配置 LLM（base_url/key/model）与 Embedding（base_url/key/model），带连接测试，避免手改文件的出错率。

## What Changes

- **设置接口**：`GET /api/settings`（返回当前生效配置，API Key 脱敏）、`POST /api/settings`（保存到 `.env` 文件，带格式校验）、`POST /api/settings/test`（用待保存配置测试 LLM/Embedding 连通性，返回成功/失败与错误信息）
- **配置持久化**：设置写入项目根 `.env`（config.Load 已支持），保存后重启服务生效（接口返回提示）；`.env` 已有内容保留
- **Web 设置页**：顶部导航「设置」——LLM 与 Embedding 两组表单（base_url/key/model），保存/测试按钮，key 输入框带显隐切换，保存后提示重启

## Capabilities

### New Capabilities

- `settings`: mirror-mian 的设置能力——LLM/Embedding 配置的表单化读写、连接测试、持久化到 .env

### Modified Capabilities

（无）

## Impact

- **新增**：`internal/web` 设置接口与页面（settings handler + 前端页），`.env` 读写工具（internal/config 增加 ReadEnv/WriteEnv）
- **修改**：`internal/config`（env 文件读写）、`internal/web`（路由 + 导航）
- **风险**：低——设置只写 .env 与测试连接，不影响核心闭环；key 脱敏展示避免泄露
