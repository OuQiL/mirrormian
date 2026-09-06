// 掌握度 EMA 更新（移植自 TechSpar profile-service.ts:175-193）。
//
// weight = max(0.15, 1/(session_count+1)) × coverage
// score = prev×(1-weight) + 本次×weight
package profile

import (
	"time"

	"mirror-mian/internal/model"
)

const (
	minEmaWeight = 0.15
	defaultCoverage = 1.0
)

// UpdateMastery 按 EMA 更新主题掌握度。
// sessionScore 为 0-100 归一分数（由 SessionScore 或调用方归一）。
func UpdateMastery(m *model.Mastery, sessionScore float64, today time.Time) {
	if m == nil {
		return
	}
	prev := m.Score
	weight := 1.0 / float64(m.SessionCount+1)
	if weight < minEmaWeight {
		weight = minEmaWeight
	}
	weight *= defaultCoverage

	m.Score = prev*(1-weight) + sessionScore*weight
	m.SessionCount++
	m.LastAssessed = today
}

// SessionScore 由逐题评分（0-10）计算一场会话的主题得分（0-100）。
func SessionScore(answers []model.Answer) float64 {
	if len(answers) == 0 {
		return 0
	}
	var sum float64
	for _, a := range answers {
		sum += a.Score
	}
	return sum / float64(len(answers)) * 10
}
