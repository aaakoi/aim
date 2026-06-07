package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3ODAwNDExODEsInVzZXJuYW1lIjoidGVzdHVzZXIxIn0.vLcASa2X8XElwYVW2uK_T0pUfVeliHs7qFJqdS-tfmI"

	serverURL := url.URL{
		Scheme:   "ws",
		Host:     "localhost:8080",
		Path:     "/ws",
		RawQuery: "token=" + token,
	}

	conn, _, err := websocket.DefaultDialer.Dial(serverURL.String(), nil)
	if err != nil {
		fmt.Println("连接失败:", err)
		return
	}
	defer conn.Close()

	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			fmt.Printf("%s\n", msg)
		}
	}()

	// 测试计费功能
	fmt.Println("===== 测试计费功能 =====")

	// 查看初始额度
	fmt.Println("\n--- 查看初始额度 ---")
	conn.WriteMessage(websocket.TextMessage, []byte("/quota"))
	time.Sleep(1 * time.Second)

	// 发送消息给Bot
	fmt.Println("\n--- 发送消息给Bot ---")
	conn.WriteMessage(websocket.TextMessage, []byte("@Bot 你好，介绍一下自己"))
	time.Sleep(8 * time.Second)

	// 查看使用记录
	fmt.Println("\n--- 查看使用记录 ---")
	conn.WriteMessage(websocket.TextMessage, []byte("/usage"))
	time.Sleep(1 * time.Second)

	// 再次查看额度
	fmt.Println("\n--- 查看剩余额度 ---")
	conn.WriteMessage(websocket.TextMessage, []byte("/quota"))
	time.Sleep(1 * time.Second)

	fmt.Println("\n===== 测试完成 =====")
}
