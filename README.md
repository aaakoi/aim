# AIM - 智能即时通讯系统

## 目录
- [项目简介](#一项目简介)
- [核心功能模块](#二核心功能模块)
- [技术架构](#三技术架构)
- [技术挑战与实现](#四技术挑战与实现)
- [快速开始](#五快速开始)
- [指令大全](#六指令大全)
- [部署指南](#七部署指南)
- [开发指南](#八开发指南)
- [项目结构](#九项目结构)
- [配置说明](#十配置说明)

---

## 一、项目简介

AIM 是一个基于 Go 语言开发的即时通讯系统，内置可自部署的 AI 助手，将大模型能力深度集成到聊天场景中，实现"通讯 + AI"的深度融合。

### 系统架构（简化版）

```
┌─────────────────────────────────────────────────────────────┐
│                      CLI 客户端 / WebSocket                    │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ WebSocket 连接
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  入口层                                                       │
│  main.go + Gin 路由                                          │
│  ├── HTTP API (注册/登录/上传)                                │
│  └── WebSocket (/ws)                                         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  业务层                                                       │
│                                                              │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐      │
│  │    Hub      │    │ BotManager  │    │   其他管理器 │      │
│  │  消息路由   │───▶│   AI 调用   │    │ 群组/好友/存储│      │
│  └─────────────┘    └─────────────┘    └─────────────┘      │
│        │                                                    │
│        ▼                                                    │
│  判断消息类型 → 私聊 / 群聊 / Bot                              │
└────────────────────────────── ───────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  数据层                                                       │
│  SQLite (消息 + 向量 + 用户)                                  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  外部服务                                                     │
│  智谱AI / DeepSeek / Prometheus / Grafana                    │
└─────────────────────────────────────────────────────────────┘
```

**核心流程**：用户消息 → WebSocket → Hub路由 →  SQLite存储→ AI/私聊/群聊

### 项目的四大架构亮点：

1. **AI 提供商接口抽象**
2. **Hub 广播路由模式**
3. **SSE 流式响应处理**
4. **命令系统集中处理**

### 核心特性

| 特性 | 说明 |
|-----|------|
| **AI 深度集成** | 智谱/DeepSeek 双提供商切换、流式输出打字机效果、跨会话记忆能力 |
| **RAG 知识库** | 支持 PDF/MD/DOCX 文档上传，向量嵌入检索，基于知识库精准回答 |
| **服务治理** | 熔断器（连续失败5次触发）、限流器（令牌桶QPS=10）、降级响应、重试机制 |
| **可观测性** | Prometheus 指标采集、Grafana 可视化看板、OpenTelemetry 链路追踪、zap 结构化日志 |
| **内容安全** | 敏感词过滤 + AI 审核双重保障，违规消息拦截不入库 |
| **消息增强** | 2分钟撤回、15分钟编辑、已读回执、离线推送、实时翻译 |
| **插件化 Bot** | Provider 接口抽象，新增 AI 提供商只需实现 3 个方法 |

### 技术栈

| 分类 | 技术 |
|-----|------|
| 顶层框架 | Gin |
| 编程语言 | Go 1.22+ |
| 实时通信 | websocket |
| 数据存储 | SQLite |
| 用户认证 | JWT+ bcrypt 密码加密 |
| 人工智能 | 智谱 AI GLM-4-flash / DeepSeek Chat |
| 容器化 | Docker + Docker Compose |
| 监控 | Prometheus + Grafana |
| 链路追踪 | OpenTelemetry |
| 日志 | zap |
| 向量存储 | SQLite + 余弦相似度检索 |

> 本 README 是项目概览，完整的技术笔记和架构已整理在语雀，链接见文末。
---

## 二、核心功能模块

### 2.1 基础通讯功能

提供即时通讯核心能力，所有用户可直接使用。

| 功能 | 语法示例 | 描述 |
|-----|---------|------|
| 私聊 | `@张三 你好` | 发送私聊消息给指定用户 |
| 群聊 | `#技术群 大家好` | 发送群聊消息 |
| 广播 | `大家好` | 发送给所有在线用户 |
| 消息撤回 | `/revoke a1b2c3d` | 2分钟内可撤回自己发的消息 |
| 消息编辑 | `/edit a1b2c3d 新内容` | 15分钟内可编辑消息内容 |
| 已读回执 | `/read a1b2c3d` | 标记消息已读，通知发送者 |
| 离线推送 | 上线自动推送 | 离线消息上线后自动推送 |
| 历史搜索 | `/search 关键词` | 搜索历史消息内容 |
| 时间搜索 | `/search today` | 查看今天的消息 |

### 2.2 AI 功能模块

#### 2.2.1 AI 对话

| 功能 | 语法示例 | 描述 |
|-----|---------|------|
| AI对话 | `@Bot 今天天气怎么样` | 与 AI 助手对话 |
| 流式输出 | 自动生效 | 打字机效果实时显示回复 |
| 记忆能力 | 自动生效 | AI 记住上下文，连续对话 |
| 切换模型 | `/provider deepseek` | 切换 AI 提供商 |
| 候选回复 | `/ask 这个问题怎么解决` | 生成多个候选答案 |
| 选择回复 | `/select 1 @张三` | 选择候选回复发送给用户 |
| 记忆管理 | `/memory` | 查看对话记忆条数 |
| 清空记忆 | `/memory_clear` | 清空与 Bot 的对话记忆 |

#### 2.2.2 RAG 知识库

用户上传文档构建私有知识库，Bot 基于知识库回答问题。

| 功能 | 语法示例 | 描述 |
|-----|---------|------|
| 创建知识库 | `/kb_create 项目文档` | 创建新知识库 |
| 添加文档 | `/kb_add 项目文档 ./spec.pdf` | 上传 PDF/MD/DOCX 文档 |
| 查询知识库 | `/kb_query 项目文档 部署流程` | 基于知识库回答问题 |
| 知识库列表 | `/kb_list` | 查看所有知识库 |
| 删除知识库 | `/kb_delete 项目文档` | 删除知识库 |

**技术实现：**
- 文档分块：500字符/块，重叠50字符
- 向量嵌入：调用智谱 AI Embedding API
- 检索方式：余弦相似度 Top-K 检索
- 存储位置：SQLite + vector BLOB

#### 2.2.3 计费系统

| 功能 | 语法示例 | 描述 |
|-----|---------|------|
| 查看余额 | `/quota` | 查看账户余额和用量统计 |
| 自动计费 | 自动生效 | Token 用量实时统计，费用计算 |

### 2.3 好友与群组管理

#### 2.3.1 好友管理

| 功能 | 语法示例 | 描述 |
|-----|---------|------|
| 添加好友 | `/add 张三` | 发送好友请求 |
| 接受好友 | `/accept 张三` | 接受好友请求 |
| 删除好友 | `/remove 张三` | 删除好友关系 |
| 好友列表 | `/friends` | 查看好友列表 |
| 设置备注 | `/setremark 张三 同事` | 设置好友备注名 |
| 好友分组 | `/setgroup 张三 工作` | 设置好友分组 |
| 查看分组 | `/groups` | 查看所有分组 |

#### 2.3.2 群组管理

| 功能 | 语法示例 | 描述 |
|-----|---------|------|
| 创建群 | `/创建 技术群` | 创建新群组 |
| 加入群 | `/加入 技术群` | 加入已有群组 |
| 退出群 | `/退群 技术群` | 退出群组 |
| 邀请成员 | `/invite 技术群 张三` | 邀请用户加入群 |
| 踢出成员 | `/kick 技术群 张三` | 踢出群成员（群主） |
| 禁言成员 | `/禁言 技术群 张三` | 禁言群成员（群主） |
| 解除禁言 | `/解禁 技术群 张三` | 解除禁言（群主） |
| 转让群主 | `/转让 技术群 张三` | 转让群主身份 |
| 设置公告 | `/公告 技术群 明天开会` | 设置群公告 |
| 查看成员 | `/群成员 技术群` | 查看群成员列表 |

### 2.4 安全与治理

#### 2.4.1 内容审核

敏感词过滤 + AI 审核双重保障，违规消息不入库、不转发。

**审核维度：**
- ✅ 敏感词检测（内置词库 + 可扩展）
- ✅ AI 审核可选（侮辱谩骂、色情低俗、暴力恐怖）
- ✅ 违规拦截后通知发送者

**管理命令：**
```
/moderation          # 查看审核状态
/moderation on       # 开启审核
/moderation off      # 关闭审核（测试用）
/addword 敏感词       # 添加敏感词
/delword 敏感词       # 删除敏感词
/words               # 查看敏感词列表
```

#### 2.4.2 服务治理

| 功能 | 配置值 | 描述 |
|-----|-------|------|
| 熔断触发 | 5次连续失败 | 触发后30秒内快速失败 |
| 限流阈值 | 10 QPS | 令牌桶算法，超限返回"请求频繁" |
| 降级响应 | 自动生效 | AI不可用时返回友好提示 |
| 重试机制 | 3次最大重试 | 5xx错误自动重试，指数退避 |

### 2.5 可观测性

#### 2.5.1 Prometheus 指标

| 指标 | 说明 |
|-----|------|
| `aim_online_users` | 当前在线用户数 |
| `aim_messages_total` | 消息吞吐量（单聊/群聊/广播） |
| `aim_bot_requests` | Bot 请求统计（成功/失败） |
| `aim_bot_latency` | Bot 响应延迟 |
| `aim_bot_tokens` | Token 使用量统计 |

#### 2.5.2 Grafana 看板

访问 http://localhost:3000（admin/admin），内置仪表盘展示：
- 实时在线人数曲线
- 消息类型分布饼图
- Bot 响应延迟热力图
- Token 消耗柱状图

---

## 三、技术架构

### 系统架构图

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

### 消息处理流程

```
用户 @机器人 发消息
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
         │         ├──→ 熔断检查（CircuitBreaker）
         │         ├──→ 限流检查（RateLimiter）
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

### 项目模块结构

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

### 我对项目的理解

这个项目是一个聊天系统，核心是消息转发：用户发消息 → Hub 收到 → 判断发给谁 → 转发出去。用了 WebSocket 保持长连接，goroutine 处理每个用户，channel 传递消息。三个关键点：长连接、并发处理、消息路由。

### 我学到的

- goroutine：轻量线程，每个用户连接用一个 goroutine 处理，互不干扰
- channel：消息管道，Hub 用 channel 接收消息，再用 channel 分发给各用户
- 接口：AIProvider 接口定义了 AI 调用的规范，换 AI 模型不用改主逻辑

### 下一步

尝试支持多人同时在线的压力测试。
---

## 四、技术挑战与实现

### 4.1 WebSocket 并发安全

**挑战：** 多用户同时连接、并发发送消息，如何保证数据一致性？

**解决方案：**

- Hub 使用 channel 驱动（register/unregister/broadcast）
- 每个 Client 独立 goroutine 处理读写（ReadPump/WritePump）
- 共享数据用 sync.RWMutex 保护（用户列表、群成员列表）

```go
type Hub struct {
    register   chan *Client    // 用户上线
    unregister chan *Client    // 用户下线
    broadcast  chan []byte     // 消息广播
    mu         sync.RWMutex    // 保护 userClients
}
```

### 4.2 AI 流式输出

**挑战：** 智谱/DeepSeek 流式 API 返回 SSE 格式，如何实时推送给客户端？

**解决方案：**
- 解析 SSE 格式：`data: {"choices":[{"delta":{"content":"xxx"}}]}`
- StreamMessage 结构通知客户端：stream_start/stream_chunk/stream_end
- glamour 库渲染 Markdown，实时更新 TUI 显示

```go
type StreamMessage struct {
    Type    string `json:"type"`    // "stream_start", "stream_chunk", "stream_end"
    Content string `json:"content"` // token 内容
    From    string `json:"from"`    // Bot ID
}
```

### 4.3 熔断器状态机

**挑战：** AI 服务不可用时如何保护系统？如何自动恢复？

**解决方案：**
- 三态状态机：closed(正常) → open(熔断) → half-open(试探)
- 连续失败5次触发熔断，30秒后尝试恢复
- half-open 状态放行1个请求，成功则关闭，失败则重新打开

```go
type CircuitBreaker struct {
    failures     int       // 连续失败次数
    threshold    int       // 熔断阈值 = 5
    state        string    // closed/open/half-open
    lastFailTime time.Time // 上次失败时间
    timeout      Duration  // 恢复时间 = 30s
}
```

---

## 五、快速开始

### 5.1 前置要求

- Go 1.22 或更高版本
- Docker + Docker Compose（推荐）
- 智谱 AI 或 DeepSeek API Key

### 5.2 Docker 部署（推荐）

```bash
# 克隆项目
git clone https://github.com/aaakoi/aim.git
cd aim

# 构建并启动所有服务
docker-compose up -d

# 查看运行状态
docker ps

# 查看日志
docker-compose logs -f aim
```

服务启动后：
- AIM 服务：localhost:8080
- Prometheus：http://localhost:9090
- Grafana：http://localhost:3000（admin/admin）

### 5.3 源码运行

```bash
# 安装依赖
go mod download

# 配置 API Key（编辑 bot.go）
const ZhipuAPIKey = "你的智谱API Key"
const DeepSeekAPIKey = "你的DeepSeek API Key"

# 启动服务端
go run .

# 新开终端，启动客户端
go run cmd/aim-cli/main.go -u 1   # 用户 user1
go run cmd/aim-cli/main.go -u 2   # 用户 user2
```

### 5.4 功能测试

```bash
# 启动两个客户端
./aim-cli.exe -u 1    # 终端1
./aim-cli.exe -u 2    # 终端2

# 测试私聊
@user2 你好

# 测试 AI 对话
@Bot 今天天气怎么样

# 测试群聊
/创建 test
/加入 test
#test 群消息测试

# 测试撤回
@user2 测试消息    # 发送后显示消息ID
/revoke 消息ID
```

---

## 六、指令大全

### 6.1 权限说明

| 级别 | 标志 | 说明 |
|-----|------|------|
| 公开 | 🟢 | 所有用户可使用 |
| 管理 | 🔴 | 仅群主/管理员可使用 |

### 6.2 完整指令表

#### 聊天指令

| 指令 | 权限 | 功能描述 |
|-----|------|---------|
| `@用户名 消息` | 🟢 | 私聊消息 |
| `#群名 消息` | 🟢 | 群聊消息 |
| `消息内容` | 🟢 | 广播消息（无前缀） |

#### 消息指令

| 指令 | 权限 | 功能描述 |
|-----|------|---------|
| `/revoke 消息ID` | 🟢 | 撤回消息（2分钟内） |
| `/edit 消息ID 内容` | 🟢 | 编辑消息（15分钟内） |
| `/read 消息ID` | 🟢 | 标记已读 |
| `/typing 用户` | 🟢 | 发送输入状态提示 |
| `/search 关键词` | 🟢 | 搜索历史消息 |
| `/search today` | 🟢 | 查看今天消息 |
| `/search yesterday` | 🟢 | 查看昨天消息 |
| `/history` | 🟢 | 查看历史消息 |
| `/translate 消息ID` | 🟢 | 翻译消息内容 |

#### 群组指令

| 指令 | 权限 | 功能描述 |
|-----|------|---------|
| `/创建 群名` | 🟢 | 创建群组 |
| `/加入 群名` | 🟢 | 加入群组 |
| `/退群 群名` | 🟢 | 退出群组 |
| `/invite 群名 用户` | 🟢 | 邀请成员 |
| `/kick 群名 用户` | 🔴 | 踢出成员（群主） |
| `/禁言 群名 用户` | 🔴 | 禁言成员（群主） |
| `/解禁 群名 用户` | 🔴 | 解除禁言（群主） |
| `/转让 群名 用户` | 🔴 | 转让群主 |
| `/公告 群名 内容` | 🔴 | 设置群公告 |
| `/群成员 群名` | 🟢 | 查看群成员 |

#### 好友指令

| 指令 | 权限 | 功能描述 |
|-----|------|---------|
| `/add 用户` | 🟢 | 添加好友 |
| `/accept 用户` | 🟢 | 接受好友请求 |
| `/remove 用户` | 🟢 | 删除好友 |
| `/friends` | 🟢 | 好友列表 |
| `/setremark 用户 备注` | 🟢 | 设置备注 |
| `/setgroup 用户 分组` | 🟢 | 设置分组 |
| `/groups` | 🟢 | 查看分组 |

#### AI 指令

| 指令 | 权限 | 功能描述 |
|-----|------|---------|
| `@Bot 消息` | 🟢 | AI对话 |
| `/provider zhipu` | 🟢 | 切换智谱AI |
| `/provider deepseek` | 🟢 | 切换DeepSeek |
| `/ask 问题` | 🟢 | 生成候选回复 |
| `/select 序号 [@用户]` | 🟢 | 选择候选回复发送 |
| `/quota` | 🟢 | 查看余额 |
| `/memory` | 🟢 | 查看记忆数量 |
| `/memory_clear` | 🟢 | 清空记忆 |
| `/summarize` | 🟢 | 总结历史消息 |
| `/todos` | 🟢 | 提取待办事项 |

#### 知识库指令

| 指令 | 权限 | 功能描述 |
|-----|------|---------|
| `/kb_create 名称` | 🟢 | 创建知识库 |
| `/kb_list` | 🟢 | 知识库列表 |
| `/kb_add 库名 文件路径` | 🟢 | 添加文档 |
| `/kb_query 库名 问题` | 🟢 | 查询知识库 |
| `/kb_delete 库名` | 🟢 | 删除知识库 |

#### 其他指令

| 指令 | 权限 | 功能描述 |
|-----|------|---------|
| `/online` | 🟢 | 在线用户列表 |
| `/帮助` | 🟢 | 显示帮助 |

---

## 七、部署指南

### 7.1 Docker Compose 配置

```yaml
version: '3.8'
services:
  aim:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./uploads:/app/uploads

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
```

### 7.2 配置 Prometheus

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'aim'
    static_configs:
      - targets: ['aim:8080']
```

---

## 八、开发指南

### 8.1 如何添加新的 AI 提供商

实现 `Provider` 接口：

```go
type Provider interface {
    Name() string
    CallAPI(content string) (string, error)
    CallAPIStream(content string, onToken func(string, bool)) error
}

// 新提供商
type MyProvider struct{}

func (p *MyProvider) Name() string { return "myprovider" }
func (p *MyProvider) CallAPI(content string) (string, error) {
    // 调用你的 API
}
func (p *MyProvider) CallAPIStream(content string, onToken func(string, bool)) error {
    // 流式调用
}

// 注册到 BotManager
bm.RegisterProvider("myprovider", &MyProvider{})
```

### 8.2 如何添加新功能

1. 在 `hub.go` 或 `client.go` 中添加处理逻辑
2. 更新 README 指令列表

---

## 九、项目结构

```
aim/
├── main.go              # 入口，HTTP路由
├── hub.go               # 消息路由中心
├── client.go            # WebSocket 连接处理
├── message.go           # 消息结构定义
├── storage.go           # SQLite 消息存储
├── user.go              # 用户认证(JWT)
├── group.go             # 群组管理
├── friend.go            # 好友管理
├── bot.go               # AI Bot 核心
├── ratelimit.go         # 熔断器 + 限流器
├── moderation.go        # 内容审核
├── translate.go         # 实时翻译
├── memory.go            # AI 记忆管理
├── billing.go           # 计费系统
├── metrics.go           # Prometheus 指标
├── middleware.go        # HTTP 中间件
├── tracing.go           # OpenTelemetry 配置
├── rag/                 # RAG 知识库
│   ├── rag.go
│   ├── embedding.go
│   └── chunk.go
├── cmd/aim-cli/         # TUI 客户端
├── test/                # 测试脚本
├── Dockerfile
├── docker-compose.yml
├── prometheus.yml
└── README.md
```
### 我对项目的理解

这个项目是一个基于 WebSocket 的即时通讯系统，它的核心设计是 Hub 模式——所有消息都通过 Hub 的 broadcast 通道汇聚，然后由 Hub 统一路由。它让客户端只管收发，不需要知道对方在不在线，也不用处理转发逻辑。

架构上分为三层：入口层是 main.go 的 Gin 路由，业务层是 hub.go、client.go、friend.go、group.go，数据层是 SQLite 和内存存储。用户认证用 JWT，AI Bot 用接口抽象，支持切换不同的 AI 提供商。

### 我学到的
1. WebSocket 的双向通信机制，以及它跟 HTTP 的区别。
2. Hub 模式如何解耦客户端，让路由和存储逻辑集中在服务端。
3. 并发编程方面，我理解了 channel + goroutine 配合使用，以及多个 goroutine 同时操作 map 会出问题。
4. 服务治理方面，我知道了熔断、限流、降级分别解决什么问题，也实现了基础的监控链路。
5. 接口方面AIProvider 接口定义了 AI 调用的规范，换 AI 模型不用改主逻辑
6. 用channel作消息管道，Hub 用 channel 接收消息，再用 channel 分发给各用户，避免并发问题

### 下一步

1. 把内存存储换成 MySQL，让消息持久化更可靠。
2. 把可观测性做得更完整，把链路追踪也串起来。
3. 把项目往分布式方向扩展，看看拆成微服务会有什么变化

---

## 十、配置说明

| 配置项 | 位置 | 说明 |
|-------|------|------|
| 智谱 API Key | bot.go:15 | 智谱 AI GLM-4-flash |
| DeepSeek API Key | bot.go:19 | DeepSeek Chat |
| 熔断阈值 | bot.go:31 | 连续失败5次触发 |
| 熔断超时 | bot.go:32 | 30秒后尝试恢复 |
| 限流 QPS | bot.go:30 | 每秒10个请求 |
| 重试次数 | bot.go:28 | 最大3次重试 |

---

## 交付产品

### 文档

| 文档 | 说明 |
|------|------|
| README.md | 项目完整说明文档 |
| Dockerfile | Docker 构建配置 |
| docker-compose.yml | 容器编排配置 |

### 📚 架构设计详解
本项目在开发过程中，对每一步功能的实现、数据流转、关键实现逻辑以及中间遇到的典型问题，均向ai提问并进行了系统性的梳理和记录，整理成语雀文档供参考。

https://www.yuque.com/g/momingmeimiao/zd8bcw/collaborator/join?token=grzdUq453UKQEMh8# 邀你加入知识库「学习之路」(可阅读)

### 演示资源

| 资源 | 说明 |
|------|------|
| Grafana 看板 | http://localhost:3000 |
| Prometheus 指标 | http://localhost:9090 |

### 仓库地址

- GitHub: https://github.com/aaakoi/aim