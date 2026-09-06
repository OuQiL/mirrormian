// 薄弱点沉淀：训练/复盘结果 → 画像。
//
// 与 TechSpar 对齐：
// - 新薄弱点初始化 SM-2 状态；
// - 同主题重复触发：times_seen++、标记退化、重置 SM-2 间隔；
// - 高分（≥8）触发改进标记，不再进入复习队列。
// 去重：Embedder 可用时按向量余弦相似度（≥0.75）合并；否则回退精确匹配。
package profile

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"mirror-mian/internal/embedding"
	"mirror-mian/internal/model"
)

// WeaknessThreshold 触发薄弱点的评分阈值（< 阈值记薄弱点）。
const WeaknessThreshold = 6.0

// WeakPointSimilarity 向量去重阈值（TechSpar WEAK_POINT_SIMILARITY=0.75）。
const WeakPointSimilarity = 0.75

// AbsorbAnswer 将一道题的回答沉淀进画像：
// score < 6 时，薄弱点文本按向量相似度（或精确）合并既有薄弱点；
// score ≥ 8 时尝试标记改进（命中既有薄弱点）。
// embedder 为 nil 或不可用时回退精确匹配（降级，不崩溃）。
func AbsorbAnswer(p *model.Profile, topic, weakText string, score float64, today time.Time, emb embedding.Embedder) bool {
	norm := normalizeKey(weakText)
	if p.WeakPoints == nil {
		p.WeakPoints = []model.WeakPoint{}
	}

	idx := matchIndex(p, topic, weakText, norm, emb)
	if idx >= 0 {
		w := &p.WeakPoints[idx]
		if score < WeaknessThreshold {
			w.TimesSeen++
			w.LastSeen = today
			w.Improved = false
			w.SR = sm2Update(w.SR, score, today) // 低分重置间隔
			w.SR.History = append(w.SR.History, model.SM2Event{
				Date: today.Format("2006-01-02"), Event: "regressed", Score: score,
			})
		} else if score >= improvedThreshold {
			w.Improved = true
			w.LastSeen = today
			w.SR.History = append(w.SR.History, model.SM2Event{
				Date: today.Format("2006-01-02"), Event: "improved", Score: score,
			})
		}
		return true
	}

	// 未命中：低分创建新薄弱点
	if score < WeaknessThreshold {
		w := model.WeakPoint{
			ID:        uuid.NewString(),
			Point:     weakText,
			Topic:     topic,
			FirstSeen: today,
			LastSeen:  today,
			TimesSeen: 1,
			SR:        NewSM2State(score, today),
		}
		p.WeakPoints = append(p.WeakPoints, w)
	}
	return false
}

// matchIndex 查找命中的既有薄弱点下标；-1 表示未命中。
// Embedder 可用时按向量相似度，否则精确匹配。
func matchIndex(p *model.Profile, topic, weakText, norm string, emb embedding.Embedder) int {
	if emb != nil && emb.Available() {
		if idx, ok := vectorMatch(p, topic, weakText, emb); ok {
			return idx
		}
		// 向量未命中再回退精确匹配（同主题同文本）
		for i := range p.WeakPoints {
			if p.WeakPoints[i].Topic == topic && normalizeKey(p.WeakPoints[i].Point) == norm {
				return i
			}
		}
		return -1
	}
	for i := range p.WeakPoints {
		if p.WeakPoints[i].Topic == topic && normalizeKey(p.WeakPoints[i].Point) == norm {
			return i
		}
	}
	return -1
}

// vectorMatch 批量向量化（新文本 + 同主题既有弱点），余弦 ≥ 阈值命中。
// 任何调用失败返回未命中（由调用方回退精确匹配）。
func vectorMatch(p *model.Profile, topic, weakText string, emb embedding.Embedder) (int, bool) {
	texts := []string{weakText}
	for _, w := range p.WeakPoints {
		if w.Topic == topic {
			texts = append(texts, w.Point)
		}
	}
	if len(texts) == 1 {
		return -1, false
	}
	vecs, err := emb.Embed(context.Background(), texts)
	if err != nil || len(vecs) != len(texts) {
		return -1, false
	}
	q := vecs[0]
	for i := 1; i < len(vecs); i++ {
		if embedding.Cosine(q, vecs[i]) >= WeakPointSimilarity {
			return i - 1, true
		}
	}
	return -1, false
}

// normalizeKey 薄弱点精确去重的规范化键：去空白、小写。
func normalizeKey(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}
