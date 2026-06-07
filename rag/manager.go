package rag

import (
	"sync"
)

// KnowledgeBaseManager 知识库管理器（支持多用户私有知识库）
type KnowledgeBaseManager struct {
	bots       map[string]*RAGBot // userID -> RAGBot
	mu         sync.RWMutex
	config     RAGConfig
	dbPath     string // SQLite数据库路径
	useSQLite  bool
}

// NewKnowledgeBaseManager 创建知识库管理器
func NewKnowledgeBaseManager(config RAGConfig) *KnowledgeBaseManager {
	return &KnowledgeBaseManager{
		bots:      make(map[string]*RAGBot),
		config:    config,
		dbPath:    "./messages.db",
		useSQLite: true,
	}
}

// NewKnowledgeBaseManagerWithDB 创建带数据库的知识库管理器
func NewKnowledgeBaseManagerWithDB(config RAGConfig, dbPath string) *KnowledgeBaseManager {
	return &KnowledgeBaseManager{
		bots:      make(map[string]*RAGBot),
		config:    config,
		dbPath:    dbPath,
		useSQLite: true,
	}
}

// GetOrCreateBot 获取或创建用户的知识库
func (m *KnowledgeBaseManager) GetOrCreateBot(userID string) *RAGBot {
	m.mu.RLock()
	bot, exists := m.bots[userID]
	m.mu.RUnlock()

	if exists {
		return bot
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查
	if bot, exists := m.bots[userID]; exists {
		return bot
	}

	// 创建RAG Bot
	bot = NewRAGBot(m.config)

	// 使用SQLite存储
	if m.useSQLite {
		store, err := NewSQLiteVectorStore(m.dbPath, userID)
		if err == nil {
			bot.VectorStore = store
		}
	}

	m.bots[userID] = bot
	return bot
}

// IngestDocument 用户导入文档
func (m *KnowledgeBaseManager) IngestDocument(userID, filePath string) error {
	bot := m.GetOrCreateBot(userID)
	return bot.IngestDocument(filePath)
}

// IngestContent 用户导入文本内容
func (m *KnowledgeBaseManager) IngestContent(userID, name, content string) error {
	bot := m.GetOrCreateBot(userID)
	return bot.IngestContent(name, content)
}

// Query 用户查询知识库
func (m *KnowledgeBaseManager) Query(userID, question string) (string, error) {
	bot := m.GetOrCreateBot(userID)
	return bot.Query(question)
}

// GetKnowledgeBaseInfo 获取用户知识库信息
func (m *KnowledgeBaseManager) GetKnowledgeBaseInfo(userID string) map[string]interface{} {
	bot := m.GetOrCreateBot(userID)
	return bot.GetKnowledgeBaseInfo()
}

// ClearKnowledgeBase 清空用户知识库
func (m *KnowledgeBaseManager) ClearKnowledgeBase(userID string) {
	bot := m.GetOrCreateBot(userID)
	bot.ClearKnowledgeBase()
}

// ListKnowledgeBases 列出所有知识库
func (m *KnowledgeBaseManager) ListKnowledgeBases() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]int)
	for userID, bot := range m.bots {
		info := bot.GetKnowledgeBaseInfo()
		result[userID] = info["total_chunks"].(int)
	}
	return result
}
