package profile

import (
	"testing"
	"time"

	"mirror-mian/internal/model"
)

func today() time.Time {
	return time.Date(2026, 8, 28, 0, 0, 0, 0, time.Local)
}

func TestSM2TypicalSequence(t *testing.T) {
	// 连续高分：间隔 0→1→3→trunc(3×ease)，验证典型序列
	state := NewSM2State(9, today()) // 第 1 次：间隔 1 天
	if state.IntervalDays != 1 || state.Repetitions != 1 {
		t.Fatalf("first: interval=%d reps=%d", state.IntervalDays, state.Repetitions)
	}
	if state.NextReview != "2026-08-29" {
		t.Fatalf("next review = %s", state.NextReview)
	}

	state = sm2Update(state, 9, today().AddDate(0, 0, 1)) // 第 2 次：间隔 3 天
	if state.IntervalDays != 3 || state.Repetitions != 2 {
		t.Fatalf("second: interval=%d reps=%d", state.IntervalDays, state.Repetitions)
	}

	// ease 从 1.3 起步：9 分 → q5 → ease = 1.3 + 0.1 = 1.4（浮点近似）
	if state.EaseFactor < 1.39 || state.EaseFactor > 1.41 {
		t.Fatalf("ease = %v", state.EaseFactor)
	}

	state = sm2Update(state, 9, today().AddDate(0, 0, 4)) // 第 3 次：trunc(3×1.4)=4 天
	if state.IntervalDays != 4 || state.Repetitions != 3 {
		t.Fatalf("third: interval=%d reps=%d", state.IntervalDays, state.Repetitions)
	}
}

func TestSM2LowScoreReset(t *testing.T) {
	state := NewSM2State(4, today()) // q2 < 3 → 重置：间隔 1 天、repetitions=0
	if state.IntervalDays != 1 || state.Repetitions != 0 {
		t.Fatalf("low score should reset: interval=%d reps=%d", state.IntervalDays, state.Repetitions)
	}
	// 且不应计入复习次数递增：下一次高分仍是 1 天
	state = sm2Update(state, 9, today().AddDate(0, 0, 1))
	if state.IntervalDays != 1 || state.Repetitions != 1 {
		t.Fatalf("after reset+high: interval=%d reps=%d", state.IntervalDays, state.Repetitions)
	}
}

func TestSM2EaseFloor(t *testing.T) {
	// 低分连续重置时 ease 不跌破 1.3
	state := NewSM2State(1, today()) // q0
	if state.EaseFactor < 1.3 {
		t.Fatalf("ease below floor: %v", state.EaseFactor)
	}
	state = sm2Update(state, 1, today().AddDate(0, 0, 1))
	if state.EaseFactor < 1.3 {
		t.Fatalf("ease below floor after second low: %v", state.EaseFactor)
	}
}

func TestDueReviews(t *testing.T) {
	mk := func(id string, next string, ease float64, improved bool) WeakPoint {
		return WeakPoint{
			ID: id, Point: "p-" + id, Topic: "t",
			Improved: improved,
			SR:       model.SM2State{IntervalDays: 1, EaseFactor: ease, NextReview: next},
		}
	}
	now := today()
	ws := []WeakPoint{
		mk("due-hard", "2026-08-27", 1.4, false),   // 到期，ease 小 → 排前
		mk("due-easy", "2026-08-28", 2.5, false),   // 到期
		mk("future", "2026-08-29", 1.5, false),     // 未到期
		mk("improved", "2026-08-27", 1.3, true),    // 已改进 → 排除
	}
	due := DueReviews(ws, now)
	if len(due) != 2 {
		t.Fatalf("due count = %d, want 2", len(due))
	}
	if due[0].ID != "due-hard" || due[1].ID != "due-easy" {
		t.Fatalf("due order wrong: %s, %s", due[0].ID, due[1].ID)
	}
}

func TestReviewWeakPoint(t *testing.T) {
	w := &WeakPoint{ID: "w", Point: "p", Topic: "t", SR: NewSM2State(4, today())}
	// 高分复习：标记改进
	ReviewWeakPoint(w, 9, today().AddDate(0, 0, 5))
	if !w.Improved {
		t.Fatal("high score review should mark improved")
	}
	if w.SR.IntervalDays != 1 { // 之前被低分重置，reps=0 → 1 天
		t.Fatalf("interval = %d", w.SR.IntervalDays)
	}
	// 再低分复习：清除改进
	ReviewWeakPoint(w, 3, today().AddDate(0, 0, 10))
	if w.Improved {
		t.Fatal("low score review should clear improved")
	}
	if len(w.SR.History) != 3 {
		t.Fatalf("history length = %d, want 3", len(w.SR.History))
	}
}
