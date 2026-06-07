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
	// 1. 登录
	loginData, _ := json.Marshal(map[string]string{"username": "persisttest", "password": "123456"})
	http.Post("http://localhost:8080/register", "application/json", bytes.NewReader(loginData))

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

	// 3. 接收消息
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			fmt.Println("\n📩 " + string(msg))
		}
	}()

	// 4. 添加知识
	fmt.Println("\n=== 添加知识到知识库 ===")
	conn.WriteMessage(websocket.TextMessage, []byte("/kb_add AIM系统支持知识库持久化，数据会保存到SQLite数据库，重启服务后不会丢失。"))
	time.Sleep(5 * time.Second)

	// 5. 查看知识库信息
	fmt.Println("\n=== 查看知识库信息 ===")
	conn.WriteMessage(websocket.TextMessage, []byte("/kb_info"))
	time.Sleep(2 * time.Second)

	// 6. 查询
	fmt.Println("\n=== 查询知识库 ===")
	conn.WriteMessage(websocket.TextMessage, []byte("/kb_ask 知识库数据会丢失吗？"))
	time.Sleep(5 * time.Second)

	fmt.Println("\n\n✅ 测试完成！")
	fmt.Println("====================================")
	fmt.Println("现在重启服务，再运行一次此测试")
	fmt.Println("如果知识库信息还在，说明持久化成功！")
	fmt.Println("====================================")
}
