package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	// 创建一个简单的docx文件（docx本质是zip+xml）
	// 先测试解析逻辑

	fmt.Println("=== 测试DOCX解析 ===")

	// 模拟解析过程
	filePath := "C:/Users/YOGA/Desktop/aim/test.txt"

	// 创建测试txt
	content := "这是一个测试文档。AIM系统支持多种文档格式：PDF、MD、DOC、PPT。系统使用Go语言开发，架构清晰。"
	os.WriteFile(filePath, []byte(content), 0644)

	fmt.Println("测试文件已创建:", filePath)
	fmt.Println("内容:", content)
	fmt.Println("\n请用 /kb_file 命令测试上传")
}
