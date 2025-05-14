package messaging

import (
	"encoding/json"
	"time"

	"github.com/doucya/wallet"
)

// Channel represents a messaging channel (one-way communication)
type Channel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Owner       string    `json:"owner"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
	Subscribers []string  `json:"subscribers"`
	Admins      []string  `json:"admins"`
}

// NewChannel creates a new channel
func NewChannel(owner, name string, isPublic bool) *Channel {
	// Generate a channel address
	channelID := wallet.GenerateGroupAddress()

	return &Channel{
		ID:          channelID,
		Name:        name,
		Owner:       owner,
		IsPublic:    isPublic,
		CreatedAt:   time.Now(),
		Subscribers: []string{owner}, // Owner is automatically a subscriber
		Admins:      []string{owner}, // Owner is automatically an admin
	}
}

// AddSubscriber adds a subscriber to the channel
func (c *Channel) AddSubscriber(address string) bool {
	// Check if subscriber already exists
	for _, subscriber := range c.Subscribers {
		if subscriber == address {
			return false // Already a subscriber
		}
	}

	// Add subscriber
	c.Subscribers = append(c.Subscribers, address)
	return true
}

// RemoveSubscriber removes a subscriber from the channel
func (c *Channel) RemoveSubscriber(address string) bool {
	// Cannot remove owner
	if address == c.Owner {
		return false
	}

	// Find and remove subscriber
	for i, subscriber := range c.Subscribers {
		if subscriber == address {
			// Remove by swapping with last element and truncating
			c.Subscribers[i] = c.Subscribers[len(c.Subscribers)-1]
			c.Subscribers = c.Subscribers[:len(c.Subscribers)-1]
			
			// Also remove from admins if they are an admin
			c.RemoveAdmin(address)
			
			return true
		}
	}

	return false // Subscriber not found
}

// IsSubscriber checks if an address is a subscriber to the channel
func (c *Channel) IsSubscriber(address string) bool {
	for _, subscriber := range c.Subscribers {
		if subscriber == address {
			return true
		}
	}
	return false
}

// AddAdmin adds an admin to the channel
func (c *Channel) AddAdmin(address string) bool {
	// Check if address is a subscriber
	if !c.IsSubscriber(address) {
		return false
	}

	// Check if admin already exists
	for _, admin := range c.Admins {
		if admin == address {
			return false // Already an admin
		}
	}

	// Add admin
	c.Admins = append(c.Admins, address)
	return true
}

// RemoveAdmin removes an admin from the channel
func (c *Channel) RemoveAdmin(address string) bool {
	// Cannot remove owner
	if address == c.Owner {
		return false
	}

	// Find and remove admin
	for i, admin := range c.Admins {
		if admin == address {
			// Remove by swapping with last element and truncating
			c.Admins[i] = c.Admins[len(c.Admins)-1]
			c.Admins = c.Admins[:len(c.Admins)-1]
			return true
		}
	}

	return false // Admin not found
}

// IsAdmin checks if an address is an admin of the channel
func (c *Channel) IsAdmin(address string) bool {
	for _, admin := range c.Admins {
		if admin == address {
			return true
		}
	}
	return false
}

// ChangeOwner changes the owner of the channel
func (c *Channel) ChangeOwner(newOwner string) bool {
	// Check if new owner is a subscriber
	if !c.IsSubscriber(newOwner) {
		return false
	}

	c.Owner = newOwner
	
	// Make sure new owner is an admin
	if !c.IsAdmin(newOwner) {
		c.Admins = append(c.Admins, newOwner)
	}
	
	return true
}

// MarshalJSON customizes the JSON marshaling for Channel
func (c *Channel) MarshalJSON() ([]byte, error) {
	type ChannelAlias Channel
	return json.Marshal(&struct {
		CreatedAt string `json:"created_at"`
		*ChannelAlias
	}{
		CreatedAt:    c.CreatedAt.Format(time.RFC3339),
		ChannelAlias: (*ChannelAlias)(c),
	})
}

// UnmarshalJSON customizes the JSON unmarshaling for Channel
func (c *Channel) UnmarshalJSON(data []byte) error {
	type ChannelAlias Channel
	aux := &struct {
		CreatedAt string `json:"created_at"`
		*ChannelAlias
	}{
		ChannelAlias: (*ChannelAlias)(c),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	createdAt, err := time.Parse(time.RFC3339, aux.CreatedAt)
	if err != nil {
		return err
	}
	c.CreatedAt = createdAt

	return nil
}

// ChannelMessage represents a message sent to a channel
type ChannelMessage struct {
	Message
	ChannelID string `json:"channel_id"`
}

// NewChannelMessage creates a new channel message
func NewChannelMessage(sender, channelID, content string) *ChannelMessage {
	// Create base message
	baseMsg := NewMessage(sender, channelID, content)
	
	return &ChannelMessage{
		Message:   *baseMsg,
		ChannelID: channelID,
	}
}
