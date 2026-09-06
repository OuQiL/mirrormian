// 技能会话表读写。
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"mirror-mian/internal/model"
)

// SaveSkillSession 保存技能会话（upsert）。
func (s *sqliteStore) SaveSkillSession(sess *model.SkillSession) error {
	msgs, err := json.Marshal(sess.Messages)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO skill_sessions (id, user_id, skill_name, state, messages, finished, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET state=excluded.state, messages=excluded.messages,
			finished=excluded.finished, updated_at=excluded.updated_at`,
		sess.ID, s.userID, sess.SkillName, string(sess.State), string(msgs),
		boolInt(sess.Finished),
		sess.CreatedAt.Format(time.RFC3339), sess.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("store: save skill session: %w", err)
	}
	return nil
}

// GetSkillSession 读取技能会话。
func (s *sqliteStore) GetSkillSession(id string) (*model.SkillSession, error) {
	row := s.db.QueryRow(`SELECT id, skill_name, state, messages, finished, created_at, updated_at
		FROM skill_sessions WHERE user_id = ? AND id = ?`, s.userID, id)
	var (
		sess                    model.SkillSession
		state, msgs, created, updated string
	)
	if err := row.Scan(&sess.ID, &sess.SkillName, &state, &msgs, &sess.Finished,
		&created, &updated); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("技能会话 %s 不存在", id)
		}
		return nil, err
	}
	sess.State = json.RawMessage(state)
	if err := json.Unmarshal([]byte(msgs), &sess.Messages); err != nil {
		return nil, err
	}
	sess.CreatedAt, _ = time.Parse(time.RFC3339, created)
	sess.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return &sess, nil
}

// ListSkillSessions 列出当前用户的全部技能会话（按最近更新倒序）。
func (s *sqliteStore) ListSkillSessions() ([]model.SkillSession, error) {
	rows, err := s.db.Query(`SELECT id, skill_name, state, messages, finished, created_at, updated_at
		FROM skill_sessions WHERE user_id = ? ORDER BY updated_at DESC`, s.userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.SkillSession{}
	for rows.Next() {
		var (
			sess                  model.SkillSession
			state, msgs, created, updated string
		)
		if err := rows.Scan(&sess.ID, &sess.SkillName, &state, &msgs, &sess.Finished,
			&created, &updated); err != nil {
			return nil, err
		}
		sess.State = json.RawMessage(state)
		if err := json.Unmarshal([]byte(msgs), &sess.Messages); err != nil {
			return nil, err
		}
		sess.CreatedAt, _ = time.Parse(time.RFC3339, created)
		sess.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
		out = append(out, sess)
	}
	return out, rows.Err()
}
