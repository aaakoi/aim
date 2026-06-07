package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	// 注册用户
	users := []string{"owner", "admin_user", "member1", "member2"}
	for _, u := range users {
		registerUser(u, "123456")
	}

	// 登录获取token
	tokens := make(map[string]string)
	for _, u := range users {
		tokens[u] = loginUser(u, "123456")
		if tokens[u] == "" {
			log.Fatalf("登录失败: %s", u)
		}
		fmt.Printf("用户 %s 登录成功\n", u)
	}

	// 创建连接
	ownerConn := connectWS(tokens["owner"])
	defer ownerConn.Close()
	adminConn := connectWS(tokens["admin_user"])
	defer adminConn.Close()
	member1Conn := connectWS(tokens["member1"])
	defer member1Conn.Close()
	member2Conn := connectWS(tokens["member2"])
	defer member2Conn.Close()

	// 启动消息接收
	go readMessages(ownerConn, "owner")
	go readMessages(adminConn, "admin_user")
	go readMessages(member1Conn, "member1")
	go readMessages(member2Conn, "member2")

	time.Sleep(500 * time.Millisecond)

	// 测试1: 创建群组
	fmt.Println("\n=== 测试1: 创建群组 ===")
	sendCommand(ownerConn, "/create testgroup")
	time.Sleep(200 * time.Millisecond)

	// 测试2: 成员加入群
	fmt.Println("\n=== 测试2: 成员加入群 ===")
	sendCommand(adminConn, "/join testgroup")
	sendCommand(member1Conn, "/join testgroup")
	sendCommand(member2Conn, "/join testgroup")
	time.Sleep(300 * time.Millisecond)

	// 测试3: 查看群信息
	fmt.Println("\n=== 测试3: 查看群信息 ===")
	sendCommand(ownerConn, "/groupinfo testgroup")
	time.Sleep(200 * time.Millisecond)

	// 测试4: 设置管理员
	fmt.Println("\n=== 测试4: 设置管理员 ===")
	sendCommand(ownerConn, "/setadmin testgroup admin_user")
	time.Sleep(200 * time.Millisecond)

	// 测试5: 设置群公告
	fmt.Println("\n=== 测试5: 设置群公告 ===")
	sendCommand(ownerConn, "/announce testgroup 这是群公告，请大家遵守群规")
	time.Sleep(200 * time.Millisecond)

	// 测试6: 禁言成员
	fmt.Println("\n=== 测试6: 禁言成员 ===")
	sendCommand(ownerConn, "/mute testgroup member1")
	time.Sleep(200 * time.Millisecond)

	// 测试7: 被禁言成员发送消息（应该失败）
	fmt.Println("\n=== 测试7: 被禁言成员尝试发消息 ===")
	sendCommand(member1Conn, "#testgroup 我被禁言了还能发消息吗？")
	time.Sleep(300 * time.Millisecond)

	// 测试8: 解除禁言
	fmt.Println("\n=== 测试8: 解除禁言 ===")
	sendCommand(ownerConn, "/unmute testgroup member1")
	time.Sleep(200 * time.Millisecond)

	// 测试9: 正常发送群消息
	fmt.Println("\n=== 测试9: 正常发送群消息 ===")
	sendCommand(member1Conn, "#testgroup 现在可以发消息了")
	time.Sleep(300 * time.Millisecond)

	// 测试10: 踢出成员
	fmt.Println("\n=== 测试10: 踢出成员 ===")
	sendCommand(ownerConn, "/kick testgroup member2")
	time.Sleep(200 * time.Millisecond)

	// 测试11: 再次查看群信息
	fmt.Println("\n=== 测试11: 查看更新后的群信息 ===")
	sendCommand(ownerConn, "/groupinfo testgroup")
	time.Sleep(200 * time.Millisecond)

	// 测试12: 非管理员尝试禁言
	fmt.Println("\n=== 测试12: 非管理员尝试禁言（应失败） ===")
	sendCommand(member1Conn, "/mute testgroup admin_user")
	time.Sleep(200 * time.Millisecond)

	fmt.Println("\n=== 所有测试完成 ===")
	time.Sleep(500 * time.Millisecond)
}

func registerUser(username, password string) {
	data := map[string]string{"username": username, "password": password}
	jsonData, _ := json.Marshal(data)
	resp, err := http.Post("http://localhost:8080/register", "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		log.Printf("注册 %s 失败: %v", username, err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	if msg, ok := result["message"]; ok {
		fmt.Printf("注册 %s: %s\n", username, msg)
	}
}

func loginUser(username, password string) string {
	data := map[string]string{"username": username, "password": password}
	jsonData, _ := json.Marshal(data)
	resp, err := http.Post("http://localhost:8080/login", "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	if token, ok := result["token"].(string); ok {
		return token
	}
	return ""
}

func connectWS(token string) *websocket.Conn {
	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws", RawQuery: "token=" + token}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("WebSocket连接失败:", err)
	}
	return conn
}

func readMessages(conn *websocket.Conn, name string) {
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		fmt.Printf("[%s 收到] %s\n", name, string(msg))
	}
}

func sendCommand(conn *websocket.Conn, cmd string) {
	err := conn.WriteMessage(websocket.TextMessage, []byte(cmd))
	if err != nil {
		log.Printf("发送失败: %v", err)
	}
	fmt.Printf("发送命令: %s\n", cmd)
}
