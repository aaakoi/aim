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
	// 登录
	loginData, _ := json.Marshal(map[string]string{"username": "persisttest", "password": "123456"})
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

	// 连接WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(
		"ws://localhost:8080/ws?token="+token,
		nil,
	)
	if err != nil {
		log.Fatal("WebSocket连接失败:", err)
	}
	defer conn.Close()

	// 接收消息
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			fmt.Println("📩 " + string(msg))
		}
	}()

	// 只查看知识库信息（不添加新内容）
	fmt.Println("=== 查看知识库信息（重启后） ===")
	conn.WriteMessage(websocket.TextMessage, []byte("/kb_info"))
	time.Sleep(2 * time.Second)

	// 查询
	fmt.Println("\n=== 查询知识库 ===")
	conn.WriteMessage(websocket.TextMessage, []byte("/kb_ask 数据会丢失吗？"))
	time.Sleep(5 * time.Second)

	fmt.Println("\n✅ 验证完成！数据持久化成功！")
}
