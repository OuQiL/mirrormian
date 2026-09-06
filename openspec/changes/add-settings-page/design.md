## Context

mirror-mian 的 LLM/Embedding 配置目前来自环境变量与 `.env`（config.Load 支持 godotenv）。Web 服务 8012 已有多页面（面试/画像/复习/领域）。本 change 增加设置页，表单化读写这两组配置并支持连接测试。

## Goals / Non-Goals

**Goals:**
- GET/POST /api/settings + POST /api/settings/test
- 配置持久化到 `.env`（保留既有内容），重启生效（接口明示）
- Key 脱敏、表单显隐切换、测试反馈
- 用现有 config 模型，零新依赖

**Non-Goals:**
- 不做运行时热更新（保存后重启生效，与 config 启动加载模型一致）
- 不做多环境配置（仅本地 .env）
- 不做密钥加密存储（.env 本地文件 + 脱敏展示足够）

## Decisions

### 1. .env 读写（internal/config 扩展）
```go
// ReadEnvFile 读取 .env 为 map（godotenv.Read）
func ReadEnvFile(path string) (map[string]string, error)
// WriteEnvFile 写回 .env（godotenv.Write，保留既有 key）
func WriteEnvFile(path string, env map[string]string) error
```
保存时：`ReadEnvFile` 读现有 → 合并新值 → `WriteEnvFile` 写回。覆盖键：`LLM_BASE_URL/LLM_API_KEY/LLM_MODEL/LLM_EMBEDDING_BASE_URL/LLM_EMBEDDING_API_KEY/LLM_EMBEDDING_MODEL`。空值表示删除该键（从 map 移除）。
**理由**：godotenv 已依赖；合并写避免覆盖用户其他配置。

### 2. 设置接口（internal/web/settings_handlers.go）
- `GET /api/settings`：`ReadEnvFile` + 环境变量覆盖 → 返回 `{llm:{base_url,model,api_key_configured,api_key_masked}, embedding:{...}}`；masked 形如 `sk-ab****xyz`
- `POST /api/settings`：body `{llm:{...}, embedding:{...}}`；校验 base_url/model 非空；写 .env；返回 `{ok:true, need_restart:true}`
- `POST /api/settings/test`：body 同保存（不落盘）；测试 LLM：POST {base_url}/chat/completions（model + "ping" 最小请求，`max_tokens:1`）；测试 Embedding：POST {base_url}/embeddings（"ping"）；分别返回 `{llm:{ok,error?}, embedding:{ok,dim?,error?}}`
**理由**：测试用待保存配置不落盘，符合「先测后存」心智；http.Client 10s 超时。

### 3. 前端设置页
- 导航加「设置」；表单两组字段（base_url/key/model + key 显隐 checkbox）
- 打开时 GET 回填（key 留空表示保持现有，保存时空 key 不覆盖——除非用户显式清除按钮）
- 「测试连接」按钮逐组调用 test 接口，显示 ✓/✗ 与错误；「保存配置」校验后 POST，成功后 toast「已保存，重启服务生效」
**理由**：key 不回填完整值（脱敏语义），留空 = 不修改，符合常见安全实践。

### 4. key 脱敏
`sk-abcdefgh1234` → `sk-abc****h1234`（保留前后 4 位与前缀）；长度 <10 时全掩 `****`。

## Risks / Trade-offs

- [重启才生效的体验落差] → 接口与页面均明示；设置页同时提供「测试连接」即时反馈，降低试错成本
- [写 .env 并发冲突] → 单用户本地场景；写文件用 os.OpenFile O_WRONLY|O_TRUNC，无并发需求
- [保存空 key 误清空] → 空 key 语义为「保持现有」（页面提示），提供显式清除按钮

## Migration Plan

纯新增。.env 文件由设置页写入后，重启即生效；与现有环境变量加载兼容（环境变量仍优先）。

## Open Questions

无——范围已确认（LLM/Embedding 两组配置 + 测试 + .env 持久化）。
