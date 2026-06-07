package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	baseURL := "http://localhost:8080"

	for {
		fmt.Println("\n===== AIM 测试工具 =====")
		fmt.Println("1. 注册用户")
		fmt.Println("2. 登录获取Token")
		fmt.Println("3. 退出")
		fmt.Print("请选择: ")

		var choice int
		fmt.Scanln(&choice)

		switch choice {
		case 1:
			register(baseURL)
		case 2:
			login(baseURL)
		case 3:
			return
		default:
			fmt.Println("无效选择")
		}
	}
}

func register(baseURL string) {
	fmt.Print("用户名: ")
	var username string
	fmt.Scanln(&username)

	fmt.Print("密码: ")
	var password string
	fmt.Scanln(&password)

	data := map[string]string{
		"username": username,
		"password": password,
	}
	jsonData, _ := json.Marshal(data)

	resp, err := http.Post(baseURL+"/register", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("请求失败:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println("响应:", string(body))
}

func login(baseURL string) {
	fmt.Print("用户名: ")
	var username string
	fmt.Scanln(&username)

	fmt.Print("密码: ")
	var password string
	fmt.Scanln(&password)

	data := map[string]string{
		"username": username,
		"password": password,
	}
	jsonData, _ := json.Marshal(data)

	resp, err := http.Post(baseURL+"/login", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("请求失败:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if token, ok := result["token"].(string); ok {
		fmt.Println("\n登录成功！")
		fmt.Println("Token:", token)
		fmt.Println("\n连接WebSocket命令:")
		fmt.Printf("go run test_client.go -token %s\n", token)
	} else {
		fmt.Println("响应:", string(body))
	}
}
