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

	done := make(chan bool)

	// 读取消息
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				close(done)
				return
			}
			fmt.Printf("%s\n", msg)
		}
	}()

	// 测试基本Bot功能
	fmt.Println("===== 测试基本Bot功能 =====")
	conn.WriteMessage(websocket.TextMessage, []byte("/private Bot 你好"))

	// 等待响应
	time.Sleep(8 * time.Second)

	// 测试候选回复
	fmt.Println("\n===== 测试候选回复功能 =====")
	conn.WriteMessage(websocket.TextMessage, []byte("/ask 什么是Go？"))

	time.Sleep(15 * time.Second)

	// 测试选择候选回复
	fmt.Println("\n===== 测试选择候选回复 =====")
	conn.WriteMessage(websocket.TextMessage, []byte("/use 1"))

	time.Sleep(3 * time.Second)

	fmt.Println("\n===== 测试完成 =====")
}
