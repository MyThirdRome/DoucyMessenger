package messaging

import (
	"sync"
	"time"
)

// RewardTracker tracks message rewards
type RewardTracker struct {
	mu                sync.RWMutex
	pendingRewards    map[string]map[string]float64 // sender -> receiver -> amount
	processedMessages map[string]bool              // message ID -> processed
	senderReward      float64
	receiverReward    float64
}

// NewRewardTracker creates a new reward tracker
func NewRewardTracker(senderReward, receiverReward float64) *RewardTracker {
	return &RewardTracker{
		pendingRewards:    make(map[string]map[string]float64),
		processedMessages: make(map[string]bool),
		senderReward:      senderReward,
		receiverReward:    receiverReward,
	}
}

// AddMessageReward adds a pending reward for a message
func (rt *RewardTracker) AddMessageReward(message *Message, isPenalty bool) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	// Check if already processed
	if rt.processedMessages[message.ID] {
		return
	}

	sender := message.Sender
	receiver := message.Receiver

	if isPenalty {
		// Sender pays a fee instead of getting a reward
		// This is handled elsewhere in the blockchain
	} else {
		// Add sender reward (pending until receiver responds)
		if _, exists := rt.pendingRewards[sender]; !exists {
			rt.pendingRewards[sender] = make(map[string]float64)
		}
		rt.pendingRewards[sender][receiver] = rt.senderReward

		// Receiver reward is processed immediately in the blockchain
	}

	// Mark as processed
	rt.processedMessages[message.ID] = true
}

// GetPendingRewards returns all pending rewards
func (rt *RewardTracker) GetPendingRewards() map[string]map[string]float64 {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	// Create a copy to prevent concurrent modification
	result := make(map[string]map[string]float64, len(rt.pendingRewards))
	for sender, receivers := range rt.pendingRewards {
		result[sender] = make(map[string]float64, len(receivers))
		for receiver, amount := range receivers {
			result[sender][receiver] = amount
		}
	}

	return result
}

// ProcessReward processes a reward when a response is received
func (rt *RewardTracker) ProcessReward(sender, receiver string) (float64, bool) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	// Check if reward exists
	rewards, exists := rt.pendingRewards[sender]
	if !exists {
		return 0, false
	}

	amount, exists := rewards[receiver]
	if !exists {
		return 0, false
	}

	// Remove reward
	delete(rewards, receiver)
	if len(rewards) == 0 {
		delete(rt.pendingRewards, sender)
	}

	return amount, true
}

// MessageRewardBundle holds all data for a message reward
type MessageRewardBundle struct {
	Message        *Message
	SenderReward   float64
	ReceiverReward float64
	IsPenalty      bool
	Timestamp      time.Time
}

// NewMessageRewardBundle creates a new message reward bundle
func NewMessageRewardBundle(message *Message, senderReward, receiverReward float64, isPenalty bool) *MessageRewardBundle {
	return &MessageRewardBundle{
		Message:        message,
		SenderReward:   senderReward,
		ReceiverReward: receiverReward,
		IsPenalty:      isPenalty,
		Timestamp:      time.Now(),
	}
}

// CalculateValidatorReward calculates the validator reward for a message
func (rb *MessageRewardBundle) CalculateValidatorReward(multiplier float64) float64 {
	if rb.IsPenalty {
		// For penalties, validators get the sender's fee
		return rb.SenderReward * multiplier
	}

	// For normal messages, validators get a percentage of receiver reward
	return rb.ReceiverReward * multiplier
}
