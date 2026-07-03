package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ========== 配置区域 ==========
// 请配置你自己的 API Key，获取地址：
// 智谱AI: https://open.bigmodel.cn/
// DeepSeek: https://platform.deepseek.com/
const ZhipuAPIKey = "dd0c45a81ee14876840d41ba223dd66c.E1PMOeB7Wdn33wR8"
const ZhipuAPIURL = "https://open.bigmodel.cn/api/paas/v4/chat/completions"

// DeepSeek 配置
const DeepSeekAPIKey = "sk-448041a5313041bbbd353916dfa96bf1"
const DeepSeekAPIURL = "https://api.deepseek.com/v1/chat/completions"

// AI 服务地址（分布式架构）
const AIServiceURL = "http://localhost:8081"

// ========== 服务治理配置 ==========
const (
	APITimeout       = 30 * time.Second // API 超时时间
	MaxRetries       = 3                // 最大重试次数
	RetryBaseWait    = 1 * time.Second  // 重试基础等待时间
	RateLimitQPS     = 10               // 每秒最多10个请求
	CircuitThreshold = 5                // 连续失败5次触发熔断
	CircuitTimeout   = 30 * time.Second // 熔断持续30秒
)

// ========== 限流器和熔断器实例 ==========
var (
	apiRateLimiter    = NewRateLimiter(RateLimitQPS)
	apiCircuitBreaker = NewCircuitBreaker(CircuitThreshold, CircuitTimeout)
)

// ================================

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries int
	BaseWait   time.Duration
	Timeout    time.Duration
}

// DefaultRetryConfig 默认重试配置
var DefaultRetryConfig = RetryConfig{
	MaxRetries: MaxRetries,
	BaseWait:   RetryBaseWait,
	Timeout:    APITimeout,
}

// RetryableError 可重试的错误
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

// isRetryable 判断错误是否可重试
func isRetryable(err error, statusCode int) bool {
	if err != nil {
		// 网络错误、超时错误可重试
		return true
	}
	// 5xx 错误可重试，4xx 不重试
	return statusCode >= 500
}

// doRetry 执行重试逻辑
func doRetry(config RetryConfig, fn func() (string, int, int, error)) (string, int, int, error) {
	var lastErr error

	for i := 0; i < config.MaxRetries; i++ {
		content, promptTokens, completionTokens, err := fn()

		if err == nil {
			return content, promptTokens, completionTokens, nil
		}

		lastErr = err

		// 判断是否可重试
		if !isRetryable(err, 0) {
			break
		}

		// 最后一次不等待
		if i < config.MaxRetries-1 {
			waitTime := config.BaseWait * time.Duration(i+1)
			fmt.Printf("[重试] 第 %d 次失败: %v，%v 后重试...\n", i+1, err, waitTime)
			time.Sleep(waitTime)
		}
	}

	return "", 0, 0, fmt.Errorf("重试 %d 次后仍失败: %v", config.MaxRetries, lastErr)
}

// AIProvider AI提供商接口
type AIProvider interface {
	Name() string
	CallAPI(content string) (string, error)
	CallAPIMulti(content string, n int) []CandidateReply
}

// APIResult API调用结果（包含token使用信息）
type APIResult struct {
	Content          string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// ZhipuProvider 智谱AI提供商
type ZhipuProvider struct{}

func (p *ZhipuProvider) Name() string { return "zhipu" }

func (p *ZhipuProvider) CallAPI(content string) (string, error) {
	reply, _, _, err := callZhipuAPI(content)
	return reply, err
}

func (p *ZhipuProvider) CallAPIMulti(content string, n int) []CandidateReply {
	return callZhipuAPIMulti(content, n)
}

// DeepSeekProvider DeepSeek提供商
type DeepSeekProvider struct{}

func (p *DeepSeekProvider) Name() string { return "deepseek" }

func (p *DeepSeekProvider) CallAPI(content string) (string, error) {
	reply, _, _, err := callDeepSeekAPI(content)
	return reply, err
}

func (p *DeepSeekProvider) CallAPIMulti(content string, n int) []CandidateReply {
	return callDeepSeekAPIMulti(content, n)
}

// Bot 机器人结构
type Bot struct {
	ID   string
	Name string
}

// BotManager 机器人管理器
type BotManager struct {
	bots     map[string]*Bot
	provider AIProvider // 当前使用的AI提供商
	// 最近一次API调用的token使用量
	LastPromptTokens     int
	LastCompletionTokens int
	LastTotalTokens      int
}

// NewBotManager 创建机器人管理器
func NewBotManager() *BotManager {
	bm := &BotManager{
		bots:     make(map[string]*Bot),
		provider: &ZhipuProvider{}, // 默认使用智谱AI
	}

	bm.bots["Bot"] = &Bot{
		ID:   "Bot",
		Name: "AI助手",
	}

	return bm
}

// SetProvider 设置AI提供商
func (bm *BotManager) SetProvider(provider AIProvider) {
	bm.provider = provider
}

// GetProvider 获取当前AI提供商
func (bm *BotManager) GetProvider() AIProvider {
	return bm.provider
}

// IsBot 检查是否是 Bot
func (bm *BotManager) IsBot(id string) bool {
	_, exists := bm.bots[id]
	return exists
}

// GetBot 获取 Bot
func (bm *BotManager) GetBot(id string) *Bot {
	return bm.bots[id]
}

// HandleBotMessage 处理发给 Bot 的消息
func (bm *BotManager) HandleBotMessage(botID, from, content string) string {
	bot := bm.GetBot(botID)
	if bot == nil {
		return fmt.Sprintf("Bot [%s] 不存在", botID)
	}

	return bm.generateReply(from, content)
}

// HandleBotMessageWithMemory 处理发给 Bot 的消息（带记忆）
func (bm *BotManager) HandleBotMessageWithMemory(botID, from, content string, history []ChatMessage) string {
	bot := bm.GetBot(botID)
	if bot == nil {
		return fmt.Sprintf("Bot [%s] 不存在", botID)
	}

	return bm.generateReplyWithMemory(from, content, history)
}

// generateReplyWithMemory 生成回复（带历史记忆）
func (bm *BotManager) generateReplyWithMemory(from, content string, history []ChatMessage) string {
	// 构建带历史的Prompt
	prompt := bm.buildPromptWithHistory(content, history)

	// 调用 AI 服务（记录延迟）
	start := time.Now()
	resp, err := callAIService(bm.provider.Name(), prompt)
	latency := time.Since(start).Seconds()

	if err != nil {
		// 指标：记录失败请求
		RecordBotRequest(bm.provider.Name(), "error")
		return fmt.Sprintf("抱歉，AI 服务暂时不可用: %v", err)
	}

	// 指标：记录成功请求和延迟
	RecordBotRequest(bm.provider.Name(), "success")
	RecordBotLatency(latency)

	// 记录token使用量
	bm.LastPromptTokens = resp.PromptTokens
	bm.LastCompletionTokens = resp.CompletionTokens
	bm.LastTotalTokens = resp.TotalTokens

	// 指标：记录Token使用量
	RecordBotTokens("prompt", bm.provider.Name(), resp.PromptTokens)
	RecordBotTokens("completion", bm.provider.Name(), resp.CompletionTokens)

	return resp.Content
}

// buildPromptWithHistory 构建带历史记录的Prompt
func (bm *BotManager) buildPromptWithHistory(content string, history []ChatMessage) string {
	var prompt strings.Builder

	// 添加历史对话
	if len(history) > 0 {
		prompt.WriteString("以下是之前的对话记录：\n")
		for _, msg := range history {
			switch msg.Role {
			case "user":
				prompt.WriteString(fmt.Sprintf("用户：%s\n", msg.Content))
			case "assistant":
				prompt.WriteString(fmt.Sprintf("助手：%s\n", msg.Content))
			}
		}
		prompt.WriteString("\n")
	}

	// 添加当前问题
	prompt.WriteString(content)

	return prompt.String()
}

// generateReply 生成回复，同时记录token使用量
func (bm *BotManager) generateReply(from, content string) string {
	// 调用 AI 服务（分布式）
	resp, err := callAIService(bm.provider.Name(), content)
	if err != nil {
		return fmt.Sprintf("抱歉，AI 服务暂时不可用: %v", err)
	}

	// 记录token使用量
	bm.LastPromptTokens = resp.PromptTokens
	bm.LastCompletionTokens = resp.CompletionTokens
	bm.LastTotalTokens = resp.TotalTokens

	return resp.Content
}

// callAIService 调用 AI 微服务
func callAIService(provider, content string) (*AIResponse, error) {
	reqBody := AIRequest{
		Provider: provider,
		Content:  content,
	}

	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post(AIServiceURL+"/chat", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		// 如果 AI 服务不可用，回退到直接调用
		return callAIDirect(provider, content)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result AIResponse
	json.Unmarshal(body, &result)

	if result.Error != "" {
		return nil, fmt.Errorf(result.Error)
	}

	return &result, nil
}

// callAIDirect 直接调用 AI（回退方案）
func callAIDirect(provider, content string) (*AIResponse, error) {
	switch provider {
	case "zhipu":
		content, pt, ct, err := callZhipuAPI(content)
		return &AIResponse{Content: content, PromptTokens: pt, CompletionTokens: ct}, err
	case "deepseek":
		content, pt, ct, err := callDeepSeekAPI(content)
		return &AIResponse{Content: content, PromptTokens: pt, CompletionTokens: ct}, err
	default:
		content, pt, ct, err := callZhipuAPI(content)
		return &AIResponse{Content: content, PromptTokens: pt, CompletionTokens: ct}, err
	}
}

// AIRequest AI服务请求
type AIRequest struct {
	Provider string `json:"provider"`
	Content  string `json:"content"`
}

// AIResponse AI服务响应
type AIResponse struct {
	Content          string `json:"content"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	Error            string `json:"error,omitempty"`
}

// GetLastTokenUsage 获取最近一次API调用的token使用量
func (bm *BotManager) GetLastTokenUsage() (prompt, completion, total int) {
	return bm.LastPromptTokens, bm.LastCompletionTokens, bm.LastTotalTokens
}

// 智谱 AI API 请求结构
type ZhipuRequest struct {
	Model    string         `json:"model"`
	Messages []ZhipuMessage `json:"messages"`
	N        int            `json:"n,omitempty"` // 生成N个候选
}

// CandidateReply 候选回复
type CandidateReply struct {
	Index   int    `json:"index"`
	Content string `json:"content"`
}

type ZhipuMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// 智谱 AI API 响应结构
type ZhipuResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// callZhipuAPI 调用智谱 AI API，返回内容和token使用量
func callZhipuAPI(content string) (string, int, int, error) {
	var result string
	var promptTokens, completionTokens int

	// ========== 熔断保护 ==========
	err := apiCircuitBreaker.Call(func() error {
		// 重试机制
		var apiErr error
		result, promptTokens, completionTokens, apiErr = doRetry(DefaultRetryConfig, func() (string, int, int, error) {
			return callZhipuAPIOnce(content)
		})
		return apiErr
	})

	return result, promptTokens, completionTokens, err
}

// callZhipuAPIOnce 单次调用智谱 AI API
func callZhipuAPIOnce(content string) (string, int, int, error) {
	// ========== 限流检查 ==========
	if !apiRateLimiter.Allow() {
		return "", 0, 0, fmt.Errorf("请求过于频繁，请稍后重试")
	}

	// 检查 API Key 是否配置
	if ZhipuAPIKey == "" {
		return "请先配置智谱 AI API Key（在 bot.go 文件中）", 0, 0, nil
	}

	// 构建请求
	reqBody := ZhipuRequest{
		Model: "glm-4-flash", // 免费模型
		Messages: []ZhipuMessage{
			{
				Role:    "system",
				Content: "你是一个友好的 AI 助手，帮助用户解答问题。回答要简洁、有帮助。",
			},
			{
				Role:    "user",
				Content: content,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, 0, err
	}

	// 创建 HTTP 请求
	req, err := http.NewRequest("POST", ZhipuAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", 0, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ZhipuAPIKey)

	// 发送请求（带超时）
	client := &http.Client{Timeout: APITimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, 0, err
	}
	defer resp.Body.Close()

	// 检查 HTTP 状态码
	if resp.StatusCode >= 500 {
		return "", 0, 0, fmt.Errorf("服务端错误: %d", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, 0, fmt.Errorf("客户端错误: %d, %s", resp.StatusCode, string(body))
	}

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, 0, err
	}

	// 解析响应
	var zhipuResp ZhipuResponse
	if err := json.Unmarshal(body, &zhipuResp); err != nil {
		return "", 0, 0, err
	}

	// 检查错误
	if zhipuResp.Error != nil {
		return "", 0, 0, fmt.Errorf("API 错误: %s", zhipuResp.Error.Message)
	}

	// 返回结果和token使用量
	promptTokens := 0
	completionTokens := 0
	if zhipuResp.Usage != nil {
		promptTokens = zhipuResp.Usage.PromptTokens
		completionTokens = zhipuResp.Usage.CompletionTokens
	}

	if len(zhipuResp.Choices) > 0 {
		return strings.TrimSpace(zhipuResp.Choices[0].Message.Content), promptTokens, completionTokens, nil
	}

	return "AI 没有返回有效回复", promptTokens, completionTokens, nil
}

// GenerateCandidates 生成多个候选回复
func (bm *BotManager) GenerateCandidates(from, content string, count int) []CandidateReply {
	return bm.provider.CallAPIMulti(content, count)
}

// callZhipuAPIMulti 调用智谱 AI API 生成多个候选
// 通过多次调用生成多个不同风格的回复
func callZhipuAPIMulti(content string, n int) []CandidateReply {
	if ZhipuAPIKey == "" {
		return []CandidateReply{{Index: 0, Content: "请先配置智谱 AI API Key"}}
	}

	var candidates []CandidateReply

	// 多次调用 API 生成多个候选
	for i := 0; i < n; i++ {
		reqBody := ZhipuRequest{
			Model: "glm-4-flash",
			Messages: []ZhipuMessage{
				{Role: "system", Content: "你是一个友好的 AI 助手。每次回复风格可以略有不同。"},
				{Role: "user", Content: content},
			},
		}

		jsonData, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", ZhipuAPIURL, bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+ZhipuAPIKey)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var zhipuResp ZhipuResponse
		json.Unmarshal(body, &zhipuResp)

		if zhipuResp.Error != nil {
			continue
		}

		if len(zhipuResp.Choices) > 0 {
			candidates = append(candidates, CandidateReply{
				Index:   len(candidates) + 1,
				Content: strings.TrimSpace(zhipuResp.Choices[0].Message.Content),
			})
		}
	}

	if len(candidates) == 0 {
		return []CandidateReply{{Index: 1, Content: "AI 没有返回有效回复"}}
	}

	return candidates
}

// SummarizeMessages 总结消息
func (bm *BotManager) SummarizeMessages(messages []StoredMessage) string {
	if len(messages) == 0 {
		return "没有消息需要总结"
	}

	// 构建消息文本
	var msgText strings.Builder
	for _, msg := range messages {
		timeStr := msg.Timestamp.Format("15:04")
		content := msg.Content
		if msg.Revoked {
			content = "[已撤回]"
		}
		msgText.WriteString(fmt.Sprintf("[%s] %s: %s\n", timeStr, msg.From, content))
	}

	// 调用 AI 总结
	prompt := fmt.Sprintf("请总结以下聊天记录的要点（用简洁的中文，3-5条）：\n\n%s", msgText.String())

	reply, err := bm.provider.CallAPI(prompt)
	if err != nil {
		return fmt.Sprintf("总结失败: %v", err)
	}
	return reply
}

// ExtractTodos 提取待办事项
func (bm *BotManager) ExtractTodos(messages []StoredMessage) string {
	if len(messages) == 0 {
		return "没有消息可分析"
	}

	// 构建消息文本
	var msgText strings.Builder
	for _, msg := range messages {
		timeStr := msg.Timestamp.Format("15:04")
		content := msg.Content
		if msg.Revoked {
			continue // 跳过已撤回的消息
		}
		msgText.WriteString(fmt.Sprintf("[%s] %s: %s\n", timeStr, msg.From, content))
	}

	// 调用 AI 提取待办
	prompt := fmt.Sprintf("从以下聊天记录中提取待办事项（需要完成的任务），格式：- [人名] 任务内容\n\n%s", msgText.String())

	reply, err := bm.provider.CallAPI(prompt)
	if err != nil {
		return fmt.Sprintf("提取失败: %v", err)
	}
	return reply
}

// ========== DeepSeek API ==========

// DeepSeekRequest DeepSeek请求结构
type DeepSeekRequest struct {
	Model    string            `json:"model"`
	Messages []DeepSeekMessage `json:"messages"`
	N        int               `json:"n,omitempty"`
}

type DeepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// DeepSeekResponse DeepSeek响应结构
type DeepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// callDeepSeekAPI 调用DeepSeek API，返回内容和token使用量
func callDeepSeekAPI(content string) (string, int, int, error) {
	var result string
	var promptTokens, completionTokens int

	// ========== 熔断保护 ==========
	err := apiCircuitBreaker.Call(func() error {
		// 重试机制
		var apiErr error
		result, promptTokens, completionTokens, apiErr = doRetry(DefaultRetryConfig, func() (string, int, int, error) {
			return callDeepSeekAPIOnce(content)
		})
		return apiErr
	})

	return result, promptTokens, completionTokens, err
}

// callDeepSeekAPIOnce 单次调用 DeepSeek API
func callDeepSeekAPIOnce(content string) (string, int, int, error) {
	// ========== 限流检查 ==========
	if !apiRateLimiter.Allow() {
		return "", 0, 0, fmt.Errorf("请求过于频繁，请稍后重试")
	}
	if DeepSeekAPIKey == "" {
		return "DeepSeek API Key 未配置", 0, 0, fmt.Errorf("DeepSeek API Key 未配置")
	}

	reqBody := DeepSeekRequest{
		Model: "deepseek-chat",
		Messages: []DeepSeekMessage{
			{Role: "system", Content: "你是一个友好的 AI 助手。"},
			{Role: "user", Content: content},
		},
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", DeepSeekAPIURL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+DeepSeekAPIKey)

	client := &http.Client{Timeout: APITimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, 0, err
	}
	defer resp.Body.Close()

	// 检查 HTTP 状态码
	if resp.StatusCode >= 500 {
		return "", 0, 0, fmt.Errorf("服务端错误: %d", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, 0, fmt.Errorf("客户端错误: %d, %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	var deepseekResp DeepSeekResponse
	json.Unmarshal(body, &deepseekResp)

	if deepseekResp.Error != nil {
		return "", 0, 0, fmt.Errorf("DeepSeek 错误: %s", deepseekResp.Error.Message)
	}

	promptTokens := 0
	completionTokens := 0
	if deepseekResp.Usage != nil {
		promptTokens = deepseekResp.Usage.PromptTokens
		completionTokens = deepseekResp.Usage.CompletionTokens
	}

	if len(deepseekResp.Choices) > 0 {
		return strings.TrimSpace(deepseekResp.Choices[0].Message.Content), promptTokens, completionTokens, nil
	}

	return "DeepSeek 没有返回有效回复", promptTokens, completionTokens, nil
}

// callDeepSeekAPIMulti 调用DeepSeek API生成多个候选回复
// DeepSeek 不支持 n 参数，通过多次调用生成多个候选
func callDeepSeekAPIMulti(content string, n int) []CandidateReply {
	if DeepSeekAPIKey == "" {
		return []CandidateReply{{Index: 1, Content: "DeepSeek API Key 未配置"}}
	}

	var candidates []CandidateReply

	// 多次调用 API 生成多个候选
	for i := 0; i < n; i++ {
		reqBody := DeepSeekRequest{
			Model: "deepseek-chat",
			Messages: []DeepSeekMessage{
				{Role: "system", Content: "你是一个友好的 AI 助手。每次回复风格可以略有不同。"},
				{Role: "user", Content: content},
			},
		}

		jsonData, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", DeepSeekAPIURL, bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+DeepSeekAPIKey)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			continue // 跳过失败的调用
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var deepseekResp DeepSeekResponse
		json.Unmarshal(body, &deepseekResp)

		if deepseekResp.Error != nil {
			continue
		}

		if len(deepseekResp.Choices) > 0 {
			candidates = append(candidates, CandidateReply{
				Index:   len(candidates) + 1,
				Content: strings.TrimSpace(deepseekResp.Choices[0].Message.Content),
			})
		}
	}

	if len(candidates) == 0 {
		return []CandidateReply{{Index: 1, Content: "DeepSeek 没有返回有效回复"}}
	}

	return candidates
}

// ========== 流式调用 ==========

// StreamCallback 流式回调函数类型
type StreamCallback func(token string, done bool)

// HandleBotMessageStream 流式处理 Bot 消息
func (bm *BotManager) HandleBotMessageStream(botID, from, content string, onToken StreamCallback) {
	bot := bm.GetBot(botID)
	if bot == nil {
		onToken(fmt.Sprintf("Bot [%s] 不存在", botID), true)
		return
	}

	// ========== 链路追踪开始 ==========
	tracer := otel.Tracer("aim-bot")
	_, span := tracer.Start(context.Background(), "HandleBotMessageStream",
		trace.WithAttributes(
			attribute.String("bot.id", botID),
			attribute.String("user", from),
			attribute.String("provider", bm.provider.Name()),
		))
	defer span.End()

	// 根据提供商调用不同的流式 API
	var err error
	switch bm.provider.Name() {
	case "deepseek":
		err = callDeepSeekStream(content, onToken)
		span.SetAttributes(attribute.String("api.provider", "deepseek"))
	default:
		err = callZhipuStream(content, onToken)
		span.SetAttributes(attribute.String("api.provider", "zhipu"))
	}

	// ========== 降级处理 ==========
	if err != nil {
		// 记录失败
		RecordBotRequest(bm.provider.Name(), "error")

		// ========== 链路追踪记录错误 ==========
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		// 根据错误类型返回友好的降级消息
		var downgradeMsg string
		if err == ErrCircuitOpen {
			downgradeMsg = "⚠️ AI服务繁忙，已触发熔断保护，请30秒后重试"
		} else if strings.Contains(err.Error(), "请求过于频繁") {
			downgradeMsg = "⚠️ 请求太频繁了，请稍后再试"
		} else {
			downgradeMsg = fmt.Sprintf("⚠️ AI服务暂时不可用: %v", err)
		}
		onToken(downgradeMsg, true)
	} else {
		RecordBotRequest(bm.provider.Name(), "success")
		// ========== 链路追踪记录成功 ==========
		span.SetStatus(codes.Ok, "success")
	}
}

// callZhipuStream 流式调用智谱 AI
func callZhipuStream(content string, onToken StreamCallback) error {
	// ========== 限流检查 ==========
	if !apiRateLimiter.Allow() {
		return fmt.Errorf("请求过于频繁，请稍后重试")
	}

	// ========== 熔断保护 ==========
	return apiCircuitBreaker.Call(func() error {
		return callZhipuStreamOnce(content, onToken)
	})
}

// callZhipuStreamOnce 单次流式调用（内部函数）
func callZhipuStreamOnce(content string, onToken StreamCallback) error {
	reqBody := map[string]interface{}{
		"model":  "glm-4-flash",
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
			onToken("", true)
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
	// ========== 限流检查 ==========
	if !apiRateLimiter.Allow() {
		return fmt.Errorf("请求过于频繁，请稍后重试")
	}

	// ========== 熔断保护 ==========
	return apiCircuitBreaker.Call(func() error {
		return callDeepSeekStreamOnce(content, onToken)
	})
}

// callDeepSeekStreamOnce 单次流式调用（内部函数）
func callDeepSeekStreamOnce(content string, onToken StreamCallback) error {
	reqBody := map[string]interface{}{
		"model":  "deepseek-chat",
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
