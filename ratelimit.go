package main

import (
	"time"
)

// ========== 限流器（令牌桶算法）==========

// RateLimiter 限流器
type RateLimiter struct {
	tokens    chan struct{}
	qps       int
}

// NewRateLimiter 创建限流器
func NewRateLimiter(qps int) *RateLimiter {
	r := &RateLimiter{
		tokens: make(chan struct{}, qps),
		qps:    qps,
	}

	// 启动令牌生成协程
	go r.start()

	return r
}

// start 开始生成令牌
func (r *RateLimiter) start() {
	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		// 每秒往桶里放 qps 个令牌
		for i := 0; i < r.qps; i++ {
			select {
			case r.tokens <- struct{}{}:
			default:
				// 桶满了，丢弃
			}
		}
	}
}

// Allow 是否允许请求
func (r *RateLimiter) Allow() bool {
	select {
	case <-r.tokens:
		return true // 拿到令牌，允许
	default:
		return false // 没令牌，拒绝
	}
}

// ========== 熔断器 ==========

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	failures     int       // 连续失败次数
	threshold    int       // 熔断阈值
	state        string    // "closed", "open", "half-open"
	lastFailTime time.Time // 上次失败时间
	timeout      time.Duration // 熔断超时时间
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: threshold,
		timeout:   timeout,
		state:     "closed", // 初始状态：关闭（正常）
	}
}

// Call 执行请求（带熔断保护）
func (cb *CircuitBreaker) Call(fn func() error) error {
	// 熔断状态
	if cb.state == "open" {
		// 检查是否可以尝试恢复
		if time.Since(cb.lastFailTime) > cb.timeout {
			cb.state = "half-open"
		} else {
			return ErrCircuitOpen
		}
	}

	// 执行请求
	err := fn()

	if err != nil {
		cb.failures++
		if cb.failures >= cb.threshold {
			cb.state = "open"       // 触发熔断
			cb.lastFailTime = time.Now()
		}
		return err
	}

	// 成功：重置
	cb.failures = 0
	cb.state = "closed"
	return nil
}

// State 获取当前状态
func (cb *CircuitBreaker) State() string {
	return cb.state
}

// 错误定义
var ErrCircuitOpen = CircuitBreakerError{}

type CircuitBreakerError struct{}

func (e CircuitBreakerError) Error() string {
	return "熔断器已打开，请稍后重试"
}
