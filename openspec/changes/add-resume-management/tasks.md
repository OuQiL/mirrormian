## 1. 简历存储与解析

- [x] 1.1 `internal/resume`：Service（Add/List/Get/Delete，文件存 resumes/ + 元数据入库），验证单测往返
- [x] 1.2 ParseText 分发：txt/md 直接读；pdf（ledongthuc/pdf）、docx（nguyenthenguyen/docx）解析，验证单测 txt 与格式拒绝
- [x] 1.3 config 增加 MIRROR_RESUME_PATH；store 增加 resumes 表；.gitignore 追加 resumes/

## 2. 管理入口

- [x] 2.1 CLI `resume add/list/delete/show`，验证命令行为（含解析失败报错）
- [x] 2.2 Web `/api/resume` 系列接口（列表/详情含文本/上传 multipart/删除），验证接口
- [x] 2.3 Web「简历」页：上传区 + 列表（查看文本/删除），验证浏览器上传与列表

## 3. 综合面试选择简历

- [x] 3.1 Web 综合面试表单加简历下拉，start 带 resume_id，后端按 ID 取文本；验证端到端（选简历 → 分析 → 出题）
- [x] 3.2 CLI `train full --resume-id <id>`，`--resume <file>` 兼容保留；验证两路径

## 4. 验证与收尾

- [x] 4.1 `go test ./...`、`go vet`、`go build` 全部通过；真实上传 PDF/TXT 简历 → 综合面试全链路
- [x] 4.2 openspec validate 通过；改动范围仅限 mirror-mian 内新增/修改文件
