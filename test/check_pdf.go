package main

import (
	"fmt"
	"io"
	"os"

	"github.com/ledongthuc/pdf"
)

func main() {
	filePath := "C:/Users/YOGA/Desktop/aim/传输.pdf"

	// 检查文件是否存在
	info, err := os.Stat(filePath)
	if err != nil {
		fmt.Println("文件不存在:", err)
		return
	}
	fmt.Println("文件大小:", info.Size(), "bytes")

	// 打开PDF
	f, r, err := pdf.Open(filePath)
	if err != nil {
		fmt.Println("打开PDF失败:", err)
		return
	}
	defer f.Close()

	// 获取纯文本
	b, err := r.GetPlainText()
	if err != nil {
		fmt.Println("获取文本失败:", err)
		return
	}

	content, _ := io.ReadAll(b)
	fmt.Println("提取文本长度:", len(content))

	if len(content) > 0 {
		maxLen := len(content)
		if maxLen > 500 {
			maxLen = 500
		}
		fmt.Println("\n前", maxLen, "字符:")
		fmt.Println(string(content[:maxLen]))
	} else {
		fmt.Println("PDF可能是扫描版，无法提取文本")
	}
}
