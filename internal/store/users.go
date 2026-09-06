// 个人账户：用户注册/查询/密码哈希与旧数据接管。
// 本文件使用底层库（不随 userID 过滤），账户是全局的。
package store

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"mirror-mian/internal/model"
)

// CreateUser 创建账户：生成 id/盐，写入密码哈希。email 需小写唯一。
func (s *sqliteStore) CreateUser(u *model.User) error {
	if u.Email == "" {
		return fmt.Errorf("邮箱不能为空")
	}
	u.ID = uuid.NewString()
	salt, err := randomHex(16)
	if err != nil {
		return err
	}
	u.Salt = salt
	u.PasswordHash = hashPassword(strings.TrimSpace(u.PasswordHash), salt)
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	_, err = s.db.Exec(`INSERT INTO users (id, email, password_hash, salt, name, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, strings.ToLower(strings.TrimSpace(u.Email)), u.PasswordHash, u.Salt, u.Name, u.CreatedAt.Format(time.RFC3339))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "conflict") {
			return fmt.Errorf("该邮箱已被注册")
		}
		return fmt.Errorf("store: create user: %w", err)
	}
	return nil
}

// GetUserByEmail 按邮箱查询账户（用于登录校验）。
func (s *sqliteStore) GetUserByEmail(email string) (*model.User, error) {
	row := s.db.QueryRow(`SELECT id, email, password_hash, salt, name, created_at FROM users WHERE email = ?`,
		strings.ToLower(strings.TrimSpace(email)))
	u, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("邮箱或密码错误")
	}
	return u, nil
}

// GetUserByID 按 ID 查询账户。
func (s *sqliteStore) GetUserByID(id string) (*model.User, error) {
	row := s.db.QueryRow(`SELECT id, email, password_hash, salt, name, created_at FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("用户不存在")
	}
	return u, nil
}

// CountUsers 用户总数（判断是否首个用户以接管旧数据）。
func (s *sqliteStore) CountUsers() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// VerifyPassword 校验明文密码是否匹配（供 Web 登录用）。
func (s *sqliteStore) VerifyPassword(u *model.User, password string) bool {
	return u.PasswordHash != "" && hmac.Equal(
		[]byte(hashPassword(strings.TrimSpace(password), u.Salt)),
		[]byte(u.PasswordHash))
}

// AdoptLegacyData 将旧版（无归属，user_id=''）的数据库记录全部划归某用户，
// 供首个注册者接管升级前的历史数据。
func (s *sqliteStore) AdoptLegacyData(userID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, t := range []string{"sessions", "weak_points", "mastery", "knowledge_chunks", "resumes", "skill_sessions"} {
		if _, err := tx.Exec(`UPDATE `+t+` SET user_id = ? WHERE user_id = ?`, userID, ""); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanUser(row *sql.Row) (*model.User, error) {
	var u model.User
	var created string
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Salt, &u.Name, &created); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("not found")
		}
		return nil, err
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return &u, nil
}

// hashPassword HMAC-SHA256(password, salt)，输出十六进制。
func hashPassword(password, salt string) string {
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(password))
	return hex.EncodeToString(mac.Sum(nil))
}

// randomHex 生成 n 字节加密随机数的十六进制。
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("store: random: %w", err)
	}
	return hex.EncodeToString(b), nil
}