// Package model 定义 mirror-mian 的领域模型：
// 训练会话、最小单位题、出题方向、画像（薄弱点 + SM-2 + 掌握度）。
// 本包为纯数据结构，不包含任何业务逻辑或 IO。
package model

import (
	"encoding/json"
	"time"
)

// 面试模式
const (
	ModeSpecial = "special" // 专项面试：主题驱动
	ModeFull    = "full"    // 综合面试：简历/JD 驱动
)

// 会话状态
const (
	SessionOngoing  = "ongoing"  // 进行中
	SessionFinished = "finished" // 已结束
)

// 答题方式
const (
	AnswerTypeText  = "text"  // 文本答题：输入框作答
	AnswerTypeVideo = "video" // 视频答题：录制口述，语音转写后判定（视频仅存本地）
)

// 追问判定（面试官 Agent 的结构化输出）
const (
	JudgeContinue = "continue" // 游刃有余，继续追问
	JudgeAdvance  = "advance"  // 知识点已过关，停止追问
	JudgeStop     = "stop"     // 用户无力回答，放弃追问
)

// 每知识点追问轮数硬上限（spec：默认保证 3 轮且只有 3 轮，除非无力回答）
const MaxFollowUpRounds = 3

// 薄弱点评分阈值（spec：高分 ≥8 触发改进标记）
const ImprovedScoreThreshold = 8

// Question 最小单位题：每题只考察一个知识点。
type Question struct {
	ID          string `json:"id"`
	Text        string `json:"text"`           // 题目文本
	KnowledgePt string `json:"knowledge_point"` // 考察的知识点（最小单位）
	Round       int    `json:"round"`           // 追问轮数：0=初始题，1..N=第 N 轮追问
	ParentID    string `json:"parent_id,omitempty"` // 追问来源题 ID（初始题为空）
}

// Answer 用户对一道题的回答与评分。
type Answer struct {
	QuestionID string  `json:"question_id"`
	Text       string  `json:"text"` // 判定用文本（视频答题为转写文字）
	Type       string  `json:"type,omitempty"` // text / video（视频答题标识）
	Score      float64 `json:"score"` // 0-10 评分（复盘阶段生成）
}

// Direction 出题方向（准备阶段产物）。
type Direction struct {
	Mode      string   `json:"mode"`       // special / full
	Topic     string   `json:"topic"`      // 专项面试主题（综合面试为空）
	KeyPoints []string `json:"key_points"` // 知识点方向清单
	JDSummary    string `json:"jd_summary,omitempty"`     // 综合模式：JD 分析摘要
	ResumeSummary string `json:"resume_summary,omitempty"` // 综合模式：简历匹配摘要
}

// TrainingSession 一场训练会话：记录题目/回答/评分/方向全流程。
type TrainingSession struct {
	ID         string     `json:"id"`
	Mode       string     `json:"mode"`
	Status     string     `json:"status"`
	AnswerType string     `json:"answer_type,omitempty"` // text / video（缺省 text）
	Direction  Direction  `json:"direction"`
	Questions  []Question `json:"questions"`
	Answers    []Answer   `json:"answers"`
	Review     string     `json:"review,omitempty"` // 复盘 Markdown 报告
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt time.Time  `json:"finished_at,omitzero"`
}

// SM2State SM-2 间隔重复状态（移植自 TechSpar）。
type SM2State struct {
	IntervalDays int       `json:"interval_days"`
	EaseFactor   float64   `json:"ease_factor"`
	Repetitions  int       `json:"repetitions"`
	NextReview   string    `json:"next_review"` // YYYY-MM-DD
	LastScore    float64   `json:"last_score"`
	History      []SM2Event `json:"history"`
}

// SM2Event 复习/改进/退化事件记录。
type SM2Event struct {
	Date    string  `json:"date"`
	Event   string  `json:"event"` // reviewed / improved / regressed
	Score   float64 `json:"score"`
	Evidence string `json:"evidence,omitempty"`
}

// WeakPoint 长期薄弱点。
type WeakPoint struct {
	ID        string    `json:"id"`
	Point     string    `json:"point"` // 薄弱点文本（规范化后用于精确去重）
	Topic     string    `json:"topic"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	TimesSeen int       `json:"times_seen"`
	Improved  bool      `json:"improved"`
	Archived  bool      `json:"archived"`
	SR        SM2State  `json:"sr"`
}

// Mastery 主题掌握度（EMA 更新）。
type Mastery struct {
	Topic        string    `json:"topic"`
	Score        float64   `json:"score"` // 0-100
	SessionCount int       `json:"session_count"`
	Notes        string    `json:"notes,omitempty"`
	LastAssessed time.Time `json:"last_assessed"`
}

// Profile 用户长期画像（薄弱点 + 掌握度）。
type Profile struct {
	WeakPoints []WeakPoint            `json:"weak_points"`
	Mastery    map[string]*Mastery    `json:"mastery"`
}

// User 个人账户（多用户隔离的账号主体）。
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Salt         string    `json:"-"`
	Name         string    `json:"name"`
	CreatedAt    time.Time `json:"created_at"`
}

// SkillSession 技能会话（持久化，定义在 model 以避免 skill↔store 循环依赖）。
type SkillSession struct {
	ID        string          `json:"id"`
	SkillName string          `json:"skill_name"`
	State     json.RawMessage `json:"state"` // 技能私有状态
	Messages  []string        `json:"messages"`
	Finished  bool            `json:"finished"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// Resume 简历记录（原文件存 resumes/，元数据与解析文本在库）。
type Resume struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"` // 原始文件名
	Ext       string    `json:"ext"`      // pdf / docx / txt / md
	SizeBytes int       `json:"size_bytes"`
	Text      string    `json:"text,omitempty"` // 解析文本（列表接口省略）
	CreatedAt time.Time `json:"created_at"`
}

// ReviewReport 复盘结构化结果（Markdown 报告之外的 JSON 副本）。
type ReviewReport struct {
	OverallScore   float64          `json:"overall_score"` // 平均分 0-10
	Summary        string           `json:"summary"`
	PerQuestion    []PerQuestionReview `json:"per_question"`
	WeakPoints     []string         `json:"weak_points"`
	StrongPoints   []string         `json:"strong_points"`
	Improvements   []string         `json:"improvements"`
}

// PerQuestionReview 逐题复盘。
type PerQuestionReview struct {
	QuestionID   string  `json:"question_id"`
	QuestionText string  `json:"question_text"`
	Score        float64 `json:"score"`
	Assessment   string  `json:"assessment"`
	Improvement  string  `json:"improvement"`
}
