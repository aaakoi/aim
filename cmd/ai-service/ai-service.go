package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// 配置
const ZhipuAPIKey = "dd0c45a81ee14876840d41ba223dd66c.E1PMOeB7Wdn33wR8"
const ZhipuAPIURL = "https://open.bigmodel.cn/api/paas/v4/chat/completions"
const DeepSeekAPIKey = "sk-448041a5313041bbbd353916dfa96bf1"
const DeepSeekAPIURL = "https://api.deepseek.com/v1/chat/completions"

// ========== 服务治理配置 ==========
const (
	APITimeout    = 30 * time.Second // API 超时时间
	MaxRetries    = 3                 // 最大重试次数
	RetryBaseWait = 1 * time.Second  // 重试基础等待时间
)

// doRetry 执行重试逻辑
func doRetry(fn func() AIServiceResponse) AIServiceResponse {
	var lastResp AIServiceResponse

	for i := 0; i < MaxRetries; i++ {
		resp := fn()

		// 成功则返回
		if resp.Error == "" {
			return resp
		}

		lastResp = resp

		// 判断是否可重试（网络错误、超时、5xx）
		// 4xx 错误不重试
		if i < MaxRetries-1 {
			waitTime := RetryBaseWait * time.Duration(i+1)
			log.Printf("[重试] 第 %d 次失败: %s，%v 后重试...\n", i+1, resp.Error, waitTime)
			time.Sleep(waitTime)
		}
	}

	lastResp.Error = fmt.Sprintf("重试 %d 次后仍失败: %s", MaxRetries, lastResp.Error)
	return lastResp
}

// 请求结构
type AIServiceRequest struct {
	Provider string `json:"provider"`
	Content  string `json:"content"`
	N        int    `json:"n,omitempty"`
}

// 响应结构
type AIServiceResponse struct {
	Content          string `json:"content"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	Error            string `json:"error,omitempty"`
}

// 多候选响应
type AIServiceMultiResponse struct {
	Candidates []AIServiceCandidate `json:"candidates"`
	Error      string               `json:"error,omitempty"`
}

type AIServiceCandidate struct {
	Index   int    `json:"index"`
	Content string `json:"content"`
}

func main() {
	http.HandleFunc("/chat", handleChat)
	http.HandleFunc("/chat/multi", handleChatMulti)
	http.HandleFunc("/health", handleHealth)

	log.Println("AI Service 启动在 :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	var req AIServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	var resp AIServiceResponse
	switch req.Provider {
	case "zhipu":
		resp = callZhipu(req.Content)
	case "deepseek":
		resp = callDeepSeek(req.Content)
	default:
		resp = callZhipu(req.Content)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleChatMulti(w http.ResponseWriter, r *http.Request) {
	var req AIServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	n := req.N
	if n <= 0 {
		n = 3
	}

	var candidates []AIServiceCandidate
	for i := 0; i < n; i++ {
		var resp AIServiceResponse
		switch req.Provider {
		case "deepseek":
			resp = callDeepSeek(req.Content)
		default:
			resp = callZhipu(req.Content)
		}
		if resp.Error == "" {
			candidates = append(candidates, AIServiceCandidate{
				Index:   len(candidates) + 1,
				Content: resp.Content,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AIServiceMultiResponse{Candidates: candidates})
}

func callZhipu(content string) AIServiceResponse {
	return doRetry(func() AIServiceResponse {
		return callZhipuOnce(content)
	})
}

func callZhipuOnce(content string) AIServiceResponse {
	reqBody := map[string]interface{}{
		"model": "glm-4-flash",
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个友好的AI助手。"},
			{"role": "user", "content": content},
		},
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", ZhipuAPIURL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ZhipuAPIKey)

	client := &http.Client{Timeout: APITimeout}
	resp, err := client.Do(req)
	if err != nil {
		return AIServiceResponse{Error: err.Error()}
	}
	defer resp.Body.Close()

	// 检查 HTTP 状态码
	if resp.StatusCode >= 500 {
		return AIServiceResponse{Error: fmt.Sprintf("服务端错误: %d", resp.StatusCode)}
	}
	if resp.StatusCode >= 400 {
		return AIServiceResponse{Error: fmt.Sprintf("客户端错误: %d", resp.StatusCode)}
	}

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(body, &result)

	if result.Error != nil {
		return AIServiceResponse{Error: result.Error.Message}
	}

	promptTokens, completionTokens := 0, 0
	if result.Usage != nil {
		promptTokens = result.Usage.PromptTokens
		completionTokens = result.Usage.CompletionTokens
	}

	if len(result.Choices) > 0 {
		return AIServiceResponse{
			Content:          strings.TrimSpace(result.Choices[0].Message.Content),
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		}
	}

	return AIServiceResponse{Error: "无有效回复"}
}

func callDeepSeek(content string) AIServiceResponse {
	return doRetry(func() AIServiceResponse {
		return callDeepSeekOnce(content)
	})
}

func callDeepSeekOnce(content string) AIServiceResponse {
	reqBody := map[string]interface{}{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个友好的AI助手。"},
			{"role": "user", "content": content},
		},
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", DeepSeekAPIURL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+DeepSeekAPIKey)

	client := &http.Client{Timeout: APITimeout}
	resp, err := client.Do(req)
	if err != nil {
		return AIServiceResponse{Error: err.Error()}
	}
	defer resp.Body.Close()

	// 检查 HTTP 状态码
	if resp.StatusCode >= 500 {
		return AIServiceResponse{Error: fmt.Sprintf("服务端错误: %d", resp.StatusCode)}
	}
	if resp.StatusCode >= 400 {
		return AIServiceResponse{Error: fmt.Sprintf("客户端错误: %d", resp.StatusCode)}
	}

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(body, &result)

	if result.Error != nil {
		return AIServiceResponse{Error: result.Error.Message}
	}

	promptTokens, completionTokens := 0, 0
	if result.Usage != nil {
		promptTokens = result.Usage.PromptTokens
		completionTokens = result.Usage.CompletionTokens
	}

	if len(result.Choices) > 0 {
		return AIServiceResponse{
			Content:          strings.TrimSpace(result.Choices[0].Message.Content),
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		}
	}

	return AIServiceResponse{Error: "无有效回复"}
}

// ========== 流式调用 ==========

// StreamCallback 流式回调函数类型
type StreamCallback func(token string, done bool)

// callZhipuStream 流式调用智谱 AI
func callZhipuStream(content string, onToken StreamCallback) error {
	reqBody := map[string]interface{}{
		"model": "glm-4-flash",
		"stream": true,
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个友好的AI助手。"},
			{"role": "user", "content": content},
		},
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", ZhipuAPIURL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ZhipuAPIKey)

	client := &http.Client{Timeout: APITimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 逐行读取 SSE 响应
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// SSE 格式: data: {...}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			onToken("", true) // 结束信号
			break
		}

		// 解析 JSON
		var result struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.Unmarshal([]byte(data), &result); err != nil {
			continue
		}

		if result.Error != nil {
			return fmt.Errorf(result.Error.Message)
		}

		if len(result.Choices) > 0 {
			token := result.Choices[0].Delta.Content
			if token != "" {
				onToken(token, false)
			}
			// 检查是否结束
			if result.Choices[0].FinishReason == "stop" {
				onToken("", true)
				break
			}
		}
	}

	return scanner.Err()
}

// callDeepSeekStream 流式调用 DeepSeek
func callDeepSeekStream(content string, onToken StreamCallback) error {
	reqBody := map[string]interface{}{
		"model": "deepseek-chat",
		"stream": true,
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个友好的AI助手。"},
			{"role": "user", "content": content},
		},
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", DeepSeekAPIURL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+DeepSeekAPIKey)

	client := &http.Client{Timeout: APITimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 逐行读取 SSE 响应
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			onToken("", true)
			break
		}

		var result struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.Unmarshal([]byte(data), &result); err != nil {
			continue
		}

		if result.Error != nil {
			return fmt.Errorf(result.Error.Message)
		}

		if len(result.Choices) > 0 {
			token := result.Choices[0].Delta.Content
			if token != "" {
				onToken(token, false)
			}
			if result.Choices[0].FinishReason == "stop" {
				onToken("", true)
				break
			}
		}
	}

	return scanner.Err()
}
