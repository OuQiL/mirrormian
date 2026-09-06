package domain

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mirror-mian/internal/model"
	"mirror-mian/internal/store"
)

// fakeEmbedder 字符袋向量（与 rag 测试同构）。
type fakeEmbedder struct{}

func (fakeEmbedder) Available() bool { return true }

func (fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v := make([]float32, 32)
		for _, r := range []rune(t) {
			v[int(r)%32] += 1
		}
		out[i] = v
	}
	return out, nil
}

func newTestDomain(t *testing.T) (*Service, store.Store) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "kb")
	st, err := store.Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return NewService(root, st, fakeEmbedder{}, nil), st
}

// 任务 1.1：创建/列表/删除/重命名
func TestDomainCRUD(t *testing.T) {
	svc, st := newTestDomain(t)

	// 创建
	if err := svc.Create("Redis"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := os.Stat(filepath.Join(svc.root, "Redis", "README.md")); err != nil {
		t.Fatalf("README not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(svc.root, "Redis", "high_freq.md")); err != nil {
		t.Fatalf("high_freq not created: %v", err)
	}
	// 重复创建报错
	if err := svc.Create("Redis"); err == nil {
		t.Fatal("duplicate create should error")
	}
	// 非法名
	if err := svc.Create("a/b"); err == nil {
		t.Fatal("path separator should error")
	}

	// 写入内容 + 同步
	if err := svc.WriteFile("Redis", "notes.md", "# Redis 持久化\nRDB 和 AOF 两种方式。"); err != nil {
		t.Fatal(err)
	}
	n, err := svc.Sync(context.Background(), "Redis")
	if err != nil || n == 0 {
		t.Fatalf("Sync: %v %d", err, n)
	}
	// 画像数据（模拟训练沉淀）
	p, _ := st.GetProfile()
	p.Mastery["Redis"] = &model.Mastery{Topic: "Redis", Score: 60, SessionCount: 2, LastAssessed: time.Now()}
	_ = st.SaveProfile(p)

	// 列表统计
	domains, err := svc.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(domains) != 1 || domains[0].Name != "Redis" {
		t.Fatalf("list = %+v", domains)
	}
	if domains[0].ChunkCount == 0 || domains[0].Stats.Mastery == 0 {
		t.Fatalf("stats missing: %+v", domains[0])
	}

	// 重命名：目录 + 三表一致
	if err := svc.Rename("Redis", "KVStore"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if _, err := os.Stat(filepath.Join(svc.root, "KVStore")); err != nil {
		t.Fatalf("dir not renamed: %v", err)
	}
	got, _ := st.GetProfile()
	if _, ok := got.Mastery["KVStore"]; !ok {
		t.Fatalf("mastery topic not renamed: %+v", got.Mastery)
	}
	if _, ok := got.Mastery["Redis"]; ok {
		t.Fatalf("old mastery topic still exists")
	}
	counts, _ := st.ListAllTopics()
	if counts["KVStore"] == 0 {
		t.Fatalf("chunks topic not renamed: %+v", counts)
	}

	// 删除：目录 + 数据
	if err := svc.Delete("KVStore"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(svc.root, "KVStore")); !os.IsNotExist(err) {
		t.Fatal("dir should be removed")
	}
	counts, _ = st.ListAllTopics()
	if len(counts) != 0 {
		t.Fatalf("chunks not cleaned: %+v", counts)
	}
}

// 任务 1.4：高频题库上下文
func TestHighFreqContext(t *testing.T) {
	svc, _ := newTestDomain(t)
	_ = svc.Create("Go")
	if err := svc.WriteFile("Go", "high_freq.md",
		"# 高频题库\n\n- channel 和 mutex 的区别\n- goroutine 泄漏怎么排查\n- \n"); err != nil {
		t.Fatal(err)
	}
	ctx := svc.HighFreqContext("Go")
	if !strings.Contains(ctx, "channel 和 mutex 的区别") {
		t.Fatalf("high freq missing: %q", ctx)
	}
	if strings.Contains(ctx, "# 高频题库") || strings.Contains(ctx, "- \n") {
		t.Fatalf("template noise not stripped: %q", ctx)
	}
}

// 任务 1.3：同步重建语义（文件变更 → 向量更新）
func TestSyncRebuild(t *testing.T) {
	svc, st := newTestDomain(t)
	_ = svc.Create("MySQL")
	_ = svc.WriteFile("MySQL", "a.md", strings.Repeat("索引原理", 300))
	n1, _ := svc.Sync(context.Background(), "MySQL")
	_ = svc.WriteFile("MySQL", "a.md", strings.Repeat("隔离级别", 300))
	n2, _ := svc.Sync(context.Background(), "MySQL")
	if n1 == 0 || n2 == 0 {
		t.Fatalf("sync counts: %d %d", n1, n2)
	}
	chunks, _ := st.ListTopicChunks("MySQL")
	if len(chunks) == 0 {
		t.Fatal("no chunks after sync")
	}
	found := false
	for _, c := range chunks {
		if strings.Contains(c.Content, "隔离级别") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("chunks not rebuilt with new content: %+v", chunks[:min(2, len(chunks))])
	}
}
