// SQLite 持久化：会话表（题目/回答/评分/方向 JSON）、画像表（薄弱点 + SM-2 + 掌握度）、
// 用户表与各数据表的按用户隔离。
//
// 数据隔离模型：同一 SQLite 库内，每个 Store 实例绑定一个 userID，
// 所有读写都按该 userID 过滤（ForUser 派生）。这样各用户的会话/画像/知识块/简历/技能互不可见。
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"mirror-mian/internal/model"
)

// Store 持久化接口：基础能力层对编排层暴露的存储能力。
// 所有数据操作都限定在当前实例绑定的 userID（见 ForUser）。
type Store interface {
	// 按用户派生一个只读该用户数据的 Store（共享同一底层库）。
	ForUser(userID string) Store

	// 会话
	SaveSession(s *model.TrainingSession) error
	GetSession(id string) (*model.TrainingSession, error)
	ListSessions() ([]model.TrainingSession, error)
	SaveProfile(p *model.Profile) error
	GetProfile() (*model.Profile, error)
	// 知识库块
	SaveTopicChunks(topic string, chunks []KnowledgeChunk) error
	ListTopicChunks(topic string) ([]KnowledgeChunk, error)
	ListAllTopics() (map[string]int, error)
	DeleteTopic(topic string) error
	// 领域一致性：删除/重命名某主题的全部数据（块 + 画像）
	DeleteTopicData(topic string) error
	RenameTopic(oldName, newName string) error
	// 简历
	SaveResume(r *model.Resume) error
	GetResume(id string) (*model.Resume, error)
	ListResumes() ([]model.Resume, error)
	DeleteResume(id string) error
	// 技能会话
	SaveSkillSession(s *model.SkillSession) error
	GetSkillSession(id string) (*model.SkillSession, error)
	ListSkillSessions() ([]model.SkillSession, error)

	// 账户（全局，不随 userID 过滤）
	CreateUser(u *model.User) error
	GetUserByEmail(email string) (*model.User, error)
	GetUserByID(id string) (*model.User, error)
	VerifyPassword(u *model.User, password string) bool
	CountUsers() (int, error)
	// 将旧版（无归属）数据全部划归某用户，用于首个注册者接管历史数据。
	AdoptLegacyData(userID string) error

	Close() error
}

type sqliteStore struct {
	db     *sql.DB
	userID string
}

// Open 打开（或创建）SQLite 数据库并初始化/升级表结构。
// 返回的 Store 绑定到空 userID（legacy/无归属），仅供账户管理与派生按用户 Store 使用。
func Open(path string) (Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("store: create dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	s := &sqliteStore{db: db, userID: ""}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: migrate: %w", err)
	}
	return s, nil
}

// ForUser 返回绑定到指定用户的 Store（共享底层库，读写仅限该用户）。
func (s *sqliteStore) ForUser(userID string) Store {
	return &sqliteStore{db: s.db, userID: userID}
}

// migrate 建表并升级旧库：为数据表补 user_id 列；
// 对需要「用户+内容」复合唯一键的表（mastery/weak_points/knowledge_chunks）重建。
func (s *sqliteStore) migrate() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		salt TEXT NOT NULL,
		name TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL
	)`); err != nil {
		return err
	}

	// 数据表（新建即含 user_id）
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			mode TEXT NOT NULL,
			status TEXT NOT NULL,
			answer_type TEXT NOT NULL DEFAULT 'text',
			direction TEXT NOT NULL,   -- JSON: model.Direction
			questions TEXT NOT NULL,   -- JSON: []model.Question
			answers TEXT NOT NULL,     -- JSON: []model.Answer
			review TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			finished_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS weak_points (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			point TEXT NOT NULL,
			topic TEXT NOT NULL,
			first_seen TEXT NOT NULL,
			last_seen TEXT NOT NULL,
			times_seen INTEGER NOT NULL DEFAULT 1,
			improved INTEGER NOT NULL DEFAULT 0,
			archived INTEGER NOT NULL DEFAULT 0,
			sr TEXT NOT NULL,          -- JSON: model.SM2State
			UNIQUE(user_id, point, topic)
		)`,
		`CREATE TABLE IF NOT EXISTS mastery (
			user_id TEXT NOT NULL DEFAULT '',
			topic TEXT NOT NULL,
			score REAL NOT NULL,
			session_count INTEGER NOT NULL DEFAULT 0,
			notes TEXT NOT NULL DEFAULT '',
			last_assessed TEXT NOT NULL,
			PRIMARY KEY (user_id, topic)
		)`,
		`CREATE TABLE IF NOT EXISTS knowledge_chunks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL DEFAULT '',
			topic TEXT NOT NULL,
			content TEXT NOT NULL,
			embedding BLOB NOT NULL,   -- Float32LE
			chunk_index INTEGER NOT NULL,
			UNIQUE(user_id, topic, chunk_index)
		)`,
		`CREATE TABLE IF NOT EXISTS resumes (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			filename TEXT NOT NULL,
			ext TEXT NOT NULL,
			size_bytes INTEGER NOT NULL,
			text TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS skill_sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			skill_name TEXT NOT NULL,
			state TEXT NOT NULL,      -- JSON
			messages TEXT NOT NULL,   -- JSON []string
			finished INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}

	// 旧库升级：为可直接加列的表补 user_id
	for _, t := range []string{"sessions", "resumes", "skill_sessions"} {
		ok, err := hasColumn(tx, t, "user_id")
		if err != nil {
			return err
		}
		if !ok {
			if _, err := tx.Exec(`ALTER TABLE ` + t + ` ADD COLUMN user_id TEXT NOT NULL DEFAULT ''`); err != nil {
				return err
			}
		}
	}

	// 旧库升级：sessions 补 answer_type（文本答题为历史默认）
	ok, err := hasColumn(tx, "sessions", "answer_type")
	if err != nil {
		return err
	}
	if !ok {
		if _, err := tx.Exec(`ALTER TABLE sessions ADD COLUMN answer_type TEXT NOT NULL DEFAULT 'text'`); err != nil {
			return err
		}
	}

	// 需要「用户+内容」复合约束的表：缺失 user_id 时重建
	for _, t := range []string{"mastery", "weak_points", "knowledge_chunks"} {
		ok, err := hasColumn(tx, t, "user_id")
		if err != nil {
			return err
		}
		if !ok {
			if err := rebuildTable(tx, t); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// hasColumn 判断表是否已有某列。
func hasColumn(tx *sql.Tx, table, col string) (bool, error) {
	rows, err := tx.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == col {
			return true, nil
		}
	}
	return false, rows.Err()
}

// rebuildTable 以带 user_id 的新结构重建指定表并迁移旧数据（旧数据划归空用户，待首个用户接管）。
func rebuildTable(tx *sql.Tx, table string) error {
	schemas := map[string]string{
		"mastery": `CREATE TABLE IF NOT EXISTS mastery_new (
			user_id TEXT NOT NULL DEFAULT '',
			topic TEXT NOT NULL,
			score REAL NOT NULL,
			session_count INTEGER NOT NULL DEFAULT 0,
			notes TEXT NOT NULL DEFAULT '',
			last_assessed TEXT NOT NULL,
			PRIMARY KEY (user_id, topic)
		)`,
		"weak_points": `CREATE TABLE IF NOT EXISTS weak_points_new (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			point TEXT NOT NULL,
			topic TEXT NOT NULL,
			first_seen TEXT NOT NULL,
			last_seen TEXT NOT NULL,
			times_seen INTEGER NOT NULL DEFAULT 1,
			improved INTEGER NOT NULL DEFAULT 0,
			archived INTEGER NOT NULL DEFAULT 0,
			sr TEXT NOT NULL,
			UNIQUE(user_id, point, topic)
		)`,
		"knowledge_chunks": `CREATE TABLE IF NOT EXISTS knowledge_chunks_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL DEFAULT '',
			topic TEXT NOT NULL,
			content TEXT NOT NULL,
			embedding BLOB NOT NULL,
			chunk_index INTEGER NOT NULL,
			UNIQUE(user_id, topic, chunk_index)
		)`,
	}
	inserts := map[string]string{
		"mastery":          `INSERT INTO mastery_new (user_id, topic, score, session_count, notes, last_assessed) SELECT '', topic, score, session_count, notes, last_assessed FROM mastery`,
		"weak_points":      `INSERT INTO weak_points_new (id, user_id, point, topic, first_seen, last_seen, times_seen, improved, archived, sr) SELECT id, '', point, topic, first_seen, last_seen, times_seen, improved, archived, sr FROM weak_points`,
		"knowledge_chunks": `INSERT INTO knowledge_chunks_new (user_id, topic, content, embedding, chunk_index) SELECT '', topic, content, embedding, chunk_index FROM knowledge_chunks`,
	}
	if _, err := tx.Exec(schemas[table]); err != nil {
		return err
	}
	if _, err := tx.Exec(inserts[table]); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE ` + table); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE ` + table + `_new RENAME TO ` + table); err != nil {
		return err
	}
	return nil
}

// --- 会话 ---

func (s *sqliteStore) SaveSession(sess *model.TrainingSession) error {
	dir, err := json.Marshal(sess.Direction)
	if err != nil {
		return fmt.Errorf("store: marshal direction: %w", err)
	}
	qs, err := json.Marshal(sess.Questions)
	if err != nil {
		return fmt.Errorf("store: marshal questions: %w", err)
	}
	as, err := json.Marshal(sess.Answers)
	if err != nil {
		return fmt.Errorf("store: marshal answers: %w", err)
	}
	_, err = s.db.Exec(`INSERT INTO sessions (id, user_id, mode, status, answer_type, direction, questions, answers, review, created_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status=excluded.status, answer_type=excluded.answer_type, direction=excluded.direction,
			questions=excluded.questions, answers=excluded.answers, review=excluded.review,
			finished_at=excluded.finished_at`,
		sess.ID, s.userID, sess.Mode, sess.Status, sess.AnswerType, string(dir), string(qs), string(as),
		sess.Review, sess.CreatedAt.Format(time.RFC3339), sess.FinishedAt.Format(time.RFC3339))
	return err
}

// scanner 兼容 sql.Row 与 sql.Rows 的单行扫描。
type scanner interface {
	Scan(dest ...any) error
}

// scanSession 将一条 sessions 行解析为 model.TrainingSession。
func scanSession(row scanner) (*model.TrainingSession, error) {
	var (
		sess                          model.TrainingSession
		dir, qs, as, created, finished string
	)
	if err := row.Scan(&sess.ID, &sess.Mode, &sess.Status, &sess.AnswerType, &dir, &qs, &as,
		&sess.Review, &created, &finished); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(dir), &sess.Direction); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(qs), &sess.Questions); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(as), &sess.Answers); err != nil {
		return nil, err
	}
	sess.CreatedAt, _ = time.Parse(time.RFC3339, created)
	sess.FinishedAt, _ = time.Parse(time.RFC3339, finished)
	return &sess, nil
}

func (s *sqliteStore) GetSession(id string) (*model.TrainingSession, error) {
	row := s.db.QueryRow(`SELECT id, mode, status, answer_type, direction, questions, answers, review, created_at, finished_at
		FROM sessions WHERE user_id = ? AND id = ?`, s.userID, id)
	sess, err := scanSession(row)
	if err != nil {
		return nil, fmt.Errorf("store: get session %s: %w", id, err)
	}
	return sess, nil
}

// ListSessions 返回当前用户的全部面试会话，按创建时间倒序。
func (s *sqliteStore) ListSessions() ([]model.TrainingSession, error) {
	rows, err := s.db.Query(`SELECT id, mode, status, answer_type, direction, questions, answers, review, created_at, finished_at
		FROM sessions WHERE user_id = ? ORDER BY created_at DESC`, s.userID)
	if err != nil {
		return nil, fmt.Errorf("store: list sessions: %w", err)
	}
	defer rows.Close()
	var out []model.TrainingSession
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("store: list sessions: %w", err)
		}
		out = append(out, *sess)
	}
	return out, rows.Err()
}

// --- 画像 ---

func (s *sqliteStore) SaveProfile(p *model.Profile) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 薄弱点全量重写（量小；仅当前用户）
	if _, err := tx.Exec(`DELETE FROM weak_points WHERE user_id = ?`, s.userID); err != nil {
		return err
	}
	for _, w := range p.WeakPoints {
		sr, err := json.Marshal(w.SR)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO weak_points (id, user_id, point, topic, first_seen, last_seen, times_seen, improved, archived, sr)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			w.ID, s.userID, w.Point, w.Topic,
			w.FirstSeen.Format(time.RFC3339), w.LastSeen.Format(time.RFC3339),
			w.TimesSeen, boolInt(w.Improved), boolInt(w.Archived), string(sr)); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`DELETE FROM mastery WHERE user_id = ?`, s.userID); err != nil {
		return err
	}
	for topic, m := range p.Mastery {
		if _, err := tx.Exec(`INSERT INTO mastery (user_id, topic, score, session_count, notes, last_assessed)
			VALUES (?, ?, ?, ?, ?, ?)`,
			s.userID, topic, m.Score, m.SessionCount, m.Notes, m.LastAssessed.Format(time.RFC3339)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *sqliteStore) GetProfile() (*model.Profile, error) {
	p := &model.Profile{WeakPoints: []model.WeakPoint{}, Mastery: map[string]*model.Mastery{}}

	rows, err := s.db.Query(`SELECT id, point, topic, first_seen, last_seen, times_seen, improved, archived, sr FROM weak_points WHERE user_id = ?`, s.userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			w                  model.WeakPoint
			first, last, sr    string
			improved, archived int
		)
		if err := rows.Scan(&w.ID, &w.Point, &w.Topic, &first, &last,
			&w.TimesSeen, &improved, &archived, &sr); err != nil {
			return nil, err
		}
		w.FirstSeen, _ = time.Parse(time.RFC3339, first)
		w.LastSeen, _ = time.Parse(time.RFC3339, last)
		w.Improved = improved != 0
		w.Archived = archived != 0
		if err := json.Unmarshal([]byte(sr), &w.SR); err != nil {
			return nil, err
		}
		p.WeakPoints = append(p.WeakPoints, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows2, err := s.db.Query(`SELECT topic, score, session_count, notes, last_assessed FROM mastery WHERE user_id = ?`, s.userID)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var (
			m        model.Mastery
			assessed string
		)
		if err := rows2.Scan(&m.Topic, &m.Score, &m.SessionCount, &m.Notes, &assessed); err != nil {
			return nil, err
		}
		m.LastAssessed, _ = time.Parse(time.RFC3339, assessed)
		p.Mastery[m.Topic] = &m
	}
	if err := rows2.Err(); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *sqliteStore) Close() error { return s.db.Close() }

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}