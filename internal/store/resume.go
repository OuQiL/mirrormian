// 简历表读写（元数据 + 解析文本）。
package store

import (
	"database/sql"
	"fmt"
	"time"

	"mirror-mian/internal/model"
)

// SaveResume 保存简历记录。
func (s *sqliteStore) SaveResume(r *model.Resume) error {
	_, err := s.db.Exec(`INSERT INTO resumes (id, user_id, filename, ext, size_bytes, text, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET filename=excluded.filename, ext=excluded.ext,
			size_bytes=excluded.size_bytes, text=excluded.text`,
		r.ID, s.userID, r.Filename, r.Ext, r.SizeBytes, r.Text, r.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("store: save resume: %w", err)
	}
	return nil
}

// GetResume 读取简历（含文本）。
func (s *sqliteStore) GetResume(id string) (*model.Resume, error) {
	row := s.db.QueryRow(`SELECT id, filename, ext, size_bytes, text, created_at FROM resumes WHERE user_id = ? AND id = ?`, s.userID, id)
	var r model.Resume
	var created string
	if err := row.Scan(&r.ID, &r.Filename, &r.Ext, &r.SizeBytes, &r.Text, &created); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("简历 %s 不存在", id)
		}
		return nil, err
	}
	r.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return &r, nil
}

// ListResumes 列出全部简历（含文本，列表接口自行省略）。
func (s *sqliteStore) ListResumes() ([]model.Resume, error) {
	rows, err := s.db.Query(`SELECT id, filename, ext, size_bytes, text, created_at FROM resumes WHERE user_id = ? ORDER BY created_at DESC`, s.userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Resume
	for rows.Next() {
		var r model.Resume
		var created string
		if err := rows.Scan(&r.ID, &r.Filename, &r.Ext, &r.SizeBytes, &r.Text, &created); err != nil {
			return nil, err
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteResume 删除简历记录。
func (s *sqliteStore) DeleteResume(id string) error {
	res, err := s.db.Exec(`DELETE FROM resumes WHERE user_id = ? AND id = ?`, s.userID, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("简历 %s 不存在", id)
	}
	return nil
}
