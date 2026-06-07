package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	token1 := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3ODAwNDExODEsInVzZXJuYW1lIjoidGVzdHVzZXIxIn0.vLcASa2X8XElwYVW2uK_T0pUfVeliHs7qFJqdS-tfmI"

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

	// Read messages
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			fmt.Printf("%s\n", msg)
		}
	}()

	// Request history
	fmt.Println("===== 测试 /history 命令 =====")
	err = conn.WriteMessage(websocket.TextMessage, []byte("/history"))
	if err != nil {
		fmt.Println("发送失败:", err)
		return
	}

	time.Sleep(1 * time.Second)
	fmt.Println("\n===== 历史消息测试完成 =====")
}
