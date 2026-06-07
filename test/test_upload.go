package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

func main() {
	// 创建测试文件
	testContent := []byte("这是一个测试图片")
	os.WriteFile("test_image.jpg", testContent, 0644)
	os.WriteFile("test_file.txt", []byte("这是一个测试文件"), 0644)

	fmt.Println("===== 测试上传功能 =====")

	// 测试上传图片
	fmt.Println("\n1. 上传图片...")
	uploadFile("test_image.jpg")

	// 测试上传文件
	fmt.Println("\n2. 上传文件...")
	uploadFile("test_file.txt")

	fmt.Println("\n===== 上传测试完成 =====")
	fmt.Println("复制上面的 URL，在 test_client 里发送：")
	fmt.Println("/image 用户名 /uploads/xxx.jpg")
}

func uploadFile(filename string) {
	// 读取文件
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("打开文件失败:", err)
		return
	}
	defer file.Close()

	// 创建 multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", filename)
	io.Copy(part, file)
	writer.Close()

	// 发送请求
	req, _ := http.NewRequest("POST", "http://localhost:8080/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("上传失败:", err)
		return
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]string
	json.Unmarshal(respBody, &result)

	fmt.Printf("文件: %s\n", filename)
	fmt.Printf("URL: %s\n", result["url"])
}
