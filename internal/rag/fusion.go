// RRF 融合排序：score = Σ 1/(k + rank)，k=60（对齐用户要求）。
package rag

const rrfK = 60

// rrfMerge 融合多路召回结果：以内容前缀（前 100 字符）为去重键，按 RRF 分数降序。
func rrfMerge(rankedLists ...[]Result) []Result {
	key := func(r Result) string { return prefixKey(r.Content) }

	score := map[string]float64{}
	order := []string{}
	for _, list := range rankedLists {
		for rank, r := range list {
			k := key(r)
			if _, ok := score[k]; !ok {
				order = append(order, k)
			}
			score[k] += 1.0 / float64(rrfK+rank+1) // rank 从 0 起，公式 1/(k+rank+1)
		}
	}

	// 分数降序
	for i := 1; i < len(order); i++ {
		for j := i; j > 0 && score[order[j]] > score[order[j-1]]; j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}

	// 恢复 Result（取第一个出现的内容）
	contentOf := map[string]Result{}
	for _, list := range rankedLists {
		for _, r := range list {
			if _, ok := contentOf[key(r)]; !ok {
				contentOf[key(r)] = r
			}
		}
	}
	out := make([]Result, 0, len(order))
	for _, k := range order {
		out = append(out, contentOf[k])
	}
	return out
}
