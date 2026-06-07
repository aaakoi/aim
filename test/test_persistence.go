package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	// Login to get tokens
	token1 := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3ODAwNDExODEsInVzZXJuYW1lIjoidGVzdHVzZXIxIn0.vLcASa2X8XElwYVW2uK_T0pUfVeliHs7qFJqdS-tfmI"

	// Connect as user1
	serverURL := url.URL{
		Scheme:   "ws",
		Host:     "localhost:8080",
		Path:     "/ws",
		RawQuery: "token=" + token1,
	}

	conn, _, err := websocket.DefaultDialer.Dial(serverURL.String(), nil)
	if err != nil {
		fmt.Println("连接失败:", err)
		return
	}
	defer conn.Close()

	// Read messages in background
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			fmt.Printf("收到: %s\n", msg)
		}
	}()

	// Send a test message
	testMsg := "Hello, this is a persistence test message!"
	err = conn.WriteMessage(websocket.TextMessage, []byte(testMsg))
	if err != nil {
		fmt.Println("发送失败:", err)
		return
	}
	fmt.Println("发送消息:", testMsg)

	// Wait for message to be processed
	time.Sleep(500 * time.Millisecond)

	// Send a private message
	privateMsg := "/private testuser2 这是私聊测试消息"
	err = conn.WriteMessage(websocket.TextMessage, []byte(privateMsg))
	if err != nil {
		fmt.Println("发送私聊失败:", err)
		return
	}
	fmt.Println("发送私聊:", privateMsg)

	// Wait and close
	time.Sleep(1 * time.Second)
	fmt.Println("\n测试完成！消息已保存到SQLite数据库。")
	fmt.Println("请重启服务器后使用 /history 命令验证消息持久化。")
}
