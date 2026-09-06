## Purpose

定义 mirror-mian 的设置能力：LLM 与 Embedding 配置的 Web 表单化读写（base_url/api_key/model）、连接测试与持久化；API Key 脱敏展示，配置变更后提示重启生效。

## ADDED Requirements

### Requirement: 设置读取与 Key 明文显示

系统 SHALL 提供设置读取接口：返回当前生效的 LLM 与 Embedding 配置（base_url/model/完整 API Key），设置页 SHALL 以明文输入框原样显示 Key，可直接修改。

#### Scenario: 读取设置

- **WHEN** 请求设置接口
- **THEN** 返回 LLM/Embedding 的 base_url、model 与完整 API Key

#### Scenario: Key 明文显示

- **WHEN** 打开设置页
- **THEN** Key 以明文输入框原样显示（无隐藏/复制按钮），可直接编辑修改

#### Scenario: 安全提示

- **WHEN** 打开设置页
- **THEN** 页面显示「Key 完整显示，仅限本地使用，勿暴露公网」提示

### Requirement: 设置保存

系统 SHALL 支持保存 LLM/Embedding 配置到 `.env` 文件（保留文件既有内容）；保存 SHALL 校验必填字段（base_url/model 非空）；保存成功后 SHALL 提示需重启服务生效。

#### Scenario: 保存配置

- **WHEN** 提交完整配置
- **THEN** `.env` 写入新值且既有配置保留，响应提示「重启服务生效」

#### Scenario: 校验失败

- **WHEN** base_url 或 model 为空
- **THEN** 返回明确错误，不写入

### Requirement: 连接测试

系统 SHALL 提供连接测试接口：用提交的配置（不落盘）实际调用 LLM chat 与 Embedding 接口最小请求，分别返回成功或失败（含错误信息）。

#### Scenario: 测试 LLM 连接

- **WHEN** 提交 LLM 配置并请求测试
- **THEN** 用该配置发起最小 chat 请求，成功返回 ok、失败返回错误信息

#### Scenario: 测试 Embedding 连接

- **WHEN** 提交 Embedding 配置并请求测试
- **THEN** 用该配置发起最小 embeddings 请求，成功返回 ok（含维度）、失败返回错误信息

### Requirement: Web 设置页

Web 端 SHALL 提供「设置」页面：LLM 与 Embedding 两组表单（base_url/api_key/model），key 输入支持显隐切换；保存与测试按钮独立；保存后显示重启提示。

#### Scenario: 表单展示与保存

- **WHEN** 打开设置页
- **THEN** 表单回填当前配置（key 脱敏），可编辑后保存并看到重启提示

#### Scenario: 测试反馈

- **WHEN** 点击测试按钮
- **THEN** 页面显示对应服务连通性结果（成功/失败原因），不阻塞保存
