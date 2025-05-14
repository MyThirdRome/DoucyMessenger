package messaging

import (
	"time"

	"github.com/doucya/utils"
)

// Group represents a messaging group
type Group struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Creator   string    `json:"creator"`
	CreatedAt time.Time `json:"created_at"`
	IsPublic  bool      `json:"is_public"`
	Members   []string  `json:"members"`
}

// NewGroup creates a new messaging group
func NewGroup(creator, name string, isPublic bool) *Group {
	group := &Group{
		ID:        utils.GenerateRandomID(),
		Name:      name,
		Creator:   creator,
		CreatedAt: time.Now(),
		IsPublic:  isPublic,
		Members:   []string{creator}, // Creator is automatically a member
	}
	
	return group
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
func (g *Group) RemoveMember(address string) {
	for i, member := range g.Members {
		if member == address {
			// Remove member by replacing with last element and truncating
			g.Members[i] = g.Members[len(g.Members)-1]
			g.Members = g.Members[:len(g.Members)-1]
			return
		}
	}
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

// GetMemberCount returns the number of members in the group
func (g *Group) GetMemberCount() int {
	return len(g.Members)
}

// GetCreator returns the creator of the group
func (g *Group) GetCreator() string {
	return g.Creator
}

// GetName returns the name of the group
func (g *Group) GetName() string {
	return g.Name
}

// GetID returns the ID of the group
func (g *Group) GetID() string {
	return g.ID
}

// IsPublicGroup returns whether the group is public
func (g *Group) IsPublicGroup() bool {
	return g.IsPublic
}