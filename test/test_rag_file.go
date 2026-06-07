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
	registerData, _ := json.Marshal(map[string]string{"username": "ragtest2", "password": "123456"})
	http.Post("http://localhost:8080/register", "application/json", bytes.NewReader(registerData))

	loginData, _ := json.Marshal(map[string]string{"username": "ragtest2", "password": "123456"})
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

	// 4. 测试添加txt文件
	fmt.Println("\n=== 测试: 添加txt文件到知识库 ===")
	conn.WriteMessage(websocket.TextMessage, []byte("/kb_file C:/Users/YOGA/Desktop/aim/test_doc.txt"))
	time.Sleep(5 * time.Second)

	// 5. 查看知识库信息
	fmt.Println("\n=== 查看知识库信息 ===")
	conn.WriteMessage(websocket.TextMessage, []byte("/kb_info"))
	time.Sleep(2 * time.Second)

	// 6. 查询知识库
	fmt.Println("\n=== 查询知识库 ===")
	conn.WriteMessage(websocket.TextMessage, []byte("/kb_ask AIM系统是用什么语言开发的？"))
	time.Sleep(5 * time.Second)

	fmt.Println("\n✅ 测试完成！")
}
