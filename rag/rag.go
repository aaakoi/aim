package rag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RAGBot RAG机器人
type RAGBot struct {
	EmbeddingService *EmbeddingService
	VectorStore      VectorStore
	Chunker          *Chunker
	Parser           *DocumentParser
	ChatAPIKey       string
	ChatAPIURL       string
	HTTPClient       *http.Client
}

// RAGConfig RAG配置
type RAGConfig struct {
	EmbeddingAPIKey string
	ChatAPIKey      string
	ChatAPIURL      string
	ChunkSize       int
	ChunkOverlap    int
}

// NewRAGBot 创建RAG机器人
func NewRAGBot(config RAGConfig) *RAGBot {
	return &RAGBot{
		EmbeddingService: NewEmbeddingService(config.EmbeddingAPIKey),
		VectorStore:      NewMemoryVectorStore(),
		Chunker:          NewChunker(config.ChunkSize, config.ChunkOverlap),
		Parser:           NewDocumentParser(),
		ChatAPIKey:       config.ChatAPIKey,
		ChatAPIURL:       config.ChatAPIURL,
		HTTPClient:       &http.Client{Timeout: 60 * time.Second},
	}
}

// IngestDocument 导入文档到知识库
func (b *RAGBot) IngestDocument(filePath string) error {
	// 1. 解析文档
	doc, err := b.Parser.ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("解析文档失败: %v", err)
	}

	return b.IngestContent(doc.Name, doc.Content)
}

// IngestContent 导入文本内容到知识库
func (b *RAGBot) IngestContent(name, content string) error {
	doc, err := b.Parser.ParseContent(name, content)
	if err != nil {
		return err
	}

	// 2. 分块
	chunks := b.Chunker.SplitDocument(doc)
	if len(chunks) == 0 {
		return fmt.Errorf("文档无有效内容")
	}

	// 3. 向量化
	for _, chunk := range chunks {
		vector, err := b.EmbeddingService.GetEmbedding(chunk.Content)
		if err != nil {
			return fmt.Errorf("向量化失败: %v", err)
		}
		chunk.Vector = vector
	}

	// 4. 存储
	err = b.VectorStore.AddBatch(chunks)
	if err != nil {
		return fmt.Errorf("存储失败: %v", err)
	}

	return nil
}

// Query 基于知识库回答问题
func (b *RAGBot) Query(question string) (string, error) {
	// 1. 问题向量化
	queryVector, err := b.EmbeddingService.GetEmbedding(question)
	if err != nil {
		return "", fmt.Errorf("问题向量化失败: %v", err)
	}

	// 2. 检索相关文档
	results, err := b.VectorStore.Search(queryVector, 3) // 取top3
	if err != nil {
		return "", fmt.Errorf("检索失败: %v", err)
	}

	if len(results) == 0 {
		return "知识库中没有找到相关信息。", nil
	}

	// 3. 构建上下文
	var contextBuilder strings.Builder
	contextBuilder.WriteString("以下是知识库中的相关内容：\n\n")
	for i, result := range results {
		contextBuilder.WriteString(fmt.Sprintf("【文档%d】(相关度: %.2f)\n%s\n\n",
			i+1, result.Score, result.Chunk.Content))
	}

	// 4. 调用LLM生成回答
	answer, err := b.generateAnswer(question, contextBuilder.String())
	if err != nil {
		return "", fmt.Errorf("生成回答失败: %v", err)
	}

	return answer, nil
}

// generateAnswer 调用LLM生成回答
func (b *RAGBot) generateAnswer(question, context string) (string, error) {
	systemPrompt := `你是一个知识库助手。请根据提供的知识库内容回答用户问题。
要求：
1. 优先使用知识库中的信息回答
2. 如果知识库中没有相关信息，请明确说明
3. 回答要准确、简洁、有帮助`

	reqBody := map[string]interface{}{
		"model": "glm-4-flash",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": fmt.Sprintf("%s\n\n用户问题：%s", context, question)},
		},
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", b.ChatAPIURL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.ChatAPIKey)

	resp, err := b.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(body, &result)

	if result.Error != nil {
		return "", fmt.Errorf("API错误: %s", result.Error.Message)
	}

	if len(result.Choices) > 0 {
		return strings.TrimSpace(result.Choices[0].Message.Content), nil
	}

	return "无法生成回答", nil
}

// GetKnowledgeBaseInfo 获取知识库信息
func (b *RAGBot) GetKnowledgeBaseInfo() map[string]interface{} {
	chunks := b.VectorStore.List()
	docs := make(map[string]int)
	for _, chunk := range chunks {
		docs[chunk.DocID]++
	}

	return map[string]interface{}{
		"total_chunks":   len(chunks),
		"total_docs":     len(docs),
		"chunk_details":  docs,
	}
}

// ClearKnowledgeBase 清空知识库
func (b *RAGBot) ClearKnowledgeBase() {
	b.VectorStore.Clear()
}
