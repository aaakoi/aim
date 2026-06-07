package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	// 1. 注册并登录
	registerData, _ := json.Marshal(map[string]string{"username": "ragtest", "password": "123456"})
	http.Post("http://localhost:8080/register", "application/json", bytes.NewReader(registerData))

	loginData, _ := json.Marshal(map[string]string{"username": "ragtest", "password": "123456"})
	resp, _ := http.Post("http://localhost:8080/login", "application/json", bytes.NewReader(loginData))
	var loginResp struct {
		Token string `json:"token"`
	}
	json.NewDecoder(resp.Body).Decode(&loginResp)
	resp.Body.Close()

	token := loginResp.Token
	if token == "" {
		log.Fatal("登录失败")
	}
	fmt.Println("✅ Token获取成功")

	// 2. 连接WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(
		"ws://localhost:8080/ws?token="+token,
		nil,
	)
	if err != nil {
		log.Fatal("WebSocket连接失败:", err)
	}
	defer conn.Close()
	fmt.Println("✅ WebSocket连接成功")

	// 3. 接收消息的goroutine
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			fmt.Println("\n📩 " + string(msg))
		}
	}()

	// 4. 添加知识到知识库
	fmt.Println("\n=== 测试1: 添加知识到知识库 ===")
	conn.WriteMessage(websocket.TextMessage, []byte("/kb_add AIM系统是一个即时通讯系统，支持单聊、群聊、机器人对话等功能。它使用Go语言开发，采用WebSocket实现实时通信。"))
	time.Sleep(3 * time.Second)

	// 5. 添加更多知识
	fmt.Println("\n=== 测试2: 添加更多知识 ===")
	conn.WriteMessage(websocket.TextMessage, []byte("/kb_add 系统支持多种AI模型，包括智谱GLM和DeepSeek。用户可以通过Bot进行AI对话，系统会自动计费。"))
	time.Sleep(3 * time.Second)

	// 6. 查看知识库信息
	fmt.Println("\n=== 测试3: 查看知识库信息 ===")
	conn.WriteMessage(websocket.TextMessage, []byte("/kb_info"))
	time.Sleep(2 * time.Second)

	// 7. 查询知识库
	fmt.Println("\n=== 测试4: 查询知识库 ===")
	conn.WriteMessage(websocket.TextMessage, []byte("/kb_ask AIM系统是用什么语言开发的？"))
	time.Sleep(5 * time.Second)

	fmt.Println("\n✅ RAG测试完成！")
}
