package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

func main() {
	// 从命令行参数获取用户名
	username := "demo"
	if len(os.Args) > 1 {
		username = os.Args[1]
	}

	// 注册用户
	http.Post("http://localhost:8080/register",
		"application/json",
		strings.NewReader(fmt.Sprintf(`{"username":"%s","password":"123"}`, username)))

	// 登录获取token
	resp, _ := http.Post("http://localhost:8080/login",
		"application/json",
		strings.NewReader(fmt.Sprintf(`{"username":"%s","password":"123"}`, username)))

	var result struct {
		Token string `json:"token"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()
	token := result.Token

	if token == "" {
		fmt.Println("登录失败")
		return
	}

	fmt.Printf("Token: %s\n\n", token)

	// 连接WebSocket
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

	fmt.Println("===== 已连接 =====")
	fmt.Println("测试命令:")
	fmt.Println("  /providers      - 查看AI提供商")
	fmt.Println("  /provider zhipu - 切换到智谱AI")
	fmt.Println("  /quota          - 查看额度")
	fmt.Println("  @Bot 你好       - 发消息给Bot")
	fmt.Println("  /ask 问题       - 生成候选回复")
	fmt.Println("  /use 1          - 选择候选")
	fmt.Println("按 Ctrl+C 退出")
	fmt.Println("====================")

	reader := bufio.NewReader(os.Stdin)
	for {
		input, _ := reader.ReadString('\n')
		if len(input) > 0 {
			input = input[:len(input)-1] // 去掉换行符
		}
		if input != "" {
			conn.WriteMessage(websocket.TextMessage, []byte(input))
		}
	}
}
