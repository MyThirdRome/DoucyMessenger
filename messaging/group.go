package messaging

import (
        "encoding/json"
        "time"

        "github.com/doucya/wallet"
)

// Group represents a messaging group
type Group struct {
        ID        string    `json:"id"`
        Name      string    `json:"name"`
        Owner     string    `json:"owner"`
        IsPublic  bool      `json:"is_public"`
        CreatedAt time.Time `json:"created_at"`
        Members   []string  `json:"members"`
}

// NewGroup creates a new group
func NewGroup(owner, name string, isPublic bool) *Group {
        // Generate a group address
        groupID := wallet.GenerateGroupAddress()

        return &Group{
                ID:        groupID,
                Name:      name,
                Owner:     owner,
                IsPublic:  isPublic,
                CreatedAt: time.Now(),
                Members:   []string{owner}, // Owner is automatically a member
        }
}

// AddMember adds a member to the group
func (g *Group) AddMember(address string) bool {
        // Check if member already exists
        for _, member := range g.Members {
                if member == address {
                        return false // Already a member
                }
        }

        // Add member
        g.Members = append(g.Members, address)
        return true
}

// RemoveMember removes a member from the group
func (g *Group) RemoveMember(address string) bool {
        // Cannot remove owner
        if address == g.Owner {
                return false
        }

        // Find and remove member
        for i, member := range g.Members {
                if member == address {
                        // Remove by swapping with last element and truncating
                        g.Members[i] = g.Members[len(g.Members)-1]
                        g.Members = g.Members[:len(g.Members)-1]
                        return true
                }
        }

        return false // Member not found
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

// ChangeOwner changes the owner of the group
func (g *Group) ChangeOwner(newOwner string) bool {
        // Check if new owner is a member
        if !g.IsMember(newOwner) {
                return false
        }

        g.Owner = newOwner
        return true
}

// MarshalJSON customizes the JSON marshaling for Group
func (g *Group) MarshalJSON() ([]byte, error) {
        type GroupAlias Group
        return json.Marshal(&struct {
                CreatedAt string `json:"created_at"`
                *GroupAlias
        }{
                CreatedAt:  g.CreatedAt.Format(time.RFC3339),
                GroupAlias: (*GroupAlias)(g),
        })
}

// UnmarshalJSON customizes the JSON unmarshaling for Group
func (g *Group) UnmarshalJSON(data []byte) error {
        type GroupAlias Group
        aux := &struct {
                CreatedAt string `json:"created_at"`
                *GroupAlias
        }{
                GroupAlias: (*GroupAlias)(g),
        }

        if err := json.Unmarshal(data, &aux); err != nil {
                return err
        }

        createdAt, err := time.Parse(time.RFC3339, aux.CreatedAt)
        if err != nil {
                return err
        }
        g.CreatedAt = createdAt

        return nil
}

// GroupMessage represents a message sent to a group
type GroupMessage struct {
        Message
        GroupID string `json:"group_id"`
}

// NewGroupMessage creates a new group message
func NewGroupMessage(sender, groupID, content string) *GroupMessage {
        // Create base message
        baseMsg := NewMessage(sender, groupID, content)
        
        return &GroupMessage{
                Message: *baseMsg,
                GroupID: groupID,
        }
}
