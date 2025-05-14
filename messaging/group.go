package messaging

import (
        "crypto/ecdsa"
        "crypto/sha256"
        "encoding/hex"
        "encoding/json"
        "errors"
        "fmt"
        "sync"
        "time"

        "github.com/doucya/utils"
)

// Group represents a messaging group
type Group struct {
        ID          string            `json:"id"`
        Name        string            `json:"name"`
        Creator     string            `json:"creator"`
        CreatedAt   time.Time         `json:"created_at"`
        IsPublic    bool              `json:"is_public"`
        Members     []string          `json:"members"`
        Admins      []string          `json:"admins"`
        Description string            `json:"description"`
        Tags        []string          `json:"tags"`
        Metadata    map[string]string `json:"metadata"`
}

// GroupManager handles operations for message groups
type GroupManager struct {
        groups       map[string]*Group // ID -> Group
        userGroups   map[string][]string // User address -> list of group IDs
        mutex        sync.RWMutex
        maxUserGroups int // Maximum number of groups a user can create
}

// NewGroupManager creates a new group manager
func NewGroupManager() *GroupManager {
        return &GroupManager{
                groups:        make(map[string]*Group),
                userGroups:    make(map[string][]string),
                maxUserGroups: 20, // Default limit on group creation per user
        }
}

// NewGroup creates a new messaging group
func NewGroup(creator, name, description string, isPublic bool, tags []string) *Group {
        group := &Group{
                ID:          utils.GenerateRandomID(),
                Name:        name,
                Creator:     creator,
                CreatedAt:   time.Now(),
                IsPublic:    isPublic,
                Members:     []string{creator}, // Creator is automatically a member
                Admins:      []string{creator}, // Creator is automatically an admin
                Description: description,
                Tags:        tags,
                Metadata:    make(map[string]string),
        }
        
        return group
}

// CreateGroup creates a new group in the manager
func (gm *GroupManager) CreateGroup(creator, name, description string, isPublic bool, tags []string) (*Group, error) {
        gm.mutex.Lock()
        defer gm.mutex.Unlock()
        
        // Check if user has reached their group creation limit
        if groups, exists := gm.userGroups[creator]; exists && len(groups) >= gm.maxUserGroups {
                return nil, fmt.Errorf("user has reached the maximum of %d groups", gm.maxUserGroups)
        }
        
        // Create new group
        group := NewGroup(creator, name, description, isPublic, tags)
        
        // Add to manager
        gm.groups[group.ID] = group
        
        // Add to user's group list
        if _, exists := gm.userGroups[creator]; !exists {
                gm.userGroups[creator] = []string{}
        }
        gm.userGroups[creator] = append(gm.userGroups[creator], group.ID)
        
        return group, nil
}

// GetGroup retrieves a group by ID
func (gm *GroupManager) GetGroup(groupID string) (*Group, error) {
        gm.mutex.RLock()
        defer gm.mutex.RUnlock()
        
        group, exists := gm.groups[groupID]
        if !exists {
                return nil, fmt.Errorf("group not found: %s", groupID)
        }
        
        return group, nil
}

// GetUserGroups returns all groups a user is a member of
func (gm *GroupManager) GetUserGroups(address string) []*Group {
        gm.mutex.RLock()
        defer gm.mutex.RUnlock()
        
        var result []*Group
        
        // Check all groups for membership
        for _, group := range gm.groups {
                if group.IsMember(address) {
                        result = append(result, group)
                }
        }
        
        return result
}

// AddMember adds a member to the group
func (g *Group) AddMember(address string) {
        // Check if already a member
        for _, member := range g.Members {
                if member == address {
                        return
                }
        }
        
        g.Members = append(g.Members, address)
}

// RemoveMember removes a member from the group
func (g *Group) RemoveMember(address string, removerAddr string) error {
        // Only admins or the member themselves can remove
        if !g.IsAdmin(removerAddr) && removerAddr != address {
                return errors.New("only admins or the member themselves can remove a member")
        }
        
        // Cannot remove the creator
        if address == g.Creator {
                return errors.New("cannot remove the group creator")
        }
        
        for i, member := range g.Members {
                if member == address {
                        // Remove member by replacing with last element and truncating
                        g.Members[i] = g.Members[len(g.Members)-1]
                        g.Members = g.Members[:len(g.Members)-1]
                        
                        // Also remove from admins if applicable
                        for i, admin := range g.Admins {
                                if admin == address {
                                        g.Admins[i] = g.Admins[len(g.Admins)-1]
                                        g.Admins = g.Admins[:len(g.Admins)-1]
                                        break
                                }
                        }
                        
                        return nil
                }
        }
        
        return errors.New("member not found")
}

// IsMember checks if an address is a member of the group
func (g *Group) IsMember(address string) bool {
        for _, member := range g.Members {
                if member == address {
                        return true
                }
        }
        return false
}

// IsAdmin checks if an address is an admin of the group
func (g *Group) IsAdmin(address string) bool {
        for _, admin := range g.Admins {
                if admin == address {
                        return true
                }
        }
        return false
}

// AddAdmin adds an admin to the group
func (g *Group) AddAdmin(address string, promoterAddr string) error {
        // Only existing admins can add admins
        if !g.IsAdmin(promoterAddr) {
                return errors.New("only existing admins can add new admins")
        }
        
        // Check if already an admin
        if g.IsAdmin(address) {
                return nil
        }
        
        // Must be a member first
        if !g.IsMember(address) {
                return errors.New("user must be a member before becoming an admin")
        }
        
        g.Admins = append(g.Admins, address)
        return nil
}

// RemoveAdmin removes an admin from the group
func (g *Group) RemoveAdmin(address string, removerAddr string) error {
        // Only existing admins can remove admins
        if !g.IsAdmin(removerAddr) {
                return errors.New("only existing admins can remove admins")
        }
        
        // Cannot remove the creator from admin role
        if address == g.Creator {
                return errors.New("cannot remove creator from admin role")
        }
        
        for i, admin := range g.Admins {
                if admin == address {
                        // Remove admin by replacing with last element and truncating
                        g.Admins[i] = g.Admins[len(g.Admins)-1]
                        g.Admins = g.Admins[:len(g.Admins)-1]
                        return nil
                }
        }
        
        return errors.New("admin not found")
}

// SetMetadata sets a metadata value for the group
func (g *Group) SetMetadata(key, value string, updaterAddr string) error {
        // Only admins can update metadata
        if !g.IsAdmin(updaterAddr) {
                return errors.New("only admins can update group metadata")
        }
        
        if g.Metadata == nil {
                g.Metadata = make(map[string]string)
        }
        
        g.Metadata[key] = value
        return nil
}

// GetMemberCount returns the number of members in the group
func (g *Group) GetMemberCount() int {
        return len(g.Members)
}

// GroupMessage represents a message sent to a group
type GroupMessage struct {
        ID        string    `json:"id"`
        GroupID   string    `json:"group_id"`
        Sender    string    `json:"sender"`
        Content   string    `json:"content"`
        Timestamp time.Time `json:"timestamp"`
        Signature string    `json:"signature"`
        Encrypted bool      `json:"encrypted"`
}

// CreateGroupMessage creates a new group message
func CreateGroupMessage(groupID, sender, content string, encrypted bool, privateKey *ecdsa.PrivateKey) (*GroupMessage, error) {
        // Generate message ID
        messageID := utils.GenerateRandomID()
        
        // Create message
        message := &GroupMessage{
                ID:        messageID,
                GroupID:   groupID,
                Sender:    sender,
                Content:   content,
                Timestamp: time.Now(),
                Encrypted: encrypted,
        }
        
        // Sign message
        messageJSON, err := json.Marshal(message)
        if err != nil {
                return nil, fmt.Errorf("failed to marshal message: %v", err)
        }
        
        messageHash := sha256.Sum256(messageJSON)
        signature, err := ecdsa.SignASN1(nil, privateKey, messageHash[:])
        if err != nil {
                return nil, fmt.Errorf("failed to sign message: %v", err)
        }
        
        message.Signature = hex.EncodeToString(signature)
        return message, nil
}