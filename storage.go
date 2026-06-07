package main

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// StoredMessage 存储的消息
type StoredMessage struct {
	ID        string
	Type      string
	From      string
	To        string
	Content   string
	Timestamp time.Time
	ReadBy    []string
	ReplyTo   string
	Revoked   bool
}

// MessageStorage 消息存储器（SQLite）
type MessageStorage struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewMessageStorage 创建消息存储器
func NewMessageStorage() *MessageStorage {
	db, err := sql.Open("sqlite", "./messages.db")
	if err != nil {
		panic(err)
	}

	// 创建表
	db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			type TEXT,
			from_user TEXT,
			to_user TEXT,
			content TEXT,
			timestamp DATETIME,
			reply_to TEXT,
			revoked BOOLEAN DEFAULT 0
		);
	`)
	db.Exec(`
		CREATE TABLE IF NOT EXISTS read_status (
			message_id TEXT,
			user_id TEXT,
			PRIMARY KEY (message_id, user_id)
		);
	`)
	db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE,
			password TEXT,
			quota REAL DEFAULT 10.0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	// 知识库表
	db.Exec(`
		CREATE TABLE IF NOT EXISTS knowledge_base (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			doc_id TEXT,
			doc_name TEXT,
			content TEXT,
			vector BLOB,
			chunk_index INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_kb_user ON knowledge_base(user_id)`)
	// 对话历史表（记忆能力）
	db.Exec(`
		CREATE TABLE IF NOT EXISTS chat_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT,
			role TEXT,
			content TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_chat_user ON chat_history(user_id)`)

	return &MessageStorage{db: db}
}

// Save 保存消息，返回消息ID
func (ms *MessageStorage) Save(msgType, from, to, content string) string {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	msgID := uuid.New().String()[:8]
	now := time.Now().Format("2006-01-02 15:04:05")

	ms.db.Exec(`
		INSERT INTO messages (id, type, from_user, to_user, content, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)
	`, msgID, msgType, from, to, content, now)

	return msgID
}

// SaveReply 保存回复消息（带引用）
func (ms *MessageStorage) SaveReply(msgType, from, to, content, replyTo string) string {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	msgID := uuid.New().String()[:8]
	now := time.Now().Format("2006-01-02 15:04:05")

	ms.db.Exec(`
		INSERT INTO messages (id, type, from_user, to_user, content, timestamp, reply_to)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, msgID, msgType, from, to, content, now, replyTo)

	return msgID
}

// GetByID 根据ID获取消息
func (ms *MessageStorage) GetByID(msgID string) *StoredMessage {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	var msg StoredMessage
	var timestampStr string
	var replyTo sql.NullString
	var revoked int

	err := ms.db.QueryRow(`
		SELECT id, type, from_user, to_user, content, timestamp, reply_to, revoked
		FROM messages WHERE id = ?
	`, msgID).Scan(&msg.ID, &msg.Type, &msg.From, &msg.To, &msg.Content, &timestampStr, &replyTo, &revoked)

	if err != nil {
		return nil
	}

	// 解析时间字符串
	msg.Timestamp, _ = time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", timestampStr)
	if replyTo.Valid {
		msg.ReplyTo = replyTo.String
	}
	msg.Revoked = revoked == 1
	msg.ReadBy = ms.getReadBy(msgID)
	return &msg
}

// MarkRead 标记消息已读
func (ms *MessageStorage) MarkRead(msgID, userID string) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// 检查是否已读
	var count int
	ms.db.QueryRow(`SELECT COUNT(*) FROM read_status WHERE message_id = ? AND user_id = ?`, msgID, userID).Scan(&count)
	if count > 0 {
		return false
	}

	_, err := ms.db.Exec(`INSERT INTO read_status (message_id, user_id) VALUES (?, ?)`, msgID, userID)
	return err == nil
}

// Revoke 撤回消息
func (ms *MessageStorage) Revoke(msgID, userID string) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	result, _ := ms.db.Exec(`
		UPDATE messages SET revoked = 1
		WHERE id = ? AND from_user = ? AND revoked = 0
		AND timestamp > datetime('now', '-2 minutes')
	`, msgID, userID)

	rows, _ := result.RowsAffected()
	return rows > 0
}

// Edit 编辑消息（15分钟内可编辑）
func (ms *MessageStorage) Edit(msgID, userID, newContent string) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// 只能编辑未撤回的自己发的消息，且在15分钟内
	result, _ := ms.db.Exec(`
		UPDATE messages SET content = ?
		WHERE id = ? AND from_user = ? AND revoked = 0
		AND timestamp > datetime('now', '-15 minutes')
	`, newContent, msgID, userID)

	rows, _ := result.RowsAffected()
	return rows > 0
}

// GetRecent 获取最近N条消息
func (ms *MessageStorage) GetRecent(n int) []StoredMessage {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	rows, _ := ms.db.Query(`
		SELECT id, type, from_user, to_user, content, timestamp, reply_to, revoked
		FROM messages ORDER BY timestamp DESC LIMIT ?
	`, n)
	defer rows.Close()

	var messages []StoredMessage
	for rows.Next() {
		var msg StoredMessage
		var timestampStr string
		var replyTo sql.NullString
		var revoked int
		rows.Scan(&msg.ID, &msg.Type, &msg.From, &msg.To, &msg.Content, &timestampStr, &replyTo, &revoked)
		msg.Timestamp = parseTimestamp(timestampStr)
		if replyTo.Valid {
			msg.ReplyTo = replyTo.String
		}
		msg.Revoked = revoked == 1
		msg.ReadBy = ms.getReadBy(msg.ID)
		messages = append(messages, msg)
	}

	// 反转，最新的在后面
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.Before(messages[j].Timestamp)
	})

	return messages
}

// GetUnreadByUser 获取发给某用户的未读消息
func (ms *MessageStorage) GetUnreadByUser(userID string) []StoredMessage {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	rows, _ := ms.db.Query(`
		SELECT id, type, from_user, to_user, content, timestamp
		FROM messages
		WHERE to_user = ? AND id NOT IN (
			SELECT message_id FROM read_status WHERE user_id = ?
		)
		ORDER BY timestamp DESC
	`, userID, userID)
	defer rows.Close()

	var messages []StoredMessage
	for rows.Next() {
		var msg StoredMessage
		var timestampStr string
		rows.Scan(&msg.ID, &msg.Type, &msg.From, &msg.To, &msg.Content, &timestampStr)
		fmt.Printf("[DEBUG] 原始时间字符串: [%s], 长度: %d\n", timestampStr, len(timestampStr))
		msg.Timestamp = parseTimestamp(timestampStr)
		fmt.Printf("[DEBUG] 解析后时间: %v\n", msg.Timestamp)
		messages = append(messages, msg)
	}

	return messages
}

// SearchByKeywordWithFilter 带过滤条件的关键词搜索
func (ms *MessageStorage) SearchByKeywordWithFilter(keyword, msgType, user string) []StoredMessage {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	query := `SELECT id, type, from_user, to_user, content, timestamp
			  FROM messages WHERE revoked = 0`
	args := []any{}

	if keyword != "" {
		query += " AND content LIKE ?"
		args = append(args, "%"+keyword+"%")
	}
	if msgType != "" {
		query += " AND type = ?"
		args = append(args, msgType)
	}
	if user != "" {
		query += " AND (from_user = ? OR to_user = ?)"
		args = append(args, user, user)
	}
	query += " ORDER BY timestamp DESC LIMIT 50"

	rows, _ := ms.db.Query(query, args...)
	defer rows.Close()

	var messages []StoredMessage
	for rows.Next() {
		var msg StoredMessage
		var timestampStr string
		rows.Scan(&msg.ID, &msg.Type, &msg.From, &msg.To, &msg.Content, &timestampStr)
		msg.Timestamp = parseTimestamp(timestampStr)
		messages = append(messages, msg)
	}

	return messages
}

// SearchByTimeRange 按时间范围搜索消息
func (ms *MessageStorage) SearchByTimeRange(start, end time.Time) []StoredMessage {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	rows, _ := ms.db.Query(`
		SELECT id, type, from_user, to_user, content, timestamp
		FROM messages
		WHERE timestamp >= ? AND timestamp <= ? AND revoked = 0
		ORDER BY timestamp DESC
	`, start, end)
	defer rows.Close()

	var messages []StoredMessage
	for rows.Next() {
		var msg StoredMessage
		var timestampStr string
		rows.Scan(&msg.ID, &msg.Type, &msg.From, &msg.To, &msg.Content, &timestampStr)
		msg.Timestamp = parseTimestamp(timestampStr)
		messages = append(messages, msg)
	}

	return messages
}

// GetAll 获取所有消息
func (ms *MessageStorage) GetAll() []StoredMessage {
	return ms.GetRecent(1000)
}

// GetByUser 获取与某用户相关的消息
func (ms *MessageStorage) GetByUser(userID string) []StoredMessage {
	return ms.SearchByKeywordWithFilter("", "single", userID)
}

// GetByGroup 获取群聊消息
func (ms *MessageStorage) GetByGroup(groupName string) []StoredMessage {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	rows, _ := ms.db.Query(`
		SELECT id, type, from_user, to_user, content, timestamp
		FROM messages WHERE type = 'group' AND to_user = ?
		ORDER BY timestamp DESC
	`, groupName)
	defer rows.Close()

	var messages []StoredMessage
	for rows.Next() {
		var msg StoredMessage
		var timestampStr string
		rows.Scan(&msg.ID, &msg.Type, &msg.From, &msg.To, &msg.Content, &timestampStr)
		msg.Timestamp = parseTimestamp(timestampStr)
		messages = append(messages, msg)
	}

	return messages
}

// Search 搜索消息（按关键词）
func (ms *MessageStorage) Search(keyword string) []StoredMessage {
	return ms.SearchByKeywordWithFilter(keyword, "", "")
}

// GetReadStatus 获取消息已读状态
func (ms *MessageStorage) GetReadStatus(msgID string) []string {
	return ms.getReadBy(msgID)
}

// AdvancedSearch 高级搜索
type SearchParams struct {
	Keyword   string
	MsgType   string
	User      string
	StartTime time.Time
	EndTime   time.Time
}

func (ms *MessageStorage) AdvancedSearch(params SearchParams) []StoredMessage {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	query := `SELECT id, type, from_user, to_user, content, timestamp FROM messages WHERE revoked = 0`
	args := []any{}

	if params.Keyword != "" {
		query += " AND content LIKE ?"
		args = append(args, "%"+params.Keyword+"%")
	}
	if params.MsgType != "" {
		query += " AND type = ?"
		args = append(args, params.MsgType)
	}
	if params.User != "" {
		query += " AND (from_user = ? OR to_user = ?)"
		args = append(args, params.User, params.User)
	}
	if !params.StartTime.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, params.StartTime)
	}
	if !params.EndTime.IsZero() {
		query += " AND timestamp <= ?"
		args = append(args, params.EndTime)
	}
	query += " ORDER BY timestamp DESC LIMIT 100"

	rows, _ := ms.db.Query(query, args...)
	defer rows.Close()

	var messages []StoredMessage
	for rows.Next() {
		var msg StoredMessage
		var timestampStr string
		rows.Scan(&msg.ID, &msg.Type, &msg.From, &msg.To, &msg.Content, &timestampStr)
		msg.Timestamp = parseTimestamp(timestampStr)
		messages = append(messages, msg)
	}

	return messages
}

// getReadBy 获取已读用户列表（内部方法）
func (ms *MessageStorage) getReadBy(msgID string) []string {
	rows, _ := ms.db.Query(`SELECT user_id FROM read_status WHERE message_id = ?`, msgID)
	defer rows.Close()

	var users []string
	for rows.Next() {
		var user string
		rows.Scan(&user)
		users = append(users, user)
	}
	return users
}

// parseTimestamp 解析数据库中的时间字符串
func parseTimestamp(s string) time.Time {
	// 尝试多种格式
	formats := []string{
		"2006-01-02 15:04:05",           // 我们存储的格式
		time.RFC3339,                     // SQLite 驱动可能返回的格式: 2026-06-06T11:31:14Z
		"2006-01-02T15:04:05Z",           // 不带时区偏移
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05 -0700 MST",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}

	// 兼容旧数据格式（Go runtime 单调时钟）
	if idx := strings.Index(s, " m="); idx > 0 {
		s = s[:idx]
		for _, format := range formats {
			if t, err := time.Parse(format, s); err == nil {
				return t
			}
		}
	}

	return time.Time{}
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && strings.Contains(s, substr)))
}
