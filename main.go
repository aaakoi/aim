package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var hub = NewHub()
var userManager = NewUserManager()

func main() {
	// ========== 初始化可观测性 ==========
	// 初始化日志
	logger := InitLogger()
	defer logger.Sync()

	// 初始化链路追踪
	shutdown, err := InitTracerSimple("aim-server")
	if err != nil {
		log.Fatal("链路追踪初始化失败:", err)
	}
	defer shutdown(nil)

	logger.Info("可观测性组件初始化完成")

	// 启动 Hub
	go hub.Run()

	// 创建上传目录
	os.MkdirAll("./uploads", os.ModePerm)

	// 创建 Gin 路由
	r := gin.Default()

	// ========== 首页 ==========
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "AIM 即时通讯服务运行中",
			"version": "1.0.0",
			"endpoints": map[string]string{
				"register":   "POST /register",
				"login":      "POST /login",
				"websocket":  "GET /ws?token=xxx",
				"metrics":    "GET /metrics",
				"prometheus": "http://localhost:9090",
				"grafana":    "http://localhost:3000",
			},
		})
	})

	// ========== 添加监控端点 ==========
	// Prometheus 指标端点
	r.GET("/metrics", gin.WrapH(MetricsHandler()))

	// 注册接口
	r.POST("/register", func(c *gin.Context) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
			return
		}

		err := userManager.Register(req.Username, req.Password)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "注册成功"})
	})

	// 登录接口
	r.POST("/login", func(c *gin.Context) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
			return
		}

		token, err := userManager.Login(req.Username, req.Password)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": token})
	})

	// 文件上传接口
	r.POST("/upload", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "未上传文件"})
			return
		}

		// 生成唯一文件名
		ext := filepath.Ext(file.Filename)
		filename := uuid.New().String() + ext
		savePath := "./uploads/" + filename

		// 保存文件
		if err := c.SaveUploadedFile(file, savePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
			return
		}

		// 返回文件URL
		c.JSON(http.StatusOK, gin.H{
			"url":      "http://localhost:8080/uploads/" + filename,
			"filename": file.Filename,
		})
	})

	// 静态文件服务
	r.Static("/uploads", "./uploads")

	// WebSocket 连接（需要 Token）
	r.GET("/ws", func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少Token"})
			return
		}

		// 验证 Token
		username, err := userManager.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的Token"})
			return
		}

		// 升级为 WebSocket 连接
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Print("升级失败:", err)
			return
		}

		client := &Client{
			ID:   username,
			Send: make(chan []byte, 256),
			conn: conn,
			hub:  hub,
		}

		hub.register <- client
		go client.WritePump()
		go client.ReadPump()
	})

	log.Println("服务器启动在 http://localhost:8080")
	r.Run(":8080")
}
