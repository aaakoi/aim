package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"

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

	// 读取消息
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			fmt.Printf("%s\n", msg)
		}
	}()

	fmt.Println("已连接，输入消息测试候选回复功能")
	fmt.Println("命令: /ask 问题")
	fmt.Println("命令: /use N 选择第N条回复")
	fmt.Println("按 Ctrl+C 退出")

	reader := bufio.NewReader(os.Stdin)
	for {
		input, _ := reader.ReadString('\n')
		conn.WriteMessage(websocket.TextMessage, []byte(input[:len(input)-1]))
	}
}
