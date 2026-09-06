// 简历管理服务：上传解析（PDF/DOCX/TXT/MD）、列表/删除/查看。
// 不保存原始文件，仅将解析文本（统一 Markdown）存 SQLite。
package resume

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	pdf "github.com/Detective-XH/gopdf"
	"github.com/google/uuid"
	docx "github.com/nguyenthenguyen/docx"

	"mirror-mian/internal/model"
	"mirror-mian/internal/store"
)

// MaxSize 上传大小上限（10MB）。
const MaxSize = 10 << 20

// supportedExts 支持的文件格式。
var supportedExts = map[string]bool{"pdf": true, "docx": true, "txt": true, "md": true}

// Service 简历管理服务。
type Service struct {
	store store.Store
}

// NewService 创建简历服务。
func NewService(st store.Store) *Service {
	return &Service{store: st}
}

// Add 上传简历文件：解析 → 入库（仅存解析文本，不保存原文件）。
func (s *Service) Add(filename string, data []byte) (*model.Resume, error) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	if !supportedExts[ext] {
		return nil, fmt.Errorf("不支持的格式 .%s（支持 pdf/docx/txt/md）", ext)
	}
	if len(data) > MaxSize {
		return nil, fmt.Errorf("文件超过 10MB 上限")
	}
	text, err := ParseText(ext, data)
	if err != nil {
		return nil, fmt.Errorf("解析失败: %w", err)
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("解析结果为空（扫描版 PDF 需先 OCR）")
	}
	r := &model.Resume{
		ID: uuid.NewString(), Filename: filename, Ext: ext, // Ext 记录原文件格式；Text 为解析出的 Markdown
		SizeBytes: len(data), Text: toMarkdown(text), CreatedAt: time.Now(),
	}
	return r, s.store.SaveResume(r)
}

// AddText 粘贴文本上传简历：文本直接按 Markdown 入库（无原文件）。
func (s *Service) AddText(name, text string) (*model.Resume, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("粘贴内容为空")
	}
	if len(text) > MaxSize {
		return nil, fmt.Errorf("内容超过 10MB 上限")
	}
	if name == "" {
		name = "粘贴简历.md"
	}
	r := &model.Resume{
		ID: uuid.NewString(), Filename: name, Ext: "md",
		SizeBytes: len(text), Text: toMarkdown(text), CreatedAt: time.Now(),
	}
	return r, s.store.SaveResume(r)
}

// toMarkdown 将解析文本规范化为 Markdown：简单文本加标题/列表符号补全，
// 已有 Markdown 结构（#、- 等）则原样保留。
func toMarkdown(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	var b strings.Builder
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			b.WriteString("\n")
			continue
		}
		// 已是 Markdown 元素（标题/列表/引用/代码）则保留
		if strings.HasPrefix(l, "#") || strings.HasPrefix(l, "-") || strings.HasPrefix(l, "*") ||
			strings.HasPrefix(l, ">") || strings.HasPrefix(l, "```") {
			b.WriteString(l)
			b.WriteString("\n")
			continue
		}
		// 行末以：结尾视为小节标题 → 二级标题
		if strings.HasSuffix(l, "：") || strings.HasSuffix(l, ":") {
			b.WriteString("## ")
			b.WriteString(l)
			b.WriteString("\n")
			continue
		}
		b.WriteString("- ")
		b.WriteString(l)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

// List 列出简历元数据（含文本，调用方自行省略）。
func (s *Service) List() ([]model.Resume, error) { return s.store.ListResumes() }

// Get 获取简历（含文本）。
func (s *Service) Get(id string) (*model.Resume, error) { return s.store.GetResume(id) }

// Delete 删除简历记录（无文件，仅库记录）。
func (s *Service) Delete(id string) error {
	return s.store.DeleteResume(id)
}

// ParseText 按扩展名解析文本。
func ParseText(ext string, data []byte) (string, error) {
	switch ext {
	case "txt", "md":
		return string(data), nil
	case "pdf":
		return parsePDF(data)
	case "docx":
		return parseDOCX(data)
	default:
		return "", fmt.Errorf("不支持的格式 .%s", ext)
	}
}

// parsePDF 用 Detective-XH/gopdf（GoPDF）按行提取全文：对中文 CMap 与阅读顺序支持好，
// Lines 按视觉行分组，保留换行结构。
func parsePDF(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("PDF 读取失败（可能是损坏或加密文件）: %v", err)
	}
	var lines []string
	for i := 1; i <= r.NumPage(); i++ {
		pageLines, err := r.Page(i).Lines()
		if err != nil {
			continue // 跳过解析失败页
		}
		for _, l := range pageLines {
			if s := strings.TrimSpace(l.S); s != "" {
				lines = append(lines, s)
			}
		}
		lines = append(lines, "") // 页间空行
	}
	text := strings.Join(lines, "\n")
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("PDF 无文本内容（扫描版需要先 OCR）")
	}
	return text, nil
}

// parseDOCX 用 nguyenthenguyen/docx 提取段落文本（段落/制表转可读格式后剥 XML 标签）。
func parseDOCX(data []byte) (string, error) {
	doc, err := docx.ReadDocxFromMemory(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("DOCX 读取失败: %v", err)
	}
	defer doc.Close()
	xml := doc.Editable().GetContent()
	xml = strings.ReplaceAll(xml, "</w:p>", "\n")
	xml = strings.ReplaceAll(xml, "<w:tab/>", "\t")
	text := xmlTagRe.ReplaceAllString(xml, "")

	// 压缩多余空行
	lines := strings.Split(text, "\n")
	var b strings.Builder
	blank := 0
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			blank++
			if blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		b.WriteString(l)
		b.WriteString("\n")
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "", fmt.Errorf("DOCX 无文本内容")
	}
	return out, nil
}

// xmlTagRe 匹配 XML 标签。
var xmlTagRe = regexp.MustCompile(`<[^>]+>`)
