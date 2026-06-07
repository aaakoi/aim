package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// ReadPump 从WebSocket连接读取消息
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		log.Printf("收到来自 %s 的消息: %s", c.ID, message)

		// 尝试解析 JSON 格式消息
		input := strings.TrimSpace(string(message))
		var msg Message

		// 检查是否是 JSON 格式
		if strings.HasPrefix(input, "{") {
			if err := json.Unmarshal(message, &msg); err == nil {
				// JSON 消息，直接转发
				if msg.From == "" {
					msg.From = c.ID
				}
				c.hub.broadcast <- msg.ToBytes()
				continue
			}
		}

		if strings.HasPrefix(input, "/create ") {
			// 创建群组命令: /create 技术群
			groupName := strings.TrimPrefix(input, "/create ")
			groupName = strings.TrimSpace(groupName)
			if c.hub.groupManager.CreateGroup(groupName, c.ID) != nil {
				c.Send <- []byte(fmt.Sprintf("群组 [%s] 创建成功", groupName))
			} else {
				c.Send <- []byte(fmt.Sprintf("群组 [%s] 已存在", groupName))
			}
			continue
		} else if strings.HasPrefix(input, "/join ") {
			// 加入群组命令: /join 技术群
			groupName := strings.TrimPrefix(input, "/join ")
			groupName = strings.TrimSpace(groupName)
			if c.hub.groupManager.JoinGroup(groupName, c.ID) {
				c.Send <- []byte(fmt.Sprintf("已加入群组 [%s]", groupName))
			} else {
				c.Send <- []byte(fmt.Sprintf("群组 [%s] 不存在", groupName))
			}
			continue
		} else if input == "/online" {
			// 查看在线用户
			users := c.hub.GetOnlineUsers()
			onlineMsg := fmt.Sprintf("当前在线用户 (%d人): %s", len(users), strings.Join(users, ", "))
			c.Send <- []byte(onlineMsg)
			continue
		} else if input == "/history" {
			// 查看最近20条消息
			messages := c.hub.storage.GetRecent(20)
			if len(messages) == 0 {
				c.Send <- []byte("暂无历史消息")
			} else {
				var history strings.Builder
				history.WriteString("===== 历史消息 =====\n")
				for i := len(messages) - 1; i >= 0; i-- {
					m := messages[i]
					timeStr := m.Timestamp.Format("15:04:05")
						content := m.Content
						if m.Revoked {
							content = "[已撤回]"
						}
					switch m.Type {
					case "single":
						history.WriteString(fmt.Sprintf("[%s] %s -> %s: %s\n", timeStr, m.From, m.To, content))
					case "group":
						history.WriteString(fmt.Sprintf("[%s] %s@%s: %s\n", timeStr, m.From, m.To, content))
					default:
						history.WriteString(fmt.Sprintf("[%s] %s: %s\n", timeStr, m.From, content))
					}
				}
				c.Send <- []byte(history.String())
			}
			continue
		} else if strings.HasPrefix(input, "/add ") {
			// 添加好友: /add 李四
			target := strings.TrimPrefix(input, "/add ")
			target = strings.TrimSpace(target)
			if target == "" {
				c.Send <- []byte("请指定用户名")
			} else if target == c.ID {
				c.Send <- []byte("不能加自己为好友")
			} else if c.hub.friendManager.IsFriend(c.ID, target) {
				c.Send <- []byte(fmt.Sprintf("[%s] 已经是你的好友", target))
			} else {
				if c.hub.friendManager.AddRequest(c.ID, target) {
					c.Send <- []byte(fmt.Sprintf("已发送好友请求给 [%s]", target))
					// 如果对方在线，通知他
					if targetClient := c.hub.GetClient(target); targetClient != nil {
						targetClient.Send <- []byte(fmt.Sprintf("[%s] 请求加你为好友，输入 /accept %s 接受", c.ID, c.ID))
					}
				} else {
					c.Send <- []byte(fmt.Sprintf("已经发送过请求给 [%s]", target))
				}
			}
			continue
		} else if strings.HasPrefix(input, "/accept ") {
			// 接受好友: /accept 张三
			target := strings.TrimPrefix(input, "/accept ")
			target = strings.TrimSpace(target)
			if c.hub.friendManager.AcceptRequest(c.ID, target) {
				c.Send <- []byte(fmt.Sprintf("你已添加 [%s] 为好友", target))
				// 通知对方
				if targetClient := c.hub.GetClient(target); targetClient != nil {
					targetClient.Send <- []byte(fmt.Sprintf("[%s] 接受了你的好友请求", c.ID))
				}
			} else {
				c.Send <- []byte(fmt.Sprintf("没有来自 [%s] 的好友请求", target))
			}
			continue
		} else if strings.HasPrefix(input, "/remove ") {
			// 删除好友: /remove 李四
			target := strings.TrimPrefix(input, "/remove ")
			target = strings.TrimSpace(target)
			if c.hub.friendManager.RemoveFriend(c.ID, target) {
				c.Send <- []byte(fmt.Sprintf("已删除好友 [%s]", target))
			} else {
				c.Send <- []byte(fmt.Sprintf("[%s] 不是你的好友", target))
			}
			continue
		} else if input == "/friends" {
			// 查看好友列表
			friends := c.hub.friendManager.GetFriends(c.ID)
			requests := c.hub.friendManager.GetRequests(c.ID)

			var result strings.Builder
			result.WriteString("===== 好友列表 =====\n")
			if len(friends) == 0 {
				result.WriteString("暂无好友\n")
			} else {
				for _, f := range friends {
					online := "离线"
					if c.hub.IsOnline(f) {
						online = "在线"
					}
					result.WriteString(fmt.Sprintf("  %s (%s)\n", f, online))
				}
			}

			if len(requests) > 0 {
				result.WriteString("\n===== 好友请求 =====\n")
				for _, r := range requests {
					result.WriteString(fmt.Sprintf("  %s 请求加你为好友\n", r))
				}
				result.WriteString("输入 /accept 用户名 接受请求\n")
			}

			c.Send <- []byte(result.String())
			continue
		} else if strings.HasPrefix(input, "/remark ") {
			// 设置好友备注: /remark 用户名 备注名
			args := strings.TrimPrefix(input, "/remark ")
			parts := strings.SplitN(args, " ", 2)
			if len(parts) < 2 {
				c.Send <- []byte("用法: /remark 用户名 备注名")
			} else {
				friendID, remark := parts[0], parts[1]
				if c.hub.friendManager.SetRemark(c.ID, friendID, remark) {
					c.Send <- []byte(fmt.Sprintf("已设置 [%s] 的备注为 [%s]", friendID, remark))
				} else {
					c.Send <- []byte(fmt.Sprintf("设置失败，[%s] 不是你的好友", friendID))
				}
			}
			continue
		} else if strings.HasPrefix(input, "/setgroup ") {
			// 设置好友分组: /setgroup 用户名 分组名
			args := strings.TrimPrefix(input, "/setgroup ")
			parts := strings.SplitN(args, " ", 2)
			if len(parts) < 2 {
				c.Send <- []byte("用法: /setgroup 用户名 分组名")
			} else {
				friendID, group := parts[0], parts[1]
				if c.hub.friendManager.SetGroup(c.ID, friendID, group) {
					c.Send <- []byte(fmt.Sprintf("已将 [%s] 移入分组 [%s]", friendID, group))
				} else {
					c.Send <- []byte(fmt.Sprintf("设置失败，[%s] 不是你的好友", friendID))
				}
			}
			continue
		} else if strings.HasPrefix(input, "/friendinfo ") {
			// 查看好友详情: /friendinfo 用户名
			friendID := strings.TrimPrefix(input, "/friendinfo ")
			friendID = strings.TrimSpace(friendID)
			if !c.hub.friendManager.IsFriend(c.ID, friendID) {
				c.Send <- []byte(fmt.Sprintf("[%s] 不是你的好友", friendID))
			} else {
				remark := c.hub.friendManager.GetRemark(c.ID, friendID)
				group := c.hub.friendManager.GetGroup(c.ID, friendID)
				online := "离线"
				if c.hub.IsOnline(friendID) {
					online = "在线"
				}
				var info strings.Builder
				info.WriteString(fmt.Sprintf("===== 好友 [%s] =====\n", friendID))
				info.WriteString(fmt.Sprintf("状态: %s\n", online))
				if remark != "" {
					info.WriteString(fmt.Sprintf("备注: %s\n", remark))
				}
				if group != "" {
					info.WriteString(fmt.Sprintf("分组: %s\n", group))
				}
				c.Send <- []byte(info.String())
			}
			continue
		} else if input == "/groups" {
			// 查看所有分组
			groups := c.hub.friendManager.GetAllGroups(c.ID)
			if len(groups) == 0 {
				c.Send <- []byte("暂无分组")
			} else {
				var result strings.Builder
				result.WriteString("===== 好友分组 =====\n")
				for _, g := range groups {
					friends := c.hub.friendManager.GetFriendsByGroup(c.ID, g)
					result.WriteString(fmt.Sprintf("\n[%s] (%d人)\n", g, len(friends)))
					for _, f := range friends {
						displayName := f.UserID
						if f.Remark != "" {
							displayName = f.Remark + "(" + f.UserID + ")"
						}
						online := "离线"
						if c.hub.IsOnline(f.UserID) {
							online = "在线"
						}
						result.WriteString(fmt.Sprintf("  %s [%s]\n", displayName, online))
					}
				}
				c.Send <- []byte(result.String())
			}
			continue
		} else if strings.HasPrefix(input, "/groupmem ") {
			// 查看分组内的好友: /groupmem 分组名
			group := strings.TrimPrefix(input, "/groupmem ")
			group = strings.TrimSpace(group)
			friends := c.hub.friendManager.GetFriendsByGroup(c.ID, group)
			if len(friends) == 0 {
				c.Send <- []byte(fmt.Sprintf("分组 [%s] 为空或不存在", group))
			} else {
				var result strings.Builder
				result.WriteString(fmt.Sprintf("===== 分组 [%s] (%d人) =====\n", group, len(friends)))
				for _, f := range friends {
					displayName := f.UserID
					if f.Remark != "" {
						displayName = f.Remark + "(" + f.UserID + ")"
					}
					online := "离线"
					if c.hub.IsOnline(f.UserID) {
						online = "在线"
					}
					result.WriteString(fmt.Sprintf("  %s [%s]\n", displayName, online))
				}
				c.Send <- []byte(result.String())
			}
			continue
		} else if input == "/summary" {
			// 总结最近50条消息
			messages := c.hub.storage.GetRecent(50)
			c.Send <- []byte("正在生成总结...")
			summary := c.hub.botManager.SummarizeMessages(messages)
			c.Send <- []byte("===== 消息总结 =====\n" + summary)
			continue
		} else if input == "/todo" {
			// 提取待办事项
			messages := c.hub.storage.GetRecent(50)
			c.Send <- []byte("正在提取待办事项...")
			todos := c.hub.botManager.ExtractTodos(messages)
			c.Send <- []byte("===== 待办事项 =====\n" + todos)
			continue
		} else if strings.HasPrefix(input, "/ask ") {
			// 请求AI生成多个候选回复: /ask 问题
			question := strings.TrimPrefix(input, "/ask ")

			// 检查额度
			if !c.hub.billingManager.CanUse(c.ID) {
				c.Send <- []byte("额度不足，请使用 /setquota 设置额度")
				continue
			}

			c.Send <- []byte("正在生成候选回复...")
			candidates := c.hub.botManager.GenerateCandidates(c.ID, question, 3)

			// 记录计费
			provider := c.hub.botManager.GetProvider().Name()
			for range candidates {
				promptTokens, completionTokens, totalTokens := c.hub.botManager.GetLastTokenUsage()
				cost := CalculateTokenCost(provider, promptTokens, completionTokens)
				c.hub.billingManager.RecordUsage(c.ID, provider, totalTokens, cost)
			}

			c.hub.SetCandidates(c.ID, candidates)
			var result strings.Builder
			result.WriteString("===== 候选回复 =====\n")
			for _, cand := range candidates {
				result.WriteString(fmt.Sprintf("[%d] %s\n", cand.Index, cand.Content))
			}
			result.WriteString("使用 /use N 选择第N条回复发送")
			c.Send <- []byte(result.String())
			continue
		} else if strings.HasPrefix(input, "/use ") {
			// 选择候选回复发送: /use 1 @用户名 或 /use 1 (广播)
			args := strings.TrimPrefix(input, "/use ")
			args = strings.TrimSpace(args)
			parts := strings.Fields(args)
			if len(parts) == 0 {
				c.Send <- []byte("用法: /use N @用户名 或 /use N (广播)")
				continue
			}
			var index int
			fmt.Sscanf(parts[0], "%d", &index)
			var target string
			if len(parts) > 1 && strings.HasPrefix(parts[1], "@") {
				target = strings.TrimPrefix(parts[1], "@")
			}

			candidates := c.hub.GetCandidates(c.ID)
			if len(candidates) == 0 {
				c.Send <- []byte("没有可用的候选回复，请先用 /ask 生成")
				continue
			}
			found := false
			for _, cand := range candidates {
				if cand.Index == index {
					if target != "" {
						// 发送给指定用户
						msg = NewMessage("single", c.ID, target, cand.Content)
						c.hub.broadcast <- msg.ToBytes()
						c.Send <- []byte(fmt.Sprintf("已选择回复 [%d] 发送给 %s: %s", index, target, cand.Content))
					} else {
						// 广播发送
						msg = NewMessage("broadcast", c.ID, "", cand.Content)
						c.hub.broadcast <- msg.ToBytes()
						c.Send <- []byte(fmt.Sprintf("已选择回复 [%d] 广播: %s", index, cand.Content))
					}
					c.hub.ClearCandidates(c.ID)
					found = true
					break
				}
			}
			if !found {
				c.Send <- []byte(fmt.Sprintf("无效的序号 %d，请重新选择", index))
			}
			continue
		} else if strings.HasPrefix(input, "/provider ") {
			// 切换AI提供商: /provider zhipu 或 /provider deepseek
			provider := strings.TrimPrefix(input, "/provider ")
			provider = strings.TrimSpace(provider)
			switch provider {
			case "zhipu":
				c.hub.botManager.SetProvider(&ZhipuProvider{})
				c.Send <- []byte("已切换到智谱AI")
			case "deepseek":
				c.hub.botManager.SetProvider(&DeepSeekProvider{})
				c.Send <- []byte("已切换到DeepSeek")
			default:
				c.Send <- []byte("不支持的提供商，可选: zhipu, deepseek")
			}
			continue
		} else if input == "/providers" {
			// 查看当前AI提供商
			c.Send <- []byte(fmt.Sprintf("当前AI提供商: %s", c.hub.botManager.GetProvider().Name()))
			c.Send <- []byte("可用提供商: zhipu, deepseek")
			continue
		} else if input == "/quota" {
			// 查看额度
			stats := c.hub.billingManager.GetUserStats(c.ID)
			c.Send <- []byte("===== 账户额度 =====")
			c.Send <- []byte(stats)
			continue
		} else if strings.HasPrefix(input, "/setquota ") {
			// 设置额度（管理员功能）
			args := strings.TrimPrefix(input, "/setquota ")
			parts := strings.Fields(args)
			if len(parts) < 2 {
				c.Send <- []byte("用法: /setquota 用户名 额度")
			} else {
				var quota float64
				fmt.Sscanf(parts[1], "%f", &quota)
				c.hub.billingManager.SetQuota(parts[0], quota)
				c.Send <- []byte(fmt.Sprintf("已为 %s 设置额度: %.2f元", parts[0], quota))
			}
			continue
		} else if input == "/usage" {
			// 查看使用历史
			records := c.hub.billingManager.GetUsageHistory(c.ID, 10)
			c.Send <- []byte("===== 最近使用记录 =====")
			if len(records) == 0 {
				c.Send <- []byte("暂无使用记录")
			} else {
				for _, r := range records {
					c.Send <- []byte(fmt.Sprintf("[%s] %s - %.4f元", r.Timestamp.Format("01-02 15:04"), r.Provider, r.Cost))
				}
			}
			continue
		} else if strings.HasPrefix(input, "/kb_add ") {
			// 添加知识到知识库: /kb_add 内容
			content := strings.TrimPrefix(input, "/kb_add ")
			c.Send <- []byte("正在添加知识到知识库...")
			err := c.hub.kbManager.IngestContent(c.ID, "用户输入", content)
			if err != nil {
				c.Send <- []byte(fmt.Sprintf("添加失败: %v", err))
			} else {
				c.Send <- []byte("知识已添加到知识库！")
			}
			continue
		} else if strings.HasPrefix(input, "/kb_file ") {
			// 添加文件到知识库: /kb_file 文件路径
			filePath := strings.TrimPrefix(input, "/kb_file ")
			c.Send <- []byte("正在解析并添加文档...")
			err := c.hub.kbManager.IngestDocument(c.ID, filePath)
			if err != nil {
				c.Send <- []byte(fmt.Sprintf("添加失败: %v", err))
			} else {
				c.Send <- []byte(fmt.Sprintf("文档 %s 已添加到知识库！", filePath))
			}
			continue
		} else if strings.HasPrefix(input, "/kb_ask ") {
			// 查询知识库: /kb_ask 问题
			question := strings.TrimPrefix(input, "/kb_ask ")
			c.Send <- []byte("正在查询知识库...")
			answer, err := c.hub.kbManager.Query(c.ID, question)
			if err != nil {
				c.Send <- []byte(fmt.Sprintf("查询失败: %v", err))
			} else {
				c.Send <- []byte("===== 知识库回答 =====")
				c.Send <- []byte(answer)
			}
			continue
		} else if input == "/kb_info" {
			// 查看知识库信息
			info := c.hub.kbManager.GetKnowledgeBaseInfo(c.ID)
			c.Send <- []byte("===== 知识库信息 =====")
			c.Send <- []byte(fmt.Sprintf("文档块数量: %d", info["total_chunks"]))
			c.Send <- []byte(fmt.Sprintf("文档数量: %d", info["total_docs"]))
			continue
		} else if input == "/kb_clear" {
			// 清空知识库
			c.hub.kbManager.ClearKnowledgeBase(c.ID)
			c.Send <- []byte("知识库已清空")
			continue
		} else if input == "/memory" {
			// 查看记忆数量
			count := c.hub.memoryManager.GetHistoryCount(c.ID)
			c.Send <- []byte("===== 记忆信息 =====")
			c.Send <- []byte(fmt.Sprintf("历史对话条数: %d", count))
			continue
		} else if input == "/memory_clear" {
			// 清空记忆
			c.hub.memoryManager.ClearHistory(c.ID)
			c.Send <- []byte("记忆已清空")
			continue
		} else if strings.HasPrefix(input, "/file ") {
			// 发送文件: /file 用户名 文件URL
			args := strings.TrimPrefix(input, "/file ")
			parts := strings.Fields(args)
			if len(parts) < 2 {
				c.Send <- []byte("用法: /file 用户名 文件URL")
			} else {
				to, fileURL := parts[0], parts[1]
				msg = NewMessage("single", c.ID, to, "[文件] "+fileURL)
				c.hub.broadcast <- msg.ToBytes()
			}
			continue
		} else if strings.HasPrefix(input, "/image ") {
			// 发送图片: /image 用户名 图片URL
			args := strings.TrimPrefix(input, "/image ")
			parts := strings.Fields(args)
			if len(parts) < 2 {
				c.Send <- []byte("用法: /image 用户名 图片URL")
			} else {
				to, imgURL := parts[0], parts[1]
				msg = NewMessage("single", c.ID, to, "[图片] "+imgURL)
				c.hub.broadcast <- msg.ToBytes()
			}
			continue
		} else if strings.HasPrefix(input, "/audio ") {
			// 发送语音: /audio 用户名 语音URL
			args := strings.TrimPrefix(input, "/audio ")
			parts := strings.Fields(args)
			if len(parts) < 2 {
				c.Send <- []byte("用法: /audio 用户名 语音URL")
			} else {
				to, audioURL := parts[0], parts[1]
				msg = NewMessage("single", c.ID, to, "[语音] "+audioURL)
				c.hub.broadcast <- msg.ToBytes()
			}
			continue
		} else if strings.HasPrefix(input, "/reply ") {
			// 回复消息: /reply 消息ID 回复内容
			args := strings.TrimPrefix(input, "/reply ")
			parts := strings.SplitN(args, " ", 2)
			if len(parts) < 2 {
				c.Send <- []byte("用法: /reply 消息ID 回复内容")
			} else {
				replyToID, content := parts[0], parts[1]
				// 获取原消息
				origMsg := c.hub.storage.GetByID(replyToID)
				if origMsg == nil {
					c.Send <- []byte("消息不存在")
				} else if origMsg.Revoked {
					c.Send <- []byte("原消息已撤回，无法回复")
				} else {
					// 发送回复（使用原消息的类型和接收者）
					msgID := c.hub.storage.SaveReply(origMsg.Type, c.ID, origMsg.To, content, replyToID)
					c.Send <- []byte(fmt.Sprintf("已回复消息 [%s]", replyToID))
					// 通知接收者
					if origMsg.Type == "single" {
						if targetClient := c.hub.GetClient(origMsg.From); targetClient != nil {
							targetClient.Send <- []byte(fmt.Sprintf("[%s 回复] [%s]: %s", c.ID, msgID, content))
						}
					} else if origMsg.Type == "group" {
						members := c.hub.groupManager.GetMembers(origMsg.To)
						for _, member := range members {
							if member != c.ID {
								if memberClient := c.hub.GetClient(member); memberClient != nil {
									memberClient.Send <- []byte(fmt.Sprintf("[%s@%s 回复] [%s]: %s", c.ID, origMsg.To, msgID, content))
								}
							}
						}
					}
				}
			}
			continue
		} else if strings.HasPrefix(input, "/revoke ") {
			// 撤回消息: /revoke 消息ID
			msgID := strings.TrimPrefix(input, "/revoke ")
			msgID = strings.TrimSpace(msgID)
			msg := c.hub.storage.GetByID(msgID)
			if msg == nil {
				c.Send <- []byte("消息不存在")
			} else if c.hub.storage.Revoke(msgID, c.ID) {
				c.Send <- []byte(fmt.Sprintf("已撤回消息 [%s]", msgID))
				// 发送结构化的撤回通知
				if msg.Type == "single" {
					if targetClient := c.hub.GetClient(msg.To); targetClient != nil {
						notify := RevokeNotification{
							Action: "revoke",
							MsgID:  msgID,
							From:   c.ID,
							To:     msg.To,
						}
						data, _ := json.Marshal(notify)
						targetClient.Send <- data
					}
				} else if msg.Type == "group" {
					members := c.hub.groupManager.GetMembers(msg.To)
					for _, member := range members {
						if member != c.ID {
							if memberClient := c.hub.GetClient(member); memberClient != nil {
								notify := RevokeNotification{
									Action: "revoke",
									MsgID:  msgID,
									From:   c.ID,
									To:     member,
									Group:  msg.To,
								}
								data, _ := json.Marshal(notify)
								memberClient.Send <- data
							}
						}
					}
				}
			} else {
				c.Send <- []byte("撤回失败，只能撤回自己2分钟内发送的消息")
			}
			continue
		} else if strings.HasPrefix(input, "/edit ") {
			// 编辑消息: /edit 消息ID 新内容
			args := strings.TrimPrefix(input, "/edit ")
			parts := strings.SplitN(args, " ", 2)
			if len(parts) < 2 {
				c.Send <- []byte("用法: /edit 消息ID 新内容")
			} else {
				msgID, newContent := parts[0], parts[1]
				msg := c.hub.storage.GetByID(msgID)
				if msg == nil {
					c.Send <- []byte("消息不存在")
				} else if c.hub.storage.Edit(msgID, c.ID, newContent) {
					c.Send <- []byte(fmt.Sprintf("已编辑消息 [%s]", msgID))
					// 通知接收者
					if msg.Type == "single" {
						if targetClient := c.hub.GetClient(msg.To); targetClient != nil {
							targetClient.Send <- []byte(fmt.Sprintf("[系统] %s 编辑了消息 [%s]: %s", c.ID, msgID, newContent))
						}
					} else if msg.Type == "group" {
						members := c.hub.groupManager.GetMembers(msg.To)
						for _, member := range members {
							if member != c.ID {
								if memberClient := c.hub.GetClient(member); memberClient != nil {
									memberClient.Send <- []byte(fmt.Sprintf("[系统] %s 在群 [%s] 编辑了消息 [%s]: %s", c.ID, msg.To, msgID, newContent))
								}
							}
						}
					}
				} else {
					c.Send <- []byte("编辑失败，只能编辑自己15分钟内发送的未撤回消息")
				}
			}
			continue
		} else if strings.HasPrefix(input, "/kick ") {
			// 踢出群成员: /kick 群名 用户名
			args := strings.TrimPrefix(input, "/kick ")
			parts := strings.Fields(args)
			if len(parts) < 2 {
				c.Send <- []byte("用法: /kick 群名 用户名")
			} else {
				groupName, target := parts[0], parts[1]
				if c.hub.groupManager.KickMember(groupName, c.ID, target) {
					c.Send <- []byte(fmt.Sprintf("已将 [%s] 从群 [%s] 踢出", target, groupName))
					if targetClient := c.hub.GetClient(target); targetClient != nil {
						targetClient.Send <- []byte(fmt.Sprintf("你已被移出群 [%s]", groupName))
					}
				} else {
					c.Send <- []byte("踢出失败，请检查权限或用户是否在群里")
				}
			}
			continue
		} else if strings.HasPrefix(input, "/mute ") {
			// 禁言成员: /mute 群名 用户名
			args := strings.TrimPrefix(input, "/mute ")
			parts := strings.Fields(args)
			if len(parts) < 2 {
				c.Send <- []byte("用法: /mute 群名 用户名")
			} else {
				groupName, target := parts[0], parts[1]
				if c.hub.groupManager.MuteMember(groupName, c.ID, target) {
					c.Send <- []byte(fmt.Sprintf("已禁言 [%s] 在群 [%s]", target, groupName))
					if targetClient := c.hub.GetClient(target); targetClient != nil {
						targetClient.Send <- []byte(fmt.Sprintf("你在群 [%s] 已被禁言", groupName))
					}
				} else {
					c.Send <- []byte("禁言失败，请检查权限")
				}
			}
			continue
		} else if strings.HasPrefix(input, "/unmute ") {
			// 解除禁言: /unmute 群名 用户名
			args := strings.TrimPrefix(input, "/unmute ")
			parts := strings.Fields(args)
			if len(parts) < 2 {
				c.Send <- []byte("用法: /unmute 群名 用户名")
			} else {
				groupName, target := parts[0], parts[1]
				if c.hub.groupManager.UnmuteMember(groupName, c.ID, target) {
					c.Send <- []byte(fmt.Sprintf("已解除 [%s] 在群 [%s] 的禁言", target, groupName))
				} else {
					c.Send <- []byte("解除禁言失败，请检查权限")
				}
			}
			continue
		} else if strings.HasPrefix(input, "/transfer ") {
			// 转让群主: /transfer 群名 新群主
			args := strings.TrimPrefix(input, "/transfer ")
			parts := strings.Fields(args)
			if len(parts) < 2 {
				c.Send <- []byte("用法: /transfer 群名 新群主")
			} else {
				groupName, newOwner := parts[0], parts[1]
				if c.hub.groupManager.TransferOwner(groupName, c.ID, newOwner) {
					c.Send <- []byte(fmt.Sprintf("已将群 [%s] 转让给 [%s]", groupName, newOwner))
					if newOwnerClient := c.hub.GetClient(newOwner); newOwnerClient != nil {
						newOwnerClient.Send <- []byte(fmt.Sprintf("你已成为群 [%s] 的新群主", groupName))
					}
				} else {
					c.Send <- []byte("转让失败，请检查权限或用户是否在群里")
				}
			}
			continue
		} else if strings.HasPrefix(input, "/announce ") {
			// 设置群公告: /announce 群名 公告内容
			args := strings.TrimPrefix(input, "/announce ")
			parts := strings.SplitN(args, " ", 2)
			if len(parts) < 2 {
				c.Send <- []byte("用法: /announce 群名 公告内容")
			} else {
				groupName, announce := parts[0], parts[1]
				if c.hub.groupManager.SetAnnounce(groupName, c.ID, announce) {
					c.Send <- []byte(fmt.Sprintf("群 [%s] 公告已设置", groupName))
					members := c.hub.groupManager.GetMembers(groupName)
					for _, member := range members {
						if memberClient := c.hub.GetClient(member); memberClient != nil {
							memberClient.Send <- []byte(fmt.Sprintf("[群公告] %s: %s", groupName, announce))
						}
					}
				} else {
					c.Send <- []byte("设置公告失败，请检查权限")
				}
			}
			continue
		} else if strings.HasPrefix(input, "/setadmin ") {
			// 设置管理员: /setadmin 群名 用户名
			args := strings.TrimPrefix(input, "/setadmin ")
			parts := strings.Fields(args)
			if len(parts) < 2 {
				c.Send <- []byte("用法: /setadmin 群名 用户名")
			} else {
				groupName, target := parts[0], parts[1]
				if c.hub.groupManager.SetAdmin(groupName, c.ID, target) {
					c.Send <- []byte(fmt.Sprintf("已设置 [%s] 为群 [%s] 的管理员", target, groupName))
					if targetClient := c.hub.GetClient(target); targetClient != nil {
						targetClient.Send <- []byte(fmt.Sprintf("你已成为群 [%s] 的管理员", groupName))
					}
				} else {
					c.Send <- []byte("设置管理员失败，请检查权限或用户是否在群里")
				}
			}
			continue
		} else if strings.HasPrefix(input, "/removeadmin ") {
			// 移除管理员: /removeadmin 群名 用户名
			args := strings.TrimPrefix(input, "/removeadmin ")
			parts := strings.Fields(args)
			if len(parts) < 2 {
				c.Send <- []byte("用法: /removeadmin 群名 用户名")
			} else {
				groupName, target := parts[0], parts[1]
				if c.hub.groupManager.RemoveAdmin(groupName, c.ID, target) {
					c.Send <- []byte(fmt.Sprintf("已移除 [%s] 在群 [%s] 的管理员身份", target, groupName))
				} else {
					c.Send <- []byte("移除管理员失败，请检查权限")
				}
			}
			continue
		} else if strings.HasPrefix(input, "/groupinfo ") {
			// 查看群信息: /groupinfo 群名
			groupName := strings.TrimPrefix(input, "/groupinfo ")
			groupName = strings.TrimSpace(groupName)
			group := c.hub.groupManager.GetGroup(groupName)
			if group == nil {
				c.Send <- []byte(fmt.Sprintf("群 [%s] 不存在", groupName))
			} else {
				var info strings.Builder
				info.WriteString(fmt.Sprintf("===== 群 [%s] 信息 =====\n", groupName))
				info.WriteString(fmt.Sprintf("群主: %s\n", group.Owner))
				info.WriteString(fmt.Sprintf("成员数: %d\n", len(group.Members)))
				if len(group.Admins) > 0 {
					admins := make([]string, 0, len(group.Admins))
					for admin := range group.Admins {
						admins = append(admins, admin)
					}
					info.WriteString(fmt.Sprintf("管理员: %s\n", strings.Join(admins, ", ")))
				}
				if group.Announce != "" {
					info.WriteString(fmt.Sprintf("群公告: %s\n", group.Announce))
				}
				if len(group.Muted) > 0 {
					muted := make([]string, 0, len(group.Muted))
					for m := range group.Muted {
						muted = append(muted, m)
					}
					info.WriteString(fmt.Sprintf("禁言中: %s\n", strings.Join(muted, ", ")))
				}
				c.Send <- []byte(info.String())
			}
			continue
		} else if strings.HasPrefix(input, "/invite ") {
			// 邀请用户加入群: /invite 群名 用户名
			args := strings.TrimPrefix(input, "/invite ")
			parts := strings.Fields(args)
			if len(parts) < 2 {
				c.Send <- []byte("用法: /invite 群名 用户名")
			} else {
				groupName, target := parts[0], parts[1]
				if c.hub.groupManager.InviteMember(groupName, c.ID, target) {
					c.Send <- []byte(fmt.Sprintf("已邀请 [%s] 加入群 [%s]", target, groupName))
					if targetClient := c.hub.GetClient(target); targetClient != nil {
						targetClient.Send <- []byte(fmt.Sprintf("[%s] 邀请你加入群 [%s]，输入 /acceptinvite %s 接受", c.ID, groupName, groupName))
					}
				} else {
					c.Send <- []byte("邀请失败，请检查权限或用户是否已在群里")
				}
			}
			continue
		} else if strings.HasPrefix(input, "/acceptinvite ") {
			// 接受群邀请: /acceptinvite 群名
			groupName := strings.TrimPrefix(input, "/acceptinvite ")
			groupName = strings.TrimSpace(groupName)
			if c.hub.groupManager.AcceptInvite(c.ID, groupName) {
				c.Send <- []byte(fmt.Sprintf("你已加入群 [%s]", groupName))
			} else {
				c.Send <- []byte(fmt.Sprintf("加入失败，没有收到群 [%s] 的邀请", groupName))
			}
			continue
		} else if strings.HasPrefix(input, "/rejectinvite ") {
			// 拒绝群邀请: /rejectinvite 群名
			groupName := strings.TrimPrefix(input, "/rejectinvite ")
			groupName = strings.TrimSpace(groupName)
			if c.hub.groupManager.RejectInvite(c.ID, groupName) {
				c.Send <- []byte(fmt.Sprintf("已拒绝加入群 [%s]", groupName))
			} else {
				c.Send <- []byte(fmt.Sprintf("拒绝失败，没有收到群 [%s] 的邀请", groupName))
			}
			continue
		} else if input == "/invites" {
			// 查看收到的群邀请
			invites := c.hub.groupManager.GetInvites(c.ID)
			if len(invites) == 0 {
				c.Send <- []byte("暂无群邀请")
			} else {
				var result strings.Builder
				result.WriteString("===== 群邀请列表 =====\n")
				for _, inv := range invites {
					result.WriteString(fmt.Sprintf("  %s\n", inv))
				}
				result.WriteString("输入 /acceptinvite 群名 接受邀请\n")
				result.WriteString("输入 /rejectinvite 群名 拒绝邀请\n")
				c.Send <- []byte(result.String())
			}
			continue
		} else if strings.HasPrefix(input, "/read ") {
			// 标记消息已读: /read 消息ID
			msgID := strings.TrimPrefix(input, "/read ")
			msgID = strings.TrimSpace(msgID)
			if c.hub.storage.MarkRead(msgID, c.ID) {
				c.Send <- []byte(fmt.Sprintf("已标记消息 [%s] 为已读", msgID))
				// 通知发送者
				c.hub.readNotify <- ReadEvent{MsgID: msgID, Reader: c.ID}
			} else {
				c.Send <- []byte("标记失败，消息不存在或已读过")
			}
			continue
		} else if strings.HasPrefix(input, "/typing ") {
			// 发送输入状态: /typing 用户名
			target := strings.TrimPrefix(input, "/typing ")
			target = strings.TrimSpace(target)
			c.hub.typing <- TypingEvent{From: c.ID, To: target}
			c.Send <- []byte(fmt.Sprintf("已通知 [%s] 你正在输入", target))
			continue
		} else if input == "/unread" {
			// 查看未读消息
			unread := c.hub.storage.GetUnreadByUser(c.ID)
			if len(unread) == 0 {
				c.Send <- []byte("暂无未读消息")
			} else {
				var result strings.Builder
				result.WriteString(fmt.Sprintf("===== 未读消息 (%d条) =====\n", len(unread)))
				for _, msg := range unread {
					timeStr := msg.Timestamp.Format("15:04:05")
					result.WriteString(fmt.Sprintf("[%s] %s -> 你: %s (ID: %s)\n", timeStr, msg.From, msg.Content, msg.ID))
				}
				result.WriteString("输入 /read 消息ID 标记已读\n")
				c.Send <- []byte(result.String())
			}
			continue
		} else if strings.HasPrefix(input, "/translate ") {
			// 翻译消息: /translate 语言 消息ID (如: /translate en abc123)
			args := strings.TrimPrefix(input, "/translate ")
			args = strings.TrimSpace(args)
			parts := strings.Fields(args)
			if len(parts) < 2 {
				c.Send <- []byte("用法: /translate 语言 消息ID (如: /translate en abc123)")
				c.Send <- []byte("支持语言: en(英文), ja(日文), ko(韩文), fr(法文), de(德文), es(西班牙文)")
			} else {
				targetLang, msgID := parts[0], parts[1]
				c.Send <- []byte("正在翻译...")
				result := c.hub.TranslateMessage(msgID, targetLang)
				c.Send <- []byte(result)
			}
		} else if input == "/moderation" {
			// 查看审核状态
			enabled := c.hub.moderationManager.IsEnabled()
			words := c.hub.moderationManager.GetWords()
			status := "关闭"
			if enabled {
				status = "开启"
			}
			c.Send <- []byte(fmt.Sprintf("===== 消息审核状态 =====\n状态: %s\n敏感词数量: %d", status, len(words)))
			continue
		} else if input == "/moderation on" {
			c.hub.moderationManager.Enable()
			c.Send <- []byte("消息审核已开启")
			continue
		} else if input == "/moderation off" {
			c.hub.moderationManager.Disable()
			c.Send <- []byte("消息审核已关闭")
			continue
		} else if strings.HasPrefix(input, "/addword ") {
			// 添加敏感词: /addword 敏感词
			word := strings.TrimPrefix(input, "/addword ")
			word = strings.TrimSpace(word)
			if word == "" {
				c.Send <- []byte("用法: /addword 敏感词")
			} else {
				c.hub.moderationManager.AddWord(word)
				c.Send <- []byte(fmt.Sprintf("已添加敏感词: %s", word))
			}
			continue
		} else if strings.HasPrefix(input, "/delword ") {
			// 删除敏感词: /delword 敏感词
			word := strings.TrimPrefix(input, "/delword ")
			word = strings.TrimSpace(word)
			c.hub.moderationManager.RemoveWord(word)
			c.Send <- []byte(fmt.Sprintf("已移除敏感词: %s", word))
			continue
		} else if input == "/words" {
			// 查看敏感词列表
			words := c.hub.moderationManager.GetWords()
			if len(words) == 0 {
				c.Send <- []byte("暂无敏感词")
			} else {
				c.Send <- []byte(fmt.Sprintf("敏感词列表 (%d个): %s", len(words), strings.Join(words, ", ")))
			}
			continue
			continue
		} else if strings.HasPrefix(input, "/msginfo ") {
			// 查看消息详情（已读状态）: /msginfo 消息ID
			msgID := strings.TrimPrefix(input, "/msginfo ")
			msgID = strings.TrimSpace(msgID)
			msg := c.hub.storage.GetByID(msgID)
			if msg == nil {
				c.Send <- []byte(fmt.Sprintf("消息 [%s] 不存在", msgID))
			} else {
				var info strings.Builder
				info.WriteString(fmt.Sprintf("===== 消息 [%s] =====\n", msgID))
				info.WriteString(fmt.Sprintf("发送者: %s\n", msg.From))
				info.WriteString(fmt.Sprintf("接收者: %s\n", msg.To))
				info.WriteString(fmt.Sprintf("内容: %s\n", msg.Content))
				info.WriteString(fmt.Sprintf("时间: %s\n", msg.Timestamp.Format("2006-01-02 15:04:05")))
				if len(msg.ReadBy) > 0 {
					info.WriteString(fmt.Sprintf("已读者: %s\n", strings.Join(msg.ReadBy, ", ")))
				} else {
					info.WriteString("已读者: 暂无\n")
				}
				c.Send <- []byte(info.String())
			}
			continue
		} else if strings.HasPrefix(input, "/search ") {
			// 搜索消息: /search 关键词 [user:用户名] [type:类型]
			args := strings.TrimPrefix(input, "/search ")
			args = strings.TrimSpace(args)
			if args == "" {
				c.Send <- []byte("用法: /search 关键词 [user:用户名] [type:single/group/broadcast]")
				continue
			}

			// 解析参数
			parts := strings.Fields(args)
			keyword := ""
			user := ""
			msgType := ""

			for _, part := range parts {
				if strings.HasPrefix(part, "user:") {
					user = strings.TrimPrefix(part, "user:")
				} else if strings.HasPrefix(part, "type:") {
					msgType = strings.TrimPrefix(part, "type:")
				} else {
					keyword = part
				}
			}

			// 执行搜索
			results := c.hub.storage.SearchByKeywordWithFilter(keyword, msgType, user)

			if len(results) == 0 {
				c.Send <- []byte("未找到匹配的消息")
			} else {
				var output strings.Builder
				output.WriteString(fmt.Sprintf("===== 搜索结果 (%d条) =====\n", len(results)))
				for i, msg := range results {
					if i >= 20 {
						output.WriteString(fmt.Sprintf("... 还有 %d 条结果\n", len(results)-20))
						break
					}
					timeStr := msg.Timestamp.Format("01-02 15:04")
					switch msg.Type {
					case "single":
						output.WriteString(fmt.Sprintf("[%s] %s -> %s: %s\n", timeStr, msg.From, msg.To, msg.Content))
					case "group":
						output.WriteString(fmt.Sprintf("[%s] %s@%s: %s\n", timeStr, msg.From, msg.To, msg.Content))
					default:
						output.WriteString(fmt.Sprintf("[%s] %s: %s\n", timeStr, msg.From, msg.Content))
					}
				}
				c.Send <- []byte(output.String())
			}
			continue
		} else if strings.HasPrefix(input, "/history ") {
			// 按时间范围查看历史: /history today / /history from:2024-01-01 to:2024-01-31
			args := strings.TrimPrefix(input, "/history ")
			args = strings.TrimSpace(args)

			var results []StoredMessage

			if args == "today" {
				// 今天的消息
				now := time.Now()
				start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
				end := start.Add(24 * time.Hour)
				results = c.hub.storage.SearchByTimeRange(start, end)
			} else if args == "yesterday" {
				// 昨天的消息
				now := time.Now()
				end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
				start := end.Add(-24 * time.Hour)
				results = c.hub.storage.SearchByTimeRange(start, end)
			} else {
				// 解析 from:xxx to:xxx 格式
				var startTime, endTime time.Time
				fromIdx := strings.Index(args, "from:")
				toIdx := strings.Index(args, "to:")

				if fromIdx != -1 {
					fromStr := args[fromIdx+5:]
					if spaceIdx := strings.Index(fromStr, " "); spaceIdx != -1 {
						fromStr = fromStr[:spaceIdx]
					}
					startTime, _ = time.Parse("2006-01-02", fromStr)
				}
				if toIdx != -1 {
					toStr := args[toIdx+3:]
					if spaceIdx := strings.Index(toStr, " "); spaceIdx != -1 {
						toStr = toStr[:spaceIdx]
					}
					endTime, _ = time.Parse("2006-01-02", toStr)
					endTime = endTime.Add(24*time.Hour - time.Second)
				}

				if !startTime.IsZero() || !endTime.IsZero() {
					params := SearchParams{
						StartTime: startTime,
						EndTime:   endTime,
					}
					results = c.hub.storage.AdvancedSearch(params)
				} else {
					c.Send <- []byte("用法: /history today|yesterday|from:日期 to:日期")
					continue
				}
			}

			if len(results) == 0 {
				c.Send <- []byte("该时间段内没有消息")
			} else {
				var output strings.Builder
				output.WriteString(fmt.Sprintf("===== 历史消息 (%d条) =====\n", len(results)))
				for i, msg := range results {
					if i >= 50 {
						output.WriteString(fmt.Sprintf("... 还有 %d 条消息\n", len(results)-50))
						break
					}
					timeStr := msg.Timestamp.Format("01-02 15:04:05")
					switch msg.Type {
					case "single":
						output.WriteString(fmt.Sprintf("[%s] %s -> %s: %s\n", timeStr, msg.From, msg.To, msg.Content))
					case "group":
						output.WriteString(fmt.Sprintf("[%s] %s@%s: %s\n", timeStr, msg.From, msg.To, msg.Content))
					default:
						output.WriteString(fmt.Sprintf("[%s] %s: %s\n", timeStr, msg.From, msg.Content))
					}
				}
				c.Send <- []byte(output.String())
			}
			continue
		} else if strings.HasPrefix(input, "@") {
			// 单聊格式: @李四 你好
			parts := strings.SplitN(input, " ", 2)
			if len(parts) == 2 {
				to := strings.TrimPrefix(parts[0], "@")
				content := parts[1]
				msg = NewMessage("single", c.ID, to, content)
			} else {
				msg = NewMessage("broadcast", c.ID, "", input)
			}
		} else if strings.HasPrefix(input, "#") {
			// 群聊格式: #技术群 大家好
			parts := strings.SplitN(input, " ", 2)
			if len(parts) == 2 {
				groupName := strings.TrimPrefix(parts[0], "#")
				content := parts[1]
				msg = NewMessage("group", c.ID, groupName, content)
			} else {
				msg = NewMessage("broadcast", c.ID, "", input)
			}
		} else {
			// 普通消息，广播
			msg = NewMessage("broadcast", c.ID, "", input)
		}

		// 转成JSON字节，发送到Hub
		c.hub.broadcast <- msg.ToBytes()
	}
}

// WritePump 向WebSocket连接写入消息
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
