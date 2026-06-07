package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// 语言代码映射
var languageNames = map[string]string{
	"en": "英文",
	"zh": "中文",
	"ja": "日文",
	"ko": "韩文",
	"fr": "法文",
	"de": "德文",
	"es": "西班牙文",
	"ru": "俄文",
	"pt": "葡萄牙文",
	"it": "意大利文",
}

// TranslateMessage 翻译消息
// msgID: 消息ID
// targetLang: 目标语言代码 (en, ja, ko等)
func (h *Hub) TranslateMessage(msgID, targetLang string) string {
	// 获取原消息
	msg := h.storage.GetByID(msgID)
	if msg == nil {
		return "消息不存在"
	}
	if msg.Revoked {
		return "消息已被撤回，无法翻译"
	}

	// 获取语言名称
	langName := languageNames[targetLang]
	if langName == "" {
		langName = targetLang
	}

	// 构建翻译prompt
	prompt := fmt.Sprintf("请将以下内容翻译成%s，只返回翻译结果，不要解释：\n\n%s", langName, msg.Content)

	// 调用智谱API翻译
	result, err := callTranslateAPI(prompt)
	if err != nil {
		return fmt.Sprintf("翻译失败: %v", err)
	}

	return fmt.Sprintf("[翻译 %s] %s", langName, result)
}

// callTranslateAPI 调用智谱API进行翻译
func callTranslateAPI(prompt string) (string, error) {
	if ZhipuAPIKey == "" {
		return "", fmt.Errorf("未配置智谱API Key")
	}

	// 构建请求
	reqBody := map[string]any{
		"model": "glm-4-flash",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	jsonData, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", ZhipuAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ZhipuAPIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// 解析响应
	var zhipuResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &zhipuResp); err != nil {
		return "", err
	}

	if zhipuResp.Error != nil {
		return "", fmt.Errorf("API错误: %s", zhipuResp.Error.Message)
	}

	if len(zhipuResp.Choices) > 0 {
		return strings.TrimSpace(zhipuResp.Choices[0].Message.Content), nil
	}

	return "", fmt.Errorf("无翻译结果")
}
