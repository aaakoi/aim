package main

import (
	"encoding/json"
	"time"
)

// Message 消息结构体
type Message struct {
	Type    string `json:"type"`    // single(单聊) / broadcast(广播)
	From    string `json:"from"`    // 发送者ID
	To      string `json:"to"`      // 接收者ID（单聊时用）
	Content string `json:"content"` // 消息内容
	Time    int64  `json:"time"`    // 时间戳
	Stream  bool   `json:"stream"`  // 是否流式响应
}

// RevokeNotification 撤回通知
type RevokeNotification struct {
	Action  string `json:"action"`  // "revoke"
	MsgID   string `json:"msgId"`   // 被撤回的消息ID
	From    string `json:"from"`    // 谁撤回的
	To      string `json:"to"`      // 撤回通知发给谁
	Group   string `json:"group"`   // 群名（群聊撤回时）
}

// NewMessage 创建消息
func NewMessage(msgType, from, to, content string) Message {
	return Message{
		Type:    msgType,
		From:    from,
		To:      to,
		Content: content,
		Time:    time.Now().Unix(),
	}
}

// ToBytes 转成JSON字节，用于传输
func (m Message) ToBytes() []byte {
	data, _ := json.Marshal(m)
	return data
}

// ParseMessage 从JSON字节解析成Message
func ParseMessage(data []byte) (Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	return msg, err
}
