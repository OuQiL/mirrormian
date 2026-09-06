package orchestration

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mirror-mian/internal/model"
	"mirror-mian/internal/profile"
	"mirror-mian/internal/store"
)

// fakeLLM 端到端测试桩：按提示词特征返回预设 JSON。
type fakeLLM struct {
	calls []string
}

func (f *fakeLLM) ModelName() string { return "fake" }

func (f *fakeLLM) Generate(_ context.Context, system, user string) (string, error) {
	f.calls = append(f.calls, system)
	// JD/简历匹配度分析（JDResumeAnalyzer）
	if strings.Contains(system, "资深招聘专家") {
		return `{"responsibilities": ["开发", "维护"], "requirements": [{"item": "Go", "level": "hard", "status": "met", "evidence": "有 Go 经验"}, {"item": "MySQL", "level": "hard", "status": "missing", "evidence": ""}], "tech_stack": ["Go", "MySQL"], "match_score": 70, "gaps": [{"item": "MySQL", "severity": "high", "suggestion": "补索引知识"}], "summary": "整体匹配，缺 MySQL"}`, nil
	}
	// 方向规划（专项）：主题 -> key_points
	if strings.Contains(system, "出题方向规划师") && strings.Contains(user, "主题：") {
		return `{"key_points": ["Redis 持久化", "Redis 淘汰策略"]}`, nil
	}
	// 方向规划（综合）：基于匹配分析 -> key_points（差距优先）
	if strings.Contains(system, "出题方向规划师") && strings.Contains(user, "匹配度分析") {
		return `{"key_points": ["Go 并发", "MySQL 索引"], "jd_summary": "JD 看重 Go 与数据库", "resume_summary": "匹配 70 分，缺 MySQL"}`, nil
	}
	// 出题（初始「出题官」/ 追问「生成一道追问」——注意面试官 prompt 也含"追问"字样，须用更精确的短语）
	if strings.Contains(system, "出题官") || strings.Contains(system, "生成一道追问") {
		kp := "Redis 持久化"
		if rest, _, ok := strings.Cut(user, "知识点："); ok {
			if kpLine, _, ok := strings.Cut(rest, "\n"); ok {
				kp = kpLine
			}
		}
		b, _ := json.Marshal(map[string]string{"question": "什么是" + kp + "？", "knowledge_point": kp})
		return string(b), nil
	}
	// 面试官判定
	if strings.Contains(system, "评估候选人对一道题的作答") {
		return `{"judge": "advance", "score": 8, "reason": "回答正确", "follow_up_hint": ""}`, nil
	}
	// 复盘
	if strings.Contains(system, "复盘教练") {
		return `{"markdown": "# 复盘\n整体表现良好。", "overall_score": 8, "summary": "表现良好",
			"per_question": [{"question_id": "q1", "question_text": "t", "score": 8, "assessment": "好", "improvement": "继续"}],
			"weak_points": ["Redis 淘汰策略细节不足"], "strong_points": ["基础扎实"], "improvements": ["多练场景题"]}`, nil
	}
	return `{}`, nil
}

func (f *fakeLLM) GenerateJSON(ctx context.Context, system, user string, out any) error {
	text, err := f.Generate(ctx, system, user)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(text), out)
}

func newTestService(t *testing.T) (*Service, *fakeLLM, store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "e2e.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	f := &fakeLLM{}
	return NewService(f, st, nil, ""), f, st
}

// 任务 4.3：专项面试链路端到端（准备→面试→复盘→画像沉淀）
func TestRunSpecialEndToEnd(t *testing.T) {
	svc, _, st := newTestService(t)
	answers := []string{"RDB 和 AOF", "全量重写", "看配置"}
	var n int
	ask := func(q model.Question) (string, error) {
		if n < len(answers) {
			a := answers[n]
			n++
			return a, nil
		}
		return "（回答）", nil
	}

	sess, err := svc.RunSpecial(context.Background(), "Redis", ask)
	if err != nil {
		t.Fatalf("RunSpecial: %v", err)
	}

	// 复盘报告与逐题记录完整
	if sess.Review == "" || !strings.Contains(sess.Review, "复盘") {
		t.Fatalf("review missing: %q", sess.Review)
	}
	if len(sess.Questions) == 0 || len(sess.Answers) == 0 {
		t.Fatalf("questions/answers empty: q=%d a=%d", len(sess.Questions), len(sess.Answers))
	}
	if sess.Status != model.SessionFinished {
		t.Fatalf("status = %s", sess.Status)
	}
	if sess.Direction.Topic != "Redis" || len(sess.Direction.KeyPoints) != 2 {
		t.Fatalf("direction wrong: %+v", sess.Direction)
	}

	// 持久化验证
	got, err := st.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if len(got.Questions) != len(sess.Questions) || got.Review != sess.Review {
		t.Fatalf("persisted session mismatch")
	}

	// 画像沉淀：掌握度 + 薄弱点
	p, err := st.GetProfile()
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if m, ok := p.Mastery["Redis"]; !ok || m.SessionCount != 1 {
		t.Fatalf("mastery not absorbed: %+v", p.Mastery)
	}
	if len(p.WeakPoints) == 0 {
		t.Fatalf("weak points not absorbed: %+v", p.WeakPoints)
	}
}

// 任务 4.4：综合面试链路端到端（JD/简历分析 → 面试 → 复盘 → 画像沉淀）
func TestRunFullEndToEnd(t *testing.T) {
	svc, _, st := newTestService(t)
	ask := func(q model.Question) (string, error) { return "我了解", nil }

	sess, err := svc.RunFull(context.Background(), "简历文本", "JD 文本", ask)
	if err != nil {
		t.Fatalf("RunFull: %v", err)
	}

	// 方向含岗位关键能力点与 JD 摘要
	if sess.Direction.Mode != model.ModeFull {
		t.Fatalf("mode = %s", sess.Direction.Mode)
	}
	if len(sess.Direction.KeyPoints) != 2 || !strings.Contains(sess.Direction.JDSummary, "Go") {
		t.Fatalf("direction missing JD analysis: %+v", sess.Direction)
	}
	if sess.Review == "" {
		t.Fatal("review missing")
	}
	if sess.Direction.Topic == "" {
		// 综合模式无 topic，掌握度应落入「综合面试」主题
		p, err := st.GetProfile()
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := p.Mastery["综合面试"]; !ok {
			t.Fatalf("full-mode mastery not absorbed: %+v", p.Mastery)
		}
	}
}

// 复习闭环：训练沉淀 → 到期复习 → 提交评分 → 间隔更新
func TestReviewLoopEndToEnd(t *testing.T) {
	svc, _, st := newTestService(t)
	ask := func(q model.Question) (string, error) { return "答", nil }
	if _, err := svc.RunSpecial(context.Background(), "Redis", ask); err != nil {
		t.Fatalf("RunSpecial: %v", err)
	}

	p, err := st.GetProfile()
	if err != nil {
		t.Fatal(err)
	}
	// 薄弱点存在且 next_review 在今天（低分触发，间隔 1 天）
	if len(p.WeakPoints) == 0 {
		t.Fatal("no weak points")
	}
	// 提交复习评分（模拟 review 命令）
	today := time.Now()
	for i := range p.WeakPoints {
		profile.ReviewWeakPoint(&p.WeakPoints[i], 4, today) // 低分：仍在队列
	}
	if err := st.SaveProfile(p); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetProfile()
	if got.WeakPoints[0].SR.IntervalDays != 1 || got.WeakPoints[0].SR.Repetitions != 0 {
		t.Fatalf("low-score review should reset: %+v", got.WeakPoints[0].SR)
	}
}
