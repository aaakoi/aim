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

	// 测试多提供商
	fmt.Println("===== 测试多提供商功能 =====")

	// 查看当前提供商
	conn.WriteMessage(websocket.TextMessage, []byte("/providers"))
	time.Sleep(1 * time.Second)

	// 切换到智谱AI
	fmt.Println("\n--- 切换到智谱AI ---")
	conn.WriteMessage(websocket.TextMessage, []byte("/provider zhipu"))
	time.Sleep(1 * time.Second)

	// 验证切换
	conn.WriteMessage(websocket.TextMessage, []byte("/providers"))
	time.Sleep(1 * time.Second)

	// 测试切换到OpenAI（会提示未配置）
	fmt.Println("\n--- 切换到OpenAI ---")
	conn.WriteMessage(websocket.TextMessage, []byte("/provider openai"))
	time.Sleep(1 * time.Second)

	conn.WriteMessage(websocket.TextMessage, []byte("/providers"))
	time.Sleep(1 * time.Second)

	// 切回智谱AI
	conn.WriteMessage(websocket.TextMessage, []byte("/provider zhipu"))
	time.Sleep(1 * time.Second)

	fmt.Println("\n===== 测试完成 =====")
}
