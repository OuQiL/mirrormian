# 部署指南（Cloudflare Tunnel + Docker）

本项目是 **Go 原生服务 + SQLite 文件库 + 本地 kb/ 文件 + 可选 Milvus**。

> ⚠️ 它**不能**部署在 Cloudflare Pages / Workers 的 serverless 上（不支持 Go 原生二进制与持久化磁盘）。正确做法是：Go 服务跑在 Docker，用 **Cloudflare Tunnel**（`cloudflared` 旁车）把它安全挂到你的域名，Cloudflare 负责域名 / CDN / 公网入口。

所需的前置产已在仓库内配置好：

- `Dockerfile` — 多阶段构建，入口 `web -addr :8012`
- `docker-compose.yml` — `app`（应用）+ `cloudflared`（隧道旁车）+ `milvus/etcd/minio`（向量栈）
- `.env.example` — 环境变量模板（含 `CLOUDFLARE_TUNNEL_TOKEN`）

## 一、准备

1. 一台有 Docker + Docker Compose 的服务器（任意 VPS / 云主机）。
2. 一个 Cloudflare 账号，且**已把你的域名接入** Cloudflare（站点处于 Active）。
3. LLM / Embedding 相关 API Key。

## 二、在服务器启动

```bash
git clone https://github.com/<你的账号>/mirrormian.git
cd mirror-mian
cp .env.example .env
# 编辑 .env，至少填：
#   LLM_BASE_URL / LLM_API_KEY / LLM_MODEL
#   CLOUDFLARE_TUNNEL_TOKEN=<第3步的令牌>
docker compose up -d --build
docker compose ps        # 确认 app / cloudflared 均 Up
```

## 三、创建 Cloudflare 远程隧道

1. 登录 Cloudflare 控制台 → **Zero Trust → Networks → Tunnels** → **Create a tunnel**。
2. 选择 **Cloudflared**，把生成命令形如
   `cloudflared tunnel run --token ABCDEFG` 中的 `ABCDEFG` 复制下来。
   它就是远程托管隧道的连接令牌，填到服务器 `.env` 的 `CLOUDFLARE_TUNNEL_TOKEN`。
3. 在隧道设置里添加一条 **Public Hostname**：
   - 域名 / 子域：你的域名（或 `app.你的域名`）
   - Path：`/`
   - Type：`HTTP`；URL：`http://app:8012`
   > 注意：URL 写 compose 服务名 `app`（同一内部网络可达），**不要**写 `localhost`，否则 cloudflared 连不到。
4. 在 **Websites →（你的域名）→ DNS** 确认已存在
   `CNAME  你的域名  →  <隧道ID>.cfargotunnel.com`（隧道页通常会引导自动添加）。

改完服务端点后，`docker compose restart cloudflared` 生效。

## 四、生效与验证

- 浏览器访问 `https://你的域名`，应显示魔镜面试登录页。
- 后台日志排查：
  ```bash
  docker compose logs -f app
  docker compose logs -f cloudflared   # 隧道已连接 / 报错看这里
  ```

## 数据持久化说明

- SQLite 数据库在服务器**本地卷** `mian-data`（挂载 `/app/data/mirror-mian.db`）。
- 知识库文件在卷 `mian-kb`（挂载 `/app/kb`）。
- 简历不上传文件：仅把解析文本写入 SQLite，无需额外存储。
- 这些**不是** Cloudflare D1 / R2。若要迁移到 Cloudflare 云存储，需重写后端（Go→TS + 存储替换），超出本部署范围。

## 常见问题

- **Pages 构建报 `ENOENT package.json`**：本项目不是 Node 项目，不要在 Cloudflare Pages 里连它；使用上方 Tunnel 方案。
- **站点打不开**：确认隧道 Public Hostname 指向 `http://app:8012`，且 `CLOUDFLARE_TUNNEL_TOKEN` 正确；`cloudflared` 日志会给出连接错误。
- **想用默认端口而非 8012**：改 `docker-compose.yml` 中 app 的 `command` 与隧道 URL 保持一致。
- **不需要向量检索（Milvus）**：可删掉 `docker-compose.yml` 里的 `milvus/etcd/minio` 服务，并把 app 的 `MILVUS_ADDR` 留空以禁用。