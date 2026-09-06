// BM25 关键词检索：纯内存倒排索引。
// 分词：中文连续段按 bigram（相邻字符对）+ 英文/数字按词（小写）。
package rag

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"mirror-mian/internal/store"
)

const (
	bm25K1 = 1.5
	bm25B  = 0.75
)

// bm25Doc 索引文档。
type bm25Doc struct {
	id    string
	terms []string
}

// BM25Index 内存倒排索引。
type BM25Index struct {
	docs       []bm25Doc
	postings   map[string][]int // term → doc id 列表
	df         map[string]int   // term 文档频率
	docLen     []int
	avgDocLen  float64
}

// BuildBM25 从知识块构建索引。
func BuildBM25(chunks []store.KnowledgeChunk) *BM25Index {
	idx := &BM25Index{
		postings: map[string][]int{},
		df:       map[string]int{},
	}
	var totalLen int
	for _, c := range chunks {
		terms := tokenize(c.Content)
		docID := len(idx.docs)
		idx.docs = append(idx.docs, bm25Doc{id: fmt.Sprintf("%d", c.ID), terms: terms})
		idx.docLen = append(idx.docLen, len(terms))
		totalLen += len(terms)

		// 文档内去重后登记 postings/df
		seen := map[string]bool{}
		for _, t := range terms {
			if seen[t] {
				continue
			}
			seen[t] = true
			if _, ok := idx.postings[t]; !ok {
				idx.postings[t] = []int{}
			}
			idx.postings[t] = append(idx.postings[t], docID)
		}
	}
	for t, ids := range idx.postings {
		idx.df[t] = len(ids)
		_ = ids
	}
	if len(idx.docs) > 0 {
		idx.avgDocLen = float64(totalLen) / float64(len(idx.docs))
	}
	return idx
}

// Search 按查询返回文档下标（BM25 分数降序），topK 上限。
func (b *BM25Index) Search(query string, topK int) []int {
	if topK <= 0 {
		topK = defaultTopK
	}
	queryTerms := tokenize(query)
	if len(queryTerms) == 0 || len(b.docs) == 0 {
		return nil
	}
	// 词频统计（查询内）
	qf := map[string]int{}
	for _, t := range queryTerms {
		qf[t]++
	}

	scores := make([]float64, len(b.docs))
	for term, count := range qf {
		ids, ok := b.postings[term]
		if !ok {
			continue
		}
		df := b.df[term]
		idf := math.Log(1 + (float64(len(b.docs))-float64(df)+0.5)/(float64(df)+0.5))
		for _, docID := range ids {
			// 文档内词频
			tf := 0
			for _, t := range b.docs[docID].terms {
				if t == term {
					tf++
				}
			}
			denom := float64(tf) + bm25K1*(1-bm25B+bm25B*float64(b.docLen[docID])/b.avgDocLen)
			if denom == 0 {
				continue
			}
			scores[docID] += idf * float64(count) * float64(tf) * (bm25K1 + 1) / denom
		}
	}

	// 按分数降序取 topK
	order := make([]int, 0, len(b.docs))
	for i := range scores {
		if scores[i] > 0 {
			order = append(order, i)
		}
	}
	sort.Slice(order, func(i, j int) bool { return scores[order[i]] > scores[order[j]] })
	if len(order) > topK {
		order = order[:topK]
	}
	return order
}

// tokenize 分词：中文连续段 bigram + 英文/数字按词。
func tokenize(s string) []string {
	var out []string
	var latin strings.Builder
	flushLatin := func() {
		if latin.Len() > 0 {
			out = append(out, strings.ToLower(latin.String()))
			latin.Reset()
		}
	}
	// 中文段收集 rune
	var han []rune
	flushHan := func() {
		for i := 0; i < len(han)-1; i++ {
			out = append(out, string(han[i:i+2]))
		}
		han = nil
	}
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Han, r):
			flushLatin()
			han = append(han, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			flushHan()
			latin.WriteRune(r)
		default:
			flushLatin()
			flushHan()
		}
	}
	flushLatin()
	flushHan()
	return out
}
