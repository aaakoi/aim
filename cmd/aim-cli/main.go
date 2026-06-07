package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
)

// ========== 消息类型 ==========

type Message struct {
	Type    string `json:"type"`
	From    string `json:"from"`
	To      string `json:"to"`
	Content string `json:"content"`
	Stream  bool   `json:"stream"`
}

type StreamMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	From    string `json:"from"`
}

// RevokeNotification 撤回通知
type RevokeNotification struct {
	Action string `json:"action"`
	MsgID  string `json:"msgId"`
	From   string `json:"from"`
	To     string `json:"to"`
	Group  string `json:"group"`
}

// ========== Bubbletea 消息 ==========

type wsConnectedMsg *websocket.Conn
type wsDataMsg []byte
type wsErrMsg error

// ========== 模型 ==========

type model struct {
	messages    []string
	msgIDs      []string // 每条消息对应的ID
	input       textinput.Model
	conn        *websocket.Conn
	currentAI   string
	isStreaming bool
	userID      string
	password    string
	targetID    string
	token       string
	width       int
	height      int
}

// ========== 样式 ==========

var (
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true)

	aiStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")).
		Bold(true)

	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)
)

// ========== 初始化 ==========

func initialModel(userID, password string) model {
	ti := textinput.New()
	ti.Placeholder = "输入消息... (/帮助 查看命令)"
	ti.Focus()
	ti.CharLimit = 1000
	ti.Width = 60

	return model{
		input:    ti,
		userID:   userID,
		password: password,
		targetID: "Bot",
		messages: []string{systemStyle.Render(fmt.Sprintf("[系统] 正在登录用户: %s...", userID))},
		msgIDs:   []string{""},
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		loginAndConnect(m.userID, m.password),
	)
}

// ========== 登录 + WebSocket ==========

func loginAndConnect(userID, password string) tea.Cmd {
	return func() tea.Msg {
		// 1. 先尝试注册
		registerData := map[string]string{"username": userID, "password": password}
		jsonData, _ := json.Marshal(registerData)
		http.Post("http://localhost:8080/register", "application/json", bytes.NewBuffer(jsonData))

		// 2. 登录获取 token
		loginData := map[string]string{"username": userID, "password": password}
		jsonData, _ = json.Marshal(loginData)

		resp, err := http.Post("http://localhost:8080/login", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			return wsErrMsg(fmt.Errorf("登录失败: %v", err))
		}
		defer resp.Body.Close()

		var result struct {
			Token string `json:"token"`
		}
		json.NewDecoder(resp.Body).Decode(&result)

		if result.Token == "" {
			return wsErrMsg(fmt.Errorf("登录失败: 未获取到 token"))
		}

		// 3. 连接 WebSocket
		conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/ws?token="+result.Token, nil)
		if err != nil {
			return wsErrMsg(fmt.Errorf("WebSocket 连接失败: %v", err))
		}

		return wsConnectedMsg(conn)
	}
}

func readWS(conn *websocket.Conn) tea.Cmd {
	return func() tea.Msg {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return wsErrMsg(err)
		}
		return wsDataMsg(data)
	}
}

// ========== 更新 ==========

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			if m.conn != nil {
				m.conn.Close()
			}
			return m, tea.Quit

		case tea.KeyEnter:
			if m.input.Value() == "" || m.isStreaming {
				break
			}

			input := m.input.Value()
			m.input.SetValue("")

			// 处理命令
			if strings.HasPrefix(input, "/") {
				m.handleCommand(input)
				break
			}

			// 解析消息格式
			msgType, target, content := m.parseInput(input)
			if content == "" {
				break
			}

			// 显示发送的消息
			if msgType == "group" {
				m.messages = append(m.messages,
					userStyle.Render(fmt.Sprintf("[%s@%s]: ", m.userID, target))+content)
				m.msgIDs = append(m.msgIDs, "")
			} else {
				m.messages = append(m.messages,
					userStyle.Render(fmt.Sprintf("[%s -> %s]: ", m.userID, target))+content)
				m.msgIDs = append(m.msgIDs, "")
			}

			// 发送消息
			outMsg := Message{
				Type:    msgType,
				From:    m.userID,
				To:      target,
				Content: content,
				Stream:  target == "Bot",
			}
			data, _ := json.Marshal(outMsg)
			if m.conn != nil {
				m.conn.WriteMessage(websocket.TextMessage, data)
			}

			if target == "Bot" {
				m.isStreaming = true
				m.currentAI = ""
			}
			m.targetID = target
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case wsConnectedMsg:
		m.conn = msg
		m.messages = append(m.messages, successStyle.Render(fmt.Sprintf("[系统] 用户 %s 登录成功！", m.userID)))
		m.messages = append(m.messages, systemStyle.Render("[系统] 输入 /帮助 查看所有命令"))
		m.msgIDs = append(m.msgIDs, "", "")
		cmds = append(cmds, readWS(m.conn))

	case wsDataMsg:
		// 尝试解析撤回通知
		var revokeNotify RevokeNotification
		if err := json.Unmarshal(msg, &revokeNotify); err == nil && revokeNotify.Action == "revoke" {
			// 找到并更新被撤回的消息
			for i, msgID := range m.msgIDs {
				if msgID == revokeNotify.MsgID && i < len(m.messages) {
					m.messages[i] = systemStyle.Render(fmt.Sprintf("[已撤回] %s 撤回了一条消息", revokeNotify.From))
					break
				}
			}
		} else {
			// 尝试解析流式消息
			var streamMsg StreamMessage
			if err := json.Unmarshal(msg, &streamMsg); err == nil && streamMsg.Type != "" {
				switch streamMsg.Type {
				case "stream_start":
					m.messages = append(m.messages, aiStyle.Render("[AI]: "))
					m.msgIDs = append(m.msgIDs, "")
				case "stream_chunk":
					m.currentAI += streamMsg.Content
					if len(m.messages) > 0 {
						rendered, _ := glamour.Render(m.currentAI, "dark")
						m.messages[len(m.messages)-1] = aiStyle.Render("[AI]: ") + strings.TrimSpace(rendered)
					}
				case "stream_end":
					m.isStreaming = false
				}
			} else {
				// 普通消息，尝试提取消息ID
				msgStr := string(msg)
				msgID := ""
				// 格式: [发送者(私聊)] [消息ID]: 内容
				if idx := strings.Index(msgStr, "] ["); idx > 0 {
					if endIdx := strings.Index(msgStr[idx+3:], "]:"); endIdx > 0 {
						msgID = msgStr[idx+3 : idx+3+endIdx]
					}
				}
				m.messages = append(m.messages, msgStr)
				m.msgIDs = append(m.msgIDs, msgID)
			}
		}
		cmds = append(cmds, readWS(m.conn))

	case wsErrMsg:
		m.messages = append(m.messages, errorStyle.Render(fmt.Sprintf("[错误] %v", msg)))
		m.msgIDs = append(m.msgIDs, "")
	}

	// 更新输入框
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// parseInput 解析输入
func (m model) parseInput(input string) (string, string, string) {
	// @用户 消息 -> 单聊
	if strings.HasPrefix(input, "@") {
		parts := strings.SplitN(input, " ", 2)
		if len(parts) == 2 {
			target := strings.TrimPrefix(parts[0], "@")
			return "single", target, parts[1]
		}
		return "single", strings.TrimPrefix(parts[0], "@"), ""
	}

	// #群名 消息 -> 群聊
	if strings.HasPrefix(input, "#") {
		parts := strings.SplitN(input, " ", 2)
		if len(parts) == 2 {
			group := strings.TrimPrefix(parts[0], "#")
			return "group", group, parts[1]
		}
		return "group", strings.TrimPrefix(parts[0], "#"), ""
	}

	// 默认发给当前目标
	return "single", m.targetID, input
}

// handleCommand 处理命令
func (m *model) handleCommand(cmd string) {
	cmd = strings.TrimSpace(cmd)
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "/帮助", "/help", "/?":
		m.messages = append(m.messages, systemStyle.Render(`
	╔═══════════════════════════════════════════════════════════════╗
	║                         命令帮助                              ║
	╠═══════════════════════════════════════════════════════════════╣
	║  聊天                                                         ║
	║    @用户 消息          私聊用户                               ║
	║    #群名 消息          群聊                                   ║
	║    /revoke 消息ID      撤回消息                               ║
	║    /read 消息ID        标记已读                               ║
	║    /typing 用户        发送输入状态                           ║
	║    /search 关键词      搜索历史消息                           ║
	║    /history            查看历史消息                           ║
	║    /summarize          一键总结历史                           ║
	║    /todos              提取待办事项                           ║
	║                                                               ║
	║  群组                                                         ║
	║    /创建 群名           创建群组                              ║
	║    /加入 群名           加入群组                              ║
	║    /退群 群名           退出群组                              ║
	║    /invite 群名 用户    邀请用户                              ║
	║    /kick 群名 用户      踢出成员(群主)                        ║
	║    /禁言 群名 用户      禁言用户(群主)                        ║
	║    /解禁 群名 用户      解禁用户(群主)                        ║
	║    /transfer 群名 用户  转让群主                              ║
	║    /announce 群名 内容  设置群公告                            ║
	║    /群成员 群名         查看群成员                            ║
	║                                                               ║
	║  好友                                                         ║
	║    /add 用户            添加好友                             ║
	║    /accept 用户         接受好友请求                         ║
	║    /remove 用户         删除好友                             ║
	║    /friends             好友列表                             ║
	║    /setremark 用户 备注 设置好友备注                         ║
	║    /setgroup 用户 分组  设置好友分组                          ║
	║    /groups              查看所有分组                          ║
	║    /friendinfo 用户     查看好友详情                          ║
	║                                                               ║
	║  记忆                                                         ║
	║    /memory              查看记忆数量                         ║
	║    /memory_clear        清空记忆                             ║
	║                                                               ║
	║  Bot & AI                                                     ║
	║    /provider zhipu/deepseek  切换AI提供商                    ║
	║    /ask 问题            生成候选回复                         ║
	║    /select 序号 [@用户] 选择候选回复发送                     ║
	║    /quota               查看余额                             ║
	║                                                               ║
	║  文件                                                         ║
	║    /file 用户 URL       发送文件                             ║
	║    /image 用户 URL      发送图片                             ║
	║    /audio 用户 URL      发送语音                             ║
	║                                                               ║
	║  知识库 (RAG)                                                 ║
	║    /kb_create 名称      创建知识库                           ║
	║    /kb_list             知识库列表                           ║
	║    /kb_add 库名 路径    添加文档                             ║
	║    /kb_query 库名 问题  查询知识库                           ║
	║    /kb_delete 库名      删除知识库                           ║
	║                                                               ║
	║  其他                                                         ║
	║    /online              在线用户                             ║
	║    /help                显示帮助                             ║
	║    Ctrl+C               退出                                 ║
	╚═══════════════════════════════════════════════════════════════╝`))
		m.msgIDs = append(m.msgIDs, "")

	// ============ 聊天相关 ============
	case "/revoke", "/撤回":
		if len(parts) >= 2 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/revoke "+parts[1]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /revoke 消息ID"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/read":
		if len(parts) >= 2 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/read "+parts[1]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /read 消息ID"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/typing":
		if len(parts) >= 2 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/typing "+parts[1]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /typing 用户"))
			m.msgIDs = append(m.msgIDs, "")
		}

	// ============ 群组相关 ============
	case "/创建", "/create":
		if len(parts) >= 2 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/create "+parts[1]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /创建 群名"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/加入", "/join":
		if len(parts) >= 2 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/join "+parts[1]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /加入 群名"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/退群", "/quit":
		if len(parts) >= 2 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/quit "+parts[1]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /退群 群名"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/禁言", "/mute":
		if len(parts) >= 3 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/mute "+parts[1]+" "+parts[2]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /禁言 群名 用户"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/解禁", "/unmute":
		if len(parts) >= 3 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/unmute "+parts[1]+" "+parts[2]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /解禁 群名 用户"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/群成员", "/members":
		if len(parts) >= 2 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/members "+parts[1]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /群成员 群名"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/invite", "/邀请":
		if len(parts) >= 3 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/invite "+parts[1]+" "+parts[2]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /invite 群名 用户"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/kick", "/踢出":
		if len(parts) >= 3 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/kick "+parts[1]+" "+parts[2]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /kick 群名 用户"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/transfer", "/转让":
		if len(parts) >= 3 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/transfer "+parts[1]+" "+parts[2]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /transfer 群名 新群主"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/announce", "/公告":
		if len(parts) >= 3 {
			content := strings.Join(parts[2:], " ")
			m.conn.WriteMessage(websocket.TextMessage, []byte("/announce "+parts[1]+" "+content))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /announce 群名 公告内容"))
			m.msgIDs = append(m.msgIDs, "")
		}

	// ============ 好友相关 ============
	case "/add":
		if len(parts) >= 2 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/add "+parts[1]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /add 用户"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/accept":
		if len(parts) >= 2 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/accept "+parts[1]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /accept 用户"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/friends", "/好友":
		m.conn.WriteMessage(websocket.TextMessage, []byte("/friends"))

	case "/remove", "/删除好友":
		if len(parts) >= 2 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/remove "+parts[1]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /remove 用户"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/setremark":
		if len(parts) >= 3 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/setremark "+parts[1]+" "+parts[2]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /setremark 用户 备注"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/setgroup":
		if len(parts) >= 3 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/setgroup "+parts[1]+" "+parts[2]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /setgroup 用户 分组"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/groups":
		m.conn.WriteMessage(websocket.TextMessage, []byte("/groups"))

	case "/friendinfo":
		if len(parts) >= 2 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/friendinfo "+parts[1]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /friendinfo 用户"))
			m.msgIDs = append(m.msgIDs, "")
		}

	// ============ 记忆 & 历史 ============
	case "/memory":
		m.conn.WriteMessage(websocket.TextMessage, []byte("/memory"))

	case "/memory_clear":
		m.conn.WriteMessage(websocket.TextMessage, []byte("/memory_clear"))

	case "/history":
		m.conn.WriteMessage(websocket.TextMessage, []byte("/history"))

	case "/search", "/搜索":
		if len(parts) >= 2 {
			query := strings.Join(parts[1:], " ")
			m.conn.WriteMessage(websocket.TextMessage, []byte("/search "+query))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /search 关键词 [user:用户名] [type:类型]"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/summarize", "/总结":
		m.conn.WriteMessage(websocket.TextMessage, []byte("/summarize"))

	case "/todos", "/待办":
		m.conn.WriteMessage(websocket.TextMessage, []byte("/todos"))

	// ============ Bot & AI ============
	case "/provider":
		if len(parts) >= 2 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/provider "+parts[1]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /provider zhipu 或 /provider deepseek"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/ask":
		if len(parts) >= 2 {
			question := strings.Join(parts[1:], " ")
			m.conn.WriteMessage(websocket.TextMessage, []byte("/ask "+question))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /ask 问题"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/select":
		if len(parts) >= 2 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/select "+strings.Join(parts[1:], " ")))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /select 序号 [@用户]"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/quota", "/余额":
		m.conn.WriteMessage(websocket.TextMessage, []byte("/quota"))

	// ============ 文件 ============
	case "/file":
		if len(parts) >= 3 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/file "+parts[1]+" "+parts[2]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /file 用户 URL"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/image":
		if len(parts) >= 3 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/image "+parts[1]+" "+parts[2]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /image 用户 URL"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/audio":
		if len(parts) >= 3 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/audio "+parts[1]+" "+parts[2]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /audio 用户 URL"))
			m.msgIDs = append(m.msgIDs, "")
		}

	// ============ 知识库 RAG ============
	case "/kb_create":
		if len(parts) >= 2 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/kb_create "+parts[1]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /kb_create 名称"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/kb_list":
		m.conn.WriteMessage(websocket.TextMessage, []byte("/kb_list"))

	case "/kb_add":
		if len(parts) >= 3 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/kb_add "+parts[1]+" "+parts[2]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /kb_add 库名 文件路径"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/kb_query":
		if len(parts) >= 3 {
			question := strings.Join(parts[2:], " ")
			m.conn.WriteMessage(websocket.TextMessage, []byte("/kb_query "+parts[1]+" "+question))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /kb_query 库名 问题"))
			m.msgIDs = append(m.msgIDs, "")
		}

	case "/kb_delete":
		if len(parts) >= 2 {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/kb_delete "+parts[1]))
		} else {
			m.messages = append(m.messages, errorStyle.Render("用法: /kb_delete 库名"))
			m.msgIDs = append(m.msgIDs, "")
		}

	// ============ 其他 ============
	case "/在线", "/online":
		if m.conn != nil {
			m.conn.WriteMessage(websocket.TextMessage, []byte("/online"))
		}

	default:
		// 其他命令直接转发
		if m.conn != nil {
			m.conn.WriteMessage(websocket.TextMessage, []byte(cmd))
		}
	}
}

// ========== 视图 ==========

func (m model) View() string {
	var b strings.Builder

	// 标题 + 状态
	title := statusStyle.Render(fmt.Sprintf("╔══ AIM Chat [用户: %s] [目标: %s] ══╗", m.userID, m.targetID))
	b.WriteString(title + "\n\n")

	// 消息区域
	msgArea := ""
	for _, msg := range m.messages {
		msgArea += msg + "\n"
	}

	// 限制显示行数
	lines := strings.Split(msgArea, "\n")
	maxLines := m.height - 8
	if maxLines < 10 {
		maxLines = 10
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	msgArea = strings.Join(lines, "\n")

	boxWidth := m.width - 4
	if boxWidth < 40 {
		boxWidth = 40
	}
	b.WriteString(boxStyle.Width(boxWidth).Render(msgArea))
	b.WriteString("\n\n")

	// 输入区域
	inputLine := "> " + m.input.View()
	if m.isStreaming {
		inputLine += " " + lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("(AI 正在回复...)")
	}
	b.WriteString(inputLine)

	return b.String()
}

// ========== 主函数 ==========

func main() {
	// 解析命令行参数
	userNum := flag.String("u", "1", "用户编号 (如: -u 1 表示 user1)")
	flag.Parse()

	// 根据编号生成用户名
	userID := "user" + *userNum
	password := "123456"

	// 如果参数是纯数字，生成 userN 格式
	if _, err := fmt.Sscanf(*userNum, "%d", new(int)); err == nil {
		userID = "user" + *userNum
	} else {
		// 否则直接作为用户名
		userID = *userNum
	}

	p := tea.NewProgram(
		initialModel(userID, password),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}
