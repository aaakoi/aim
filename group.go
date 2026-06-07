package main

import "sync"

// Group 群组结构体
type Group struct {
	ID        string            // 群ID
	Name      string            // 群名
	Owner     string            // 群主ID
	Members   []string          // 群成员ID列表
	Muted     map[string]bool   // 被禁言的成员
	Admins    map[string]bool   // 管理员
	Announce  string            // 群公告
}

// GroupManager 群组管理器
type GroupManager struct {
	groups     map[string]*Group
	invites    map[string][]string // 用户收到的群邀请: userID -> []groupName
	mu         sync.RWMutex
}

// NewGroupManager 创建群组管理器
func NewGroupManager() *GroupManager {
	return &GroupManager{
		groups:  make(map[string]*Group),
		invites: make(map[string][]string),
	}
}

// CreateGroup 创建群组
func (gm *GroupManager) CreateGroup(name, owner string) *Group {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if _, exists := gm.groups[name]; exists {
		return nil
	}

	group := &Group{
		ID:      name,
		Name:    name,
		Owner:   owner,
		Members: []string{owner},
		Muted:   make(map[string]bool),
		Admins:  make(map[string]bool),
	}
	gm.groups[name] = group
	return group
}

// JoinGroup 加入群组
func (gm *GroupManager) JoinGroup(name, userID string) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	group, exists := gm.groups[name]
	if !exists {
		return false
	}

	for _, member := range group.Members {
		if member == userID {
			return true
		}
	}

	group.Members = append(group.Members, userID)
	return true
}

// KickMember 踢出成员
func (gm *GroupManager) KickMember(groupName, operator, target string) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	group, exists := gm.groups[groupName]
	if !exists {
		return false
	}

	// 只有群主和管理员能踢人
	if group.Owner != operator && !group.Admins[operator] {
		return false
	}

	// 不能踢群主
	if target == group.Owner {
		return false
	}

	// 从成员列表删除
	for i, member := range group.Members {
		if member == target {
			group.Members = append(group.Members[:i], group.Members[i+1:]...)
			delete(group.Muted, target)
			delete(group.Admins, target)
			return true
		}
	}
	return false
}

// MuteMember 禁言成员
func (gm *GroupManager) MuteMember(groupName, operator, target string) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	group, exists := gm.groups[groupName]
	if !exists {
		return false
	}

	// 只有群主和管理员能禁言
	if group.Owner != operator && !group.Admins[operator] {
		return false
	}

	// 不能禁言群主
	if target == group.Owner {
		return false
	}

	// 检查目标是否在群里
	inGroup := false
	for _, member := range group.Members {
		if member == target {
			inGroup = true
			break
		}
	}
	if !inGroup {
		return false
	}

	group.Muted[target] = true
	return true
}

// UnmuteMember 解除禁言
func (gm *GroupManager) UnmuteMember(groupName, operator, target string) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	group, exists := gm.groups[groupName]
	if !exists {
		return false
	}

	if group.Owner != operator && !group.Admins[operator] {
		return false
	}

	delete(group.Muted, target)
	return true
}

// IsMuted 检查是否被禁言
func (gm *GroupManager) IsMuted(groupName, userID string) bool {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	group, exists := gm.groups[groupName]
	if !exists {
		return false
	}
	return group.Muted[userID]
}

// TransferOwner 转让群主
func (gm *GroupManager) TransferOwner(groupName, owner, newOwner string) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	group, exists := gm.groups[groupName]
	if !exists {
		return false
	}

	// 只有群主能转让
	if group.Owner != owner {
		return false
	}

	// 检查新群主是否在群里
	inGroup := false
	for _, member := range group.Members {
		if member == newOwner {
			inGroup = true
			break
		}
	}
	if !inGroup {
		return false
	}

	group.Owner = newOwner
	delete(group.Admins, newOwner) // 新群主不再是管理员
	return true
}

// SetAnnounce 设置群公告
func (gm *GroupManager) SetAnnounce(groupName, operator, announce string) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	group, exists := gm.groups[groupName]
	if !exists {
		return false
	}

	// 只有群主和管理员能设置公告
	if group.Owner != operator && !group.Admins[operator] {
		return false
	}

	group.Announce = announce
	return true
}

// GetAnnounce 获取群公告
func (gm *GroupManager) GetAnnounce(groupName string) string {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	group, exists := gm.groups[groupName]
	if !exists {
		return ""
	}
	return group.Announce
}

// SetAdmin 设置管理员
func (gm *GroupManager) SetAdmin(groupName, owner, target string) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	group, exists := gm.groups[groupName]
	if !exists {
		return false
	}

	if group.Owner != owner {
		return false
	}

	// 检查目标是否在群里
	inGroup := false
	for _, member := range group.Members {
		if member == target {
			inGroup = true
			break
		}
	}
	if !inGroup {
		return false
	}

	group.Admins[target] = true
	return true
}

// RemoveAdmin 移除管理员
func (gm *GroupManager) RemoveAdmin(groupName, owner, target string) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	group, exists := gm.groups[groupName]
	if !exists {
		return false
	}

	if group.Owner != owner {
		return false
	}

	delete(group.Admins, target)
	return true
}

// IsAdmin 检查是否是管理员
func (gm *GroupManager) IsAdmin(groupName, userID string) bool {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	group, exists := gm.groups[groupName]
	if !exists {
		return false
	}
	return group.Admins[userID]
}

// GetGroup 获取群组
func (gm *GroupManager) GetGroup(name string) *Group {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return gm.groups[name]
}

// GetMembers 获取群成员
func (gm *GroupManager) GetMembers(groupName string) []string {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	group, exists := gm.groups[groupName]
	if !exists {
		return nil
	}

	members := make([]string, len(group.Members))
	copy(members, group.Members)
	return members
}

// IsInGroup 检查用户是否在群里
func (gm *GroupManager) IsInGroup(groupName, userID string) bool {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	group, exists := gm.groups[groupName]
	if !exists {
		return false
	}

	for _, member := range group.Members {
		if member == userID {
			return true
		}
	}
	return false
}

// InviteMember 邀请用户加入群
func (gm *GroupManager) InviteMember(groupName, operator, target string) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	group, exists := gm.groups[groupName]
	if !exists {
		return false
	}

	// 只有群主和管理员能邀请
	if group.Owner != operator && !group.Admins[operator] {
		return false
	}

	// 检查目标是否已在群里
	for _, member := range group.Members {
		if member == target {
			return false // 已在群里
		}
	}

	// 检查是否已发送邀请
	for _, inv := range gm.invites[target] {
		if inv == groupName {
			return true // 已邀请过
		}
	}

	// 添加邀请
	gm.invites[target] = append(gm.invites[target], groupName)
	return true
}

// GetInvites 获取用户收到的群邀请
func (gm *GroupManager) GetInvites(userID string) []string {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	invites := make([]string, len(gm.invites[userID]))
	copy(invites, gm.invites[userID])
	return invites
}

// AcceptInvite 接受群邀请
func (gm *GroupManager) AcceptInvite(userID, groupName string) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	group, exists := gm.groups[groupName]
	if !exists {
		return false
	}

	// 检查是否有邀请
	hasInvite := false
	newInvites := []string{}
	for _, inv := range gm.invites[userID] {
		if inv == groupName {
			hasInvite = true
		} else {
			newInvites = append(newInvites, inv)
		}
	}
	if !hasInvite {
		return false
	}

	// 加入群
	group.Members = append(group.Members, userID)
	gm.invites[userID] = newInvites
	return true
}

// RejectInvite 拒绝群邀请
func (gm *GroupManager) RejectInvite(userID, groupName string) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	newInvites := []string{}
	found := false
	for _, inv := range gm.invites[userID] {
		if inv == groupName {
			found = true
		} else {
			newInvites = append(newInvites, inv)
		}
	}
	gm.invites[userID] = newInvites
	return found
}
