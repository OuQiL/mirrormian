package store

import (
	"path/filepath"
	"testing"
	"time"

	"mirror-mian/internal/model"
)

func newTestStore(t *testing.T) Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSessionRoundTrip(t *testing.T) {
	s := newTestStore(t)
	sess := &model.TrainingSession{
		ID:     "sess-1",
		Mode:   model.ModeSpecial,
		Status: model.SessionFinished,
		Direction: model.Direction{
			Mode:      model.ModeSpecial,
			Topic:     "Redis",
			KeyPoints: []string{"持久化", "淘汰策略"},
		},
		Questions: []model.Question{
			{ID: "q1", Text: "Redis 持久化有哪几种？", KnowledgePt: "持久化", Round: 0},
		},
		Answers: []model.Answer{
			{QuestionID: "q1", Text: "RDB 和 AOF", Score: 8},
		},
		Review:    "# 复盘",
		CreatedAt: time.Now(),
		FinishedAt: time.Now(),
	}
	if err := s.SaveSession(sess); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	got, err := s.GetSession("sess-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Mode != model.ModeSpecial || got.Review != "# 复盘" {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
	if len(got.Questions) != 1 || got.Questions[0].Text != "Redis 持久化有哪几种？" {
		t.Fatalf("questions roundtrip mismatch: %+v", got.Questions)
	}
	if len(got.Answers) != 1 || got.Answers[0].Score != 8 {
		t.Fatalf("answers roundtrip mismatch: %+v", got.Answers)
	}
	if len(got.Direction.KeyPoints) != 2 {
		t.Fatalf("direction roundtrip mismatch: %+v", got.Direction)
	}
}

func TestSessionUpdate(t *testing.T) {
	s := newTestStore(t)
	sess := &model.TrainingSession{ID: "s1", Mode: model.ModeSpecial, Status: model.SessionOngoing, CreatedAt: time.Now()}
	if err := s.SaveSession(sess); err != nil {
		t.Fatal(err)
	}
	sess.Status = model.SessionFinished
	sess.Answers = []model.Answer{{QuestionID: "q", Text: "a", Score: 5}}
	if err := s.SaveSession(sess); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetSession("s1")
	if got.Status != model.SessionFinished || len(got.Answers) != 1 {
		t.Fatalf("update failed: %+v", got)
	}
}

func TestProfileRoundTrip(t *testing.T) {
	s := newTestStore(t)
	now := time.Now()
	p := &model.Profile{
		WeakPoints: []model.WeakPoint{
			{
				ID: "w1", Point: "Redis 持久化理解不深", Topic: "Redis",
				FirstSeen: now, LastSeen: now, TimesSeen: 2, Improved: false,
				SR: model.SM2State{IntervalDays: 1, EaseFactor: 2.5, Repetitions: 0, NextReview: "2026-08-29", LastScore: 4,
					History: []model.SM2Event{{Date: "2026-08-28", Event: "reviewed", Score: 4}}},
			},
		},
		Mastery: map[string]*model.Mastery{
			"Redis": {Topic: "Redis", Score: 62.5, SessionCount: 2, LastAssessed: now},
		},
	}
	if err := s.SaveProfile(p); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}
	got, err := s.GetProfile()
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if len(got.WeakPoints) != 1 || got.WeakPoints[0].Point != "Redis 持久化理解不深" {
		t.Fatalf("weakpoints roundtrip: %+v", got.WeakPoints)
	}
	if got.WeakPoints[0].SR.IntervalDays != 1 || got.WeakPoints[0].SR.EaseFactor != 2.5 {
		t.Fatalf("sm2 roundtrip: %+v", got.WeakPoints[0].SR)
	}
	if len(got.WeakPoints[0].SR.History) != 1 {
		t.Fatalf("sm2 history roundtrip: %+v", got.WeakPoints[0].SR.History)
	}
	m, ok := got.Mastery["Redis"]
	if !ok || m.Score != 62.5 || m.SessionCount != 2 {
		t.Fatalf("mastery roundtrip: %+v", got.Mastery)
	}
}
