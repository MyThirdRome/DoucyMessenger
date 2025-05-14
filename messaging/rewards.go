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

// Constants for messaging reward system
const (
        // Default reward amounts for messaging
        SenderRewardAmount   = 0.75 // DOU amount for sender
        ReceiverRewardAmount = 0.25 // DOU amount for receiver

        // Rate limits for spam protection
        MaxMessagesPerHour      = 200 // Maximum messages one address can send per hour
        MaxMessagesToSameTarget = 30  // Maximum messages to same address per hour
)

// RewardTracker tracks message rewards and rate limits
type RewardTracker struct {
        mu                sync.RWMutex
        pendingRewards    map[string]map[string]float64 // sender -> receiver -> amount
        processedMessages map[string]bool               // message ID -> processed
        senderReward      float64
        receiverReward    float64
        maxMessagesPerHour int                         // limit total messages per hour (rate limiting)
        maxMessagesToSameTarget int                    // limit messages to same address per hour
        hourlyMessageCount map[string]int              // address -> count of messages sent in current hour
        targetMessageCount map[string]map[string]int   // sender -> receiver -> count in current hour
        lastResetHour      time.Time                   // tracks when hourly limits were last reset
        whitelists         map[string]map[string]bool  // address -> whitelisted addresses -> true
}

// NewRewardTracker creates a new reward tracker
func NewRewardTracker() *RewardTracker {
        return &RewardTracker{
                pendingRewards:       make(map[string]map[string]float64),
                processedMessages:    make(map[string]bool),
                senderReward:         SenderRewardAmount,
                receiverReward:       ReceiverRewardAmount,
                maxMessagesPerHour:   MaxMessagesPerHour,
                maxMessagesToSameTarget: MaxMessagesToSameTarget,
                hourlyMessageCount:   make(map[string]int),
                targetMessageCount:   make(map[string]map[string]int),
                lastResetHour:        time.Now(),
                whitelists:           make(map[string]map[string]bool),
        }
}

// resetHourlyCountersIfNeeded resets hourly counters if it's a new hour
func (rt *RewardTracker) resetHourlyCountersIfNeeded() {
        now := time.Now()
        elapsed := now.Sub(rt.lastResetHour)
        
        // Reset if more than an hour has passed
        if elapsed.Hours() >= 1 {
                rt.hourlyMessageCount = make(map[string]int)
                rt.targetMessageCount = make(map[string]map[string]int)
                rt.lastResetHour = now
        }
}

// AddToWhitelist adds an address to another address's whitelist
func (rt *RewardTracker) AddToWhitelist(owner, target string) {
        rt.mu.Lock()
        defer rt.mu.Unlock()
        
        if _, exists := rt.whitelists[owner]; !exists {
                rt.whitelists[owner] = make(map[string]bool)
        }
        rt.whitelists[owner][target] = true
}

// RemoveFromWhitelist removes an address from another address's whitelist
func (rt *RewardTracker) RemoveFromWhitelist(owner, target string) {
        rt.mu.Lock()
        defer rt.mu.Unlock()
        
        if whitelist, exists := rt.whitelists[owner]; exists {
                delete(whitelist, target)
        }
}

// IsWhitelisted checks if an address is in another address's whitelist
func (rt *RewardTracker) IsWhitelisted(owner, target string) bool {
        rt.mu.RLock()
        defer rt.mu.RUnlock()
        
        whitelist, exists := rt.whitelists[owner]
        if !exists {
                return false
        }
        return whitelist[target]
}

// IsMutuallyWhitelisted checks if both addresses have whitelisted each other
func (rt *RewardTracker) IsMutuallyWhitelisted(addr1, addr2 string) bool {
        return rt.IsWhitelisted(addr1, addr2) && rt.IsWhitelisted(addr2, addr1)
}

// CanSendMessage checks if an address can send more messages based on rate limits
func (rt *RewardTracker) CanSendMessage(sender, receiver string) (bool, string) {
        rt.mu.Lock()
        defer rt.mu.Unlock()
        
        rt.resetHourlyCountersIfNeeded()
        
        // Check if mutually whitelisted
        if !rt.IsMutuallyWhitelisted(sender, receiver) {
                return false, "addresses must mutually whitelist each other"
        }
        
        // Check hourly limit
        if rt.hourlyMessageCount[sender] >= rt.maxMessagesPerHour {
                return false, fmt.Sprintf("exceeded maximum of %d messages per hour", rt.maxMessagesPerHour)
        }
        
        // Check per-target limit
        if _, exists := rt.targetMessageCount[sender]; !exists {
                rt.targetMessageCount[sender] = make(map[string]int)
        }
        
        if rt.targetMessageCount[sender][receiver] >= rt.maxMessagesToSameTarget {
                return false, fmt.Sprintf("exceeded maximum of %d messages to same address per hour", rt.maxMessagesToSameTarget)
        }
        
        return true, ""
}

// RecordMessageSent updates the message count for rate limiting
func (rt *RewardTracker) RecordMessageSent(sender, receiver string) {
        rt.mu.Lock()
        defer rt.mu.Unlock()
        
        rt.resetHourlyCountersIfNeeded()
        
        // Update hourly counter
        rt.hourlyMessageCount[sender]++
        
        // Update per-target counter
        if _, exists := rt.targetMessageCount[sender]; !exists {
                rt.targetMessageCount[sender] = make(map[string]int)
        }
        rt.targetMessageCount[sender][receiver]++
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
// Returns sender reward amount, receiver reward amount, and success status
func (rt *RewardTracker) ProcessReward(sender, receiver string) (float64, float64, bool) {
        rt.mu.Lock()
        defer rt.mu.Unlock()

        // Check if reward exists
        rewards, exists := rt.pendingRewards[sender]
        if !exists {
                return 0, 0, false
        }

        senderAmount, exists := rewards[receiver]
        if !exists {
                return 0, 0, false
        }

        // Remove reward
        delete(rewards, receiver)
        if len(rewards) == 0 {
                delete(rt.pendingRewards, sender)
        }

        // Return both sender and receiver reward amounts
        return senderAmount, rt.receiverReward, true
}

// RewardTransaction represents a transaction for a message reward
type RewardTransaction struct {
        ID            string    `json:"id"`
        Timestamp     time.Time `json:"timestamp"`
        Sender        string    `json:"sender"`    // SYSTEM or validator address
        Receiver      string    `json:"receiver"`  // address receiving the reward
        Amount        float64   `json:"amount"`
        MessageID     string    `json:"message_id"`
        OriginalSender string    `json:"original_sender"`
        Type          string    `json:"type"`     // "SENDER_REWARD" or "RECEIVER_REWARD"
        Signature     string    `json:"signature"`
}

// CreateRewardTransaction creates a signed reward transaction
func CreateRewardTransaction(
        rewardType string,
        receiverAddr string,
        amount float64,
        messageID string,
        originalSender string,
        privateKey *ecdsa.PrivateKey,
) (*RewardTransaction, error) {
        // Create transaction ID (hash of data)
        idData := fmt.Sprintf("%s-%s-%.8f-%s-%s-%d", 
                rewardType, 
                receiverAddr, 
                amount, 
                messageID, 
                originalSender, 
                time.Now().UnixNano())
        hash := sha256.Sum256([]byte(idData))
        id := hex.EncodeToString(hash[:])
        
        // Create transaction
        tx := &RewardTransaction{
                ID:             id,
                Timestamp:      time.Now(),
                Sender:         "SYSTEM", // System rewards come from "SYSTEM"
                Receiver:       receiverAddr,
                Amount:         amount,
                MessageID:      messageID,
                OriginalSender: originalSender,
                Type:           rewardType,
        }
        
        // Sign transaction
        txJSON, err := json.Marshal(tx)
        if err != nil {
                return nil, fmt.Errorf("failed to marshal transaction: %v", err)
        }
        
        txHash := sha256.Sum256(txJSON)
        signature, err := ecdsa.SignASN1(nil, privateKey, txHash[:])
        if err != nil {
                return nil, fmt.Errorf("failed to sign transaction: %v", err)
        }
        
        tx.Signature = hex.EncodeToString(signature)
        return tx, nil
}

// VerifyRewardTransaction verifies a reward transaction signature
func VerifyRewardTransaction(tx *RewardTransaction, pubKey *ecdsa.PublicKey) bool {
        // Create a copy of the transaction without signature for verification
        txCopy := *tx
        signature := tx.Signature
        txCopy.Signature = ""
        
        // Marshal transaction
        txJSON, err := json.Marshal(txCopy)
        if err != nil {
                return false
        }
        
        // Verify signature
        txHash := sha256.Sum256(txJSON)
        signatureBytes, err := hex.DecodeString(signature)
        if err != nil {
                return false
        }
        
        return ecdsa.VerifyASN1(pubKey, txHash[:], signatureBytes)
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
