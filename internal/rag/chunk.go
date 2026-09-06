// 知识库分块（移植自 TechSpar chunkText：按空行分段落累积，
// 块 ≤1000 字符、块间 150 字符重叠，超长段再滑动切分）。
package rag

import "strings"

const (
	defaultChunkSize    = 1000
	defaultChunkOverlap = 150
	maxSlideSize        = 2000
)

// ChunkText 把文本切分为带重叠的块。
func ChunkText(text string, size, overlap int) []string {
	if size <= 0 {
		size = defaultChunkSize
	}
	if overlap < 0 {
		overlap = defaultChunkOverlap
	}
	paragraphs := strings.Split(text, "\n\n")

	var chunks []string
	var buf strings.Builder

	flush := func() {
		if buf.Len() > 0 {
			chunks = append(chunks, buf.String())
			buf.Reset()
		}
	}

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// 段落过长：先 flush 累积，再滑动切分
		if len(p) > maxSlideSize {
			flush()
			chunks = append(chunks, slideChunk(p, size, overlap)...)
			continue
		}
		// 累积后超限：flush（带 overlap 重叠）
		if buf.Len() > 0 && buf.Len()+len(p)+2 > size {
			if overlap > 0 && buf.Len() > overlap {
				chunks = append(chunks, buf.String())
				rest := buf.String()
				buf.Reset()
				buf.WriteString(overlapTail(rest, overlap))
			} else {
				flush()
			}
		}
		if buf.Len() > 0 {
			buf.WriteString("\n\n")
		}
		buf.WriteString(p)
	}
	flush()
	return chunks
}

// slideChunk 超长文本按 size 滑动切分（按字符，避免切坏 UTF-8），相邻块间 overlap 重叠。
func slideChunk(text string, size, overlap int) []string {
	runes := []rune(text)
	var out []string
	for i := 0; i < len(runes); i += size - overlap {
		end := min(i+size, len(runes))
		out = append(out, string(runes[i:end]))
		if end == len(runes) {
			break
		}
	}
	return out
}

// overlapTail 取块末尾 overlap 个字符作为下一块的开头重叠（按字符截断）。
func overlapTail(s string, overlap int) string {
	runes := []rune(s)
	if len(runes) <= overlap {
		return s
	}
	return string(runes[len(runes)-overlap:])
}
