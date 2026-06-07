package main

import (
	"encoding/json"
	"fmt"
	"aim/rag"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	ID   string
	Send chan []byte
	conn *websocket.Conn
	hub  *Hub
}

// StreamMessage 流式消息
type StreamMessage struct {
	Type    string `json:"type"`    // "stream_start", "stream_chunk", "stream_end"
	Content string `json:"content"` // token 内容
	From    string `json:"from"`    // 发送者（Bot ID）
}

// TypingEvent 输入状态事件
type TypingEvent struct {
	From string // 谁在输入
	To   string // 给谁输入
}

// ReadEvent 已读事件
type ReadEvent struct {
	MsgID  string // 消息ID
	Reader string // 谁读了
}

type Hub struct {
	clients           map[*Client]bool     // 所有连接
	userClients       map[string]*Client   // 用户ID -> Client（按ID索引）
	register          chan *Client
	unregister        chan *Client
	broadcast         chan []byte
	typing            chan TypingEvent     // 输入状态
	readNotify        chan ReadEvent       // 已读通知
	mu                sync.RWMutex
	groupManager      *GroupManager
	friendManager     *FriendManager
	botManager        *BotManager          // Bot 管理器
	storage           *MessageStorage
	candidates        map[string][]CandidateReply // 用户ID -> 候选回复列表
	billingManager    *BillingManager      // 计费管理器
	kbManager         *rag.KnowledgeBaseManager // 知识库管理器
	memoryManager     *MemoryManager       // 记忆管理器
	moderationManager *ModerationManager   // 消息审核管理器
}

func NewHub() *Hub {
	// RAG配置
	ragConfig := rag.RAGConfig{
		EmbeddingAPIKey: ZhipuAPIKey,
		ChatAPIKey:      ZhipuAPIKey,
		ChatAPIURL:      ZhipuAPIURL,
		ChunkSize:       500,
		ChunkOverlap:    50,
	}

	hub := &Hub{
		clients:           make(map[*Client]bool),
		userClients:       make(map[string]*Client),
		register:          make(chan *Client, 10),
		unregister:        make(chan *Client, 10),
		broadcast:         make(chan []byte, 100),
		typing:            make(chan TypingEvent, 50),
		readNotify:        make(chan ReadEvent, 50),
		groupManager:      NewGroupManager(),
		friendManager:     NewFriendManager(),
		botManager:        NewBotManager(),
		storage:           NewMessageStorage(),
		candidates:        make(map[string][]CandidateReply),
		billingManager:    NewBillingManager(),
		kbManager:         rag.NewKnowledgeBaseManager(ragConfig),
		memoryManager:     NewMemoryManager(),
		moderationManager: NewModerationManager(),
	}

	// 设置默认用户额度
	hub.billingManager.SetQuota("testuser1", 10.0) // 10元额度

	return hub
}

// SetCandidates 存储用户的候选回复
func (h *Hub) SetCandidates(userID string, candidates []CandidateReply) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.candidates[userID] = candidates
}

// GetCandidates 获取用户的候选回复
func (h *Hub) GetCandidates(userID string) []CandidateReply {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.candidates[userID]
}

// ClearCandidates 清除用户的候选回复
func (h *Hub) ClearCandidates(userID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.candidates, userID)
}

// pushOfflineMessages 推送离线消息给刚上线的用户
func (h *Hub) pushOfflineMessages(client *Client) {
	// 查询该用户的离线消息
	unread := h.storage.GetUnreadByUser(client.ID)
	if len(unread) == 0 {
		return
	}

	// 发送离线消息提示
	client.Send <- []byte(fmt.Sprintf("[系统] 你有 %d 条离线消息", len(unread)))

	// 逐条推送离线消息
	for _, msg := range unread {
		timeStr := msg.Timestamp.Format("15:04:05")
		var displayMsg string
		if msg.Type == "single" {
			displayMsg = fmt.Sprintf("[离线私聊] [%s] %s -> 你: %s (ID: %s)", timeStr, msg.From, msg.Content, msg.ID)
		} else if msg.Type == "group" {
			displayMsg = fmt.Sprintf("[离线群聊] [%s] %s@%s: %s (ID: %s)", timeStr, msg.From, msg.To, msg.Content, msg.ID)
		} else {
			displayMsg = fmt.Sprintf("[离线广播] [%s] %s: %s (ID: %s)", timeStr, msg.From, msg.Content, msg.ID)
		}
		client.Send <- []byte(displayMsg)
	}
}

// 根据用户ID找到Client
func (h *Hub) GetClient(userID string) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.userClients[userID]
}

// GetOnlineUsers 获取所有在线用户ID
func (h *Hub) GetOnlineUsers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	users := make([]string, 0, len(h.userClients))
	for userID := range h.userClients {
		users = append(users, userID)
	}
	return users
}

// IsOnline 检查用户是否在线
func (h *Hub) IsOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, exists := h.userClients[userID]
	return exists
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.userClients[client.ID] = client
			h.mu.Unlock()
			// 指标：在线用户数增加
			IncOnlineUsers()
			fmt.Printf("客户端 %s 已连接，当前在线人数: %d\n", client.ID, len(h.clients))

			// 离线消息推送：用户上线时自动推送离线消息
			go h.pushOfflineMessages(client)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				delete(h.userClients, client.ID)
				close(client.Send)
				// 指标：在线用户数减少
				DecOnlineUsers()
			}
			h.mu.Unlock()
			fmt.Printf("客户端 %s 已断开，当前在线人数: %d\n", client.ID, len(h.clients))

		case typing := <-h.typing:
			// 输入状态通知
			if targetClient := h.GetClient(typing.To); targetClient != nil {
				targetClient.Send <- []byte(fmt.Sprintf("[系统] %s 正在输入...", typing.From))
			}

		case readEvt := <-h.readNotify:
			// 已读通知：告诉发送者，谁读了他的消息
			msg := h.storage.GetByID(readEvt.MsgID)
			if msg != nil {
				if senderClient := h.GetClient(msg.From); senderClient != nil {
					senderClient.Send <- []byte(fmt.Sprintf("[已读] %s 已读消息 [%s]", readEvt.Reader, readEvt.MsgID))
				}
			}

		case message := <-h.broadcast:
			// 解析消息，判断是单聊、群聊还是广播
			msg, err := ParseMessage(message)
			if err != nil {
				continue
			}

			// 消息内容审核
			moderationResult := h.moderationManager.Moderate(msg.Content)
			if !moderationResult.Pass {
				// 审核不通过，通知发送者
				if senderClient := h.GetClient(msg.From); senderClient != nil {
					// 如果是发给 Bot 的消息，需要发送流式结束信号
					if h.botManager.IsBot(msg.To) && msg.Stream {
						endMsg, _ := json.Marshal(StreamMessage{Type: "stream_end", From: msg.To})
						senderClient.Send <- endMsg
					}
					senderClient.Send <- []byte("[审核] 消息发送失败：" + moderationResult.Reason)
				}
				continue // 拦截消息
			}
			// 存储消息，获取消息ID
			msgID := h.storage.Save(msg.Type, msg.From, msg.To, msg.Content)

			// 指标：消息吞吐量
			RecordMessage(msg.Type)

			if msg.Type == "single" {
				// 检查是不是发给 Bot
				if h.botManager.IsBot(msg.To) {
					// 检查额度
					if !h.billingManager.CanUse(msg.From) {
						if senderClient := h.GetClient(msg.From); senderClient != nil {
							senderClient.Send <- []byte("额度不足，请联系管理员充值")
						}
						continue
					}

					// === 记忆能力 ===
					h.memoryManager.SaveMessage(msg.From, "user", msg.Content)
					history := h.memoryManager.GetHistory(msg.From, 10)

					// 判断是否流式响应
					if msg.Stream {
						// 流式响应
						go h.handleStreamMessage(msg, history)
					} else {
						// 普通响应
						reply := h.botManager.HandleBotMessageWithMemory(msg.To, msg.From, msg.Content, history)
						h.memoryManager.SaveMessage(msg.From, "assistant", reply)

						promptTokens, completionTokens, totalTokens := h.botManager.GetLastTokenUsage()
						provider := h.botManager.GetProvider().Name()
						cost := CalculateTokenCost(provider, promptTokens, completionTokens)
						h.billingManager.RecordUsage(msg.From, provider, totalTokens, cost)

						if senderClient := h.GetClient(msg.From); senderClient != nil {
							displayMsg := fmt.Sprintf("[%s]: %s [tokens:%d+%d, %.6f元]",
								msg.To, reply, promptTokens, completionTokens, cost)
							senderClient.Send <- []byte(displayMsg)
						}
					}
				} else {
					// 普通单聊：发给目标用户
					targetClient := h.GetClient(msg.To)
					if targetClient != nil {
						displayMsg := fmt.Sprintf("[%s(私聊)] [%s]: %s", msg.From, msgID, msg.Content)
						select {
						case targetClient.Send <- []byte(displayMsg):
						default:
							go func(c *Client) {
								h.unregister <- c
							}(targetClient)
						}
						// 告诉发送者消息已发送
						if senderClient := h.GetClient(msg.From); senderClient != nil {
							senderClient.Send <- []byte(fmt.Sprintf("[系统] 消息已发送，ID: %s", msgID))
						}
					} else {
						// 目标用户不在线
						if senderClient := h.GetClient(msg.From); senderClient != nil {
							senderClient.Send <- []byte(fmt.Sprintf("[系统] 用户 %s 不在线", msg.To))
						}
					}
				}
			} else if msg.Type == "group" {
				// 群聊：发给群成员
				// 先检查发送者是否被禁言
				if h.groupManager.IsMuted(msg.To, msg.From) {
					if senderClient := h.GetClient(msg.From); senderClient != nil {
						senderClient.Send <- []byte(fmt.Sprintf("你在群 [%s] 已被禁言，无法发送消息", msg.To))
					}
					continue
				}
				members := h.groupManager.GetMembers(msg.To)
				if members != nil {
					displayMsg := fmt.Sprintf("[%s@%s] [%s]: %s", msg.From, msg.To, msgID, msg.Content)
					for _, memberID := range members {
						memberClient := h.GetClient(memberID)
						if memberClient != nil {
							select {
							case memberClient.Send <- []byte(displayMsg):
							default:
								go func(c *Client) {
									h.unregister <- c
								}(memberClient)
							}
						}
					}
				}
			} else {
				// 广播：发给所有人
				h.mu.RLock()
				displayMsg := fmt.Sprintf("[%s] [%s]: %s", msg.From, msgID, msg.Content)
				for client := range h.clients {
					select {
					case client.Send <- []byte(displayMsg):
					default:
						go func(c *Client) {
							h.unregister <- c
						}(client)
					}
				}
				h.mu.RUnlock()
			}
		}
	}
}

// handleStreamMessage 处理流式消息
func (h *Hub) handleStreamMessage(msg Message, history []ChatMessage) {
	senderClient := h.GetClient(msg.From)
	if senderClient == nil {
		return
	}

	// 发送流式开始消息
	startMsg, _ := json.Marshal(StreamMessage{
		Type: "stream_start",
		From: msg.To,
	})
	senderClient.Send <- startMsg

	// 收集完整回复
	var fullReply strings.Builder

	// 调用流式 API
	h.botManager.HandleBotMessageStream(msg.To, msg.From, msg.Content, func(token string, done bool) {
		if done {
			// 流式结束
			endMsg, _ := json.Marshal(StreamMessage{
				Type: "stream_end",
				From: msg.To,
			})
			senderClient.Send <- endMsg

			// 保存完整回复到记忆
			h.memoryManager.SaveMessage(msg.From, "assistant", fullReply.String())
		} else {
			// 发送 token
			fullReply.WriteString(token)
			chunkMsg, _ := json.Marshal(StreamMessage{
				Type:    "stream_chunk",
				Content: token,
				From:    msg.To,
			})
			senderClient.Send <- chunkMsg
		}
	})
}
