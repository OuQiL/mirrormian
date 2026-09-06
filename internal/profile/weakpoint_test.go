package profile

import (
	"context"
	"testing"

	"mirror-mian/internal/embedding"
	"mirror-mian/internal/model"
)

// fakeEmbedder 字符袋向量（与 rag 测试同构）：共享字符越多余弦越高。
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

// 低分 → 新薄弱点
func TestAbsorbAnswerNew(t *testing.T) {
	p := &model.Profile{WeakPoints: []model.WeakPoint{}, Mastery: map[string]*model.Mastery{}}
	now := today()

	hit := AbsorbAnswer(p, "Redis", "Redis 持久化机制理解不深", 4, now, nil)
	if hit {
		t.Fatal("new weak point should not hit")
	}
	if len(p.WeakPoints) != 1 {
		t.Fatalf("weak points = %d, want 1", len(p.WeakPoints))
	}
	w := p.WeakPoints[0]
	if w.TimesSeen != 1 || w.Improved {
		t.Fatalf("new weak point state wrong: %+v", w)
	}
	if w.SR.IntervalDays != 1 || w.SR.Repetitions != 0 {
		t.Fatalf("new weak point SM2 should be reset: %+v", w.SR)
	}
}

// 精确匹配退化（embedder 为 nil → 降级）
func TestAbsorbAnswerRegressedExact(t *testing.T) {
	now := today()
	p := &model.Profile{WeakPoints: []model.WeakPoint{
		{ID: "w1", Point: "Redis 持久化机制理解不深", Topic: "Redis",
			FirstSeen: now, LastSeen: now, TimesSeen: 1, SR: NewSM2State(9, now)},
	}, Mastery: map[string]*model.Mastery{}}

	hit := AbsorbAnswer(p, "Redis", "  Redis 持久化机制理解不深 ", 3, now.AddDate(0, 0, 3), nil)
	if !hit {
		t.Fatal("same point should hit existing")
	}
	w := p.WeakPoints[0]
	if w.TimesSeen != 2 || w.Improved {
		t.Fatalf("should regress: %+v", w)
	}
	if w.SR.IntervalDays != 1 || w.SR.Repetitions != 0 {
		t.Fatalf("regressed should reset SM2: %+v", w.SR)
	}
}

// 向量去重：相似表述合并（embedder 可用）
func TestAbsorbAnswerVectorMerge(t *testing.T) {
	now := today()
	p := &model.Profile{WeakPoints: []model.WeakPoint{
		{ID: "w1", Point: "Redis 持久化理解不深", Topic: "Redis",
			FirstSeen: now, LastSeen: now, TimesSeen: 1, SR: NewSM2State(4, now)},
	}, Mastery: map[string]*model.Mastery{}}

	// 相似表述（共享大量字符）→ 向量命中合并
	hit := AbsorbAnswer(p, "Redis", "Redis 持久化机制理解不足", 3, now.AddDate(0, 0, 1), fakeEmbedder{})
	if !hit {
		t.Fatal("similar point should merge via vector")
	}
	if len(p.WeakPoints) != 1 {
		t.Fatalf("weak points = %d, want 1 (merged)", len(p.WeakPoints))
	}
	if p.WeakPoints[0].TimesSeen != 2 {
		t.Fatalf("times_seen = %d, want 2", p.WeakPoints[0].TimesSeen)
	}
}

// 向量不相似 → 新建（embedder 可用但语义差远）
func TestAbsorbAnswerVectorNoMerge(t *testing.T) {
	now := today()
	p := &model.Profile{WeakPoints: []model.WeakPoint{
		{ID: "w1", Point: "Redis 持久化机制理解不深", Topic: "Redis",
			FirstSeen: now, LastSeen: now, TimesSeen: 1, SR: NewSM2State(4, now)},
	}, Mastery: map[string]*model.Mastery{}}

	hit := AbsorbAnswer(p, "Redis", "MySQL 索引设计原则", 4, now, fakeEmbedder{})
	if hit {
		t.Fatal("dissimilar point should not merge")
	}
	if len(p.WeakPoints) != 2 {
		t.Fatalf("weak points = %d, want 2", len(p.WeakPoints))
	}
}

// embedder 可用但调用失败 → 回退精确匹配
func TestAbsorbAnswerVectorFallback(t *testing.T) {
	now := today()
	p := &model.Profile{WeakPoints: []model.WeakPoint{
		{ID: "w1", Point: "MySQL 索引", Topic: "MySQL",
			FirstSeen: now, LastSeen: now, TimesSeen: 1, SR: NewSM2State(4, now)},
	}, Mastery: map[string]*model.Mastery{}}

	// 同主题同文本 → 精确匹配命中（即使向量失败）
	hit := AbsorbAnswer(p, "MySQL", "MySQL 索引", 4, now, failingEmbedder{})
	if !hit {
		t.Fatal("should fallback to exact match")
	}
	if len(p.WeakPoints) != 1 {
		t.Fatalf("weak points = %d", len(p.WeakPoints))
	}
}

// failingEmbedder 模拟 embedding 调用失败。
type failingEmbedder struct{}

func (failingEmbedder) Available() bool { return true }
func (failingEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	return nil, context.Canceled
}

// 高分改进
func TestAbsorbAnswerImproved(t *testing.T) {
	now := today()
	p := &model.Profile{WeakPoints: []model.WeakPoint{
		{ID: "w1", Point: "MySQL 索引", Topic: "MySQL",
			FirstSeen: now, LastSeen: now, TimesSeen: 2, SR: NewSM2State(4, now)},
	}, Mastery: map[string]*model.Mastery{}}

	hit := AbsorbAnswer(p, "MySQL", "MySQL 索引", 9, now.AddDate(0, 0, 5), nil)
	if !hit {
		t.Fatal("same point should hit existing")
	}
	if !p.WeakPoints[0].Improved {
		t.Fatal("high score should mark improved")
	}
	if len(p.WeakPoints[0].SR.History) < 2 {
		t.Fatalf("history should have improved event: %+v", p.WeakPoints[0].SR.History)
	}
}

// 不同主题不合并
func TestAbsorbAnswerDifferentTopicNoMerge(t *testing.T) {
	now := today()
	p := &model.Profile{WeakPoints: []model.WeakPoint{
		{ID: "w1", Point: "持久化", Topic: "Redis", FirstSeen: now, LastSeen: now, TimesSeen: 1, SR: NewSM2State(4, now)},
	}, Mastery: map[string]*model.Mastery{}}

	hit := AbsorbAnswer(p, "MySQL", "持久化", 4, now, fakeEmbedder{})
	if hit {
		t.Fatal("different topic should not merge")
	}
	if len(p.WeakPoints) != 2 {
		t.Fatalf("weak points = %d, want 2", len(p.WeakPoints))
	}
}

func TestUpdateMasteryConverges(t *testing.T) {
	now := today()
	m := &model.Mastery{Topic: "Redis", Score: 0, SessionCount: 0, LastAssessed: now}

	for i := 0; i < 3; i++ {
		UpdateMastery(m, 80, now.AddDate(0, 0, i))
	}
	if m.Score < 70 || m.Score > 90 {
		t.Fatalf("mastery should approach 80, got %v", m.Score)
	}
	if m.SessionCount != 3 {
		t.Fatalf("session count = %d", m.SessionCount)
	}
}

func TestSessionScore(t *testing.T) {
	answers := []model.Answer{
		{Score: 8}, {Score: 6}, {Score: 10},
	}
	if got := SessionScore(answers); got != 80 {
		t.Fatalf("SessionScore = %v, want 80", got)
	}
}

var _ embedding.Embedder = fakeEmbedder{}
