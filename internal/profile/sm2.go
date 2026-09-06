// SM-2 间隔重复算法（移植自 TechSpar profile-service.ts:99-109）。
//
// 质量分映射：score 0-2→q0、3-4→q2、5→q3、6-7→q4、8-10→q5
// 间隔：0 次→1 天、1 次→3 天、否则 trunc(interval×ease)；q<3 重置 interval=1, repetitions=0
// 难度因子：ease = max(1.3, ease + 0.1 - (5-q)×(0.08+(5-q)×0.02))
package profile

import (
	"math"
	"time"

	"mirror-mian/internal/model"
)

type (
	// SM2State / WeakPoint / SM2Event 复用 model 包定义。
	SM2State = model.SM2State
	WeakPoint = model.WeakPoint
	SM2Event  = model.SM2Event
)

const (
	minEaseFactor = 1.3
	qualityReset  = 3 // q < 3 重置
)

// NewSM2State 初始化新薄弱点的 SM-2 状态：首次间隔 1 天。
func NewSM2State(score float64, today time.Time) SM2State {
	return sm2Update(SM2State{}, score, today)
}

// sm2Update 依据本轮质量分更新 SM-2 状态。
// 与 TechSpar 行为对齐：repetitions=0 → 1 天；repetitions=1 → 3 天；否则 interval×ease。
func sm2Update(state SM2State, score float64, today time.Time) SM2State {
	q := qualityFromScore(score)

	var interval int
	switch state.Repetitions {
	case 0:
		interval = 1
	case 1:
		interval = 3
	default:
		interval = max(1, int(math.Trunc(float64(state.IntervalDays)*state.EaseFactor)))
	}

	if q < qualityReset {
		state.IntervalDays = 1
		state.Repetitions = 0
	} else {
		state.IntervalDays = interval
		state.Repetitions++
	}

	// ease 下限 1.3，标准 SM-2 公式
	ease := state.EaseFactor + 0.1 - float64(5-q)*(0.08+float64(5-q)*0.02)
	if ease < minEaseFactor {
		ease = minEaseFactor
	}
	state.EaseFactor = ease

	state.NextReview = today.AddDate(0, 0, state.IntervalDays).Format("2006-01-02")
	state.LastScore = score
	state.History = append(state.History, SM2Event{
		Date:  today.Format("2006-01-02"),
		Event: "reviewed",
		Score: score,
	})
	return state
}

// qualityFromScore 0-10 评分 → SM-2 质量分 q（0-5）。
func qualityFromScore(score float64) int {
	switch {
	case score <= 2:
		return 0
	case score <= 4:
		return 2
	case score <= 5:
		return 3
	case score <= 7:
		return 4
	default:
		return 5
	}
}

// DueReviews 过滤到期复习项：next_review ≤ 今天、未改进、未归档，
// 按 ease_factor 升序（最难的优先）——与 TechSpar dueReviews 对齐。
func DueReviews(weakPoints []WeakPoint, today time.Time) []WeakPoint {
	due := make([]WeakPoint, 0)
	todayStr := today.Format("2006-01-02")
	for _, w := range weakPoints {
		if w.Improved || w.Archived {
			continue
		}
		// YYYY-MM-DD 字典序即时间序，直接字符串比较避免时区偏移
		if w.SR.NextReview > todayStr {
			continue
		}
		due = append(due, w)
	}
	// 按难度升序：ease 越小越难，越优先
	for i := 1; i < len(due); i++ {
		for j := i; j > 0 && due[j].SR.EaseFactor < due[j-1].SR.EaseFactor; j-- {
			due[j], due[j-1] = due[j-1], due[j]
		}
	}
	return due
}

// ReviewWeakPoint 对薄弱点完成一次复习：更新 SM-2 状态并记录事件。
// 高分（≥8）标记改进（不再进入复习队列）；低分清除改进标记，
// 若此前为改进状态则记录退化事件。
func ReviewWeakPoint(w *WeakPoint, score float64, today time.Time) {
	w.SR = sm2Update(w.SR, score, today)
	w.LastSeen = today
	if len(w.SR.History) == 0 {
		return
	}
	last := &w.SR.History[len(w.SR.History)-1]
	if score >= improvedThreshold {
		w.Improved = true
		last.Event = "improved"
		return
	}
	if w.Improved {
		last.Event = "regressed"
	}
	w.Improved = false
}

const improvedThreshold = 8
