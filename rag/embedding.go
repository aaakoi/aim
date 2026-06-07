package rag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// 智谱Embedding API配置
const ZhipuEmbeddingURL = "https://open.bigmodel.cn/api/paas/v4/embeddings"
const ZhipuEmbeddingModel = "embedding-3"

// EmbeddingRequest 嵌入请求
type EmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// EmbeddingResponse 嵌入响应
type EmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
		Object    string    `json:"object"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// EmbeddingService 嵌入服务
type EmbeddingService struct {
	APIKey string
	Client *http.Client
}

// NewEmbeddingService 创建嵌入服务
func NewEmbeddingService(apiKey string) *EmbeddingService {
	return &EmbeddingService{
		APIKey: apiKey,
		Client: &http.Client{Timeout: 30 * time.Second},
	}
}

// GetEmbedding 获取文本的向量嵌入
func (e *EmbeddingService) GetEmbedding(text string) ([]float64, error) {
	// 检查文本长度，智谱API限制
	if len(text) == 0 {
		return nil, fmt.Errorf("文本为空")
	}

	// 限制文本长度，避免API报错
	maxLen := 8000
	if len(text) > maxLen {
		text = text[:maxLen]
	}

	reqBody := EmbeddingRequest{
		Model: ZhipuEmbeddingModel,
		Input: text,
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", ZhipuEmbeddingURL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.APIKey)

	resp, err := e.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result EmbeddingResponse
	json.Unmarshal(body, &result)

	if result.Error != nil {
		return nil, fmt.Errorf("API错误: %s", result.Error.Message)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("未返回嵌入向量")
	}

	return result.Data[0].Embedding, nil
}

// GetEmbeddings 批量获取向量嵌入
func (e *EmbeddingService) GetEmbeddings(texts []string) ([][]float64, error) {
	embeddings := make([][]float64, len(texts))
	for i, text := range texts {
		emb, err := e.GetEmbedding(text)
		if err != nil {
			return nil, fmt.Errorf("文本%d嵌入失败: %v", i, err)
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

// CosineSimilarity 计算余弦相似度
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}
