package main

import (
	"database/sql"
	"sync"
	"time"
)

// ChatMessage 对话消息
type ChatMessage struct {
	Role      string    // "user" 或 "assistant"
	Content   string    // 消息内容
	Timestamp time.Time // 时间
}

// MemoryManager 记忆管理器
type MemoryManager struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewMemoryManager 创建记忆管理器
func NewMemoryManager() *MemoryManager {
	db, err := sql.Open("sqlite", "./messages.db")
	if err != nil {
		panic(err)
	}
	return &MemoryManager{db: db}
}

// SaveMessage 保存对话消息
func (m *MemoryManager) SaveMessage(userID, role, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`
		INSERT INTO chat_history (user_id, role, content)
		VALUES (?, ?, ?)
	`, userID, role, content)

	return err
}

// GetHistory 获取用户的历史对话（最近N条）
func (m *MemoryManager) GetHistory(userID string, limit int) []ChatMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 10 // 默认10条
	}

	rows, err := m.db.Query(`
		SELECT role, content, created_at
		FROM chat_history
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var messages []ChatMessage
	for rows.Next() {
		var msg ChatMessage
		var timestamp time.Time
		rows.Scan(&msg.Role, &msg.Content, &timestamp)
		msg.Timestamp = timestamp
		messages = append(messages, msg)
	}

	// 反转，让最早的在前面
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages
}

// ClearHistory 清空用户的对话历史
func (m *MemoryManager) ClearHistory(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`DELETE FROM chat_history WHERE user_id = ?`, userID)
	return err
}

// GetHistoryCount 获取历史消息数量
func (m *MemoryManager) GetHistoryCount(userID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var count int
	m.db.QueryRow(`SELECT COUNT(*) FROM chat_history WHERE user_id = ?`, userID).Scan(&count)
	return count
}

// SaveUserPreference 保存用户偏好（复用chat_history表，role="preference"）
func (m *MemoryManager) SaveUserPreference(userID, preference string) error {
	return m.SaveMessage(userID, "preference", preference)
}

// GetUserPreferences 获取用户偏好
func (m *MemoryManager) GetUserPreferences(userID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query(`
		SELECT content FROM chat_history
		WHERE user_id = ? AND role = 'preference'
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var prefs []string
	for rows.Next() {
		var pref string
		rows.Scan(&pref)
		prefs = append(prefs, pref)
	}

	return prefs
}
