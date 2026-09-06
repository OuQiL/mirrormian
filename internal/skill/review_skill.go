// 复习计划 Skill：画像到期薄弱点逐个复习 → 反馈更新 SM-2。
package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"mirror-mian/internal/llm"
	"mirror-mian/internal/model"
	"mirror-mian/internal/profile"
	"mirror-mian/internal/store"
)

// ReviewState 复习会话状态。
type ReviewState struct {
	Items     []model.WeakPoint `json:"items"` // 待复习薄弱点快照
	Idx       int               `json:"idx"`
	Done      bool              `json:"done"`
}

// ReviewSkill 复习回顾技能。
type ReviewSkill struct {
	llm   llm.Client
	store store.Store
}

// NewReviewSkill 创建复习计划技能。
func NewReviewSkill(lm llm.Client, st store.Store) *ReviewSkill {
	return &ReviewSkill{llm: lm, store: st}
}

// Name 技能名。
func (r *ReviewSkill) Name() string { return "复习计划" }

// Description 描述。
func (r *ReviewSkill) Description() string {
	return "复习计划：按画像薄弱点逐个复习（到期项优先），反馈后更新复习计划"
}

// MatchScore 意图匹配。
func (r *ReviewSkill) MatchScore(_ context.Context, input string) int {
	score := 0
	for _, kw := range []string{"复习", "回顾", "复习计划", "review"} {
		if strings.Contains(input, kw) {
			score += 35
		}
	}
	return score
}

// Start 开始复习：取到期薄弱点。
func (r *ReviewSkill) Start(ctx context.Context, input string) (*Session, *TurnResult, error) {
	p, err := r.store.GetProfile()
	if err != nil {
		return nil, nil, err
	}
	due := profile.DueReviews(p.WeakPoints, time.Now())
	if len(due) == 0 {
		// 无到期项：拿全部薄弱点兜底
		if len(p.WeakPoints) == 0 {
			return nil, nil, fmt.Errorf("画像为空——先完成几场面试积累数据，再开始复习")
		}
		due = p.WeakPoints
	}
	state := &ReviewState{Items: due, Idx: 0}
	sess, err := newSession(r.Name(), state)
	if err != nil {
		return nil, nil, err
	}
	reply := fmt.Sprintf("今日复习 %d 项（按难度优先）：\n\n%s",
		len(due), r.formatItem(&due[0]))
	return sess, &TurnResult{Reply: reply}, saveState(sess, state, r.store)
}

// Turn 复习反馈：评分 → 更新 SM-2 → 下一项。
func (r *ReviewSkill) Turn(ctx context.Context, sess *Session, userInput string) (*TurnResult, error) {
	var state ReviewState
	if err := json.Unmarshal(sess.State, &state); err != nil {
		return nil, err
	}
	if state.Done {
		return nil, fmt.Errorf("会话已结束")
	}
	// 解析自评分（0-10）
	var score float64
	if _, err := fmt.Sscanf(userInput, "%f", &score); err != nil || score < 0 || score > 10 {
		return nil, fmt.Errorf("请输入 0-10 的掌握度自评分（如：7）")
	}
	// 更新薄弱点 SM-2
	p, err := r.store.GetProfile()
	if err != nil {
		return nil, err
	}
	item := state.Items[state.Idx]
	for i := range p.WeakPoints {
		if p.WeakPoints[i].ID == item.ID {
			profile.ReviewWeakPoint(&p.WeakPoints[i], score, time.Now())
			break
		}
	}
	_ = r.store.SaveProfile(p)

	state.Idx++
	if state.Idx >= len(state.Items) {
		state.Done = true
		return &TurnResult{
			Reply:    fmt.Sprintf("🎉 复习完成！%d 项已更新，画像已同步。下次到期项会在「复习」页出现。", len(state.Items)),
			Finished: true,
		}, saveState(sess, &state, r.store)
	}
	reply := fmt.Sprintf("已记录 %s（%.0f 分）✓\n\n下一项：\n%s",
		item.Point, score, r.formatItem(&state.Items[state.Idx]))
	return &TurnResult{Reply: reply}, saveState(sess, &state, r.store)
}

// formatItem 格式化复习项。
func (r *ReviewSkill) formatItem(w *model.WeakPoint) string {
	state := "已改进"
	if !w.Improved {
		state = fmt.Sprintf("复习 %s 次 · 上次 %s", plural(w.TimesSeen), w.SR.NextReview)
	}
	return fmt.Sprintf("【%s】%s（%s）\n请回忆并简述，然后输入 0-10 掌握度自评分：",
		w.Topic, w.Point, state)
}

func plural(n int) string {
	if n <= 1 {
		return "首次"
	}
	return fmt.Sprintf("%d", n)
}
