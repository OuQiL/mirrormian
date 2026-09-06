// 领域管理服务：领域 CRUD + 内容文件化管理（kb/<topic>/）+ 向量同步。
// 文件系统是领域的唯一事实来源，SQLite 向量是派生物（可随时 sync 重建）。
package domain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"mirror-mian/internal/embedding"
	"mirror-mian/internal/llm"
	"mirror-mian/internal/rag"
	"mirror-mian/internal/store"
	"mirror-mian/internal/vector"
)

const (
	readmeTemplate = "# %s\n\n## 领域总览\n\n描述这个领域覆盖的知识范围。\n\n## 核心概念\n\n- \n\n## 常见陷阱\n\n- \n"
	highFreqTemplate = "# 高频题库\n\n面试常见问题、易错点、速记清单（每行一个考点或问题）。\n\n- \n"
)

// Service 领域管理服务。
type Service struct {
	root     string // kb/ 根目录
	store    store.Store
	embedder embedding.Embedder
	rag      *rag.Service
	llm      llm.Client // 生成核心知识梳理用（可空：nil 时 GenerateCore 不可用）
	milvus   *vector.Milvus // 向量双写（可 nil：降级）
}

// NewService 创建领域服务。
func NewService(root string, st store.Store, emb embedding.Embedder, llm llm.Client) *Service {
	return &Service{root: root, store: st, embedder: emb, rag: rag.NewService(emb, st), llm: llm}
}

// SetMilvus 设置 Milvus 客户端（同步时双写向量；nil 时跳过）。
func (s *Service) SetMilvus(m *vector.Milvus) { s.milvus = m }

// Domain 领域信息（含统计）。
type Domain struct {
	Name       string         `json:"name"`
	Files      []string       `json:"files"`
	ChunkCount int            `json:"chunk_count"`
	Topics     map[string]int `json:"topics"` // 知识库块数（含 high_freq）
	Stats      DomainStats    `json:"stats"`
}

// DomainStats 领域统计（来自画像）。
type DomainStats struct {
	SessionCount int     `json:"session_count"`
	Mastery      float64 `json:"mastery"`
}

// topicDir 领域目录。
func (s *Service) topicDir(topic string) string {
	return filepath.Join(s.root, topic)
}

// sanitize 领域名合法性：禁止路径分隔符与危险字符。
func sanitize(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("领域名不能为空")
	}
	if strings.ContainsAny(name, `/\:*?"<>|`) {
		return "", fmt.Errorf("领域名不能包含路径分隔符或特殊字符：%s", name)
	}
	if strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("领域名不能以 . 开头")
	}
	return name, nil
}

// Create 创建领域（目录 + 模板文件）。
func (s *Service) Create(name string) error {
	name, err := sanitize(name)
	if err != nil {
		return err
	}
	dir := s.topicDir(name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("领域 %q 已存在", name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建目录: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), fmt.Appendf(nil, readmeTemplate, name), 0o644); err != nil {
		return fmt.Errorf("写入 README: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "high_freq.md"), []byte(highFreqTemplate), 0o644); err != nil {
		return fmt.Errorf("写入高频题库: %w", err)
	}
	return nil
}

// Delete 删除领域：目录 + 知识库块/画像数据。
func (s *Service) Delete(name string) error {
	name, err := sanitize(name)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(s.topicDir(name)); err != nil {
		return fmt.Errorf("删除目录: %w", err)
	}
	return s.store.DeleteTopicData(name)
}

// Rename 重命名领域：移动目录 + 多表主题一致性更新。
func (s *Service) Rename(oldName, newName string) error {
	oldName, err := sanitize(oldName)
	if err != nil {
		return err
	}
	newName, err = sanitize(newName)
	if err != nil {
		return err
	}
	oldDir, newDir := s.topicDir(oldName), s.topicDir(newName)
	if _, err := os.Stat(oldDir); err != nil {
		return fmt.Errorf("领域 %q 不存在", oldName)
	}
	if _, err := os.Stat(newDir); err == nil {
		return fmt.Errorf("领域 %q 已存在", newName)
	}
	if err := os.Rename(oldDir, newDir); err != nil {
		return fmt.Errorf("移动目录: %w", err)
	}
	if err := s.store.RenameTopic(oldName, newName); err != nil {
		return fmt.Errorf("更新数据主题: %w", err)
	}
	return nil
}

// List 列出全部领域及统计。
func (s *Service) List() ([]Domain, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return []Domain{}, nil
		}
		return nil, err
	}
	chunkCounts, err := s.store.ListAllTopics()
	if err != nil {
		return nil, err
	}
	p, err := s.store.GetProfile()
	if err != nil {
		return nil, err
	}

	var out []Domain
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := s.topicDir(e.Name())
		files, _ := os.ReadDir(dir)
		var fileNames []string
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
				fileNames = append(fileNames, f.Name())
			}
		}
		d := Domain{
			Name:       e.Name(),
			Files:      fileNames,
			ChunkCount: chunkCounts[e.Name()],
			Topics:     map[string]int{e.Name(): chunkCounts[e.Name()]},
		}
		if m, ok := p.Mastery[e.Name()]; ok {
			d.Stats = DomainStats{SessionCount: m.SessionCount, Mastery: m.Score}
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Files 列出领域 Markdown 文件。
func (s *Service) Files(topic string) ([]string, error) {
	topic, err := sanitize(topic)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.topicDir(topic))
	if err != nil {
		return nil, fmt.Errorf("领域 %q 不存在", topic)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// ReadFile 读取领域内文件。
func (s *Service) ReadFile(topic, file string) (string, error) {
	topic, err := sanitize(topic)
	if err != nil {
		return "", err
	}
	file = filepath.Base(file) // 防目录穿越
	raw, err := os.ReadFile(filepath.Join(s.topicDir(topic), file))
	if err != nil {
		return "", fmt.Errorf("读取文件: %w", err)
	}
	return string(raw), nil
}

// WriteFile 保存领域内文件（覆盖）。
func (s *Service) WriteFile(topic, file, content string) error {
	topic, err := sanitize(topic)
	if err != nil {
		return err
	}
	file = filepath.Base(file)
	if !strings.HasSuffix(file, ".md") {
		return fmt.Errorf("仅支持 .md 文件")
	}
	return os.WriteFile(filepath.Join(s.topicDir(topic), file), []byte(content), 0o644)
}

// Sync 重建领域向量：扫描全部 .md（含 high_freq.md）→ 分块 → 向量化 → 入库。
func (s *Service) Sync(ctx context.Context, topic string) (int, error) {
	topic, err := sanitize(topic)
	if err != nil {
		return 0, err
	}
	if !s.embedder.Available() {
		return 0, fmt.Errorf("Embedding 未配置：请设置 LLM_EMBEDDING_BASE_URL/LLM_EMBEDDING_API_KEY/LLM_EMBEDDING_MODEL")
	}
	files, err := s.Files(topic)
	if err != nil {
		return 0, err
	}
	var allText strings.Builder
	for _, f := range files {
		content, err := s.ReadFile(topic, f)
		if err != nil {
			return 0, err
		}
		// 高频题库加来源标记，供出题区分
		if f == "high_freq.md" {
			allText.WriteString("\n[高频考点]\n")
		}
		allText.WriteString(content)
		allText.WriteString("\n")
	}
	chunks := rag.ChunkText(allText.String(), 0, 0)
	if len(chunks) == 0 {
		// 空内容也清空该主题向量
		return 0, s.store.DeleteTopic(topic)
	}
	vecs, err := s.embedder.Embed(ctx, chunks)
	if err != nil {
		return 0, fmt.Errorf("向量化: %w", err)
	}
	records := make([]store.KnowledgeChunk, len(chunks))
	for i, c := range chunks {
		records[i] = store.KnowledgeChunk{Topic: topic, Content: c, Embedding: vecs[i], Index: i}
	}
	if err := s.store.SaveTopicChunks(topic, records); err != nil {
		return 0, fmt.Errorf("入库: %w", err)
	}
	// Milvus 双写（失败仅提示降级，不阻断）
	if s.milvus != nil && s.milvus.Available() {
		if err := s.milvus.Upsert(context.Background(), topic, records); err != nil {
			fmt.Printf("⚠ Milvus 双写失败（%v），向量检索将回退 SQLite\n", err)
		}
	}
	// 同步重建 BM25 索引（多路召回）
	_ = s.rag.RebuildIndex(topic)
	return len(chunks), nil
}

// GenerateCore 用 LLM 生成领域核心知识梳理（提示词对齐 TechSpar generateCore），
// 写入 README.md 并同步向量；返回生成内容。
func (s *Service) GenerateCore(ctx context.Context, topic string) (string, error) {
	topic, err := sanitize(topic)
	if err != nil {
		return "", err
	}
	if s.llm == nil {
		return "", fmt.Errorf("LLM 未配置，无法生成核心知识梳理")
	}
	if _, err := os.Stat(s.topicDir(topic)); err != nil {
		return "", fmt.Errorf("领域 %q 不存在", topic)
	}
	const system = "你是一位资深技术面试官，擅长梳理技术领域的核心知识体系。"
	user := fmt.Sprintf(`请为「%s」这个技术领域生成一份核心知识梳理，作为面试出题和评分的参考依据。

要求：
- 用 Markdown 格式
- 以 `+"`# %s`"+` 作为标题
- 列出该领域最核心的 8-12 个知识点，每个用二级标题
- 每个知识点下用简洁的要点说明关键概念、原理、常见面试考点
- 重点覆盖：核心概念、工作原理、最佳实践、常见陷阱
- 保持简洁实用，面向面试准备场景
- 直接输出 Markdown 内容，不要包裹在代码块中`, topic, topic)

	content, err := s.llm.Generate(ctx, system, user)
	if err != nil {
		return "", fmt.Errorf("生成核心梳理: %w", err)
	}
	if err := os.WriteFile(filepath.Join(s.topicDir(topic), "README.md"), []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("写入 README: %w", err)
	}
	// 生成后自动同步，内容立即参与出题检索
	if _, err := s.Sync(ctx, topic); err != nil {
		return content, fmt.Errorf("README 已写入但同步失败: %w", err)
	}
	return content, nil
}

// HighFreqContext 返回领域高频题上下文段落（出题注入用，全部内容不检索）。
func (s *Service) HighFreqContext(topic string) string {
	content, err := s.ReadFile(topic, "high_freq.md")
	if err != nil || content == "" {
		return ""
	}
	// 去掉模板默认的注释行
	lines := strings.Split(content, "\n")
	var b strings.Builder
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") || l == "-" {
			continue
		}
		b.WriteString(l)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
