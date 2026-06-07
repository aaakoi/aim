package rag

import (
	"strings"
	"unicode/utf8"
)

// Chunk 文本块
type Chunk struct {
	ID       string
	DocID    string
	Content  string
	Index    int
	Metadata map[string]string
	Vector   []float64 // 向量嵌入
}

// Chunker 文本分块器
type Chunker struct {
	ChunkSize    int // 块大小（字符数）
	ChunkOverlap int // 重叠大小
}

// NewChunker 创建分块器
func NewChunker(chunkSize, overlap int) *Chunker {
	if chunkSize <= 0 {
		chunkSize = 500 // 默认500字符
	}
	if overlap < 0 {
		overlap = 50 // 默认重叠50字符
	}
	return &Chunker{
		ChunkSize:    chunkSize,
		ChunkOverlap: overlap,
	}
}

// Split 分割文本为块
func (c *Chunker) Split(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	// 按段落分割
	paragraphs := strings.Split(text, "\n\n")

	var chunks []string
	var currentChunk strings.Builder
	currentSize := 0

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		paraLen := utf8.RuneCountInString(para)

		// 如果当前块为空，直接添加段落
		if currentSize == 0 {
			currentChunk.WriteString(para)
			currentSize = paraLen
		} else if currentSize+paraLen <= c.ChunkSize {
			// 如果添加段落不超过大小限制，添加
			currentChunk.WriteString("\n\n")
			currentChunk.WriteString(para)
			currentSize += paraLen + 2
		} else {
			// 当前块已满，保存并开始新块
			chunks = append(chunks, currentChunk.String())

			// 处理重叠：保留最后一个句子的部分
			overlapText := c.getOverlapText(currentChunk.String())
			currentChunk.Reset()
			currentChunk.WriteString(overlapText)
			currentChunk.WriteString("\n\n")
			currentChunk.WriteString(para)
			currentSize = utf8.RuneCountInString(overlapText) + 2 + paraLen
		}
	}

	// 保存最后一个块
	if currentSize > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}

// SplitDocument 分割文档为块
func (c *Chunker) SplitDocument(doc *Document) []*Chunk {
	texts := c.Split(doc.Content)
	chunks := make([]*Chunk, len(texts))

	for i, text := range texts {
		chunks[i] = &Chunk{
			ID:      generateID(),
			DocID:   doc.ID,
			Content: text,
			Index:   i,
			Metadata: map[string]string{
				"doc_name": doc.Name,
			},
		}
	}

	return chunks
}

// getOverlapText 获取重叠文本
func (c *Chunker) getOverlapText(text string) string {
	runes := []rune(text)
	if len(runes) <= c.ChunkOverlap {
		return text
	}

	// 从后往前找句号或换行
	start := len(runes) - c.ChunkOverlap
	for i := start; i < len(runes); i++ {
		if runes[i] == '。' || runes[i] == '\n' || runes[i] == '.' {
			start = i + 1
			break
		}
	}

	if start >= len(runes) {
		start = len(runes) - c.ChunkOverlap
	}

	return string(runes[start:])
}

// SplitBySentence 按句子分割（备用方法）
func (c *Chunker) SplitBySentence(text string) []string {
	// 中文和英文句子分隔符
	separators := []string{"。", "！", "？", ".", "!", "?"}

	sentences := []string{text}
	for _, sep := range separators {
		var newSentences []string
		for _, s := range sentences {
			parts := strings.Split(s, sep)
			for i, p := range parts {
				if i < len(parts)-1 {
					newSentences = append(newSentences, p+sep)
				} else if p != "" {
					newSentences = append(newSentences, p)
				}
			}
		}
		sentences = newSentences
	}

	return sentences
}
