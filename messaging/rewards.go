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
)

// RewardTracker tracks message rewards
type RewardTracker struct {
        mu                sync.RWMutex
        pendingRewards    map[string]map[string]float64 // sender -> receiver -> amount
        processedMessages map[string]bool              // message ID -> processed
        senderReward      float64
        receiverReward    float64
        maxRewardsPerDay  int                          // limit total rewards per day
        dailyRewardCount  map[string]int               // address -> count of rewards received today
        lastResetDay      time.Time                    // tracks when daily rewards were last reset
}

// NewRewardTracker creates a new reward tracker
func NewRewardTracker(senderReward, receiverReward float64) *RewardTracker {
        return &RewardTracker{
                pendingRewards:    make(map[string]map[string]float64),
                processedMessages: make(map[string]bool),
                senderReward:      senderReward,
                receiverReward:    receiverReward,
                maxRewardsPerDay:  100, // Maximum rewards one address can receive per day
                dailyRewardCount:  make(map[string]int),
                lastResetDay:      time.Now(),
        }
}

// resetDailyCountersIfNeeded resets daily counters if it's a new day
func (rt *RewardTracker) resetDailyCountersIfNeeded() {
        today := time.Now()
        if today.YearDay() != rt.lastResetDay.YearDay() || today.Year() != rt.lastResetDay.Year() {
                rt.dailyRewardCount = make(map[string]int)
                rt.lastResetDay = today
        }
}

// canReceiveReward checks if an address has not exceeded daily reward limit
func (rt *RewardTracker) canReceiveReward(address string) bool {
        rt.resetDailyCountersIfNeeded()
        return rt.dailyRewardCount[address] < rt.maxRewardsPerDay
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
                // Check if sender can receive more rewards today
                if rt.canReceiveReward(sender) {
                        // Add sender reward (pending until receiver responds)
                        if _, exists := rt.pendingRewards[sender]; !exists {
                                rt.pendingRewards[sender] = make(map[string]float64)
                        }
                        rt.pendingRewards[sender][receiver] = rt.senderReward
                        
                        // Increment sender's daily reward count
                        rt.dailyRewardCount[sender]++
                }
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

        // Check if receiver can receive more rewards today
        if !rt.canReceiveReward(receiver) {
                return 0, false
        }

        // Remove reward
        delete(rewards, receiver)
        if len(rewards) == 0 {
                delete(rt.pendingRewards, sender)
        }

        // Increment receiver's daily reward count
        rt.dailyRewardCount[receiver]++

        return amount, true
}

// RewardTransaction represents a transaction for a message reward
type RewardTransaction struct {
        ID          string    `json:"id"`
        Timestamp   time.Time `json:"timestamp"`
        Sender      string    `json:"sender"`   // SYSTEM or validator address
        Receiver    string    `json:"receiver"` // address receiving the reward
        Amount      float64   `json:"amount"`
        MessageID   string    `json:"message_id"`
        OriginalSender string `json:"original_sender"`
        Type        string    `json:"type"` // "SENDER_REWARD" or "RECEIVER_REWARD"
        Signature   string    `json:"signature"`
}

// NewRewardTransaction creates a new reward transaction
func NewRewardTransaction(
        sender string,
        receiver string,
        amount float64,
        messageID string,
        originalSender string,
        rewardType string,
) *RewardTransaction {
        tx := &RewardTransaction{
                Timestamp:   time.Now(),
                Sender:      sender,
                Receiver:    receiver,
                Amount:      amount,
                MessageID:   messageID,
                OriginalSender: originalSender,
                Type:        rewardType,
        }
        
        // Generate transaction ID
        tx.ID = tx.GenerateID()
        
        return tx
}

// GenerateID generates a unique ID for the transaction
func (tx *RewardTransaction) GenerateID() string {
        // Create a hash of transaction details
        txData := struct {
                Timestamp   time.Time
                Sender      string
                Receiver    string
                Amount      float64
                MessageID   string
                OriginalSender string
                Type        string
        }{
                Timestamp:   tx.Timestamp,
                Sender:      tx.Sender,
                Receiver:    tx.Receiver,
                Amount:      tx.Amount,
                MessageID:   tx.MessageID,
                OriginalSender: tx.OriginalSender,
                Type:        tx.Type,
        }
        
        data, err := json.Marshal(txData)
        if err != nil {
                return fmt.Sprintf("%s_%s_%d", tx.Sender, tx.Receiver, time.Now().UnixNano())
        }
        
        hash := sha256.Sum256(data)
        return hex.EncodeToString(hash[:])
}

// Sign signs the transaction with the given private key
func (tx *RewardTransaction) Sign(privateKey *ecdsa.PrivateKey) error {
        if privateKey == nil {
                return errors.New("private key is nil")
        }
        
        // Sign the transaction ID
        signature, err := tx.signTransaction(privateKey, tx.ID)
        if err != nil {
                return fmt.Errorf("failed to sign reward transaction: %v", err)
        }
        
        tx.Signature = hex.EncodeToString(signature)
        return nil
}

// signTransaction signs data with the given private key
func (tx *RewardTransaction) signTransaction(privateKey *ecdsa.PrivateKey, data string) ([]byte, error) {
        if privateKey == nil {
                return nil, errors.New("private key is nil")
        }
        
        // In a real implementation, this would use a proper signing algorithm
        // For now, we'll sign the transaction ID
        r, s, err := ecdsa.Sign(nil, privateKey, []byte(data))
        if err != nil {
                return nil, err
        }
        
        // Combine r and s into a signature
        signature := append(r.Bytes(), s.Bytes()...)
        return signature, nil
}

// RewardType defines the type of reward
type RewardType string

const (
        SenderRewardType    RewardType = "SENDER_REWARD"
        ReceiverRewardType  RewardType = "RECEIVER_REWARD"
        ValidatorRewardType RewardType = "VALIDATOR_REWARD"
        PenaltyType         RewardType = "PENALTY"
)

// MessageRewardBundle holds all data for a message reward
type MessageRewardBundle struct {
        Message        *Message
        SenderReward   float64
        ReceiverReward float64
        IsPenalty      bool
        Timestamp      time.Time
        ResponseTo     string  // Optional ID of message this is responding to
        Validator      string  // Address of validator processing this reward
        SenderTx       *RewardTransaction
        ReceiverTx     *RewardTransaction
        ValidatorTx    *RewardTransaction
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

// SetResponseTo sets the message ID this message is responding to
func (rb *MessageRewardBundle) SetResponseTo(responseToID string) {
        rb.ResponseTo = responseToID
}

// SetValidator sets the validator processing this reward
func (rb *MessageRewardBundle) SetValidator(validatorAddress string) {
        rb.Validator = validatorAddress
}

// CalculateValidatorReward calculates the validator reward for a message
func (rb *MessageRewardBundle) CalculateValidatorReward(multiplier float64) float64 {
        if rb.IsPenalty {
                // For penalties, validators get the sender's fee
                return rb.SenderReward * multiplier
        }

        // For normal messages, validators get a percentage of total rewards
        totalReward := rb.SenderReward + rb.ReceiverReward
        return totalReward * multiplier
}

// GenerateTransactions generates all reward transactions
func (rb *MessageRewardBundle) GenerateTransactions(systemKey *ecdsa.PrivateKey) error {
        if rb.Message == nil {
                return errors.New("no message available")
        }
        
        messageID := rb.Message.GetID()
        
        // Generate sender reward transaction
        if rb.SenderReward > 0 && !rb.IsPenalty {
                rb.SenderTx = NewRewardTransaction(
                        "SYSTEM",
                        rb.Message.GetSender(),
                        rb.SenderReward,
                        messageID,
                        "",
                        string(SenderRewardType),
                )
                
                if systemKey != nil {
                        if err := rb.SenderTx.Sign(systemKey); err != nil {
                                return fmt.Errorf("failed to sign sender reward: %v", err)
                        }
                }
        }
        
        // Generate receiver reward transaction
        if rb.ReceiverReward > 0 && !rb.IsPenalty {
                rb.ReceiverTx = NewRewardTransaction(
                        "SYSTEM",
                        rb.Message.GetReceiver(),
                        rb.ReceiverReward,
                        messageID,
                        rb.Message.GetSender(),
                        string(ReceiverRewardType),
                )
                
                if systemKey != nil {
                        if err := rb.ReceiverTx.Sign(systemKey); err != nil {
                                return fmt.Errorf("failed to sign receiver reward: %v", err)
                        }
                }
        }
        
        // Generate validator reward transaction
        if rb.Validator != "" {
                validatorReward := rb.CalculateValidatorReward(1.5) // 150% multiplier
                
                rb.ValidatorTx = NewRewardTransaction(
                        "SYSTEM",
                        rb.Validator,
                        validatorReward,
                        messageID,
                        "",
                        string(ValidatorRewardType),
                )
                
                if systemKey != nil {
                        if err := rb.ValidatorTx.Sign(systemKey); err != nil {
                                return fmt.Errorf("failed to sign validator reward: %v", err)
                        }
                }
        }
        
        return nil
}

// ToJSON converts the message reward bundle to JSON
func (rb *MessageRewardBundle) ToJSON() ([]byte, error) {
        return json.Marshal(rb)
}

// FromJSON loads the message reward bundle from JSON
func (rb *MessageRewardBundle) FromJSON(data []byte) error {
        return json.Unmarshal(data, rb)
}
