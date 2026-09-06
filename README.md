# mirror-mian

AI 模拟面试与学习成长系统。基于 Go + Eino 构建，提供专项/综合面试、成长技能对话、数据飞轮（画像 → 复习 → 掌握度）与 RAG 知识库的完整闭环。

## 核心特性

- **三层架构**：交互层（CLI + Web）→ 编排层（多 Agent + Eino Graph）→ 基础能力层（LLM / 向量检索 / 存储 / 画像）
- **专项面试**：按领域出最小单位题，LLM 判定追问（每知识点最多 3 轮），面试官评估 → 复盘 → 画像沉淀
- **综合面试**：JD/简历匹配度分析（差距优先出题）+ 暖场仪式（问期望 → 引导自我介绍，期间后台并行准备）
- **成长 Chat**：有状态多轮技能对话——快速测验 / 概念教学 / 专项面试 / 复习回顾，意图自动匹配
- **RAG 多路召回**：Milvus 向量（1024 维）+ BM25 双路 → RRF(k=60) 融合 → LLM Rerank
- **数据飞轮**：逐题评分 → 薄弱点（SM-2 间隔重复）→ 掌握度（EMA）→ 到期复习 → 复习计划
- **领域管理**：知识库（Markdown 文件化）+ 高频题库 + AI 生成核心知识梳理
- **简历管理**：PDF/DOCX/TXT 上传解析（统一 Markdown）+ 粘贴上传 + 面试选择
- **MCP 工具**：GitHub 学习资源推荐、招聘页面抓取（`--jd-url`）
- **Web 界面**：VSCode 风格，左侧活动栏导航，设置页可视化配置 LLM/Embedding

## 快速开始

```bash
# 1. 启动 Web（无需任何配置，未配置 API Key 也能启动）
./bin/mian.exe web -addr :8012

# 2. 浏览器打开 http://localhost:8012
#    设置页（⚙️）填写 LLM 与 Embedding 的 API Key → 保存 → 重启服务生效
```

### 环境变量（可选，.env 或环境）

```
# LLM（OpenAI 兼容端点）
LLM_BASE_URL=https://api.xiaomimimo.com/v1
LLM_API_KEY=sk-xxx
LLM_MODEL=mimo-v2.5

# Embedding
LLM_EMBEDDING_BASE_URL=https://api.siliconflow.cn/v1
LLM_EMBEDDING_API_KEY=sk-xxx
LLM_EMBEDDING_MODEL=BAAI/bge-m3

# 可选
GITHUB_TOKEN=ghp_xxx          # resources 命令与资源推荐
MILVUS_ADDR=localhost:19530   # Milvus 向量库（docker compose up -d milvus）
MIRROR_DB_PATH=data/mirror-mian.db
MIRROR_KB_PATH=kb
MIRROR_RESUME_PATH=resumes
```

## CLI 用法

```bash
# 面试
./bin/mian.exe train special "Go 并发"                     # 专项面试
./bin/mian.exe train full --jd jd.txt --resume-id <id>     # 综合面试（简历库）
./bin/mian.exe train full --jd-url <招聘页URL>              # 综合面试（抓取 JD）

# 成长技能
./bin/mian.exe skill list                                   # 技能列表
./bin/mian.exe skill run "考考我 Redis"                     # 意图匹配自动启动
./bin/mian.exe skill quick-quiz "Redis"                     # 指定技能

# 领域与知识库
./bin/mian.exe domain create Redis                          # 创建领域
./bin/mian.exe domain generate Redis                        # AI 生成核心知识梳理
./bin/mian.exe domain sync Redis                            # 同步向量（Milvus + BM25）
./bin/mian.exe kb search Redis "AOF 重写"                    # 多路召回检索

# 画像与复习
./bin/mian.exe profile                                      # 查看画像
./bin/mian.exe review                                       # 到期复习
./bin/mian.exe review --score <id> 8                        # 提交复习评分
./bin/mian.exe plan                                         # 今日复习计划
./bin/mian.exe analyze --jd jd.txt --resume resume.txt      # JD/简历匹配度分析

# 简历与资源
./bin/mian.exe resume add resume.pdf                        # 上传简历
./bin/mian.exe resume list
./bin/mian.exe resources Redis                              # GitHub 学习资源推荐

# 其他
./bin/mian.exe run                                          # 健康自检
```

## 系统架构

```
┌─────────────────────────────────────────────────┐
│ 交互层：CLI（cmd/）+ Web（internal/web，:8012）  │
├─────────────────────────────────────────────────┤
│ 编排层：internal/orchestration + agents         │
│   方向规划 / 出题 / 面试官 / 复盘 / JD分析 / 复习规划 │
│   技能系统：internal/skill（Registry + 4 技能）   │
├─────────────────────────────────────────────────┤
│ 基础能力层                                       │
│   llm（OpenAI 兼容）│ vector（Milvus）│ rag       │
│   （BM25+RRF+Rerank）│ profile（SM-2/EMA）       │
│   store（SQLite）│ resume │ domain │ mcp         │
└─────────────────────────────────────────────────┘
```

### 数据飞轮闭环

```
面试/测验 → 逐题评分 → 画像（薄弱点 + SM-2 + 掌握度 EMA）
    ↑                                  ↓
出题（差距优先 + 知识库注入）←── 到期复习 → 复习计划 → 复习反馈
```

## 面试规则（行为契约）

- **最小单位题**：每题只考察一个知识点，每次只呈现一道题
- **追问**：回答后 LLM 评估（游刃有余/无力），生成针对性追问；每知识点保证 3 轮且只有 3 轮
- **综合面试**：JD/简历匹配分析（对位/评分/差距）→ 差距优先出题
- **复盘**：整体评价 + 平均分 + 逐题点评 + 改进建议 + 薄弱点/亮点，沉淀画像

## 目录结构

```
mirror-mian/
├── cmd/                    # CLI 入口（交互层）
├── internal/
│   ├── agents/             # 6 个专职 Agent
│   ├── orchestration/      # Eino Graph 编排 + 断点控制器
│   ├── skill/              # 技能系统（接口/Registry/4 技能）
│   ├── rag/                # BM25 + RRF + LLM Rerank
│   ├── vector/             # Milvus 向量库客户端
│   ├── embedding/          # Embedding 客户端
│   ├── profile/            # SM-2 / EMA / 薄弱点
│   ├── resume/             # 简历解析与管理
│   ├── domain/             # 领域管理
│   ├── mcp/                # GitHub 搜索 / 网页抓取
│   ├── store/              # SQLite 持久化
│   ├── config/             # 配置加载
│   └── web/                # HTTP 服务 + 前端（VSCode 风格）
├── kb/                     # 领域知识库（Markdown 文件）
├── resumes/                # 简历原文件
├── data/                   # SQLite 数据库
└── openspec/               # spec-driven 规范管理
```

## 规范管理

项目使用 OpenSpec spec-driven 工作流管理能力契约：

```bash
openspec status                # 查看变更
openspec new change <name>     # 新建变更（propose → apply → archive）
```

## 开发

```bash
go build -o bin/mian.exe ./cmd   # 构建
go test ./...                    # 测试
go vet ./...                     # 静态检查

# Milvus（可选，向量路降级可用）
docker compose up -d milvus
```
