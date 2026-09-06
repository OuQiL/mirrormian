## Context

综合面试的简历输入目前是「粘贴文本」或「--resume 一次性读文件」，无复用。匹配分析（JDResumeAnalyzer）已就位。本 change 增加简历管理：文件 + 元数据双存储、多格式解析、Web/CLI 管理、面试选择。

## Goals / Non-Goals

**Goals:**
- 上传 PDF/DOCX/TXT/MD 简历 → 解析文本 + 持久化
- Web 简历页（上传/列表/删除/查看）+ CLI resume 命令
- 综合面试通过简历 ID 选择（Web 下拉 / CLI --resume-id）
- `--resume <file>` 兼容保留

**Non-Goals:**
- 不做简历 OCR（扫描版 PDF 需 OCR，属后续）
- 不做简历编辑（只读管理，编辑走重新上传）
- 不做简历与 JD 的自动配对（选择由用户完成）

## Decisions

### 1. 存储（internal/resume）
```
resumes/<id>.<ext>     # 原文件（uuid 命名，防重名/路径问题）
resumes 表（SQLite）：
  id, filename(原始名), ext, size_bytes, text(解析文本), created_at
```
`internal/resume` 包：
```go
type Service struct { root string; store store.Store }
func (s *Service) Add(ctx, name string, data []byte) (*model.Resume, error)  // 解析+存文件+入库
func (s *Service) List() ([]model.Resume, error)
func (s *Service) Get(id) (*model.Resume, error)  // 含文本
func (s *Service) Delete(id) error                 // 文件+记录
func ParseText(ext string, data []byte) (string, error) // 解析分发
```
解析分发：txt/md 直接 string；pdf → `ledongthuc/pdf`（逐页文本，蓝本同款）；docx → `nguyenthenguyen/docx`（提取段落文本，蓝本同款）。

### 2. 配置
`MIRROR_RESUME_PATH`（默认 `resumes/`），config 增加字段；`data/` 之外的独立目录，`.gitignore` 追加 `resumes/`（含个人简历，必须排除）。

### 3. Web
- `GET /api/resume`：列表（不含文本，减载）
- `GET /api/resume/<id>`：详情含文本
- `POST /api/resume`：multipart 上传（字段 file），解析失败 400
- `DELETE /api/resume/<id>`：删除
- 页面「简历」：上传区（拖拽或选择）+ 列表（文件名/格式/时间/大小 + 查看/删除）
- 综合面试表单：简历下拉（「不选简历」+ 已上传列表），选中后 start 请求带 `resume_id`；后端按 ID 取文本
- 面试页综合模式还保留 JD 文本粘贴（JD 仍粘贴或未来 --jd-url）

### 4. CLI
- `resume add <file>` / `resume list` / `resume delete <id>` / `resume show <id>`
- `train full` 增加 `--resume-id <id>`；`--resume <file>` 保留（直接读取不落库）

### 5. 综合面试接线（web/server）
start 接口 `resume_id` 字段：有 → `resume.Get(id).Text`；无 → 用表单 resume 文本（兼容旧前端）；两都没有 → 允许（纯 JD 面试，匹配分析会提示简历缺失——保持现状）。

## Risks / Trade-offs

- [PDF 解析质量参差] → 扫描版/复杂排版解析文本可能乱，上传后「查看文本」确认；解析失败明确报错
- [简历隐私] → `resumes/` 进 .gitignore；接口仅本地
- [大文件] → 上传限 10MB（http.MaxBytesReader）

## Migration Plan

纯新增。既有 `--resume <file>` 流程不变。

## Open Questions

无——范围已确认（上传解析 + 管理 + 面试选择 + CLI）。
