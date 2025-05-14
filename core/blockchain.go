package core

import (
        "errors"
        "fmt"
        "sync"
        "time"
        "github.com/doucya/messaging"
        "github.com/doucya/models"
        "github.com/doucya/storage"
        "github.com/doucya/utils"
        "github.com/doucya/wallet"
)

// Constants for message rewards
const (
        SenderReward     = 0.75
        ReceiverReward   = 0.25
        ValidatorReward  = 1.5 // 1.5 times the user rewards
        ValidatorAPY     = 0.17
        MaxMessagesTotal = 200
        MaxMessagesPerAddr = 30
)

// Blockchain represents the main blockchain structure
type Blockchain struct {
        mu                  sync.RWMutex
        storage             *storage.LevelDBStorage
        currentBlock        *models.Block
        validatorMinDeposit float64
        validators          map[string]*models.Validator
        wallets             map[string]*wallet.Wallet
        whitelists          map[string]map[string]bool // from -> to -> whitelisted
        messageRates        map[string]*MessageRateLimit
        pendingRewards      map[string]map[string]float64 // sender -> receiver -> amount
        config              *utils.Config
}

// MessageRateLimit tracks message rate limits for addresses
type MessageRateLimit struct {
        Total       map[time.Time]int            // total messages within the last hour
        PerAddress  map[string]map[time.Time]int // messages per address within the last hour
        lastCleanup time.Time
}

// NewBlockchain creates a new blockchain with the given storage
func NewBlockchain(storage *storage.LevelDBStorage, config *utils.Config) (*Blockchain, error) {
        bc := &Blockchain{
                storage:             storage,
                validatorMinDeposit: 50.0, // Default minimum deposit is 50 DOU
                validators:          make(map[string]*models.Validator),
                wallets:             make(map[string]*wallet.Wallet),
                whitelists:          make(map[string]map[string]bool),
                messageRates:        make(map[string]*MessageRateLimit),
                pendingRewards:      make(map[string]map[string]float64),
                config:              config,
        }

        // Try to load existing blockchain
        lastBlock, err := storage.GetLastBlock()
        if err != nil {
                return nil, fmt.Errorf("failed to get last block: %v", err)
        }

        // If no blocks exist, create genesis block
        if lastBlock == nil {
                genesis := models.CreateGenesisBlock()
                bc.currentBlock = genesis
                err = storage.SaveBlock(genesis)
                if err != nil {
                        return nil, fmt.Errorf("failed to save genesis block: %v", err)
                }
        } else {
                bc.currentBlock = lastBlock
        }

        // Load validators
        validators, err := storage.GetValidators()
        if err != nil {
                return nil, fmt.Errorf("failed to load validators: %v", err)
        }
        for _, v := range validators {
                bc.validators[v.GetAddress()] = v
        }

        // Load wallets
        wallets, err := storage.GetWallets()
        if err != nil {
                return nil, fmt.Errorf("failed to load wallets: %v", err)
        }
        for _, w := range wallets {
                bc.wallets[w.GetAddress()] = w
        }

        // Load pending rewards
        rewards, err := storage.GetPendingRewards()
        if err == nil && rewards != nil {
                // Convert the rewards to the expected map type
                bc.pendingRewards = make(map[string]map[string]float64)
                for sender, receiverMap := range rewards {
                        bc.pendingRewards[sender] = make(map[string]float64)
                        for receiver, amount := range receiverMap {
                                bc.pendingRewards[sender][receiver] = amount
                        }
                }
        }

        return bc, nil
}

// AddWallet adds a wallet to the blockchain
func (bc *Blockchain) AddWallet(w *wallet.Wallet) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()

        if _, exists := bc.wallets[w.GetAddress()]; exists {
                return fmt.Errorf("wallet already exists: %s", w.GetAddress())
        }

        bc.wallets[w.GetAddress()] = w
        return bc.storage.SaveWallet(w)
}

// GetAddresses returns all addresses in the blockchain
func (bc *Blockchain) GetAddresses() ([]string, error) {
        bc.mu.RLock()
        defer bc.mu.RUnlock()

        addresses := make([]string, 0, len(bc.wallets))
        for addr := range bc.wallets {
                addresses = append(addresses, addr)
        }
        return addresses, nil
}

// GetBalance returns the balance for an address
func (bc *Blockchain) GetBalance(address string) (float64, error) {
        bc.mu.RLock()
        defer bc.mu.RUnlock()

        balance, err := bc.storage.GetBalance(address)
        if err != nil {
                return 0, err
        }
        return balance, nil
}

// SendCoins sends coins from one address to another
func (bc *Blockchain) SendCoins(from, to string, amount float64) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()

        // Verify addresses
        if !wallet.ValidateAddress(from) || !wallet.ValidateAddress(to) {
                return errors.New("invalid address format")
        }

        // Check if sender has enough balance
        balance, err := bc.storage.GetBalance(from)
        if err != nil {
                return err
        }
        if balance < amount {
                return fmt.Errorf("insufficient balance: %.10f", balance)
        }

        // Create transaction
        tx := models.NewTransaction(from, to, amount, models.TransactionTypeCoin)
        
        // Create new block
        block := models.NewBlock([]*models.Transaction{tx}, bc.currentBlock)
        
        // Save block
        if err := bc.storage.SaveBlock(block); err != nil {
                return err
        }
        bc.currentBlock = block

        // Update balances
        if err := bc.storage.UpdateBalance(from, balance-amount); err != nil {
                return err
        }
        
        toBalance, err := bc.storage.GetBalance(to)
        if err != nil {
                toBalance = 0
        }
        
        if err := bc.storage.UpdateBalance(to, toBalance+amount); err != nil {
                return err
        }

        return nil
}

// SendMessage sends a message from one address to another
func (bc *Blockchain) SendMessage(message *messaging.Message) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()

        // Verify addresses
        if !wallet.ValidateAddress(message.Sender) || !wallet.ValidateAddress(message.Receiver) {
                return errors.New("invalid address format")
        }

        // Check if sender has the rights to send messages to receiver (whitelist)
        whitelisted := bc.isWhitelisted(message.Sender, message.Receiver)
        
        // Check message rate limits if not whitelisted
        if !whitelisted {
                if bc.exceedsRateLimit(message.Sender, message.Receiver) {
                        return errors.New("rate limit exceeded")
                }
        }

        // Create transaction with the message
        tx := models.NewMessageTransaction(message.Sender, message.Receiver, message, false)
        
        // Create new block
        block := models.NewBlock([]*models.Transaction{tx}, bc.currentBlock)
        
        // Save block and message
        if err := bc.storage.SaveBlock(block); err != nil {
                return err
        }
        bc.currentBlock = block

        if err := bc.storage.SaveMessage(message); err != nil {
                return err
        }

        // Record message for rate limiting
        bc.recordMessage(message.Sender, message.Receiver)

        // Setup pending reward when receiver responds
        if _, exists := bc.pendingRewards[message.Sender]; !exists {
                bc.pendingRewards[message.Sender] = make(map[string]float64)
        }
        bc.pendingRewards[message.Sender][message.Receiver] = 1.0 // Will be calculated when response comes
        
        if err := bc.storage.SavePendingRewards(bc.pendingRewards); err != nil {
                return err
        }

        return nil
}

// ProcessMessageResponse processes a response to a message and distributes rewards
func (bc *Blockchain) ProcessMessageResponse(response *messaging.Message) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()

        // Verify addresses
        if !wallet.ValidateAddress(response.Sender) || !wallet.ValidateAddress(response.Receiver) {
                return errors.New("invalid address format")
        }

        // Check if there's a message from receiver (now sender) to sender (now receiver)
        // and if they have a pending reward
        if pendingReward, exists := bc.pendingRewards[response.Receiver][response.Sender]; exists {
                // The reward exists, calculate and distribute
                _ = pendingReward // Use the variable to avoid unused variable error
                
                // Delete the pending reward
                delete(bc.pendingRewards[response.Receiver], response.Sender)
                if len(bc.pendingRewards[response.Receiver]) == 0 {
                        delete(bc.pendingRewards, response.Receiver)
                }

                // Save updated pending rewards
                if err := bc.storage.SavePendingRewards(bc.pendingRewards); err != nil {
                        return err
                }

                // Calculate rewards
                senderReward := SenderReward
                receiverReward := ReceiverReward
                
                // Update balances for sender and receiver
                originalSender := response.Receiver // The person who sent the original message
                originalReceiver := response.Sender // The person who is now responding
                
                // Get current balances
                senderBalance, err := bc.storage.GetBalance(originalSender)
                if err != nil {
                        senderBalance = 0
                }
                
                receiverBalance, err := bc.storage.GetBalance(originalReceiver)
                if err != nil {
                        receiverBalance = 0
                }
                
                // Update balances
                if err := bc.storage.UpdateBalance(originalSender, senderBalance+senderReward); err != nil {
                        return err
                }
                
                if err := bc.storage.UpdateBalance(originalReceiver, receiverBalance+receiverReward); err != nil {
                        return err
                }
                
                // Also reward validators with their share
                validatorReward := (senderReward + receiverReward) * ValidatorReward
                bc.distributeValidatorRewards(validatorReward)
        }

        // Save the response message
        if err := bc.storage.SaveMessage(response); err != nil {
                return err
        }

        return nil
}

// WhitelistAddress whitelists an address to receive messages without fees
func (bc *Blockchain) WhitelistAddress(from, to string) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()

        // Verify addresses
        if !wallet.ValidateAddress(from) || !wallet.ValidateAddress(to) {
                return errors.New("invalid address format")
        }

        // Add to whitelist
        if _, exists := bc.whitelists[from]; !exists {
                bc.whitelists[from] = make(map[string]bool)
        }
        bc.whitelists[from][to] = true

        // Save to storage
        return bc.storage.SaveWhitelist(from, to)
}

// RegisterValidator registers an address as a validator
func (bc *Blockchain) RegisterValidator(address string, deposit float64) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()

        // Verify address
        if !wallet.ValidateAddress(address) {
                return errors.New("invalid address format")
        }

        // Check minimum deposit
        if deposit < bc.validatorMinDeposit {
                return fmt.Errorf("deposit too low, minimum required: %.10f", bc.validatorMinDeposit)
        }

        // Check if already a validator
        if _, exists := bc.validators[address]; exists {
                return errors.New("address is already a validator")
        }

        // Check if enough balance
        balance, err := bc.storage.GetBalance(address)
        if err != nil {
                return err
        }
        if balance < deposit {
                return fmt.Errorf("insufficient balance: %.10f", balance)
        }

        // Create validator
        validator := models.NewValidator(address, deposit, time.Now())
        bc.validators[address] = validator

        // Update balance
        if err := bc.storage.UpdateBalance(address, balance-deposit); err != nil {
                return err
        }

        // Save validator
        return bc.storage.SaveValidator(validator)
}

// UnregisterValidator removes an address as a validator and returns the deposit
func (bc *Blockchain) UnregisterValidator(address string) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()

        // Verify address
        if !wallet.ValidateAddress(address) {
                return errors.New("invalid address format")
        }

        // Check if is a validator
        validator, exists := bc.validators[address]
        if !exists {
                return errors.New("address is not a validator")
        }

        // Get deposit
        deposit := validator.GetStake()

        // Get current balance
        balance, err := bc.storage.GetBalance(address)
        if err != nil {
                return err
        }

        // Update balance
        if err := bc.storage.UpdateBalance(address, balance+deposit); err != nil {
                return err
        }

        // Remove validator
        delete(bc.validators, address)
        return bc.storage.DeleteValidator(address)
}

// ProcessValidatorRewards processes monthly rewards for validators
func (bc *Blockchain) ProcessValidatorRewards() error {
        bc.mu.Lock()
        defer bc.mu.Unlock()

        for addr, validator := range bc.validators {
                // Check if a month has passed since last reward
                if time.Since(validator.GetLastRewardTime()) >= 30*24*time.Hour {
                        // Calculate monthly reward (APY / 12)
                        monthlyRate := ValidatorAPY / 12
                        reward := validator.GetStake() * monthlyRate

                        // Get current balance
                        balance, err := bc.storage.GetBalance(addr)
                        if err != nil {
                                balance = 0
                        }

                        // Update balance
                        if err := bc.storage.UpdateBalance(addr, balance+reward); err != nil {
                                return err
                        }

                        // Update validator last reward time
                        validator.UpdateLastRewardTime(time.Now())
                        if err := bc.storage.SaveValidator(validator); err != nil {
                                return err
                        }
                }
        }

        return nil
}

// CreateGroup creates a new messaging group
func (bc *Blockchain) CreateGroup(creator, name string, isPublic bool) (string, error) {
        bc.mu.Lock()
        defer bc.mu.Unlock()

        // Verify address
        if !wallet.ValidateAddress(creator) {
                return "", errors.New("invalid address format")
        }

        // Create group
        group := messaging.NewGroup(creator, name, isPublic)
        
        // Save group
        if err := bc.storage.SaveGroup(group); err != nil {
                return "", err
        }

        return group.ID, nil
}

// JoinGroup adds a member to a group
func (bc *Blockchain) JoinGroup(address, groupID string) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()

        // Verify address
        if !wallet.ValidateAddress(address) {
                return errors.New("invalid address format")
        }

        // Get group
        group, err := bc.storage.GetGroup(groupID)
        if err != nil {
                return err
        }

        // Check if group is public or address is the owner
        // Just use the group directly since it's already the correct type
        if !group.IsPublicGroup() && group.GetCreator() != address {
                return errors.New("cannot join private group")
        }

        // Add member to group
        return bc.storage.AddMemberToGroup(groupID, address)
}

// CreateChannel creates a new messaging channel
func (bc *Blockchain) CreateChannel(creator, name string, isPublic bool) (string, error) {
        bc.mu.Lock()
        defer bc.mu.Unlock()

        // Verify address
        if !wallet.ValidateAddress(creator) {
                return "", errors.New("invalid address format")
        }

        // Create channel
        channel := messaging.NewChannel(creator, name, isPublic)
        
        // Save channel
        if err := bc.storage.SaveChannel(channel); err != nil {
                return "", err
        }

        return channel.ID, nil
}

// GetMessagesForAddress returns all messages for an address
func (bc *Blockchain) GetMessagesForAddress(address string) ([]*messaging.Message, error) {
        bc.mu.RLock()
        defer bc.mu.RUnlock()

        // Verify address
        if !wallet.ValidateAddress(address) {
                return nil, errors.New("invalid address format")
        }

        // Get messages
        messages, err := bc.storage.GetMessagesForAddress(address)
        if err != nil {
                return nil, err
        }

        return messages, nil
}

// GetHeight returns the current blockchain height
func (bc *Blockchain) GetHeight() (int64, error) {
        bc.mu.RLock()
        defer bc.mu.RUnlock()

        if bc.currentBlock == nil {
                return 0, nil
        }
        return bc.currentBlock.Height, nil
}

// GetValidators returns all validators
func (bc *Blockchain) GetValidators() ([]*models.Validator, error) {
        bc.mu.RLock()
        defer bc.mu.RUnlock()

        validators := make([]*models.Validator, 0, len(bc.validators))
        for _, v := range bc.validators {
                validators = append(validators, v)
        }
        return validators, nil
}

// GetValidatorMinDeposit returns the minimum validator deposit
func (bc *Blockchain) GetValidatorMinDeposit() float64 {
        return bc.validatorMinDeposit
}

// Helper methods

// isWhitelisted checks if the sender has whitelisted the receiver
func (bc *Blockchain) isWhitelisted(from, to string) bool {
        if whitelist, exists := bc.whitelists[from]; exists {
                return whitelist[to]
        }
        return false
}

// exceedsRateLimit checks if sending a message would exceed rate limits
func (bc *Blockchain) exceedsRateLimit(from, to string) bool {
        // Clean up old entries
        bc.cleanupRateLimits(from)

        // Initialize rate limit tracking if not exists
        if _, exists := bc.messageRates[from]; !exists {
                bc.messageRates[from] = &MessageRateLimit{
                        Total:       make(map[time.Time]int),
                        PerAddress:  make(map[string]map[time.Time]int),
                        lastCleanup: time.Now(),
                }
        }

        // Count total messages in the last hour
        totalCount := 0
        for _, count := range bc.messageRates[from].Total {
                totalCount += count
        }
        if totalCount >= MaxMessagesTotal {
                return true
        }

        // Count messages to this address in the last hour
        if _, exists := bc.messageRates[from].PerAddress[to]; !exists {
                bc.messageRates[from].PerAddress[to] = make(map[time.Time]int)
        }
        addrCount := 0
        for _, count := range bc.messageRates[from].PerAddress[to] {
                addrCount += count
        }
        if addrCount >= MaxMessagesPerAddr {
                return true
        }

        return false
}

// recordMessage records a sent message for rate limiting
func (bc *Blockchain) recordMessage(from, to string) {
        // Clean up old entries
        bc.cleanupRateLimits(from)

        // Initialize rate limit tracking if not exists
        if _, exists := bc.messageRates[from]; !exists {
                bc.messageRates[from] = &MessageRateLimit{
                        Total:       make(map[time.Time]int),
                        PerAddress:  make(map[string]map[time.Time]int),
                        lastCleanup: time.Now(),
                }
        }

        // Record total message
        now := time.Now().Truncate(time.Minute)
        bc.messageRates[from].Total[now]++

        // Record message to this address
        if _, exists := bc.messageRates[from].PerAddress[to]; !exists {
                bc.messageRates[from].PerAddress[to] = make(map[time.Time]int)
        }
        bc.messageRates[from].PerAddress[to][now]++
}

// cleanupRateLimits removes rate limit entries older than 1 hour
func (bc *Blockchain) cleanupRateLimits(address string) {
        if rl, exists := bc.messageRates[address]; exists {
                // Only clean up once per minute to avoid excessive processing
                if time.Since(rl.lastCleanup) < time.Minute {
                        return
                }

                oneHourAgo := time.Now().Add(-time.Hour)
                
                // Clean up total counts
                for t := range rl.Total {
                        if t.Before(oneHourAgo) {
                                delete(rl.Total, t)
                        }
                }

                // Clean up per-address counts
                for addr, times := range rl.PerAddress {
                        for t := range times {
                                if t.Before(oneHourAgo) {
                                        delete(times, t)
                                }
                        }
                        if len(times) == 0 {
                                delete(rl.PerAddress, addr)
                        }
                }

                rl.lastCleanup = time.Now()
        }
}

// distributeValidatorRewards distributes a reward among all active validators
func (bc *Blockchain) distributeValidatorRewards(amount float64) error {
        // Count active validators
        activeValidators := 0
        for _, v := range bc.validators {
                if v.IsEligible() {
                        activeValidators++
                }
        }

        if activeValidators == 0 {
                return nil // No validators to reward
        }

        // Calculate reward per validator
        rewardPerValidator := amount / float64(activeValidators)

        // Distribute rewards
        for addr, v := range bc.validators {
                if v.IsEligible() {
                        // Get current balance
                        balance, err := bc.storage.GetBalance(addr)
                        if err != nil {
                                balance = 0
                        }

                        // Update balance
                        if err := bc.storage.UpdateBalance(addr, balance+rewardPerValidator); err != nil {
                                return err
                        }
                }
        }

        return nil
}// GetMessage retrieves a message by ID
func (bc *Blockchain) GetMessage(messageID string) (*messaging.Message, error) {
        message, err := bc.storage.GetMessage(messageID)
        if err != nil {
                return nil, fmt.Errorf("failed to get message: %v", err)
        }
        
        // Return the message
        return message, nil
}

// GetMessageRewards retrieves all message rewards for an address
func (bc *Blockchain) GetMessageRewards(address string) ([]*messaging.RewardTransaction, error) {
        // This would typically retrieve rewards from storage
        // For now, we'll return a simulated list of rewards
        
        // Simulate some rewards
        rewards := []*messaging.RewardTransaction{}
        
        // Add some example rewards
        // In a real implementation, these would come from storage
        msgID1 := utils.GenerateRandomID()
        msgID2 := utils.GenerateRandomID()
        
        // Sender reward
        senderReward := messaging.NewRewardTransaction(
                "SYSTEM",
                address,
                0.75,
                msgID1,
                "",
                string(messaging.SenderRewardType),
        )
        senderReward.Timestamp = time.Now().Add(-24 * time.Hour)
        
        // Receiver reward
        receiverReward := messaging.NewRewardTransaction(
                "SYSTEM",
                address,
                0.25,
                msgID2,
                "Dou123456789acyA",
                string(messaging.ReceiverRewardType),
        )
        receiverReward.Timestamp = time.Now().Add(-12 * time.Hour)
        
        rewards = append(rewards, senderReward, receiverReward)
        
        return rewards, nil
}

// IncreaseValidatorStake increases a validator's stake
func (bc *Blockchain) IncreaseValidatorStake(address string, amount float64) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()
        
        // Get current validator
        validator, err := bc.storage.GetValidator(address)
        if err != nil {
                return fmt.Errorf("failed to get validator: %v", err)
        }
        
        v, ok := validator.(*models.Validator)
        if !ok {
                return fmt.Errorf("invalid validator type")
        }
        
        // Get current balance
        balance, err := bc.GetBalance(address)
        if err != nil {
                return fmt.Errorf("failed to get balance: %v", err)
        }
        
        if balance < amount {
                return fmt.Errorf("insufficient balance")
        }
        
        // Increase stake
        v.IncreaseDeposit(amount)
        
        // Update validator
        err = bc.storage.SaveValidator(v)
        if err != nil {
                return fmt.Errorf("failed to save validator: %v", err)
        }
        
        // Update balance
        err = bc.storage.UpdateBalance(address, -amount)
        if err != nil {
                // Rollback validator change if balance update fails
                v.DecreaseDeposit(amount)
                bc.storage.SaveValidator(v)
                return fmt.Errorf("failed to update balance: %v", err)
        }
        
        return nil
}

// GetValidator returns a specific validator by address
func (bc *Blockchain) GetValidator(address string) (*models.Validator, error) {
        validator, err := bc.storage.GetValidator(address)
        if err != nil {
                return nil, fmt.Errorf("failed to get validator: %v", err)
        }
        
        if validator == nil {
                return nil, nil
        }
        
        // Check if we got a concrete Validator type
        v, ok := validator.(*models.Validator)
        if !ok {
                return nil, fmt.Errorf("invalid validator type received from storage")
        }
        
        return v, nil
}
