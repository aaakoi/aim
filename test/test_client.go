package main

import (
	"bufio"
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"

	"github.com/gorilla/websocket"
)

// 测试客户端：
// go run test_client.go -token=你的token
// 或者（旧方式，不推荐）：go run test_client.go -user=张三

func main() {
	token := flag.String("token", "", "登录Token")
	user := flag.String("user", "", "用户名（旧方式，不推荐）")
	flag.Parse()

	// 构建连接地址
	serverURL := url.URL{
		Scheme: "ws",
		Host:   "localhost:8080",
		Path:   "/ws",
	}

	// 优先使用 Token
	if *token != "" {
		serverURL.RawQuery = "token=" + *token
	} else if *user != "" {
		// 旧方式（不安全，仅用于测试）
		serverURL.RawQuery = "user_id=" + *user
	} else {
		log.Fatal("请提供 -token 或 -user 参数")
	}

	log.Printf("正在连接: %s", serverURL.String())

	// 连接服务器
	conn, _, err := websocket.DefaultDialer.Dial(serverURL.String(), nil)
	if err != nil {
		log.Fatal("连接失败:", err)
	}
	defer conn.Close()

	log.Println("连接成功！输入消息按回车发送，按 Ctrl+C 退出")

	// 启动一个协程读取服务器消息
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("读取消息出错:", err)
				return
			}
			log.Printf("收到消息: %s", message)
		}
	}()

	// 从终端读取用户输入，发送给服务器
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			input, err := reader.ReadString('\n')
			if err != nil {
				continue
			}
			// 去掉换行符
			input = input[:len(input)-1]
			if input == "" {
				continue
			}
			err = conn.WriteMessage(websocket.TextMessage, []byte(input))
			if err != nil {
				log.Println("发送失败:", err)
				return
			}
			log.Printf("已发送: %s", input)
		}
	}()

	// 等待中断信号
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt
	log.Println("断开连接...")
}
