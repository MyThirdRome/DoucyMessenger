package interfaces

import (
	"time"
)

// MessageData defines the interface for message data in transactions
type MessageData interface {
	GetID() string
	GetSender() string
	GetReceiver() string
	GetContent() string
	GetTimestamp() time.Time
	GetSignature() string
	IsEncrypted() bool
}

// MessageMetadata contains basic message information
type MessageMetadata struct {
	ID        string    `json:"id"`
	Sender    string    `json:"sender"`
	Receiver  string    `json:"receiver"`
	Timestamp time.Time `json:"timestamp"`
}

// NewMessageMetadata creates a new message metadata object
func NewMessageMetadata(id, sender, receiver string, timestamp time.Time) *MessageMetadata {
	return &MessageMetadata{
		ID:        id,
		Sender:    sender,
		Receiver:  receiver,
		Timestamp: timestamp,
	}
}

// GetID returns the message ID
func (m *MessageMetadata) GetID() string {
	return m.ID
}

// GetSender returns the sender address
func (m *MessageMetadata) GetSender() string {
	return m.Sender
}

// GetReceiver returns the receiver address
func (m *MessageMetadata) GetReceiver() string {
	return m.Receiver
}

// GetTimestamp returns the message timestamp
func (m *MessageMetadata) GetTimestamp() time.Time {
	return m.Timestamp
}

// RewardCalculator defines the interface for calculating rewards
type RewardCalculator interface {
	CalculateValidatorReward(multiplier float64) float64
	GetSenderReward() float64
	GetReceiverReward() float64
}