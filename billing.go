package main

import (
	"fmt"
	"sync"
	"time"
)

// UsageRecord 使用记录
type UsageRecord struct {
	UserID    string
	Provider  string
	Tokens    int
	Cost      float64
	Timestamp time.Time
}

// BillingManager 计费管理器
type BillingManager struct {
	records   []UsageRecord
	userQuota map[string]float64 // 用户额度（元）
	userUsage map[string]float64 // 用户已用额度
	mu        sync.RWMutex
}

// NewBillingManager 创建计费管理器
func NewBillingManager() *BillingManager {
	return &BillingManager{
		records:   make([]UsageRecord, 0),
		userQuota: make(map[string]float64),
		userUsage: make(map[string]float64),
	}
}

// SetQuota 设置用户额度
func (bm *BillingManager) SetQuota(userID string, quota float64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.userQuota[userID] = quota
}

// GetQuota 获取用户额度
func (bm *BillingManager) GetQuota(userID string) float64 {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.userQuota[userID]
}

// GetUsage 获取用户已用额度
func (bm *BillingManager) GetUsage(userID string) float64 {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.userUsage[userID]
}

// GetRemaining 获取用户剩余额度
func (bm *BillingManager) GetRemaining(userID string) float64 {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.userQuota[userID] - bm.userUsage[userID]
}

// RecordUsage 记录使用量
func (bm *BillingManager) RecordUsage(userID, provider string, tokens int, cost float64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	record := UsageRecord{
		UserID:    userID,
		Provider:  provider,
		Tokens:    tokens,
		Cost:      cost,
		Timestamp: time.Now(),
	}
	bm.records = append(bm.records, record)
	bm.userUsage[userID] += cost
}

// CanUse 检查用户是否还有额度
func (bm *BillingManager) CanUse(userID string) bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	quota, hasQuota := bm.userQuota[userID]
	if !hasQuota {
		return true // 没设置额度则不限制
	}
	return bm.userUsage[userID] < quota
}

// GetUserStats 获取用户统计
func (bm *BillingManager) GetUserStats(userID string) string {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	quota := bm.userQuota[userID]
	usage := bm.userUsage[userID]
	remaining := quota - usage

	return fmt.Sprintf("额度: %.4f元, 已用: %.4f元, 剩余: %.4f元", quota, usage, remaining)
}

// GetUsageHistory 获取用户使用历史
func (bm *BillingManager) GetUsageHistory(userID string, limit int) []UsageRecord {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var userRecords []UsageRecord
	for i := len(bm.records) - 1; i >= 0 && len(userRecords) < limit; i-- {
		if bm.records[i].UserID == userID {
			userRecords = append(userRecords, bm.records[i])
		}
	}
	return userRecords
}

// EstimateCost 估算费用（简化版，按字符数）
func EstimateCost(provider string, input, output string) float64 {
	inputLen := len(input)
	outputLen := len(output)

	// 简化计费：按字符估算
	// 实际应按token计费
	switch provider {
	case "zhipu":
		// 智谱AI 约 0.001元/千字符
		return float64(inputLen+outputLen) * 0.000001
	case "deepseek":
		// DeepSeek 约 0.001元/千字符
		return float64(inputLen+outputLen) * 0.000001
	default:
		return 0
	}
}

// CalculateTokenCost 基于实际token计算费用
func CalculateTokenCost(provider string, promptTokens, completionTokens int) float64 {
	// 各平台的token价格（元/千tokens）
	var promptPrice, completionPrice float64

	switch provider {
	case "zhipu":
		// 智谱AI glm-4-flash: 输入0.001元/千tokens, 输出0.001元/千tokens
		promptPrice = 0.000001      // 元/token
		completionPrice = 0.000001  // 元/token
	case "deepseek":
		// DeepSeek: 输入0.001元/千tokens, 输出0.002元/千tokens
		promptPrice = 0.000001
		completionPrice = 0.000002
	default:
		promptPrice = 0.000001
		completionPrice = 0.000001
	}

	return float64(promptTokens)*promptPrice + float64(completionTokens)*completionPrice
}
