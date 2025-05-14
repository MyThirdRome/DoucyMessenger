package messaging

import (
	"time"

	"github.com/doucya/utils"
)

// Channel represents a messaging channel
type Channel struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Creator   string    `json:"creator"`
	CreatedAt time.Time `json:"created_at"`
	IsPublic  bool      `json:"is_public"`
	Members   []string  `json:"members"`
	Messages  []string  `json:"messages"` // IDs of messages in this channel
}

// NewChannel creates a new messaging channel
func NewChannel(creator, name string, isPublic bool) *Channel {
	channel := &Channel{
		ID:        utils.GenerateRandomID(),
		Name:      name,
		Creator:   creator,
		CreatedAt: time.Now(),
		IsPublic:  isPublic,
		Members:   []string{creator}, // Creator is automatically a member
		Messages:  []string{},
	}
	
	return channel
}

// AddMember adds a member to the channel
func (c *Channel) AddMember(address string) {
	// Check if already a member
	for _, member := range c.Members {
		if member == address {
			return
		}
	}
	
	c.Members = append(c.Members, address)
}

// RemoveMember removes a member from the channel
func (c *Channel) RemoveMember(address string) {
	for i, member := range c.Members {
		if member == address {
			// Remove member by replacing with last element and truncating
			c.Members[i] = c.Members[len(c.Members)-1]
			c.Members = c.Members[:len(c.Members)-1]
			return
		}
	}
}

// AddMessage adds a message to the channel
func (c *Channel) AddMessage(messageID string) {
	c.Messages = append(c.Messages, messageID)
}

// IsMember checks if an address is a member of the channel
func (c *Channel) IsMember(address string) bool {
	for _, member := range c.Members {
		if member == address {
			return true
		}
	}
	
	return false
}

// GetMemberCount returns the number of members in the channel
func (c *Channel) GetMemberCount() int {
	return len(c.Members)
}

// GetCreator returns the creator of the channel
func (c *Channel) GetCreator() string {
	return c.Creator
}

// GetName returns the name of the channel
func (c *Channel) GetName() string {
	return c.Name
}

// GetID returns the ID of the channel
func (c *Channel) GetID() string {
	return c.ID
}

// IsPublicChannel returns whether the channel is public
func (c *Channel) IsPublicChannel() bool {
	return c.IsPublic
}

// GetMessageIDs returns the IDs of messages in the channel
func (c *Channel) GetMessageIDs() []string {
	return c.Messages
}

// GetMessageCount returns the number of messages in the channel
func (c *Channel) GetMessageCount() int {
	return len(c.Messages)
}