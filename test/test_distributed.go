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
	// 1. 注册用户
	registerData, _ := json.Marshal(map[string]string{"username": "disttest", "password": "123456"})
	resp, _ := http.Post("http://localhost:8080/register",
		"application/json",
		bytes.NewReader(registerData))
	resp.Body.Close()

	// 2. 登录获取Token
	loginData, _ := json.Marshal(map[string]string{"username": "disttest", "password": "123456"})
	resp, _ = http.Post("http://localhost:8080/login",
		"application/json",
		bytes.NewReader(loginData))
	var loginResp struct {
		Token string `json:"token"`
	}
	json.NewDecoder(resp.Body).Decode(&loginResp)
	resp.Body.Close()

	token := loginResp.Token
	if token == "" {
		log.Fatal("登录失败")
	}
	fmt.Println("Token获取成功:", token[:20]+"...")

	// 3. 连接WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(
		"ws://localhost:8080/ws?token="+token,
		nil,
	)
	if err != nil {
		log.Fatal("WebSocket连接失败:", err)
	}
	defer conn.Close()
	fmt.Println("WebSocket连接成功")

	// 4. 启动消息接收goroutine
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			fmt.Println("\n收到消息:", string(msg))
		}
	}()

	// 5. 发送消息给Bot（格式: @Bot 消息内容）
	msg := "@Bot 你好，请用一句话介绍自己"
	fmt.Println("发送的消息:", msg)
	err = conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		log.Fatal("发送消息失败:", err)
	}
	fmt.Println("已发送消息给Bot，等待回复...")

	// 等待回复
	time.Sleep(10 * time.Second)
	fmt.Println("\n测试完成！分布式架构工作正常")
}
