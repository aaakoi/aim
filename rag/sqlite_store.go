package rag

import (
	"database/sql"
	"encoding/gob"
	"sort"
	"sync"

	_ "modernc.org/sqlite"
)

// SQLiteVectorStore SQLite向量存储（持久化）
type SQLiteVectorStore struct {
	db     *sql.DB
	userID string
	mu     sync.RWMutex
}

// NewSQLiteVectorStore 创建SQLite向量存储
func NewSQLiteVectorStore(dbPath, userID string) (*SQLiteVectorStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	store := &SQLiteVectorStore{
		db:     db,
		userID: userID,
	}

	// 加载已有数据
	return store, nil
}

// Add 添加向量块    把一个文档块（chunk）存到 SQLite 数据库里
func (s *SQLiteVectorStore) Add(chunk *Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 序列化向量
	vectorData, err := serializeVector(chunk.Vector)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		INSERT OR REPLACE INTO knowledge_base (id, user_id, doc_id, doc_name, content, vector, chunk_index)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, chunk.ID, s.userID, chunk.DocID, chunk.Metadata["doc_name"], chunk.Content, vectorData, chunk.Index)

	return err
}

// AddBatch 批量添加向量块
func (s *SQLiteVectorStore) AddBatch(chunks []*Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, chunk := range chunks {
		vectorData, err := serializeVector(chunk.Vector)
		if err != nil {
			continue
		}

		_, err = tx.Exec(`
			INSERT OR REPLACE INTO knowledge_base (id, user_id, doc_id, doc_name, content, vector, chunk_index)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, chunk.ID, s.userID, chunk.DocID, chunk.Metadata["doc_name"], chunk.Content, vectorData, chunk.Index)
		if err != nil {
			continue
		}
	}

	return tx.Commit()
}

// Search 搜索相似向量
func (s *SQLiteVectorStore) Search(queryVector []float64, topK int) ([]*SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 加载所有块
	rows, err := s.db.Query(`
		SELECT id, doc_id, doc_name, content, vector, chunk_index
		FROM knowledge_base WHERE user_id = ?
	`, s.userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type scoreChunk struct {
		chunk *Chunk
		score float64
	}

	var scores []scoreChunk

	for rows.Next() {
		var id, docID, docName, content string
		var vectorData []byte
		var chunkIndex int

		err := rows.Scan(&id, &docID, &docName, &content, &vectorData, &chunkIndex)
		if err != nil {
			continue
		}

		vector, err := deserializeVector(vectorData)
		if err != nil {
			continue
		}

		if len(vector) > 0 {
			score := CosineSimilarity(queryVector, vector)
			chunk := &Chunk{
				ID:      id,
				DocID:   docID,
				Content: content,
				Index:   chunkIndex,
				Vector:  vector,
				Metadata: map[string]string{
					"doc_name": docName,
				},
			}
			scores = append(scores, scoreChunk{chunk: chunk, score: score})
		}
	}

	// 按相似度排序
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// 返回topK结果
	results := make([]*SearchResult, 0, topK)
	for i := 0; i < topK && i < len(scores); i++ {
		results = append(results, &SearchResult{
			Chunk: scores[i].chunk,
			Score: scores[i].score,
		})
	}

	return results, nil
}

// Delete 删除文档的所有块
func (s *SQLiteVectorStore) Delete(docID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM knowledge_base WHERE user_id = ? AND doc_id = ?`, s.userID, docID)
	return err
}

// List 列出所有块
func (s *SQLiteVectorStore) List() []*Chunk {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, doc_id, doc_name, content, vector, chunk_index
		FROM knowledge_base WHERE user_id = ?
	`, s.userID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var chunks []*Chunk

	for rows.Next() {
		var id, docID, docName, content string
		var vectorData []byte
		var chunkIndex int

		err := rows.Scan(&id, &docID, &docName, &content, &vectorData, &chunkIndex)
		if err != nil {
			continue
		}

		vector, _ := deserializeVector(vectorData)

		chunks = append(chunks, &Chunk{
			ID:      id,
			DocID:   docID,
			Content: content,
			Index:   chunkIndex,
			Vector:  vector,
			Metadata: map[string]string{
				"doc_name": docName,
			},
		})
	}

	return chunks
}

// Clear 清空知识库
func (s *SQLiteVectorStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.db.Exec(`DELETE FROM knowledge_base WHERE user_id = ?`, s.userID)
}

// Count 返回存储的块数量
func (s *SQLiteVectorStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM knowledge_base WHERE user_id = ?`, s.userID).Scan(&count)
	return count
}

// serializeVector 序列化向量
func serializeVector(v []float64) ([]byte, error) {
	var buf []byte
	// 简单二进制编码
	for _, f := range v {
		buf = append(buf, float64ToBytes(f)...)
	}
	return buf, nil
}

// deserializeVector 反序列化向量
func deserializeVector(data []byte) ([]float64, error) {
	if len(data)%8 != 0 {
		return nil, nil
	}

	vector := make([]float64, len(data)/8)
	for i := 0; i < len(vector); i++ {
		vector[i] = bytesToFloat64(data[i*8 : (i+1)*8])
	}
	return vector, nil
}

func float64ToBytes(f float64) []byte {
	buf := make([]byte, 8)
	u := uint64(f)
	for i := 0; i < 8; i++ {
		buf[i] = byte(u >> (i * 8))
	}
	return buf
}

func bytesToFloat64(buf []byte) float64 {
	u := uint64(0)
	for i := 0; i < 8; i++ {
		u |= uint64(buf[i]) << (i * 8)
	}
	return float64(u)
}

// SQLiteKnowledgeStore 知识库存储管理器
type SQLiteKnowledgeStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteKnowledgeStore 创建知识库存储管理器
func NewSQLiteKnowledgeStore(dbPath string) (*SQLiteKnowledgeStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	return &SQLiteKnowledgeStore{db: db}, nil
}

// GetStore 获取用户的向量存储
func (s *SQLiteKnowledgeStore) GetStore(userID string) VectorStore {
	return &SQLiteVectorStore{
		db:     s.db,
		userID: userID,
	}
}

// ListAllKnowledgeBases 列出所有知识库
func (s *SQLiteKnowledgeStore) ListAllKnowledgeBases() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT user_id, COUNT(*) as cnt
		FROM knowledge_base
		GROUP BY user_id
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var userID string
		var count int
		rows.Scan(&userID, &count)
		result[userID] = count
	}

	return result
}

// gob序列化（备用）
func init() {
	gob.Register([]float64{})
}
