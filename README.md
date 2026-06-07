# AIM - 智能即时通讯系统

一个面向多人在线的即时通讯系统，内置 AI 助手，实现"通讯 + AI"的深度融合。

## 项目亮点

- **完整的 IM 功能**：单聊、群聊、广播、好友、群组管理
- **AI 深度集成**：智谱/DeepSeek 双提供商、流式输出、记忆能力、RAG知识库
- **服务治理**：熔断器、限流器、服务降级，保障系统稳定性
- **可观测性**：Prometheus 指标 + Grafana 可视化 + OpenTelemetry 链路追踪
- **内容安全**：敏感词过滤 + AI审核双重保障
- **消息增强**：撤回、编辑、已读回执、离线推送、实时翻译

---

## 功能特性

### 核心通讯功能

| 功能 | 说明 |
|-----|------|
| 单聊 | `@用户名 消息` 私聊指定用户 |
| 群聊 | `#群名 消息` 发送群消息 |
| 广播 | 无前缀，发送给所有在线用户 |
| 好友管理 | 添加、删除、备注、分组 |
| 群组管理 | 创建、加入、踢人、禁言、转让群主 |
| 消息撤回 | `/revoke 消息ID` 2分钟内可撤回 |
| 消息编辑 | `/edit 消息ID 新内容` 15分钟内可编辑 |
| 已读回执 | `/read 消息ID` 标记已读 |
| 离线推送 | 上线自动推送离线消息 |

### AI 功能

| 功能 | 说明 |
|-----|------|
| AI对话 | `@Bot 消息` 与AI助手对话 |
| 流式输出 | 打字机效果，实时显示AI回复 |
| 多提供商 | 支持智谱AI、DeepSeek，可切换 |
| 记忆能力 | AI记住上下文，连续对话 |
| RAG知识库 | 上传文档，AI基于知识库回答 |
| 候选回复 | `/ask 问题` 生成多个候选答案 |
| 计费系统 | Token用量统计，费用计算 |

### 安全与治理

| 功能 | 说明 |
|-----|------|
| 敏感词过滤 | 自动检测并拦截敏感内容 |
| 熔断保护 | 连续失败5次触发熔断，30秒后恢复 |
| 请求限流 | 令牌桶算法，每秒最多10个请求 |
| 服务降级 | AI服务不可用时返回友好提示 |
| JWT认证 | 安全的用户身份验证 |

### 可观测性

| 功能 | 访问地址 |
|-----|---------|
| Prometheus 指标 | http://localhost:9090 |
| Grafana 看板 | http://localhost:3000 |
| 监控指标 | 在线用户数、消息吞吐量、API延迟 |

---

## 技术架构

### 技术栈

| 类别 | 技术 |
|-----|------|
| 语言 | Go 1.22+ |
| Web框架 | Gin |
| 实时通信 | gorilla/websocket |
| 数据存储 | SQLite (modernc.org/sqlite) |
| 用户认证 | JWT (golang-jwt/jwt) |
| AI接口 | 智谱AI / DeepSeek API |
| 容器化 | Docker + Docker Compose |
| 监控 | Prometheus + Grafana |
| 链路追踪 | OpenTelemetry |
| 日志 | zap (结构化日志) |

### 架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                        客户端 (aim-cli)                          │
│                    TUI界面 / WebSocket连接                        │
└─────────────────────────────┬───────────────────────────────────┘
                              │ WebSocket
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                         main.go                                  │
│                   HTTP路由 + WebSocket升级                        │
│              /ws  /register  /login  /metrics                    │
└─────────────────────────────┬───────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                          hub.go                                  │
│                      Hub 消息路由中心                             │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    channel 驱动                           │   │
│  │   register ─── unregister ─── broadcast ─── typing      │   │
│  └─────────────────────────────────────────────────────────┘   │
│  ┌───────────┬───────────┬───────────┬───────────┬─────────┐   │
│  │ 用户管理  │ 群组管理  │ 好友管理  │ Bot管理   │ 审核管理│   │
│  └───────────┴───────────┴───────────┴───────────┴─────────┘   │
└─────────────────────────────┬───────────────────────────────────┘
                              │
          ┌───────────────────┼───────────────────┐
          ▼                   ▼                   ▼
    ┌──────────┐        ┌──────────┐        ┌──────────┐
    │ 单聊消息 │        │ 群聊消息 │        │ AI对话  │
    │ 转发     │        │ 广播     │        │ 熔断保护 │
    └──────────┘        └──────────┘        └──────────┘
                              │
                              ▼
    ┌──────────────────────────────────────────────────────────┐
    │                      外部服务                              │
    │  智谱AI API ─── DeepSeek API ─── 翻译API ─── 审核API    │
    └──────────────────────────────────────────────────────────┘
```

### 消息流转

```
用户发送消息 "你好"
      │
      ▼
client.go: ReadPump()
解析 JSON，验证格式
      │
      ▼
hub.broadcast <- message
发送到广播通道
      │
      ▼
hub.go: Run()
      │
      ├──→ moderation.go: Moderate()
      │    敏感词检测 → 拦截违规消息
      │
      ├──→ storage.go: Save()
      │    存储到 SQLite
      │
      └──→ 路由分发
           ├── 单聊 → GetClient(to).Send
           ├── 群聊 → 遍历群成员发送
           ├── Bot → bot.go 处理
           │         │
           │         ├──→ 熔断检查
           │         ├──→ 限流检查
           │         └──→ 调用AI API
           │               │
           │               └──→ 流式/非流式响应
           │
           └──→ 广播 → 所有用户
      │
      ▼
client.go: WritePump()
发送给目标用户
```

### 项目结构

```
aim/
├── main.go              # 入口，HTTP路由
├── hub.go               # 消息路由中心
├── client.go            # 客户端连接处理
├── message.go           # 消息结构定义
├── storage.go           # SQLite消息存储
├── user.go              # 用户认证(JWT)
├── group.go             # 群组管理
├── friend.go            # 好友管理
├── bot.go               # AI Bot + 熔断 + 限流
├── ratelimit.go         # 熔断器 + 限流器
├── moderation.go        # 内容审核
├── translate.go         # 实时翻译
├── memory.go            # AI记忆管理
├── billing.go           # 计费系统
├── metrics.go           # Prometheus指标
├── middleware.go        # 中间件(日志、追踪)
├── tracing.go           # OpenTelemetry配置
├── rag/                 # RAG知识库模块
│   ├── rag.go           # RAG核心逻辑
│   ├── embedding.go     # 向量嵌入
│   └── chunk.go         # 文档分块
├── cmd/aim-cli/         # TUI客户端
├── test/                # 功能测试
├── Dockerfile           # Docker构建
├── docker-compose.yml   # 容器编排
├── prometheus.yml       # Prometheus配置
└── grafana-dashboard.json # Grafana看板
```

---

## 快速开始

### 方式一：Docker 部署

```bash
# 构建并启动所有服务
docker-compose up -d

# 查看运行状态
docker ps

# 查看日志
docker-compose logs -f aim
```

监控地址：
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)

### 方式二：源码运行

```bash
# 安装依赖
go mod download

# 启动服务端
go run .

# 新开终端，启动客户端
go run cmd/aim-cli/main.go -u 1   # 用户 user1
go run cmd/aim-cli/main.go -u 2   # 用户 user2
```

---

## 命令大全

### 聊天命令

```
@用户名 消息        # 私聊
#群名 消息          # 群聊
@Bot 消息           # 与AI对话
```

### 消息命令

```
/revoke 消息ID      # 撤回消息(2分钟内)
/edit 消息ID 内容   # 编辑消息(15分钟内)
/read 消息ID        # 标记已读
/typing 用户        # 发送输入状态
/search 关键词      # 搜索历史消息
/history            # 查看历史消息
/translate 消息ID   # 翻译消息
```

### 群组命令

```
/创建 群名          # 创建群组
/加入 群名          # 加入群组
/退群 群名          # 退出群组
/invite 群名 用户   # 邀请用户
/kick 群名 用户     # 踢出成员
/禁言 群名 用户     # 禁言用户
/解禁 群名 用户     # 解除禁言
/转让 群名 用户     # 转让群主
/公告 群名 内容     # 设置群公告
/群成员 群名        # 查看成员
```

### 好友命令

```
/add 用户           # 添加好友
/accept 用户        # 接受好友请求
/remove 用户        # 删除好友
/friends            # 好友列表
/setremark 用户 备注 # 设置备注
```

### AI命令

```
/provider zhipu     # 切换到智谱AI
/provider deepseek  # 切换到DeepSeek
/ask 问题           # 生成候选回复
/select 序号        # 选择候选回复
/quota              # 查看余额
/memory             # 查看记忆数量
/memory_clear       # 清空记忆
```

### 知识库命令

```
/kb_create 名称     # 创建知识库
/kb_list            # 知识库列表
/kb_add 库名 文件路径 # 添加文档
/kb_query 库名 问题  # 查询知识库
/kb_delete 库名     # 删除知识库
```

### 其他命令

```
/online             # 在线用户
/帮助               # 显示帮助
```

---

## 核心设计

### 1. 并发模型

```go
// Hub 使用 channel 驱动
type Hub struct {
    register   chan *Client    // 用户上线
    unregister chan *Client    // 用户下线
    broadcast  chan []byte     // 消息广播
}

// 每个用户一个 goroutine 读写
go client.writePump()  // 发送消息
go client.readPump()   // 接收消息
```

### 2. 熔断器实现

```go
type CircuitBreaker struct {
    failures     int       // 连续失败次数
    threshold    int       // 熔断阈值(5次)
    state        string    // closed/open/half-open
    lastFailTime time.Time // 上次失败时间
    timeout      Duration  // 恢复时间(30s)
}

// 调用AI前检查
err := apiCircuitBreaker.Call(func() error {
    return callAI()
})
if err == ErrCircuitOpen {
    return "AI服务繁忙，请稍后重试"  // 降级响应
}
```

### 3. 消息存储

```go
// SQLite 存储，支持复杂查询
db.Exec(`CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    type TEXT,
    from_user TEXT,
    to_user TEXT,
    content TEXT,
    timestamp DATETIME,
    reply_to TEXT,
    revoked BOOLEAN
)`)

// 支持的查询
GetUnreadByUser(userID)           // 离线消息
SearchByKeywordWithFilter(...)    // 关键词搜索
SearchByTimeRange(start, end)     // 时间范围搜索
```

---

## API 接口

### 用户注册

```bash
POST /register
Content-Type: application/json

{"username": "user1", "password": "123456"}
```

### 用户登录

```bash
POST /login
Content-Type: application/json

{"username": "user1", "password": "123456"}

# 返回
{"token": "eyJhbGciOiJIUzI1NiIs..."}
```

### WebSocket 连接

```bash
ws://localhost:8080/ws?token=JWT_TOKEN
```

### Prometheus 指标

```bash
GET /metrics
```

---

## 配置说明

### AI 配置

在 `bot.go` 中配置：

```go
const (
    ZhipuAPIKey    = "智谱API Key"
    DeepSeekAPIKey = "DeepSeek API Key"
)
```

获取地址：
- 智谱AI: https://open.bigmodel.cn/
- DeepSeek: https://platform.deepseek.com/

---

## 答辩要点

### 项目亮点

1. **完整的IM系统**：实现了即时通讯的核心功能
2. **AI深度融合**：不只是调用API，还有记忆、RAG、流式输出
3. **服务治理**：熔断、限流、降级三件套
4. **可观测性**：指标、日志、追踪完整链路
5. **内容安全**：敏感词+AI审核双重保障

### 技术难点

1. **WebSocket并发**：channel + goroutine 模式
2. **消息可靠性**：存储+离线推送+已读回执
3. **AI流式输出**：SSE协议解析+实时推送
4. **熔断器状态机**：closed → open → half-open

### 架构设计

1. **Hub模式**：中心化消息路由，易于扩展
2. **分层设计**：传输层(hub) → 业务层(group/friend/bot) → 数据层(storage)
3. **插件化AI**：Provider接口，支持多AI提供商

---

## 作者

AIM 项目由个人与Ai开发，用于学习和实践即时通讯系统开发。

## 许可证

MIT License
