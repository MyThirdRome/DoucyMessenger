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

        // First try to get balance from UTXOs (preferred)
        utxoBalance, err := bc.storage.GetBalanceFromUTXOs(address)
        if err == nil && utxoBalance > 0 {
                return utxoBalance, nil
        }

        // Fall back to legacy method
        balance, err := bc.storage.GetBalance(address)
        if err != nil {
                return 0, err
        }
        return balance, nil
}

// GetAllBalances returns a map of all addresses and their balances
func (bc *Blockchain) GetAllBalances() (map[string]float64, error) {
        bc.mu.RLock()
        defer bc.mu.RUnlock()
        
        // First try to get all balances from UTXOs (preferred method)
        utxoBalances, err := bc.storage.GetAllBalancesFromUTXOs()
        if err == nil && len(utxoBalances) > 0 {
                return utxoBalances, nil
        }
        
        // Fall back to legacy method
        return bc.storage.GetAllBalances()
}

// UpdateBalance updates the balance for an address
func (bc *Blockchain) UpdateBalance(address string, balance float64) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()
        
        // Validate address
        if !wallet.ValidateAddress(address) {
                return errors.New("invalid address format")
        }
        
        // Get current balance to see if we're increasing or decreasing
        currentBalance, err := bc.storage.GetBalance(address)
        if err != nil {
                currentBalance = 0
        }
        
        // Check if we also have UTXOs
        currentUTXOs, err := bc.storage.GetUTXOsForAddress(address)
        if err != nil {
                utils.Error("Failed to get UTXOs for address %s: %v", address, err)
        }
        
        // Only process with UTXOs if this is an increase in balance
        // (Simple balance adjustments like decreases are still handled the legacy way)
        if balance > currentBalance && len(currentUTXOs) >= 0 {
                // Mark all existing UTXOs as spent
                for _, utxo := range currentUTXOs {
                        if !utxo.IsSpent {
                                if err := bc.storage.MarkUTXOSpent(utxo.TxID, utxo.TxOutputIdx); err != nil {
                                        utils.Error("Failed to mark UTXO as spent: %v", err)
                                }
                        }
                }
                
                // Create a system transaction for this balance update
                balanceUpdateTx := models.NewTransaction("SYSTEM", address, balance, models.TransactionTypeCoin)
                
                // Add an output for the new balance
                output := &models.TxOutput{
                        Value: balance,
                        Owner: address,
                }
                balanceUpdateTx.Outputs = append(balanceUpdateTx.Outputs, output)
                
                // Create a new block with this transaction
                block := models.NewBlock([]*models.Transaction{balanceUpdateTx}, bc.currentBlock)
                
                // Save the block
                if err := bc.storage.SaveBlock(block); err != nil {
                        utils.Error("Failed to save block for balance update: %v", err)
                }
                bc.currentBlock = block
                
                // Save the transaction
                if err := bc.storage.SaveTransaction(balanceUpdateTx); err != nil {
                        utils.Error("Failed to save balance update transaction: %v", err)
                }
                
                // Create new UTXO
                newUTXO := &models.UTXO{
                        TxID:        balanceUpdateTx.ID,
                        TxOutputIdx: 0,
                        Amount:      balance,
                        Owner:       address,
                        IsSpent:     false,
                        BlockHeight: block.Height,
                }
                
                // Save the UTXO
                if err := bc.storage.SaveUTXO(newUTXO); err != nil {
                        utils.Error("Failed to save UTXO for balance update: %v", err)
                }
                
                // Update UTXO set
                utxoSet, err := bc.storage.GetUTXOSet()
                if err != nil {
                        utils.Error("Failed to get UTXO set: %v", err)
                        utxoSet = models.NewUTXOSet()
                }
                
                // Mark existing UTXOs as spent in the set
                for _, utxo := range currentUTXOs {
                        utxoKey := fmt.Sprintf("%s:%d", utxo.TxID, utxo.TxOutputIdx)
                        if utxoInSet, exists := utxoSet.UTXOs[utxoKey]; exists {
                                utxoInSet.IsSpent = true
                        }
                }
                
                // Add new UTXO to the set
                utxoSet.AddUTXO(newUTXO)
                
                // Save updated UTXO set
                if err := bc.storage.SaveUTXOSet(utxoSet); err != nil {
                        utils.Error("Failed to save UTXO set: %v", err)
                }
        }
        
        // Always update the balance in storage for backward compatibility
        return bc.storage.UpdateBalance(address, balance)
}

// ProcessTransaction processes a transaction received from the network
func (bc *Blockchain) ProcessTransaction(tx *models.Transaction) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()

        // Get transaction details
        from := tx.GetSender()
        to := tx.GetReceiver()
        amount := tx.GetAmount()
        txType := tx.GetType()

        utils.Info("Processing transaction: From=%s, To=%s, Amount=%.2f, Type=%s", 
                from, to, amount, string(txType))

        // Verify address format
        if !wallet.ValidateAddress(from) || !wallet.ValidateAddress(to) {
                return errors.New("invalid address format")
        }

        // Check if transaction already exists to avoid duplicates
        txKey := fmt.Sprintf("tx_%s", tx.GetID())
        
        // Check if the transaction exists by trying to retrieve its data
        txData, err := bc.storage.ReadKey(txKey)
        if err == nil && len(txData) > 0 {
                // Already processed this transaction
                utils.Info("Transaction %s already processed, skipping", tx.GetID())
                return nil
        }

        // Create new block with this transaction
        block := models.NewBlock([]*models.Transaction{tx}, bc.currentBlock)
        
        // Save block
        if err := bc.storage.SaveBlock(block); err != nil {
                return fmt.Errorf("failed to save block: %v", err)
        }
        
        // For UTXO model, process inputs and outputs
        if len(tx.Inputs) > 0 {
                // Process UTXO inputs by marking them as spent
                for _, input := range tx.Inputs {
                        if err := bc.storage.MarkUTXOSpent(input.TxID, input.OutputIndex); err != nil {
                                utils.Error("Failed to mark UTXO as spent: %v", err)
                                // Continue processing even if this fails
                        }
                }
                
                // Create new UTXOs from outputs
                for i, output := range tx.Outputs {
                        utxo := &models.UTXO{
                                TxID:        tx.ID,
                                TxOutputIdx: i,
                                Amount:      output.Value,
                                Owner:       output.Owner,
                                IsSpent:     false,
                                BlockHeight: block.Height,
                        }
                        
                        if err := bc.storage.SaveUTXO(utxo); err != nil {
                                utils.Error("Failed to save UTXO: %v", err)
                                // Continue processing even if this fails
                        }
                }
                
                // Update UTXO set
                utxoSet, err := bc.storage.GetUTXOSet()
                if err != nil {
                        utils.Error("Failed to get UTXO set: %v", err)
                        utxoSet = models.NewUTXOSet()
                }
                
                // Process inputs (mark as spent in the set)
                for _, input := range tx.Inputs {
                        utxoKey := fmt.Sprintf("%s:%d", input.TxID, input.OutputIndex)
                        if utxo, exists := utxoSet.UTXOs[utxoKey]; exists {
                                utxo.IsSpent = true
                        }
                }
                
                // Process outputs (add to the set)
                for i, output := range tx.Outputs {
                        utxo := &models.UTXO{
                                TxID:        tx.ID,
                                TxOutputIdx: i,
                                Amount:      output.Value,
                                Owner:       output.Owner,
                                IsSpent:     false,
                                BlockHeight: block.Height,
                        }
                        utxoSet.AddUTXO(utxo)
                }
                
                if err := bc.storage.SaveUTXOSet(utxoSet); err != nil {
                        utils.Error("Failed to save UTXO set: %v", err)
                        // Continue processing even if this fails
                }
        } else {
                // Backward compatibility: Update balances directly
                // Check if sender has enough balance
                balance, err := bc.storage.GetBalance(from)
                if err != nil {
                        return fmt.Errorf("failed to get sender balance: %v", err)
                }

                if balance < amount {
                        return fmt.Errorf("insufficient balance: %f < %f", balance, amount)
                }
                
                // Update balances
                if err := bc.storage.UpdateBalance(from, balance-amount); err != nil {
                        return fmt.Errorf("failed to update sender balance: %v", err)
                }
                
                toBalance, err := bc.storage.GetBalance(to)
                if err != nil {
                        toBalance = 0
                }
                
                if err := bc.storage.UpdateBalance(to, toBalance+amount); err != nil {
                        // Rollback the sender's balance if receiver update fails
                        _ = bc.storage.UpdateBalance(from, balance)
                        return fmt.Errorf("failed to update receiver balance: %v", err)
                }
                
                // Also create UTXOs for backward compatibility
                // Mark old balances as spent
                utxos, _ := bc.storage.GetUTXOsForAddress(from)
                for _, utxo := range utxos {
                        if !utxo.IsSpent {
                                _ = bc.storage.MarkUTXOSpent(utxo.TxID, utxo.TxOutputIdx)
                        }
                }
                
                // Create new UTXO for receiver
                if amount > 0 {
                        receiverUTXO := &models.UTXO{
                                TxID:        tx.ID,
                                TxOutputIdx: 0,
                                Amount:      amount,
                                Owner:       to,
                                IsSpent:     false,
                                BlockHeight: block.Height,
                        }
                        _ = bc.storage.SaveUTXO(receiverUTXO)
                }
                
                // Create change UTXO for sender if needed
                if balance > amount {
                        changeUTXO := &models.UTXO{
                                TxID:        tx.ID,
                                TxOutputIdx: 1,
                                Amount:      balance - amount,
                                Owner:       from,
                                IsSpent:     false,
                                BlockHeight: block.Height,
                        }
                        _ = bc.storage.SaveUTXO(changeUTXO)
                }
        }
        
        // Save the transaction
        if err := bc.storage.SaveTransaction(tx); err != nil {
                utils.Error("Failed to save transaction: %v", err)
                // No need to rollback balances as they're already updated in the blockchain
        }

        // Update current block
        bc.currentBlock = block
        
        utils.Info("Transaction %s successfully processed in block %s", 
                tx.GetID(), block.GetHash())

        return nil
}

// SendCoins sends coins from one address to another
func (bc *Blockchain) SendCoins(from, to string, amount float64) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()

        // Verify addresses
        if !wallet.ValidateAddress(from) || !wallet.ValidateAddress(to) {
                return errors.New("invalid address format")
        }

        // Get UTXO set
        utxoSet, err := bc.storage.GetUTXOSet()
        if err != nil {
                utils.Error("Failed to get UTXO set, falling back to direct balance: %v", err)
                utxoSet = models.NewUTXOSet()
        }
        
        // Try to use UTXO model first
        var tx *models.Transaction
        
        // Check if we have any UTXOs
        utxos := utxoSet.GetUTXOsForAddress(from)
        if len(utxos) > 0 {
                // Use the UTXO model
                tx, err = models.NewUTXOTransaction(from, to, amount, models.TransactionTypeCoin, utxoSet)
                if err != nil {
                        // Fall back to legacy transaction if UTXO creation fails
                        utils.Error("UTXO transaction creation failed: %v, falling back to legacy", err)
                        tx = nil
                }
        }
        
        // Fall back to legacy model if UTXO failed or no UTXOs found
        if tx == nil {
                // Check if sender has enough balance
                balance, err := bc.storage.GetBalance(from)
                if err != nil {
                        return err
                }
                if balance < amount {
                        return fmt.Errorf("insufficient balance: %.10f", balance)
                }

                // Create transaction (legacy model)
                tx = models.NewTransaction(from, to, amount, models.TransactionTypeCoin)
                
                // Add output records for UTXO tracking
                // Output to recipient
                recipientOutput := &models.TxOutput{
                        Value: amount,
                        Owner: to,
                }
                tx.Outputs = append(tx.Outputs, recipientOutput)
                
                // Change output back to sender if needed
                if balance > amount {
                        changeOutput := &models.TxOutput{
                                Value: balance - amount,
                                Owner: from,
                        }
                        tx.Outputs = append(tx.Outputs, changeOutput)
                }
        }
        
        // Create new block
        block := models.NewBlock([]*models.Transaction{tx}, bc.currentBlock)
        
        // Save block
        if err := bc.storage.SaveBlock(block); err != nil {
                return err
        }
        bc.currentBlock = block
        
        if len(tx.Inputs) > 0 {
                // Process UTXO transaction
                // Mark inputs as spent
                for _, input := range tx.Inputs {
                        if err := bc.storage.MarkUTXOSpent(input.TxID, input.OutputIndex); err != nil {
                                utils.Error("Failed to mark UTXO as spent: %v", err)
                        }
                }
                
                // Create new UTXOs from outputs
                for i, output := range tx.Outputs {
                        utxo := &models.UTXO{
                                TxID:        tx.ID,
                                TxOutputIdx: i,
                                Amount:      output.Value,
                                Owner:       output.Owner,
                                IsSpent:     false,
                                BlockHeight: block.Height,
                        }
                        
                        if err := bc.storage.SaveUTXO(utxo); err != nil {
                                utils.Error("Failed to save UTXO: %v", err)
                        }
                }
                
                // Update UTXO set
                for _, input := range tx.Inputs {
                        utxoKey := fmt.Sprintf("%s:%d", input.TxID, input.OutputIndex)
                        if utxo, exists := utxoSet.UTXOs[utxoKey]; exists {
                                utxo.IsSpent = true
                        }
                }
                
                for i, output := range tx.Outputs {
                        utxo := &models.UTXO{
                                TxID:        tx.ID,
                                TxOutputIdx: i,
                                Amount:      output.Value,
                                Owner:       output.Owner,
                                IsSpent:     false,
                                BlockHeight: block.Height,
                        }
                        utxoSet.AddUTXO(utxo)
                }
                
                if err := bc.storage.SaveUTXOSet(utxoSet); err != nil {
                        utils.Error("Failed to save UTXO set: %v", err)
                }
        } else {
                // Legacy transaction - update balances directly
                balance, _ := bc.storage.GetBalance(from)
                toBalance, _ := bc.storage.GetBalance(to)
                
                // Update balances
                if err := bc.storage.UpdateBalance(from, balance-amount); err != nil {
                        return err
                }
                
                if err := bc.storage.UpdateBalance(to, toBalance+amount); err != nil {
                        return err
                }
                
                // Also create UTXOs for backward compatibility
                // Mark old UTXOs as spent
                oldUtxos, _ := bc.storage.GetUTXOsForAddress(from)
                for _, utxo := range oldUtxos {
                        if !utxo.IsSpent {
                                _ = bc.storage.MarkUTXOSpent(utxo.TxID, utxo.TxOutputIdx)
                        }
                }
                
                // Create new UTXOs
                for i, output := range tx.Outputs {
                        utxo := &models.UTXO{
                                TxID:        tx.ID,
                                TxOutputIdx: i,
                                Amount:      output.Value,
                                Owner:       output.Owner,
                                IsSpent:     false,
                                BlockHeight: block.Height,
                        }
                        _ = bc.storage.SaveUTXO(utxo)
                }
        }
        
        // Save the transaction
        if err := bc.storage.SaveTransaction(tx); err != nil {
                utils.Error("Failed to save transaction: %v", err)
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
                
                // The actual participants
                originalSender := response.Receiver // The person who sent the original message
                originalReceiver := response.Sender // The person who is now responding
                
                // Create a rewards transaction
                rewardTx := models.NewTransaction("SYSTEM", "REWARDS", 0, models.TransactionTypeMessageReward)
                rewardTx.SetData("messageID", response.GetReplyToID())
                
                // Add outputs for UTXO tracking
                // Output to original sender
                senderOutput := &models.TxOutput{
                        Value: senderReward,
                        Owner: originalSender,
                }
                rewardTx.Outputs = append(rewardTx.Outputs, senderOutput)
                
                // Output to original receiver
                receiverOutput := &models.TxOutput{
                        Value: receiverReward,
                        Owner: originalReceiver,
                }
                rewardTx.Outputs = append(rewardTx.Outputs, receiverOutput)
                
                // Create new block with reward transaction
                block := models.NewBlock([]*models.Transaction{rewardTx}, bc.currentBlock)
                
                // Save block
                if err := bc.storage.SaveBlock(block); err != nil {
                        return fmt.Errorf("failed to save reward block: %v", err)
                }
                bc.currentBlock = block
                
                // Save the transaction
                if err := bc.storage.SaveTransaction(rewardTx); err != nil {
                        utils.Error("Failed to save reward transaction: %v", err)
                }
                
                // Process UTXO rewards
                // Get UTXO set
                utxoSet, err := bc.storage.GetUTXOSet()
                if err != nil {
                        utils.Error("Failed to get UTXO set: %v", err)
                        utxoSet = models.NewUTXOSet()
                }
                
                // Create new UTXOs from outputs
                for i, output := range rewardTx.Outputs {
                        utxo := &models.UTXO{
                                TxID:        rewardTx.ID,
                                TxOutputIdx: i,
                                Amount:      output.Value,
                                Owner:       output.Owner,
                                IsSpent:     false,
                                BlockHeight: block.Height,
                        }
                        
                        if err := bc.storage.SaveUTXO(utxo); err != nil {
                                utils.Error("Failed to save reward UTXO: %v", err)
                        }
                        
                        // Also add to UTXO set
                        utxoSet.AddUTXO(utxo)
                }
                
                // Save updated UTXO set
                if err := bc.storage.SaveUTXOSet(utxoSet); err != nil {
                        utils.Error("Failed to save UTXO set: %v", err)
                }
                
                // Legacy: Update balances directly too for backward compatibility
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
                        utils.Error("Failed to update sender balance: %v", err)
                }
                
                if err := bc.storage.UpdateBalance(originalReceiver, receiverBalance+receiverReward); err != nil {
                        utils.Error("Failed to update receiver balance: %v", err)
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

// RegisterValidator registers an address as a validator and broadcasts it to the network
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

        // Save validator to storage
        err = bc.storage.SaveValidator(validator)
        if err != nil {
                return fmt.Errorf("failed to save validator: %v", err)
        }

        // Signal that a new validator was created - this will be used by other parts
        // of the system to broadcast the validator to the network
        utils.Info("New validator registered: %s with deposit %.2f DOU", address, deposit)
        
        // Validator was successfully registered
        return nil
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
        description := ""  // Default empty description
        tags := []string{} // Default empty tags
        group := messaging.NewGroup(creator, name, description, isPublic, tags)
        
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
        // Access the Group fields directly
        if !group.IsPublic && group.Creator != address {
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

// SyncValidator syncs a validator from another node
func (bc *Blockchain) SyncValidator(validator *models.Validator) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()
        
        // Check if we already have this validator
        if existingValidator, exists := bc.validators[validator.Address]; exists {
                // If we already have this validator, check if we need to update its information
                if existingValidator.GetLastRewardTime().Before(validator.GetLastRewardTime()) {
                        // The incoming validator has newer reward information, update ours
                        bc.validators[validator.Address] = validator
                        utils.Info("Updated existing validator %s with newer information", validator.GetAddress())
                        
                        // Save updated validator to storage
                        return bc.storage.SaveValidator(validator)
                }
                return nil // Already have this validator with up-to-date info
        }
        
        // Add validator to our list
        bc.validators[validator.Address] = validator
        utils.Info("Added new validator from network sync: %s with stake %.2f DOU", 
                validator.GetAddress(), validator.GetStake())
        
        // Save to storage
        return bc.storage.SaveValidator(validator)
}

// GetValidatorMinDeposit returns the minimum validator deposit
func (bc *Blockchain) GetValidatorMinDeposit() float64 {
        return bc.validatorMinDeposit
}

// GetLastBlock returns the last block in the blockchain
func (bc *Blockchain) GetLastBlock() (*models.Block, error) {
        return bc.storage.GetLastBlock()
}

// GetNodeCount returns the current node count in the blockchain
func (bc *Blockchain) GetNodeCount() (int, error) {
        return bc.storage.GetNodeCount()
}

// GenesisAllocation allocates genesis funds to a new wallet
func (bc *Blockchain) GenesisAllocation(address string, amount float64) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()
        
        // Check for existing balance to prevent double allocation
        existingBalance, err := bc.storage.GetBalance(address)
        if err == nil && existingBalance > 0 {
                return fmt.Errorf("address %s already has funds (%f DOU)", address, existingBalance)
        }
        
        // Also check for existing UTXOs
        utxos, err := bc.storage.GetUTXOsForAddress(address)
        if err == nil && len(utxos) > 0 {
                var utxoBalance float64
                for _, utxo := range utxos {
                        if !utxo.IsSpent {
                                utxoBalance += utxo.Amount
                        }
                }
                if utxoBalance > 0 {
                        return fmt.Errorf("address %s already has funds (%f DOU) in UTXOs", address, utxoBalance)
                }
        }
        
        // Get the last block
        lastBlock, err := bc.storage.GetLastBlock()
        if err != nil {
                return fmt.Errorf("failed to get last block: %v", err)
        }
        
        // Create genesis transaction
        tx := models.NewTransaction("SYSTEM", address, amount, models.TransactionTypeGenesis)
        
        // Add output for UTXO tracking
        output := &models.TxOutput{
                Value: amount,
                Owner: address,
        }
        tx.Outputs = append(tx.Outputs, output)
        
        // Add to a new block
        block := models.NewBlock([]*models.Transaction{tx}, lastBlock)
        
        // Save block
        if err := bc.storage.SaveBlock(block); err != nil {
                return fmt.Errorf("failed to save block with genesis allocation: %v", err)
        }
        bc.currentBlock = block
        
        // Save the transaction
        if err := bc.storage.SaveTransaction(tx); err != nil {
                utils.Error("Failed to save genesis transaction: %v", err)
        }
        
        // Create UTXO for the genesis funds
        utxo := &models.UTXO{
                TxID:        tx.ID,
                TxOutputIdx: 0,
                Amount:      amount,
                Owner:       address,
                IsSpent:     false,
                BlockHeight: block.Height,
        }
        
        // Save UTXO
        if err := bc.storage.SaveUTXO(utxo); err != nil {
                utils.Error("Failed to save genesis UTXO: %v", err)
        }
        
        // Update UTXO set
        utxoSet, err := bc.storage.GetUTXOSet()
        if err != nil {
                utils.Error("Failed to get UTXO set: %v", err)
                utxoSet = models.NewUTXOSet()
        }
        
        utxoSet.AddUTXO(utxo)
        
        if err := bc.storage.SaveUTXOSet(utxoSet); err != nil {
                utils.Error("Failed to save UTXO set: %v", err)
        }
        
        // Legacy: Update balance directly too for backward compatibility
        if err := bc.storage.UpdateBalance(address, amount); err != nil {
                return fmt.Errorf("failed to update balance: %v", err)
        }
        
        // Increment node count
        if err := bc.storage.IncrementNodeCount(); err != nil {
                return fmt.Errorf("failed to increment node count: %v", err)
        }
        
        return nil
}

// Helper methods

// ConvertLegacyBalancesToUTXOs converts all legacy balances to UTXOs in the system
func (bc *Blockchain) ConvertLegacyBalancesToUTXOs() error {
        bc.mu.Lock()
        defer bc.mu.Unlock()
        
        // Get all balances from legacy system
        balances, err := bc.storage.GetAllBalances()
        if err != nil {
                return fmt.Errorf("failed to get all balances: %v", err)
        }
        
        utils.Info("Converting %d legacy balances to UTXO format", len(balances))
        
        // Create a transaction for each balance
        for address, balance := range balances {
                // Check if we already have UTXOs for this address
                utxos, err := bc.storage.GetUTXOsForAddress(address)
                if err == nil && len(utxos) > 0 {
                        utils.Info("Address %s already has %d UTXOs, skipping conversion", address, len(utxos))
                        continue
                }
                
                // Skip zero balances
                if balance <= 0 {
                        continue
                }
                
                // Create a system transaction for this balance conversion
                conversionTx := models.NewTransaction("SYSTEM", address, balance, models.TransactionTypeCoin)
                conversionTx.SetData("info", "Legacy balance conversion to UTXO")
                
                // Add an output for the balance
                output := &models.TxOutput{
                        Value: balance,
                        Owner: address,
                }
                conversionTx.Outputs = append(conversionTx.Outputs, output)
                
                // Create a new block with this transaction
                block := models.NewBlock([]*models.Transaction{conversionTx}, bc.currentBlock)
                
                // Save the block
                if err := bc.storage.SaveBlock(block); err != nil {
                        utils.Error("Failed to save block for balance conversion: %v", err)
                        continue
                }
                bc.currentBlock = block
                
                // Save the transaction
                if err := bc.storage.SaveTransaction(conversionTx); err != nil {
                        utils.Error("Failed to save conversion transaction: %v", err)
                }
                
                // Create new UTXO
                newUTXO := &models.UTXO{
                        TxID:        conversionTx.ID,
                        TxOutputIdx: 0,
                        Amount:      balance,
                        Owner:       address,
                        IsSpent:     false,
                        BlockHeight: block.Height,
                }
                
                // Save the UTXO
                if err := bc.storage.SaveUTXO(newUTXO); err != nil {
                        utils.Error("Failed to save UTXO for balance conversion: %v", err)
                        continue
                }
                
                // Update UTXO set
                utxoSet, err := bc.storage.GetUTXOSet()
                if err != nil {
                        utxoSet = models.NewUTXOSet()
                }
                
                // Add new UTXO to the set
                utxoSet.AddUTXO(newUTXO)
                
                // Save updated UTXO set
                if err := bc.storage.SaveUTXOSet(utxoSet); err != nil {
                        utils.Error("Failed to save UTXO set: %v", err)
                }
                
                utils.Info("Converted balance of %.2f DOU for address %s to UTXO format", balance, address)
        }
        
        return nil
}

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
        var eligibleValidators []string
        for addr, v := range bc.validators {
                if v.IsEligible() {
                        activeValidators++
                        eligibleValidators = append(eligibleValidators, addr)
                }
        }

        if activeValidators == 0 {
                return nil // No validators to reward
        }

        // Calculate reward per validator
        rewardPerValidator := amount / float64(activeValidators)

        // Create a validator reward transaction
        validatorTx := models.NewTransaction("SYSTEM", "VALIDATOR_REWARDS", amount, models.TransactionTypeValidatorReward)
        
        // Add outputs for each validator
        for _, addr := range eligibleValidators {
                validatorOutput := &models.TxOutput{
                        Value: rewardPerValidator,
                        Owner: addr,
                }
                validatorTx.Outputs = append(validatorTx.Outputs, validatorOutput)
        }
        
        // Create new block with validator reward transaction
        block := models.NewBlock([]*models.Transaction{validatorTx}, bc.currentBlock)
        
        // Save block
        if err := bc.storage.SaveBlock(block); err != nil {
                return fmt.Errorf("failed to save validator reward block: %v", err)
        }
        bc.currentBlock = block
        
        // Save the transaction
        if err := bc.storage.SaveTransaction(validatorTx); err != nil {
                utils.Error("Failed to save validator reward transaction: %v", err)
        }
        
        // Process UTXO rewards
        // Get UTXO set
        utxoSet, err := bc.storage.GetUTXOSet()
        if err != nil {
                utils.Error("Failed to get UTXO set: %v", err)
                utxoSet = models.NewUTXOSet()
        }
        
        // Create new UTXOs from outputs
        for i, output := range validatorTx.Outputs {
                utxo := &models.UTXO{
                        TxID:        validatorTx.ID,
                        TxOutputIdx: i,
                        Amount:      output.Value,
                        Owner:       output.Owner,
                        IsSpent:     false,
                        BlockHeight: block.Height,
                }
                
                if err := bc.storage.SaveUTXO(utxo); err != nil {
                        utils.Error("Failed to save validator reward UTXO: %v", err)
                }
                
                // Also add to UTXO set
                utxoSet.AddUTXO(utxo)
        }
        
        // Save updated UTXO set
        if err := bc.storage.SaveUTXOSet(utxoSet); err != nil {
                utils.Error("Failed to save UTXO set: %v", err)
        }
        
        // Legacy: Update balances directly for backward compatibility
        for addr, v := range bc.validators {
                if v.IsEligible() {
                        // Get current balance
                        balance, err := bc.storage.GetBalance(addr)
                        if err != nil {
                                balance = 0
                        }

                        // Update balance
                        if err := bc.storage.UpdateBalance(addr, balance+rewardPerValidator); err != nil {
                                utils.Error("Failed to update validator balance: %v", err)
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
        
        // We know this is a *models.Validator, so use it directly
        v := validator.(*models.Validator)
        
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
        
        // We know this is a *models.Validator, so use it directly
        v := validator.(*models.Validator)
        
        return v, nil
}

// GetStorage returns the blockchain's storage
func (bc *Blockchain) GetStorage() *storage.LevelDBStorage {
        return bc.storage
}

// GetCurrentBlock returns the current block in the blockchain
func (bc *Blockchain) GetCurrentBlock() *models.Block {
        bc.mu.RLock()
        defer bc.mu.RUnlock()
        return bc.currentBlock
}

// AddBlock adds a new block to the blockchain
func (bc *Blockchain) AddBlock(block *models.Block) error {
        bc.mu.Lock()
        defer bc.mu.Unlock()

        // Verify block connects to our current chain
        if block.GetPrevHash() != bc.currentBlock.GetHash() {
                return fmt.Errorf("block doesn't connect to our current chain")
        }

        // Verify block height is correct
        if block.GetHeight() != bc.currentBlock.GetHeight()+1 {
                return fmt.Errorf("incorrect block height")
        }

        // Process all transactions in the block
        for _, tx := range block.GetTransactions() {
                // Skip if transaction already exists
                txKey := fmt.Sprintf("tx_%s", tx.GetID())
                txData, err := bc.storage.ReadKey(txKey)
                if err == nil && len(txData) > 0 {
                        // Already processed this transaction
                        utils.Debug("Transaction %s already processed, skipping", tx.GetID())
                        continue
                }

                // Update balances
                from := tx.GetSender()
                to := tx.GetReceiver()
                amount := tx.GetAmount()

                // Skip the balance update for genesis transactions
                if tx.GetType() != models.TransactionTypeGenesis {
                        // Update sender balance (if not system)
                        if from != "SYSTEM" {
                                senderBalance, err := bc.storage.GetBalance(from)
                                if err != nil {
                                        return fmt.Errorf("failed to get sender balance: %v", err)
                                }

                                if err := bc.storage.UpdateBalance(from, senderBalance-amount); err != nil {
                                        return fmt.Errorf("failed to update sender balance: %v", err)
                                }
                        }

                        // Update receiver balance
                        receiverBalance, err := bc.storage.GetBalance(to)
                        if err != nil {
                                receiverBalance = 0
                        }

                        if err := bc.storage.UpdateBalance(to, receiverBalance+amount); err != nil {
                                return fmt.Errorf("failed to update receiver balance: %v", err)
                        }
                }

                // Mark transaction as processed
                if err := bc.storage.WriteKey(txKey, []byte("processed")); err != nil {
                        utils.Error("Failed to mark transaction as processed: %v", err)
                }
        }

        // Save the block
        if err := bc.storage.SaveBlock(block); err != nil {
                return fmt.Errorf("failed to save block: %v", err)
        }

        // Update current block
        bc.currentBlock = block

        // Mark block as processed
        blockKey := fmt.Sprintf("block_%s", block.GetHash())
        if err := bc.storage.WriteKey(blockKey, []byte("processed")); err != nil {
                utils.Error("Failed to mark block as processed: %v", err)
        }

        utils.Info("Added block %s at height %d with %d transactions", 
                block.GetHash(), block.GetHeight(), len(block.GetTransactions()))

        return nil
}

// ConvertLegacyBalancesToUTXOs converts legacy account balances to the UTXO model
func (bc *Blockchain) ConvertLegacyBalancesToUTXOs() error {
        bc.mu.Lock()
        defer bc.mu.Unlock()

        fmt.Println("Starting balance conversion process...")
        
        // Get all balances from legacy storage
        balances, err := bc.storage.GetAllBalances()
        if err != nil {
                return fmt.Errorf("failed to retrieve balances: %v", err)
        }

        // Create a new UTXO set
        utxoSet := models.NewUTXOSet()
        
        // Track converted addresses for logging
        var convertedAddresses []string
        var totalConverted float64

        // Generate conversion transaction for each address with non-zero balance
        txID := utils.GenerateID() // Single transaction ID for all conversions
        outputIdx := 0 // Start with output index 0
        
        for address, balance := range balances {
                if balance <= 0 {
                        continue // Skip zero or negative balances
                }
                
                // Create a new UTXO for this address
                utxo := &models.UTXO{
                        TxID:        txID,
                        TxOutputIdx: outputIdx,
                        Amount:      balance,
                        Owner:       address,
                        IsSpent:     false,
                        BlockHeight: bc.currentBlock.Height,
                }
                
                // Add the UTXO to the set
                utxoKey := txID + ":" + fmt.Sprintf("%d", outputIdx)
                utxoSet.UTXOs[utxoKey] = utxo
                
                // Save the individual UTXO
                err = bc.storage.SaveUTXO(utxo)
                if err != nil {
                        return fmt.Errorf("failed to save UTXO for address %s: %v", address, err)
                }
                
                // Log the conversion
                convertedAddresses = append(convertedAddresses, address)
                totalConverted += balance
                outputIdx++
        }
        
        // Save the UTXO set
        err = bc.storage.SaveUTXOSet(utxoSet)
        if err != nil {
                return fmt.Errorf("failed to save UTXO set: %v", err)
        }
        
        // Log conversion results
        fmt.Printf("Conversion completed successfully!\n")
        fmt.Printf("Converted %d addresses with a total of %.2f DOU\n", len(convertedAddresses), totalConverted)
        
        return nil
}


