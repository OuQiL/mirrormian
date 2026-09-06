package rag

import (
	"strings"
	"testing"
)

func TestChunkShortText(t *testing.T) {
	chunks := ChunkText("只有一段。", 1000, 150)
	if len(chunks) != 1 || chunks[0] != "只有一段。" {
		t.Fatalf("chunks = %+v", chunks)
	}
}

func TestChunkEmpty(t *testing.T) {
	if got := ChunkText("", 1000, 150); len(got) != 0 {
		t.Fatalf("empty should yield none: %+v", got)
	}
}

func TestChunkOverlap(t *testing.T) {
	// 两段拼接超过 size：第二块应携带第一块尾部重叠
	para1 := strings.Repeat("甲", 600)
	para2 := strings.Repeat("乙", 600)
	chunks := ChunkText(para1+"\n\n"+para2, 1000, 150)
	if len(chunks) != 2 {
		t.Fatalf("chunks = %d, want 2", len(chunks))
	}
	// 第二块开头应含第一块末尾的重叠部分（150 字符）
	if !strings.HasPrefix(chunks[1], strings.Repeat("甲", 150)) {
		t.Fatalf("chunk2 should overlap chunk1 tail: %q...", chunks[1][:40])
	}
}

func TestChunkLongParagraph(t *testing.T) {
	// 单段超 2000 字符：滑动切分
	long := strings.Repeat("丁", 2500)
	chunks := ChunkText(long, 1000, 150)
	if len(chunks) < 2 {
		t.Fatalf("long paragraph should slide: %d chunks", len(chunks))
	}
	total := 0
	for _, c := range chunks {
		if n := len([]rune(c)); n > 1000 {
			t.Fatalf("chunk too long: %d runes", n)
		}
		total += len([]rune(c))
	}
	// 重叠意味着总字符数超过原文
	if total <= 2500 {
		t.Fatalf("overlap should inflate total: %d", total)
	}
}

func TestChunkNoOverlap(t *testing.T) {
	para1 := strings.Repeat("甲", 600)
	para2 := strings.Repeat("乙", 600)
	chunks := ChunkText(para1+"\n\n"+para2, 1000, 0)
	if len(chunks) != 2 {
		t.Fatalf("chunks = %d", len(chunks))
	}
	if strings.Contains(chunks[1], "甲") {
		t.Fatal("overlap=0 should not carry tail")
	}
}
