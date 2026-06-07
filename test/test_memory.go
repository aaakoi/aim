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
	loginData, _ := json.Marshal(map[string]string{"username": "memorytest", "password": "123456"})
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

	// 4. 第一次对话：告诉Bot名字
	fmt.Println("\n=== 第一次对话：告诉Bot名字 ===")
	conn.WriteMessage(websocket.TextMessage, []byte("@Bot 我叫小明"))
	time.Sleep(5 * time.Second)

	// 5. 查看记忆
	fmt.Println("\n=== 查看记忆 ===")
	conn.WriteMessage(websocket.TextMessage, []byte("/memory"))
	time.Sleep(2 * time.Second)

	// 6. 第二次对话：问Bot我的名字
	fmt.Println("\n=== 第二次对话：问我的名字 ===")
	conn.WriteMessage(websocket.TextMessage, []byte("@Bot 我叫什么名字？"))
	time.Sleep(5 * time.Second)

	// 7. 再看记忆
	fmt.Println("\n=== 再看记忆 ===")
	conn.WriteMessage(websocket.TextMessage, []byte("/memory"))
	time.Sleep(2 * time.Second)

	fmt.Println("\n✅ 记忆测试完成！")
}
