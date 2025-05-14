package core

import (
        "errors"
        "fmt"
        "sync"
        "time"

        "github.com/doucya/interfaces"
        "github.com/doucya/messaging"
        "github.com/doucya/utils"
        "github.com/doucya/wallet"
)

// Blockchain represents the main blockchain structure
type Blockchain struct {
        mu                sync.RWMutex
        storage           interfaces.Storage
        currentBlock      *Block
        validatorMinDeposit float64
        validators        map[string]*Validator
        wallets           map[string]*wallet.Wallet
        whitelists        map[string]map[string]bool // from -> to -> whitelisted
        messageRates      map[string]*MessageRateLimit
        pendingRewards    map[string]map[string]float64 // sender -> receiver -> amount
        config            *utils.Config
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
                storage:           storage,
                validatorMinDeposit: config.ValidatorMinDeposit,
                validators:        make(map[string]*Validator),
                wallets:           make(map[string]*wallet.Wallet),
                whitelists:        make(map[string]map[string]bool),
                messageRates:      make(map[string]*MessageRateLimit),
                pendingRewards:    make(map[string]map[string]float64),
                config:            config,
        }

        // Try to load existing blockchain
        lastBlock, err := storage.GetLastBlock()
        if err != nil {
                // If no blockchain exists, create a genesis block
                genesis := CreateGenesisBlock()
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
                bc.validators[v.Address] = v
        }

        // Load wallets
        wallets, err := storage.GetWallets()
        if err != nil {
                return nil, fmt.Errorf("failed to load wallets: %v", err)
        }
        for _, w := range wallets {
                bc.wallets[w.GetAddress()] = w
        }

        // Load whitelists
        whitelists, err := storage.GetWhitelists()
        if err != nil {
                return nil, fmt.Errorf("failed to load whitelists: %v", err)
        }
        bc.whitelists = whitelists

        // Load pending rewards
        pendingRewards, err := storage.GetPendingRewards()
        if err != nil {
                return nil, fmt.Errorf("failed to load pending rewards: %v", err)
        }
        bc.pendingRewards = pendingRewards

        // Start background processes
        go bc.processPendingTransactions()
        go bc.processValidatorRewards()
        go bc.cleanupRateLimits()

        return bc, nil
}

// GetHeight returns the current blockchain height
func (bc *Blockchain) GetHeight() (int64, error) {
        bc.mu.RLock()
        defer bc.mu.RUnlock()
        
        if bc.currentBlock == nil {
                return 0, errors.New("blockchain not initialized")
        }
        
        return bc.currentBlock.Height, nil
}

// GetNodeCount returns the number of nodes in the network
func (bc *Blockchain) GetNodeCount() (int, error) {
        // In a real implementation, this would query the network or a dedicated counter
        // For simplicity, we'll use the number of wallets as a proxy
        bc.mu.RLock()
        defer bc.mu.RUnlock()
        
        return len(bc.wallets), nil
}

// AllocateGenesisAmount allocates the genesis amount to the local wallet
func (bc *Blockchain) AllocateGenesisAmount(amount float64) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()
        
        // Find a local wallet to allocate to
        if len(bc.wallets) == 0 {
                return errors.New("no wallets found to allocate genesis amount")
        }
        
        // Just use the first wallet
        var address string
        for addr := range bc.wallets {
                address = addr
                break
        }
        
        // Create a transaction for the genesis allocation
        tx := NewTransaction("", address, amount, TransactionTypeGenesis)
        
        // Add transaction to a new block
        block := NewBlock([]*Transaction{tx}, bc.currentBlock)
        
        // Save the block
        err := bc.storage.SaveBlock(block)
        if err != nil {
                return fmt.Errorf("failed to save block with genesis allocation: %v", err)
        }
        
        bc.currentBlock = block
        return nil
}

// GetBalance returns the balance of an address
func (bc *Blockchain) GetBalance(address string) (float64, error) {
        bc.mu.RLock()
        defer bc.mu.RUnlock()
        
        if !wallet.ValidateAddress(address) {
                return 0, errors.New("invalid address format")
        }
        
        balance, err := bc.storage.GetBalance(address)
        if err != nil {
                return 0, fmt.Errorf("failed to get balance: %v", err)
        }
        
        return balance, nil
}

// SendCoins sends coins from one address to another
func (bc *Blockchain) SendCoins(from, to string, amount float64) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()
        
        if !wallet.ValidateAddress(from) || !wallet.ValidateAddress(to) {
                return errors.New("invalid address format")
        }
        
        // Verify sender has the wallet
        w, exists := bc.wallets[from]
        if !exists {
                return errors.New("sender wallet not found")
        }
        
        // Check balance
        balance, err := bc.storage.GetBalance(from)
        if err != nil {
                return fmt.Errorf("failed to get balance: %v", err)
        }
        
        if balance < amount {
                return errors.New("insufficient balance")
        }
        
        // Create and sign transaction
        tx := NewTransaction(from, to, amount, TransactionTypeCoin)
        err = tx.Sign(w.GetPrivateKey())
        if err != nil {
                return fmt.Errorf("failed to sign transaction: %v", err)
        }
        
        // Add to a new block
        block := NewBlock([]*Transaction{tx}, bc.currentBlock)
        
        // Save the block
        err = bc.storage.SaveBlock(block)
        if err != nil {
                return fmt.Errorf("failed to save block: %v", err)
        }
        
        bc.currentBlock = block
        
        // Update balances
        err = bc.storage.UpdateBalance(from, balance-amount)
        if err != nil {
                return fmt.Errorf("failed to update sender balance: %v", err)
        }
        
        receiverBalance, err := bc.storage.GetBalance(to)
        if err != nil {
                receiverBalance = 0
        }
        
        err = bc.storage.UpdateBalance(to, receiverBalance+amount)
        if err != nil {
                return fmt.Errorf("failed to update receiver balance: %v", err)
        }
        
        return nil
}

// AddWallet adds a wallet to the blockchain
func (bc *Blockchain) AddWallet(w *wallet.Wallet) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()
        
        address := w.GetAddress()
        if _, exists := bc.wallets[address]; exists {
                return errors.New("wallet with this address already exists")
        }
        
        bc.wallets[address] = w
        err := bc.storage.SaveWallet(w)
        if err != nil {
                delete(bc.wallets, address)
                return fmt.Errorf("failed to save wallet: %v", err)
        }
        
        return nil
}

// GetAddresses returns all wallet addresses
func (bc *Blockchain) GetAddresses() ([]string, error) {
        bc.mu.RLock()
        defer bc.mu.RUnlock()
        
        addresses := make([]string, 0, len(bc.wallets))
        for addr := range bc.wallets {
                addresses = append(addresses, addr)
        }
        
        return addresses, nil
}

// SendMessage sends a message from one address to another
func (bc *Blockchain) SendMessage(message *messaging.Message) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()
        
        sender := message.Sender
        receiver := message.Receiver
        
        // Verify sender has the wallet
        w, exists := bc.wallets[sender]
        if !exists {
                return errors.New("sender wallet not found")
        }
        
        // Check if rate limits are exceeded
        if err := bc.checkMessageRateLimit(sender, receiver); err != nil {
                return err
        }
        
        // Check if receiver has whitelisted the sender
        isWhitelisted := bc.isWhitelisted(receiver, sender)
        
        // Sign the message
        err := message.Sign(w.GetPrivateKey())
        if err != nil {
                return fmt.Errorf("failed to sign message: %v", err)
        }
        
        // Create transaction for this message
        var tx *Transaction
        if isWhitelisted {
                // Message is allowed without fee, sender will receive rewards later
                // when receiver responds
                tx = NewMessageTransaction(sender, receiver, message, false)
                
                // Track pending rewards
                if _, exists := bc.pendingRewards[sender]; !exists {
                        bc.pendingRewards[sender] = make(map[string]float64)
                }
                bc.pendingRewards[sender][receiver] = bc.config.SenderReward
                
                // Immediately give receiver reward
                receiverBalance, err := bc.storage.GetBalance(receiver)
                if err != nil {
                        receiverBalance = 0
                }
                
                err = bc.storage.UpdateBalance(receiver, receiverBalance+bc.config.ReceiverReward)
                if err != nil {
                        return fmt.Errorf("failed to update receiver balance: %v", err)
                }
                
                // Give validator rewards
                bc.distributeValidatorRewards(bc.config.ReceiverReward * bc.config.ValidatorRewardMult)
        } else {
                // Sender pays a fee instead of getting a reward
                tx = NewMessageTransaction(sender, receiver, message, true)
                
                // Deduct fee from sender
                senderBalance, err := bc.storage.GetBalance(sender)
                if err != nil {
                        return fmt.Errorf("failed to get sender balance: %v", err)
                }
                
                if senderBalance < bc.config.SenderReward {
                        return errors.New("insufficient balance to pay message fee")
                }
                
                err = bc.storage.UpdateBalance(sender, senderBalance-bc.config.SenderReward)
                if err != nil {
                        return fmt.Errorf("failed to update sender balance: %v", err)
                }
                
                // Give validator rewards
                bc.distributeValidatorRewards(bc.config.SenderReward)
        }
        
        // Add to a new block
        block := NewBlock([]*Transaction{tx}, bc.currentBlock)
        
        // Save the block
        err = bc.storage.SaveBlock(block)
        if err != nil {
                return fmt.Errorf("failed to save block: %v", err)
        }
        
        bc.currentBlock = block
        
        // Save message
        err = bc.storage.SaveMessage(message)
        if err != nil {
                return fmt.Errorf("failed to save message: %v", err)
        }
        
        // Update rate limit
        bc.updateMessageRateLimit(sender, receiver)
        
        return nil
}

// GetMessagesForAddress returns all messages for a specific address
func (bc *Blockchain) GetMessagesForAddress(address string) ([]*messaging.Message, error) {
        bc.mu.RLock()
        defer bc.mu.RUnlock()
        
        if !wallet.ValidateAddress(address) {
                return nil, errors.New("invalid address format")
        }
        
        messages, err := bc.storage.GetMessagesForAddress(address)
        if err != nil {
                return nil, fmt.Errorf("failed to get messages: %v", err)
        }
        
        return messages, nil
}

// WhitelistAddress adds an address to the whitelist of another address
func (bc *Blockchain) WhitelistAddress(from, to string) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()
        
        if !wallet.ValidateAddress(from) || !wallet.ValidateAddress(to) {
                return errors.New("invalid address format")
        }
        
        // Check if sender has the wallet
        if _, exists := bc.wallets[from]; !exists {
                return errors.New("sender wallet not found")
        }
        
        // Add to whitelist
        if _, exists := bc.whitelists[from]; !exists {
                bc.whitelists[from] = make(map[string]bool)
        }
        
        bc.whitelists[from][to] = true
        
        // Save whitelist
        err := bc.storage.SaveWhitelist(from, to)
        if err != nil {
                return fmt.Errorf("failed to save whitelist: %v", err)
        }
        
        return nil
}

// RegisterValidator registers an address as a validator
func (bc *Blockchain) RegisterValidator(address string, deposit float64) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()
        
        if !wallet.ValidateAddress(address) {
                return errors.New("invalid address format")
        }
        
        // Check if address has a wallet
        if _, exists := bc.wallets[address]; !exists {
                return errors.New("wallet not found for this address")
        }
        
        // Check if deposit is sufficient
        if deposit < bc.validatorMinDeposit {
                return fmt.Errorf("deposit must be at least %.10f DOU", bc.validatorMinDeposit)
        }
        
        // Check if address has enough balance
        balance, err := bc.storage.GetBalance(address)
        if err != nil {
                return fmt.Errorf("failed to get balance: %v", err)
        }
        
        if balance < deposit {
                return errors.New("insufficient balance for deposit")
        }
        
        // Create validator
        validator := NewValidator(address, deposit, time.Now())
        
        // Add to validators
        bc.validators[address] = validator
        
        // Update balance
        err = bc.storage.UpdateBalance(address, balance-deposit)
        if err != nil {
                return fmt.Errorf("failed to update balance: %v", err)
        }
        
        // Save validator
        err = bc.storage.SaveValidator(validator)
        if err != nil {
                return fmt.Errorf("failed to save validator: %v", err)
        }
        
        return nil
}

// UnregisterValidator removes an address from validators
func (bc *Blockchain) UnregisterValidator(address string) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()
        
        if !wallet.ValidateAddress(address) {
                return errors.New("invalid address format")
        }
        
        // Check if address is a validator
        validator, exists := bc.validators[address]
        if !exists {
                return errors.New("address is not a validator")
        }
        
        // Return deposit
        balance, err := bc.storage.GetBalance(address)
        if err != nil {
                return fmt.Errorf("failed to get balance: %v", err)
        }
        
        err = bc.storage.UpdateBalance(address, balance+validator.Deposit)
        if err != nil {
                return fmt.Errorf("failed to update balance: %v", err)
        }
        
        // Remove from validators
        delete(bc.validators, address)
        
        // Remove from storage
        err = bc.storage.DeleteValidator(address)
        if err != nil {
                return fmt.Errorf("failed to delete validator: %v", err)
        }
        
        return nil
}

// GetValidators returns all validators
func (bc *Blockchain) GetValidators() (map[string]*Validator, error) {
        bc.mu.RLock()
        defer bc.mu.RUnlock()
        
        // Create a copy to prevent concurrent modification
        validators := make(map[string]*Validator, len(bc.validators))
        for addr, validator := range bc.validators {
                validators[addr] = validator
        }
        
        return validators, nil
}

// GetValidatorMinDeposit returns the minimum deposit required to be a validator
func (bc *Blockchain) GetValidatorMinDeposit() float64 {
        bc.mu.RLock()
        defer bc.mu.RUnlock()
        
        return bc.validatorMinDeposit
}

// CreateGroup creates a new group
func (bc *Blockchain) CreateGroup(address, name string, isPublic bool) (string, error) {
        bc.mu.Lock()
        defer bc.mu.Unlock()
        
        if !wallet.ValidateAddress(address) {
                return "", errors.New("invalid address format")
        }
        
        // Check if address has a wallet
        if _, exists := bc.wallets[address]; !exists {
                return "", errors.New("wallet not found for this address")
        }
        
        // Create group
        group := messaging.NewGroup(address, name, isPublic)
        
        // Save group
        err := bc.storage.SaveGroup(group)
        if err != nil {
                return "", fmt.Errorf("failed to save group: %v", err)
        }
        
        return group.ID, nil
}

// JoinGroup adds an address to a group
func (bc *Blockchain) JoinGroup(address, groupID string) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()
        
        if !wallet.ValidateAddress(address) {
                return errors.New("invalid address format")
        }
        
        // Check if address has a wallet
        if _, exists := bc.wallets[address]; !exists {
                return errors.New("wallet not found for this address")
        }
        
        // Check if group exists
        group, err := bc.storage.GetGroup(groupID)
        if err != nil {
                return fmt.Errorf("failed to get group: %v", err)
        }
        
        if group == nil {
                return errors.New("group not found")
        }
        
        // Check if it's a public group or if address is the owner
        if !group.IsPublic && group.Owner != address {
                // For private groups, owner must approve
                // This would typically involve sending a request to the owner
                return errors.New("cannot join private group without owner approval")
        }
        
        // Add address to group
        err = bc.storage.AddMemberToGroup(groupID, address)
        if err != nil {
                return fmt.Errorf("failed to add member to group: %v", err)
        }
        
        return nil
}

// CreateChannel creates a new channel
func (bc *Blockchain) CreateChannel(address, name string, isPublic bool) (string, error) {
        bc.mu.Lock()
        defer bc.mu.Unlock()
        
        if !wallet.ValidateAddress(address) {
                return "", errors.New("invalid address format")
        }
        
        // Check if address has a wallet
        if _, exists := bc.wallets[address]; !exists {
                return "", errors.New("wallet not found for this address")
        }
        
        // Create channel
        channel := messaging.NewChannel(address, name, isPublic)
        
        // Save channel
        err := bc.storage.SaveChannel(channel)
        if err != nil {
                return "", fmt.Errorf("failed to save channel: %v", err)
        }
        
        return channel.ID, nil
}

// Helper methods

// isWhitelisted checks if an address has whitelisted another address
func (bc *Blockchain) isWhitelisted(from, to string) bool {
        whitelist, exists := bc.whitelists[from]
        if !exists {
                return false
        }
        
        return whitelist[to]
}

// checkMessageRateLimit checks if message rate limits are exceeded
func (bc *Blockchain) checkMessageRateLimit(sender, receiver string) error {
        // Initialize rate limit tracker if not exists
        if _, exists := bc.messageRates[sender]; !exists {
                bc.messageRates[sender] = &MessageRateLimit{
                        Total:      make(map[time.Time]int),
                        PerAddress: make(map[string]map[time.Time]int),
                        lastCleanup: time.Now(),
                }
        }
        
        rateLimit := bc.messageRates[sender]
        
        // Count total messages in the last hour
        totalCount := 0
        now := time.Now()
        for t, count := range rateLimit.Total {
                if now.Sub(t) <= time.Hour {
                        totalCount += count
                }
        }
        
        if totalCount >= bc.config.MessageRateLimitTotal {
                return errors.New("total message rate limit exceeded (200 messages per hour)")
        }
        
        // Count messages to this address in the last hour
        if _, exists := rateLimit.PerAddress[receiver]; !exists {
                rateLimit.PerAddress[receiver] = make(map[time.Time]int)
        }
        
        addrCount := 0
        for t, count := range rateLimit.PerAddress[receiver] {
                if now.Sub(t) <= time.Hour {
                        addrCount += count
                }
        }
        
        if addrCount >= bc.config.MessageRateLimitAddr {
                return errors.New("address message rate limit exceeded (30 messages per hour to a single address)")
        }
        
        return nil
}

// updateMessageRateLimit updates the message rate limit counters
func (bc *Blockchain) updateMessageRateLimit(sender, receiver string) {
        now := time.Now().Truncate(time.Minute) // Truncate to minute for better bucketing
        
        // Initialize if not exists
        if _, exists := bc.messageRates[sender]; !exists {
                bc.messageRates[sender] = &MessageRateLimit{
                        Total:      make(map[time.Time]int),
                        PerAddress: make(map[string]map[time.Time]int),
                        lastCleanup: now,
                }
        }
        
        rateLimit := bc.messageRates[sender]
        
        // Update total count
        rateLimit.Total[now]++
        
        // Update per-address count
        if _, exists := rateLimit.PerAddress[receiver]; !exists {
                rateLimit.PerAddress[receiver] = make(map[time.Time]int)
        }
        
        rateLimit.PerAddress[receiver][now]++
}

// distributeValidatorRewards distributes rewards to validators
func (bc *Blockchain) distributeValidatorRewards(amount float64) {
        if len(bc.validators) == 0 {
                return // No validators to reward
        }
        
        // Calculate reward per validator
        rewardPerValidator := amount / float64(len(bc.validators))
        
        // Distribute rewards
        for addr := range bc.validators {
                balance, err := bc.storage.GetBalance(addr)
                if err != nil {
                        balance = 0
                }
                
                _ = bc.storage.UpdateBalance(addr, balance+rewardPerValidator)
        }
}

// processPendingTransactions processes pending transactions periodically
func (bc *Blockchain) processPendingTransactions() {
        ticker := time.NewTicker(5 * time.Minute)
        defer ticker.Stop()
        
        for range ticker.C {
                bc.mu.Lock()
                
                // Process pending rewards
                for sender, receivers := range bc.pendingRewards {
                        for receiver, amount := range receivers {
                                // Check if receiver has sent a message back to sender
                                hasSentBack, err := bc.storage.HasMessageFromTo(receiver, sender)
                                if err != nil {
                                        continue
                                }
                                
                                if hasSentBack {
                                        // Reward the sender
                                        balance, err := bc.storage.GetBalance(sender)
                                        if err != nil {
                                                balance = 0
                                        }
                                        
                                        err = bc.storage.UpdateBalance(sender, balance+amount)
                                        if err != nil {
                                                continue
                                        }
                                        
                                        // Give validator rewards
                                        bc.distributeValidatorRewards(amount * bc.config.ValidatorRewardMult)
                                        
                                        // Remove from pending rewards
                                        delete(receivers, receiver)
                                }
                        }
                        
                        // Clean up empty maps
                        if len(receivers) == 0 {
                                delete(bc.pendingRewards, sender)
                        }
                }
                
                // Save pending rewards state
                _ = bc.storage.SavePendingRewards(bc.pendingRewards)
                
                bc.mu.Unlock()
        }
}

// processValidatorRewards processes monthly validator rewards
func (bc *Blockchain) processValidatorRewards() {
        // Calculate month duration (approximately)
        monthDuration := 30 * 24 * time.Hour
        
        ticker := time.NewTicker(monthDuration)
        defer ticker.Stop()
        
        for range ticker.C {
                bc.mu.Lock()
                
                // Process validator rewards
                for addr, validator := range bc.validators {
                        // Calculate monthly reward (APY / 12)
                        monthlyRate := bc.config.ValidatorAPY / 12
                        reward := validator.Deposit * monthlyRate
                        
                        // Update balance
                        balance, err := bc.storage.GetBalance(addr)
                        if err != nil {
                                balance = 0
                        }
                        
                        err = bc.storage.UpdateBalance(addr, balance+reward)
                        if err != nil {
                                continue
                        }
                }
                
                bc.mu.Unlock()
        }
}

// cleanupRateLimits cleans up old rate limit entries
func (bc *Blockchain) cleanupRateLimits() {
        ticker := time.NewTicker(10 * time.Minute)
        defer ticker.Stop()
        
        for range ticker.C {
                bc.mu.Lock()
                
                now := time.Now()
                for _, rateLimit := range bc.messageRates {
                        // Clean up total counts
                        for t := range rateLimit.Total {
                                if now.Sub(t) > time.Hour {
                                        delete(rateLimit.Total, t)
                                }
                        }
                        
                        // Clean up per-address counts
                        for addr, times := range rateLimit.PerAddress {
                                for t := range times {
                                        if now.Sub(t) > time.Hour {
                                                delete(times, t)
                                        }
                                }
                                
                                if len(times) == 0 {
                                        delete(rateLimit.PerAddress, addr)
                                }
                        }
                }
                
                bc.mu.Unlock()
        }
}
