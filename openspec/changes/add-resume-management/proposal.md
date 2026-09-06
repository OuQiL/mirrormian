## Why

综合面试目前只能粘贴简历文本（或 `--resume <file>` 一次性读取），简历无法沉淀复用。用户需要专门的简历管理：上传多份简历（PDF/DOCX/TXT），综合面试时从列表选择，而不是每次粘贴。

## What Changes

- **简历存储**：`resumes/` 文件目录（原文件）+ SQLite `resumes` 表（元数据 + 解析文本）
- **格式支持**：PDF（ledongthuc/pdf）、DOCX（nguyenthenguyen/docx）、TXT/MD 直接读取（对齐蓝本 loader）
- **简历管理**：Web「简历」页（上传/列表/删除/查看解析文本）+ CLI `resume add <file>` / `resume list` / `resume delete <id>`
- **综合面试选择简历**：Web 综合面试表单增加「简历」下拉（从已上传中选择，自动带入文本）；CLI `train full --jd ... --resume-id <id>`（保留 `--resume <file>` 兼容）
- 匹配度分析（JDResumeAnalyzer）与综合面试链路不变，仅简历文本来源改为所选简历

## Capabilities

### New Capabilities

- `resume-management`: 简历管理能力——多简历上传与解析（PDF/DOCX/TXT）、列表/删除/查看、综合面试简历选择

### Modified Capabilities

（无——综合面试流程行为不变，仅简历输入来源扩展）

## Impact

- **新增**：`internal/resume/`（存储 + 解析）、`resumes/` 目录、Web 简历页、`resume` 命令、`--resume-id` 参数
- **依赖**：`github.com/ledongthuc/pdf`、`github.com/nguyenthenguyen/docx`
- **风险**：低——纯新增；`--resume` 兼容保留
