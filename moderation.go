package main

import (
	"fmt"
	"strings"
	"sync"
)

// ModerationResult 审核结果
type ModerationResult struct {
	Pass   bool   // 是否通过
	Reason string // 不通过原因
}

// ModerationManager 审核管理器
type ModerationManager struct {
	sensitiveWords []string
	mu             sync.RWMutex
	enabled        bool
}

// NewModerationManager 创建审核管理器
func NewModerationManager() *ModerationManager {
	return &ModerationManager{
		sensitiveWords: []string{
			"傻逼", "傻B", "傻叉", "傻X", "煞笔",
			"操你", "操你妈", "草你妈",
			"妈的", "他妈的", "TMD",
			"Fuck", "fuck", "FUCK", "Shit", "shit",
			"强奸", "轮奸",
			"杀你", "砍死你",
		},
		enabled: true,
	}
}

// CheckKeywords 关键词过滤
func (m *ModerationManager) CheckKeywords(content string) ModerationResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, word := range m.sensitiveWords {
		if strings.Contains(content, word) {
			return ModerationResult{
				Pass:   false,
				Reason: fmt.Sprintf("内容包含敏感词"),
			}
		}
	}
	return ModerationResult{Pass: true}
}

// CheckByAI AI审核（使用智谱API）
func (m *ModerationManager) CheckByAI(content string) ModerationResult {
	if ZhipuAPIKey == "" {
		return ModerationResult{Pass: true} // 没有API则跳过AI审核
	}

	prompt := fmt.Sprintf(`请判断以下消息内容是否违规。
违规包括：侮辱谩骂、色情低俗、暴力恐怖、政治敏感、广告推销等。

消息内容：%s

请只回答"正常"或"违规"，不要有其他内容。`, content)

	result, err := callModerationAPI(prompt)
	if err != nil {
		// API调用失败，默认通过（避免影响正常使用）
		return ModerationResult{Pass: true}
	}

	if strings.Contains(result, "违规") {
		return ModerationResult{
			Pass:   false,
			Reason: "AI审核不通过",
		}
	}
	return ModerationResult{Pass: true}
}

// Moderate 综合审核（关键词 + AI）
func (m *ModerationManager) Moderate(content string) ModerationResult {
	if !m.enabled {
		return ModerationResult{Pass: true}
	}

	// 1. 关键词过滤
	if result := m.CheckKeywords(content); !result.Pass {
		return result
	}

	// 2. AI审核（可选，默认关闭以节省API调用）
	// 如果需要开启AI审核，取消下面注释
	// return m.CheckByAI(content)

	return ModerationResult{Pass: true}
}

// Enable 开启审核
func (m *ModerationManager) Enable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = true
}

// Disable 关闭审核
func (m *ModerationManager) Disable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = false
}

// IsEnabled 是否开启
func (m *ModerationManager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// AddWord 添加敏感词
func (m *ModerationManager) AddWord(word string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sensitiveWords = append(m.sensitiveWords, word)
}

// RemoveWord 移除敏感词
func (m *ModerationManager) RemoveWord(word string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, w := range m.sensitiveWords {
		if w == word {
			m.sensitiveWords = append(m.sensitiveWords[:i], m.sensitiveWords[i+1:]...)
			break
		}
	}
}

// GetWords 获取敏感词列表
func (m *ModerationManager) GetWords() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string{}, m.sensitiveWords...)
}

// callModerationAPI 调用AI审核API
func callModerationAPI(prompt string) (string, error) {
	result, err := callTranslateAPI(prompt)
	if err != nil {
		return "", err
	}
	return result, nil
}
