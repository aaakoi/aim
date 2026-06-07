package main

import (
	"sort"
	"sync"
)

// FriendInfo 好友详细信息
type FriendInfo struct {
	UserID string
	Remark string // 备注名
	Group  string // 分组名
}

// FriendManager 好友管理器
type FriendManager struct {
	friends  map[string][]string          // 用户ID -> 好友ID列表
	requests map[string][]string          // 用户ID -> 收到的好友请求
	remarks  map[string]map[string]string // 用户ID -> {好友ID -> 备注名}
	groups   map[string]map[string]string // 用户ID -> {好友ID -> 分组名}
	mu       sync.RWMutex
}

// NewFriendManager 创建好友管理器
func NewFriendManager() *FriendManager {
	return &FriendManager{
		friends:  make(map[string][]string),
		requests: make(map[string][]string),
		remarks:  make(map[string]map[string]string),
		groups:   make(map[string]map[string]string),
	}
}

// AddRequest 添加好友请求
func (fm *FriendManager) AddRequest(from, to string) bool {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if from == to {
		return false
	}

	if fm.isFriend(from, to) {
		return false
	}

	for _, req := range fm.requests[to] {
		if req == from {
			return false
		}
	}

	fm.requests[to] = append(fm.requests[to], from)
	return true
}

// GetRequests 获取好友请求列表
func (fm *FriendManager) GetRequests(userID string) []string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	requests, exists := fm.requests[userID]
	if !exists {
		return []string{}
	}
	return requests
}

// AcceptRequest 接受好友请求
func (fm *FriendManager) AcceptRequest(userID, target string) bool {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	hasRequest := false
	for i, req := range fm.requests[userID] {
		if req == target {
			fm.requests[userID] = append(fm.requests[userID][:i], fm.requests[userID][i+1:]...)
			hasRequest = true
			break
		}
	}

	if !hasRequest {
		return false
	}

	fm.friends[userID] = append(fm.friends[userID], target)
	fm.friends[target] = append(fm.friends[target], userID)
	return true
}

// RemoveFriend 删除好友
func (fm *FriendManager) RemoveFriend(userID, target string) bool {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if !fm.isFriend(userID, target) {
		return false
	}

	fm.removeFriendFromList(userID, target)
	fm.removeFriendFromList(target, userID)

	// 删除备注和分组
	if fm.remarks[userID] != nil {
		delete(fm.remarks[userID], target)
	}
	if fm.groups[userID] != nil {
		delete(fm.groups[userID], target)
	}
	return true
}

// GetFriends 获取好友ID列表
func (fm *FriendManager) GetFriends(userID string) []string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	friends, exists := fm.friends[userID]
	if !exists {
		return []string{}
	}
	return friends
}

// GetFriendInfos 获取好友详细信息列表（含备注、分组）
func (fm *FriendManager) GetFriendInfos(userID string) []FriendInfo {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	friends, exists := fm.friends[userID]
	if !exists {
		return []FriendInfo{}
	}

	result := make([]FriendInfo, 0, len(friends))
	for _, friendID := range friends {
		info := FriendInfo{UserID: friendID}
		if fm.remarks[userID] != nil {
			info.Remark = fm.remarks[userID][friendID]
		}
		if fm.groups[userID] != nil {
			info.Group = fm.groups[userID][friendID]
		}
		result = append(result, info)
	}
	return result
}

// IsFriend 检查是否是好友
func (fm *FriendManager) IsFriend(userID, target string) bool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.isFriend(userID, target)
}

// SetRemark 设置好友备注
func (fm *FriendManager) SetRemark(userID, friendID, remark string) bool {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if !fm.isFriend(userID, friendID) {
		return false
	}

	if fm.remarks[userID] == nil {
		fm.remarks[userID] = make(map[string]string)
	}
	fm.remarks[userID][friendID] = remark
	return true
}

// GetRemark 获取好友备注
func (fm *FriendManager) GetRemark(userID, friendID string) string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if fm.remarks[userID] == nil {
		return ""
	}
	return fm.remarks[userID][friendID]
}

// SetGroup 设置好友分组
func (fm *FriendManager) SetGroup(userID, friendID, group string) bool {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if !fm.isFriend(userID, friendID) {
		return false
	}

	if fm.groups[userID] == nil {
		fm.groups[userID] = make(map[string]string)
	}
	fm.groups[userID][friendID] = group
	return true
}

// GetGroup 获取好友分组
func (fm *FriendManager) GetGroup(userID, friendID string) string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if fm.groups[userID] == nil {
		return ""
	}
	return fm.groups[userID][friendID]
}

// GetFriendsByGroup 按分组获取好友
func (fm *FriendManager) GetFriendsByGroup(userID, group string) []FriendInfo {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	friends, exists := fm.friends[userID]
	if !exists {
		return []FriendInfo{}
	}

	var result []FriendInfo
	for _, friendID := range friends {
		friendGroup := ""
		if fm.groups[userID] != nil {
			friendGroup = fm.groups[userID][friendID]
		}
		if friendGroup == group {
			info := FriendInfo{UserID: friendID, Group: group}
			if fm.remarks[userID] != nil {
				info.Remark = fm.remarks[userID][friendID]
			}
			result = append(result, info)
		}
	}
	return result
}

// GetAllGroups 获取用户所有分组
func (fm *FriendManager) GetAllGroups(userID string) []string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if fm.groups[userID] == nil {
		return []string{}
	}

	groupSet := make(map[string]bool)
	for _, group := range fm.groups[userID] {
		if group != "" {
			groupSet[group] = true
		}
	}

	result := make([]string, 0, len(groupSet))
	for group := range groupSet {
		result = append(result, group)
	}
	sort.Strings(result)
	return result
}

// 内部方法，不加锁
func (fm *FriendManager) isFriend(userID, target string) bool {
	friends, exists := fm.friends[userID]
	if !exists {
		return false
	}
	for _, f := range friends {
		if f == target {
			return true
		}
	}
	return false
}

func (fm *FriendManager) removeFriendFromList(userID, target string) {
	friends := fm.friends[userID]
	for i, f := range friends {
		if f == target {
			fm.friends[userID] = append(friends[:i], friends[i+1:]...)
			return
		}
	}
}
