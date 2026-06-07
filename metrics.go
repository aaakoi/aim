package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ========== Prometheus 指标定义 ==========

import (
	"sync"
	"time"
)

// 消息速率计算器
type MessageRateCalculator struct {
	mu           sync.Mutex
	counts       []int
	timestamps   []time.Time
	windowSize   time.Duration
}

var messageRate = &MessageRateCalculator{
	counts:     make([]int, 0),
	timestamps: make([]time.Time, 0),
	windowSize: time.Minute,
}

func (m *MessageRateCalculator) AddMessage() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts = append(m.counts, 1)
	m.timestamps = append(m.timestamps, time.Now())
	m.cleanup()
}

func (m *MessageRateCalculator) cleanup() {
	now := time.Now()
	validIdx := 0
	for i, ts := range m.timestamps {
		if now.Sub(ts) <= m.windowSize {
			m.timestamps[validIdx] = m.timestamps[i]
			m.counts[validIdx] = m.counts[i]
			validIdx++
		}
	}
	m.timestamps = m.timestamps[:validIdx]
	m.counts = m.counts[:validIdx]
}

func (m *MessageRateCalculator) GetRate() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanup()
	if len(m.counts) == 0 {
		return 0
	}
	return float64(len(m.counts)) / m.windowSize.Seconds()
}

var (
	// 在线人数
	OnlineUsersGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "aim_online_users",
		Help: "Current online users count",
	})

	// 消息吞吐量（每秒消息数）
	MessagesPerSecond = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "aim_messages_per_second",
		Help: "Messages per second (1 minute window)",
	})

	// WebSocket连接总数
	WsConnectionsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aim_ws_connections_total",
		Help: "Total WebSocket connections",
	})

	// WebSocket断开总数
	WsDisconnectionsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aim_ws_disconnections_total",
		Help: "Total WebSocket disconnections",
	})

	// 消息总数
	MessagesCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "aim_messages_total",
		Help: "Total messages count",
	}, []string{"type"}) // type: single, group, broadcast

	// Bot响应延迟
	BotLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "aim_bot_latency_seconds",
		Help:    "Bot response latency in seconds",
		Buckets: []float64{.1, .5, 1, 2, 5, 10, 30},
	})

	// Bot请求总数
	BotRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "aim_bot_requests_total",
		Help: "Total bot requests",
	}, []string{"provider", "status"}) // provider: zhipu, deepseek; status: success, error

	// Bot Token使用量
	BotTokensTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "aim_bot_tokens_total",
		Help: "Total tokens used by bot",
	}, []string{"type", "provider"}) // type: prompt, completion

	// HTTP请求
	HttpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "aim_http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"endpoint", "method", "status"})

	// HTTP请求延迟
	HttpLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "aim_http_latency_seconds",
		Help:    "HTTP request latency in seconds",
		Buckets: []float64{.01, .05, .1, .5, 1, 2, 5},
	}, []string{"endpoint", "method"})

	// 数据库操作延迟
	DbLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "aim_db_latency_seconds",
		Help:    "Database operation latency in seconds",
		Buckets: []float64{.001, .005, .01, .05, .1, .5},
	}, []string{"operation"})

	// 数据库错误
	DbErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "aim_db_errors_total",
		Help: "Total database errors",
	}, []string{"operation"})
)

// ========== 指标记录辅助函数 ==========

// RecordMessage 记录消息
func RecordMessage(msgType string) {
	MessagesCounter.WithLabelValues(msgType).Inc()
	messageRate.AddMessage()
	MessagesPerSecond.Set(messageRate.GetRate())
}

// RecordBotLatency 记录Bot延迟
func RecordBotLatency(seconds float64) {
	BotLatency.Observe(seconds)
}

// RecordBotRequest 记录Bot请求
func RecordBotRequest(provider, status string) {
	BotRequestsTotal.WithLabelValues(provider, status).Inc()
}

// RecordBotTokens 记录Token使用
func RecordBotTokens(tokenType, provider string, count int) {
	BotTokensTotal.WithLabelValues(tokenType, provider).Add(float64(count))
}

// RecordHttpRequest 记录HTTP请求
func RecordHttpRequest(endpoint, method, status string) {
	HttpRequestsTotal.WithLabelValues(endpoint, method, status).Inc()
}

// RecordHttpLatency 记录HTTP延迟
func RecordHttpLatency(endpoint, method string, seconds float64) {
	HttpLatency.WithLabelValues(endpoint, method).Observe(seconds)
}

// IncOnlineUsers 增加在线用户
func IncOnlineUsers() {
	OnlineUsersGauge.Inc()
	WsConnectionsTotal.Inc()
}

// DecOnlineUsers 减少在线用户
func DecOnlineUsers() {
	OnlineUsersGauge.Dec()
	WsDisconnectionsTotal.Inc()
}
