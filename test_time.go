//go:build ignore

package main

import (
	"fmt"
	"strings"
	"time"
)

// parseTimestamp 解析数据库中的时间字符串
func parseTimestamp(s string) time.Time {
	// 尝试多种格式
	formats := []string{
		"2006-01-02 15:04:05",           // 我们存储的格式
		time.RFC3339,                     // SQLite 驱动可能返回的格式: 2026-06-06T11:31:14Z
		"2006-01-02T15:04:05Z",           // 不带时区偏移
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05 -0700 MST",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}

	// 兼容旧数据格式（Go runtime 单调时钟）
	if idx := strings.Index(s, " m="); idx > 0 {
		s = s[:idx]
		for _, format := range formats {
			if t, err := time.Parse(format, s); err == nil {
				return t
			}
		}
	}

	return time.Time{}
}

func main() {
	testCases := []string{
		"2026-06-06 11:29:50",                          // 新格式
		"2026-06-06 03:29:42",                          // SQLite datetime('now')
		"2026-06-06T11:31:14Z",                         // RFC3339 格式 (SQLite 驱动返回)
		"2026-06-06T11:33:15+08:00",                    // RFC3339 带时区
		"2026-06-06 11:20:15.123456789 +0800 CST m=+0", // 旧格式
		"",                                             // 空字符串
		"invalid",                                      // 无效格式
	}

	for _, tc := range testCases {
		t := parseTimestamp(tc)
		fmt.Printf("输入: [%s]\n", tc)
		fmt.Printf("  解析结果: %v\n", t)
		fmt.Printf("  Format 15:04:05: %s\n", t.Format("15:04:05"))
		fmt.Println()
	}

	// 测试 time.Now().Format("2006-01-02 15:04:05")
	now := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("当前时间格式化: %s\n", now)
	t := parseTimestamp(now)
	fmt.Printf("  解析后: %v\n", t)
	fmt.Printf("  Format 15:04:05: %s\n", t.Format("15:04:05"))
}
