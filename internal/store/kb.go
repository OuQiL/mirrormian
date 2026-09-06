// 知识库块存储：knowledge_chunks 表的读写。
package store

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
)

// KnowledgeChunk 一条知识块。
type KnowledgeChunk struct {
	ID      int64
	Topic   string
	Content string
	Embedding []float32
	Index   int
}

// SaveTopicChunks 保存某主题的全部块（先删除该主题旧块，再插入——重建语义）。
func (s *sqliteStore) SaveTopicChunks(topic string, chunks []KnowledgeChunk) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM knowledge_chunks WHERE user_id = ? AND topic = ?`, s.userID, topic); err != nil {
		return err
	}
	for _, c := range chunks {
		blob := encodeFloat32s(c.Embedding)
		if _, err := tx.Exec(
			`INSERT INTO knowledge_chunks (user_id, topic, content, embedding, chunk_index) VALUES (?, ?, ?, ?, ?)`,
			s.userID, topic, c.Content, blob, c.Index); err != nil {
			return fmt.Errorf("store: insert chunk %d: %w", c.Index, err)
		}
	}
	return tx.Commit()
}

// ListTopicChunks 返回某主题全部块（含向量）。
func (s *sqliteStore) ListTopicChunks(topic string) ([]KnowledgeChunk, error) {
	rows, err := s.db.Query(
		`SELECT id, topic, content, embedding, chunk_index FROM knowledge_chunks WHERE user_id = ? AND topic = ? ORDER BY chunk_index`, s.userID, topic)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChunks(rows)
}

// ListAllTopics 返回全部主题与块数。
func (s *sqliteStore) ListAllTopics() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT topic, COUNT(*) FROM knowledge_chunks WHERE user_id = ? GROUP BY topic`, s.userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var topic string
		var n int
		if err := rows.Scan(&topic, &n); err != nil {
			return nil, err
		}
		out[topic] = n
	}
	return out, rows.Err()
}

// DeleteTopic 删除主题全部块。
func (s *sqliteStore) DeleteTopic(topic string) error {
	_, err := s.db.Exec(`DELETE FROM knowledge_chunks WHERE user_id = ? AND topic = ?`, s.userID, topic)
	return err
}

// DeleteTopicData 删除主题全部数据：知识库块 + 掌握度 + 薄弱点（领域删除）。
func (s *sqliteStore) DeleteTopicData(topic string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`DELETE FROM knowledge_chunks WHERE user_id = ? AND topic = ?`,
		`DELETE FROM mastery WHERE user_id = ? AND topic = ?`,
		`DELETE FROM weak_points WHERE user_id = ? AND topic = ?`,
	} {
		if _, err := tx.Exec(q, s.userID, topic); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// RenameTopic 更新主题名：知识库块 + 掌握度 + 薄弱点（领域重命名，事务内一致）。
func (s *sqliteStore) RenameTopic(oldName, newName string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`UPDATE knowledge_chunks SET topic = ? WHERE user_id = ? AND topic = ?`,
		`UPDATE mastery SET topic = ? WHERE user_id = ? AND topic = ?`,
		`UPDATE weak_points SET topic = ? WHERE user_id = ? AND topic = ?`,
	} {
		if _, err := tx.Exec(q, newName, s.userID, oldName); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanChunks(rows *sql.Rows) ([]KnowledgeChunk, error) {
	var out []KnowledgeChunk
	for rows.Next() {
		var c KnowledgeChunk
		var blob []byte
		if err := rows.Scan(&c.ID, &c.Topic, &c.Content, &blob, &c.Index); err != nil {
			return nil, err
		}
		c.Embedding = decodeFloat32s(blob)
		out = append(out, c)
	}
	return out, rows.Err()
}

// encodeFloat32s Float32 → Float32LE BLOB。
func encodeFloat32s(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// decodeFloat32s Float32LE BLOB → []float32。
func decodeFloat32s(b []byte) []float32 {
	n := len(b) / 4
	out := make([]float32, n)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out
}
