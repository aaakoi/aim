package main

import (
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// User 用户结构
type User struct {
	Username string
	Password string
	Quota    float64
}

// UserManager 用户管理器
type UserManager struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewUserManager 创建用户管理器
func NewUserManager() *UserManager {
	db, err := sql.Open("sqlite", "./messages.db")
	if err != nil {
		panic(err)
	}
	return &UserManager{
		db: db,
	}
}

// Register 用户注册
func (um *UserManager) Register(username, password string) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	// 检查用户名是否已存在
	var count int
	um.db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ?`, username).Scan(&count)
	if count > 0 {
		return errors.New("用户名已存在")
	}

	// 密码加密
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 存储到数据库，默认额度10元
	_, err = um.db.Exec(`
		INSERT INTO users (username, password, quota)
		VALUES (?, ?, 10.0)
	`, username, string(hashedPassword))

	return err
}

// Login 用户登录，返回JWT Token
func (um *UserManager) Login(username, password string) (string, error) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	// 从数据库查找用户
	var hashedPassword string
	err := um.db.QueryRow(`SELECT password FROM users WHERE username = ?`, username).Scan(&hashedPassword)
	if err == sql.ErrNoRows {
		return "", errors.New("用户不存在")
	}

	// 验证密码
	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		return "", errors.New("密码错误")
	}

	// 生成 JWT Token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString([]byte("aim-secret-key"))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken 验证Token，返回用户名
func (um *UserManager) ValidateToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte("aim-secret-key"), nil
	})

	if err != nil || !token.Valid {
		return "", errors.New("无效的Token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("无效的Token")
	}

	username, ok := claims["username"].(string)
	if !ok {
		return "", errors.New("无效的Token")
	}

	return username, nil
}

// GetUserQuota 获取用户额度
func (um *UserManager) GetUserQuota(username string) float64 {
	var quota float64
	um.db.QueryRow(`SELECT quota FROM users WHERE username = ?`, username).Scan(&quota)
	return quota
}

// SetUserQuota 设置用户额度
func (um *UserManager) SetUserQuota(username string, quota float64) {
	um.db.Exec(`UPDATE users SET quota = ? WHERE username = ?`, quota, username)
}
