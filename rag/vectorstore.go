package rag

import (
	"sort"
	"sync"
)

// VectorStore 向量存储接口
type VectorStore interface {
	Add(chunk *Chunk) error
	AddBatch(chunks []*Chunk) error
	Search(queryVector []float64, topK int) ([]*SearchResult, error)
	Delete(docID string) error
	List() []*Chunk
	Clear()
}

// SearchResult 搜索结果
type SearchResult struct {
	Chunk     *Chunk
	Score     float64
}

// MemoryVectorStore 内存向量存储（开发测试用）
type MemoryVectorStore struct {
	chunks []*Chunk
	mu     sync.RWMutex
}

// NewMemoryVectorStore 创建内存向量存储
func NewMemoryVectorStore() *MemoryVectorStore {
	return &MemoryVectorStore{
		chunks: make([]*Chunk, 0),
	}
}

// Add 添加向量块
func (s *MemoryVectorStore) Add(chunk *Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chunks = append(s.chunks, chunk)
	return nil
}

// AddBatch 批量添加向量块
func (s *MemoryVectorStore) AddBatch(chunks []*Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chunks = append(s.chunks, chunks...)
	return nil
}

// Search 搜索相似向量
func (s *MemoryVectorStore) Search(queryVector []float64, topK int) ([]*SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 计算所有块的相似度
	type scoreChunk struct {
		chunk *Chunk
		score float64
	}

	scores := make([]scoreChunk, 0, len(s.chunks))
	for _, chunk := range s.chunks {
		if len(chunk.Vector) > 0 {
			score := CosineSimilarity(queryVector, chunk.Vector)
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
func (s *MemoryVectorStore) Delete(docID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newChunks := make([]*Chunk, 0)
	for _, chunk := range s.chunks {
		if chunk.DocID != docID {
			newChunks = append(newChunks, chunk)
		}
	}
	s.chunks = newChunks
	return nil
}

// List 列出所有块
func (s *MemoryVectorStore) List() []*Chunk {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.chunks
}

// GetByDocID 按文档ID获取块
func (s *MemoryVectorStore) GetByDocID(docID string) []*Chunk {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var chunks []*Chunk
	for _, chunk := range s.chunks {
		if chunk.DocID == docID {
			chunks = append(chunks, chunk)
		}
	}
	return chunks
}

// Count 返回存储的块数量
func (s *MemoryVectorStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.chunks)
}

// Clear 清空所有数据
func (s *MemoryVectorStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chunks = make([]*Chunk, 0)
}
