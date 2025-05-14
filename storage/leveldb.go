package storage

import (
        "encoding/json"
        "errors"
        "fmt"
        "os"
        "path/filepath"
        "sync"

        "github.com/doucya/interfaces"
        "github.com/doucya/messaging"
        "github.com/doucya/models"
        "github.com/doucya/wallet"
        "github.com/syndtr/goleveldb/leveldb"
        "github.com/syndtr/goleveldb/leveldb/opt"
        "github.com/syndtr/goleveldb/leveldb/util"
)

// Key prefixes for different types of data
const (
        BlockPrefix           = "block_"
        LastBlockKey          = "last_block"
        TransactionPrefix     = "tx_"
        WalletPrefix          = "wallet_"
        BalancePrefix         = "balance_"       // Legacy - will be replaced by UTXOs
        ValidatorPrefix       = "validator_"
        MessagePrefix         = "message_"
        WhitelistPrefix       = "whitelist_"
        GroupPrefix           = "group_"
        GroupMemberPrefix     = "group_member_"
        ChannelPrefix         = "channel_"
        ChannelMemberPrefix   = "channel_member_"
        PendingRewardsKey     = "pending_rewards"
        NodeCountKey          = "node_count"
        UTXOPrefix            = "utxo_"          // For individual UTXOs
        UTXOSetKey            = "utxo_set"       // For storing the entire UTXO set
)

// LevelDBStorage implements blockchain storage using LevelDB
type LevelDBStorage struct {
        db        *leveldb.DB
        mu        sync.RWMutex
}

// NewLevelDBStorage creates a new LevelDB storage
func NewLevelDBStorage(dbPath string) (*LevelDBStorage, error) {
        // Ensure directory exists
        if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
                return nil, fmt.Errorf("failed to create database directory: %v", err)
        }

        // Open LevelDB
        options := &opt.Options{
                ErrorIfMissing: false,
                NoSync:         false,
        }
        
        db, err := leveldb.OpenFile(dbPath, options)
        if err != nil {
                return nil, fmt.Errorf("failed to open database: %v", err)
        }

        return &LevelDBStorage{
                db: db,
        }, nil
}

// Close closes the database
func (s *LevelDBStorage) Close() error {
        s.mu.Lock()
        defer s.mu.Unlock()
        return s.db.Close()
}

// SaveBlock saves a block to the database
func (s *LevelDBStorage) SaveBlock(block *models.Block) error {
        s.mu.Lock()
        defer s.mu.Unlock()

        // Marshal block to JSON
        blockData, err := json.Marshal(block)
        if err != nil {
                return fmt.Errorf("failed to marshal block: %v", err)
        }

        // Store block by height
        blockKey := fmt.Sprintf("%s%d", BlockPrefix, block.Height)
        if err := s.db.Put([]byte(blockKey), blockData, nil); err != nil {
                return fmt.Errorf("failed to save block: %v", err)
        }

        // Store last block reference
        if err := s.db.Put([]byte(LastBlockKey), []byte(blockKey), nil); err != nil {
                return fmt.Errorf("failed to update last block: %v", err)
        }

        // Store transactions
        for _, tx := range block.Transactions {
                txData, err := json.Marshal(tx)
                if err != nil {
                        return fmt.Errorf("failed to marshal transaction: %v", err)
                }

                txKey := fmt.Sprintf("%s%s", TransactionPrefix, tx.ID)
                if err := s.db.Put([]byte(txKey), txData, nil); err != nil {
                        return fmt.Errorf("failed to save transaction: %v", err)
                }
        }

        return nil
}

// GetBlock retrieves a block by height
func (s *LevelDBStorage) GetBlock(height int64) (*models.Block, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        // Get block by height
        blockKey := fmt.Sprintf("%s%d", BlockPrefix, height)
        blockData, err := s.db.Get([]byte(blockKey), nil)
        if err != nil {
                return nil, fmt.Errorf("failed to get block: %v", err)
        }

        // Unmarshal block
        var block models.Block
        if err := json.Unmarshal(blockData, &block); err != nil {
                return nil, fmt.Errorf("failed to unmarshal block: %v", err)
        }

        return &block, nil
}

// GetLastBlock retrieves the last block in the blockchain
func (s *LevelDBStorage) GetLastBlock() (*models.Block, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        // Get last block key
        lastBlockKeyData, err := s.db.Get([]byte(LastBlockKey), nil)
        if err != nil {
                if err == leveldb.ErrNotFound {
                        return nil, nil // No blocks yet
                }
                return nil, fmt.Errorf("failed to get last block key: %v", err)
        }

        // Get block data
        blockData, err := s.db.Get(lastBlockKeyData, nil)
        if err != nil {
                return nil, fmt.Errorf("failed to get last block: %v", err)
        }

        // Unmarshal block
        var block models.Block
        if err := json.Unmarshal(blockData, &block); err != nil {
                return nil, fmt.Errorf("failed to unmarshal last block: %v", err)
        }

        return &block, nil
}

// SaveWallet saves a wallet to the database
func (s *LevelDBStorage) SaveWallet(w *wallet.Wallet) error {
        s.mu.Lock()
        defer s.mu.Unlock()

        // Marshal wallet to JSON
        walletData, err := json.Marshal(w)
        if err != nil {
                return fmt.Errorf("failed to marshal wallet: %v", err)
        }

        // Store wallet by address
        walletKey := fmt.Sprintf("%s%s", WalletPrefix, w.GetAddress())
        if err := s.db.Put([]byte(walletKey), walletData, nil); err != nil {
                return fmt.Errorf("failed to save wallet: %v", err)
        }

        return nil
}

// GetWallet retrieves a wallet by address
func (s *LevelDBStorage) GetWallet(address string) (*wallet.Wallet, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        // Get wallet by address
        walletKey := fmt.Sprintf("%s%s", WalletPrefix, address)
        walletData, err := s.db.Get([]byte(walletKey), nil)
        if err != nil {
                return nil, fmt.Errorf("failed to get wallet: %v", err)
        }

        // Unmarshal wallet
        var w wallet.Wallet
        if err := json.Unmarshal(walletData, &w); err != nil {
                return nil, fmt.Errorf("failed to unmarshal wallet: %v", err)
        }

        return &w, nil
}

// GetWallets retrieves all wallets
func (s *LevelDBStorage) GetWallets() ([]*wallet.Wallet, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        wallets := []*wallet.Wallet{}
        iter := s.db.NewIterator(util.BytesPrefix([]byte(WalletPrefix)), nil)
        defer iter.Release()

        for iter.Next() {
                var w wallet.Wallet
                if err := json.Unmarshal(iter.Value(), &w); err != nil {
                        return nil, fmt.Errorf("failed to unmarshal wallet: %v", err)
                }
                wallets = append(wallets, &w)
        }

        if err := iter.Error(); err != nil {
                return nil, fmt.Errorf("error iterating wallets: %v", err)
        }

        return wallets, nil
}

// GetBalance retrieves the balance for an address
func (s *LevelDBStorage) GetBalance(address string) (float64, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        // Try to get balance from UTXOs first (preferred method)
        utxoBalance, err := s.GetBalanceFromUTXOs(address)
        if err == nil && utxoBalance > 0 {
                return utxoBalance, nil
        }

        // Get balance by address (legacy method)
        balanceKey := fmt.Sprintf("%s%s", BalancePrefix, address)
        balanceData, err := s.db.Get([]byte(balanceKey), nil)
        if err != nil {
                if err == leveldb.ErrNotFound {
                        return 0, nil // No balance yet
                }
                return 0, fmt.Errorf("failed to get balance: %v", err)
        }

        // Parse balance
        var balance float64
        if err := json.Unmarshal(balanceData, &balance); err != nil {
                return 0, fmt.Errorf("failed to unmarshal balance: %v", err)
        }

        return balance, nil
}

// UpdateBalance updates the balance for an address
func (s *LevelDBStorage) UpdateBalance(address string, balance float64) error {
        s.mu.Lock()
        defer s.mu.Unlock()

        // Marshal balance to JSON
        balanceData, err := json.Marshal(balance)
        if err != nil {
                return fmt.Errorf("failed to marshal balance: %v", err)
        }

        // Store balance by address
        balanceKey := fmt.Sprintf("%s%s", BalancePrefix, address)
        if err := s.db.Put([]byte(balanceKey), balanceData, nil); err != nil {
                return fmt.Errorf("failed to update balance: %v", err)
        }

        return nil
}

// GetAllBalances retrieves all balances from the database
func (s *LevelDBStorage) GetAllBalances() (map[string]float64, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        // Try to get balances from UTXOs first (preferred method)
        utxoBalances, err := s.GetAllBalancesFromUTXOs()
        if err == nil && len(utxoBalances) > 0 {
                return utxoBalances, nil
        }

        // Fall back to legacy balance system
        balances := make(map[string]float64)
        iter := s.db.NewIterator(util.BytesPrefix([]byte(BalancePrefix)), nil)
        defer iter.Release()

        for iter.Next() {
                // Extract address from the key (remove prefix)
                key := string(iter.Key())
                address := key[len(BalancePrefix):]
                
                // Unmarshal balance
                var balance float64
                if err := json.Unmarshal(iter.Value(), &balance); err != nil {
                        return nil, fmt.Errorf("failed to unmarshal balance for %s: %v", address, err)
                }
                
                balances[address] = balance
        }

        if err := iter.Error(); err != nil {
                return nil, fmt.Errorf("error iterating balances: %v", err)
        }

        return balances, nil
}

// SaveValidator saves a validator to the database
func (s *LevelDBStorage) SaveValidator(validator interface{}) error {
        v, ok := validator.(*models.Validator)
        if !ok {
                return fmt.Errorf("expected *models.Validator but got %T", validator)
        }
        s.mu.Lock()
        defer s.mu.Unlock()

        // Marshal validator to JSON
        validatorData, err := json.Marshal(v)
        if err != nil {
                return fmt.Errorf("failed to marshal validator: %v", err)
        }

        // Store validator by address
        validatorKey := fmt.Sprintf("%s%s", ValidatorPrefix, v.Address)
        if err := s.db.Put([]byte(validatorKey), validatorData, nil); err != nil {
                return fmt.Errorf("failed to save validator: %v", err)
        }

        return nil
}

// GetValidator retrieves a validator by address
func (s *LevelDBStorage) GetValidator(address string) (interface{}, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        // Get validator by address
        validatorKey := fmt.Sprintf("%s%s", ValidatorPrefix, address)
        validatorData, err := s.db.Get([]byte(validatorKey), nil)
        if err != nil {
                return nil, fmt.Errorf("failed to get validator: %v", err)
        }

        // Unmarshal validator
        var validator models.Validator
        if err := json.Unmarshal(validatorData, &validator); err != nil {
                return nil, fmt.Errorf("failed to unmarshal validator: %v", err)
        }

        return &validator, nil
}

// DeleteValidator removes a validator from the database
func (s *LevelDBStorage) DeleteValidator(address string) error {
        s.mu.Lock()
        defer s.mu.Unlock()

        // Delete validator by address
        validatorKey := fmt.Sprintf("%s%s", ValidatorPrefix, address)
        if err := s.db.Delete([]byte(validatorKey), nil); err != nil {
                return fmt.Errorf("failed to delete validator: %v", err)
        }

        return nil
}

// SaveTransaction saves a transaction to the database
func (s *LevelDBStorage) SaveTransaction(tx interfaces.Transaction) error {
        s.mu.Lock()
        defer s.mu.Unlock()

        // Get the transaction as a concrete type
        transaction, ok := tx.(*models.Transaction)
        if !ok {
                return fmt.Errorf("expected *models.Transaction but got %T", tx)
        }

        // Marshal transaction to JSON
        txData, err := json.Marshal(transaction)
        if err != nil {
                return fmt.Errorf("failed to marshal transaction: %v", err)
        }

        // Store transaction by ID
        txKey := fmt.Sprintf("%s%s", TransactionPrefix, tx.GetID())
        if err := s.db.Put([]byte(txKey), txData, nil); err != nil {
                return fmt.Errorf("failed to save transaction: %v", err)
        }

        return nil
}

// GetTransaction retrieves a transaction by ID
func (s *LevelDBStorage) GetTransaction(id string) (interfaces.Transaction, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        // Get transaction by ID
        txKey := fmt.Sprintf("%s%s", TransactionPrefix, id)
        txData, err := s.db.Get([]byte(txKey), nil)
        if err != nil {
                if err == leveldb.ErrNotFound {
                        return nil, nil // No transaction found
                }
                return nil, fmt.Errorf("failed to get transaction: %v", err)
        }

        // Parse transaction
        var tx models.Transaction
        if err := json.Unmarshal(txData, &tx); err != nil {
                return nil, fmt.Errorf("failed to unmarshal transaction: %v", err)
        }

        return &tx, nil
}

// GetTransactionsByAddress retrieves all transactions for an address (sender or receiver)
func (s *LevelDBStorage) GetTransactionsByAddress(address string) ([]interfaces.Transaction, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        transactions := []interfaces.Transaction{}
        iter := s.db.NewIterator(util.BytesPrefix([]byte(TransactionPrefix)), nil)
        defer iter.Release()

        for iter.Next() {
                var tx models.Transaction
                if err := json.Unmarshal(iter.Value(), &tx); err != nil {
                        return nil, fmt.Errorf("failed to unmarshal transaction: %v", err)
                }
                
                // Check if this transaction involves the address
                if tx.GetSender() == address || tx.GetReceiver() == address {
                        transactions = append(transactions, &tx)
                }
        }

        if err := iter.Error(); err != nil {
                return nil, fmt.Errorf("error iterating transactions: %v", err)
        }

        return transactions, nil
}

// GetValidators retrieves all validators
func (s *LevelDBStorage) GetValidators() ([]*models.Validator, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        validators := []*models.Validator{}
        iter := s.db.NewIterator(util.BytesPrefix([]byte(ValidatorPrefix)), nil)
        defer iter.Release()

        for iter.Next() {
                var validator models.Validator
                if err := json.Unmarshal(iter.Value(), &validator); err != nil {
                        return nil, fmt.Errorf("failed to unmarshal validator: %v", err)
                }
                validators = append(validators, &validator)
        }

        if err := iter.Error(); err != nil {
                return nil, fmt.Errorf("error iterating validators: %v", err)
        }

        return validators, nil
}

// SaveMessage saves a message to the database
func (s *LevelDBStorage) SaveMessage(message *messaging.Message) error {
        s.mu.Lock()
        defer s.mu.Unlock()

        // Marshal message to JSON
        messageData, err := json.Marshal(message)
        if err != nil {
                return fmt.Errorf("failed to marshal message: %v", err)
        }

        // Store message by ID
        messageKey := fmt.Sprintf("%s%s", MessagePrefix, message.ID)
        if err := s.db.Put([]byte(messageKey), messageData, nil); err != nil {
                return fmt.Errorf("failed to save message: %v", err)
        }

        // Add reference to recipient's inbox
        inboxKey := fmt.Sprintf("%s%s_%s", MessagePrefix, message.Receiver, message.ID)
        if err := s.db.Put([]byte(inboxKey), []byte(message.ID), nil); err != nil {
                return fmt.Errorf("failed to add message to inbox: %v", err)
        }

        // Add reference to sender's outbox
        outboxKey := fmt.Sprintf("%s%s_%s", MessagePrefix, message.Sender, message.ID)
        if err := s.db.Put([]byte(outboxKey), []byte(message.ID), nil); err != nil {
                return fmt.Errorf("failed to add message to outbox: %v", err)
        }

        return nil
}

// GetMessage retrieves a message by ID
func (s *LevelDBStorage) GetMessage(id string) (*messaging.Message, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        // Get message by ID
        messageKey := fmt.Sprintf("%s%s", MessagePrefix, id)
        messageData, err := s.db.Get([]byte(messageKey), nil)
        if err != nil {
                return nil, fmt.Errorf("failed to get message: %v", err)
        }

        // Unmarshal message
        var message messaging.Message
        if err := json.Unmarshal(messageData, &message); err != nil {
                return nil, fmt.Errorf("failed to unmarshal message: %v", err)
        }

        return &message, nil
}

// GetMessagesForAddress retrieves all messages for an address
func (s *LevelDBStorage) GetMessagesForAddress(address string) ([]*messaging.Message, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        messages := []*messaging.Message{}
        
        // Get all messages where address is receiver (inbox)
        inboxPrefix := fmt.Sprintf("%s%s_", MessagePrefix, address)
        iter := s.db.NewIterator(util.BytesPrefix([]byte(inboxPrefix)), nil)
        
        for iter.Next() {
                messageID := string(iter.Value())
                message, err := s.GetMessage(messageID)
                if err != nil {
                        continue // Skip messages with errors
                }
                messages = append(messages, message)
        }
        
        iter.Release()
        if err := iter.Error(); err != nil {
                return nil, fmt.Errorf("error iterating messages: %v", err)
        }

        return messages, nil
}

// HasMessageFromTo checks if there's a message from one address to another
func (s *LevelDBStorage) HasMessageFromTo(from, to string) (bool, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        // Get all messages where 'from' is sender
        outboxPrefix := fmt.Sprintf("%s%s_", MessagePrefix, from)
        iter := s.db.NewIterator(util.BytesPrefix([]byte(outboxPrefix)), nil)
        defer iter.Release()

        for iter.Next() {
                messageID := string(iter.Value())
                message, err := s.GetMessage(messageID)
                if err != nil {
                        continue // Skip messages with errors
                }
                
                if message.Receiver == to {
                        return true, nil
                }
        }

        if err := iter.Error(); err != nil {
                return false, fmt.Errorf("error iterating messages: %v", err)
        }

        return false, nil
}

// SaveWhitelist saves a whitelist entry
// SaveGroup saves a group to the database
func (s *LevelDBStorage) SaveGroup(group *messaging.Group) error {
        if group == nil {
                return errors.New("group is nil")
        }

        data, err := json.Marshal(group)
        if err != nil {
                return err
        }

        key := GroupPrefix + group.ID
        return s.db.Put([]byte(key), data, nil)
}

// GetGroup retrieves a group from the database
func (s *LevelDBStorage) GetGroup(groupID string) (*messaging.Group, error) {
        key := GroupPrefix + groupID
        data, err := s.db.Get([]byte(key), nil)
        if err != nil {
                if err == leveldb.ErrNotFound {
                        return nil, nil
                }
                return nil, err
        }

        var group messaging.Group
        if err := json.Unmarshal(data, &group); err != nil {
                return nil, err
        }

        return &group, nil
}

// AddMemberToGroup adds a member to a group
func (s *LevelDBStorage) AddMemberToGroup(groupID, memberAddress string) error {
        key := GroupMemberPrefix + groupID + "_" + memberAddress
        return s.db.Put([]byte(key), []byte("1"), nil)
}

// SaveChannel saves a channel to the database
func (s *LevelDBStorage) SaveChannel(channel *messaging.Channel) error {
        if channel == nil {
                return errors.New("channel is nil")
        }

        data, err := json.Marshal(channel)
        if err != nil {
                return err
        }

        key := ChannelPrefix + channel.ID
        return s.db.Put([]byte(key), data, nil)
}

func (s *LevelDBStorage) SaveWhitelist(from, to string) error {
        s.mu.Lock()
        defer s.mu.Unlock()

        // Store whitelist entry
        whitelistKey := fmt.Sprintf("%s%s_%s", WhitelistPrefix, from, to)
        if err := s.db.Put([]byte(whitelistKey), []byte("1"), nil); err != nil {
                return fmt.Errorf("failed to save whitelist: %v", err)
        }

        return nil
}

// GetWhitelists retrieves all whitelist entries
func (s *LevelDBStorage) GetWhitelists() (map[string]map[string]bool, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        whitelists := make(map[string]map[string]bool)
        iter := s.db.NewIterator(util.BytesPrefix([]byte(WhitelistPrefix)), nil)
        defer iter.Release()

        for iter.Next() {
                key := string(iter.Key())
                // Extract from and to addresses from key (format: "whitelist_FROM_TO")
                parts := splitKey(key, WhitelistPrefix)
                if len(parts) < 2 {
                        continue
                }
                
                from := parts[0]
                to := parts[1]
                
                if _, exists := whitelists[from]; !exists {
                        whitelists[from] = make(map[string]bool)
                }
                whitelists[from][to] = true
        }

        if err := iter.Error(); err != nil {
                return nil, fmt.Errorf("error iterating whitelists: %v", err)
        }

        return whitelists, nil
}

// SavePendingRewards saves pending rewards
func (s *LevelDBStorage) SavePendingRewards(rewards map[string]map[string]float64) error {
        s.mu.Lock()
        defer s.mu.Unlock()

        // Marshal rewards to JSON
        rewardsData, err := json.Marshal(rewards)
        if err != nil {
                return fmt.Errorf("failed to marshal pending rewards: %v", err)
        }

        // Save to database
        if err := s.db.Put([]byte(PendingRewardsKey), rewardsData, nil); err != nil {
                return fmt.Errorf("failed to save pending rewards: %v", err)
        }

        return nil
}

// GetPendingRewards retrieves pending rewards
func (s *LevelDBStorage) GetPendingRewards() (map[string]map[string]float64, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        // Get from database
        rewardsData, err := s.db.Get([]byte(PendingRewardsKey), nil)
        if err != nil {
                if err == leveldb.ErrNotFound {
                        return make(map[string]map[string]float64), nil // No rewards yet
                }
                return nil, fmt.Errorf("failed to get pending rewards: %v", err)
        }

        // Unmarshal rewards
        var rewards map[string]map[string]float64
        if err := json.Unmarshal(rewardsData, &rewards); err != nil {
                return nil, fmt.Errorf("failed to unmarshal pending rewards: %v", err)
        }

        return rewards, nil
}

// GetNodeCount gets the number of nodes
func (s *LevelDBStorage) GetNodeCount() (int, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        // Get from database
        countData, err := s.db.Get([]byte(NodeCountKey), nil)
        if err != nil {
                if err == leveldb.ErrNotFound {
                        return 0, nil // No count yet
                }
                return 0, fmt.Errorf("failed to get node count: %v", err)
        }

        // Parse count
        var count int
        if err := json.Unmarshal(countData, &count); err != nil {
                return 0, fmt.Errorf("failed to unmarshal node count: %v", err)
        }

        return count, nil
}

// IncrementNodeCount increments the node count
func (s *LevelDBStorage) IncrementNodeCount() error {
        s.mu.Lock()
        defer s.mu.Unlock()

        // Get current count
        count, err := s.GetNodeCount()
        if err != nil {
                return fmt.Errorf("failed to get node count: %v", err)
        }

        // Increment count
        count++

        // Marshal count to JSON
        countData, err := json.Marshal(count)
        if err != nil {
                return fmt.Errorf("failed to marshal node count: %v", err)
        }

        // Save to database
        if err := s.db.Put([]byte(NodeCountKey), countData, nil); err != nil {
                return fmt.Errorf("failed to save node count: %v", err)
        }

        return nil
}

// Helper functions

// splitKey splits a key by prefix and separators
func splitKey(key, prefix string) []string {
        // Remove prefix
        if len(key) <= len(prefix) {
                return []string{}
        }
        key = key[len(prefix):]
        
        // Split by underscore
        var parts []string
        var part string
        
        for i := 0; i < len(key); i++ {
                if key[i] == '_' {
                        if part != "" {
                                parts = append(parts, part)
                                part = ""
                        }
                } else {
                        part += string(key[i])
                }
        }
        
        if part != "" {
                parts = append(parts, part)
        }
        
        return parts
}

// ReadKey reads raw data for the given key
func (s *LevelDBStorage) ReadKey(key string) ([]byte, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        // Get value for the key
        data, err := s.db.Get([]byte(key), nil)
        if err != nil {
                if err == leveldb.ErrNotFound {
                        return nil, nil // Key not found
                }
                return nil, fmt.Errorf("failed to read key %s: %v", key, err)
        }

        return data, nil
}

// WriteKey writes raw data for the given key
func (s *LevelDBStorage) WriteKey(key string, data []byte) error {
        s.mu.Lock()
        defer s.mu.Unlock()

        // Store value for the key
        if err := s.db.Put([]byte(key), data, nil); err != nil {
                return fmt.Errorf("failed to write key %s: %v", key, err)
        }

        return nil
}

// --------- UTXO Methods ---------

// SaveUTXOSet saves the entire UTXO set to the database
func (s *LevelDBStorage) SaveUTXOSet(utxoSet *models.UTXOSet) error {
        s.mu.Lock()
        defer s.mu.Unlock()

        // Marshal UTXO set to JSON
        utxoSetData, err := json.Marshal(utxoSet)
        if err != nil {
                return fmt.Errorf("failed to marshal UTXO set: %v", err)
        }

        // Save UTXO set
        if err := s.db.Put([]byte(UTXOSetKey), utxoSetData, nil); err != nil {
                return fmt.Errorf("failed to save UTXO set: %v", err)
        }

        return nil
}

// GetUTXOSet retrieves the entire UTXO set from the database
func (s *LevelDBStorage) GetUTXOSet() (*models.UTXOSet, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        // Get UTXO set data
        utxoSetData, err := s.db.Get([]byte(UTXOSetKey), nil)
        if err != nil {
                if err == leveldb.ErrNotFound {
                        // If UTXO set not found, return a new empty set
                        return models.NewUTXOSet(), nil
                }
                return nil, fmt.Errorf("failed to get UTXO set: %v", err)
        }

        // Parse UTXO set
        var utxoSet models.UTXOSet
        if err := json.Unmarshal(utxoSetData, &utxoSet); err != nil {
                return nil, fmt.Errorf("failed to unmarshal UTXO set: %v", err)
        }

        return &utxoSet, nil
}

// SaveUTXO saves a single UTXO to the database
func (s *LevelDBStorage) SaveUTXO(utxo *models.UTXO) error {
        s.mu.Lock()
        defer s.mu.Unlock()

        // Marshal UTXO to JSON
        utxoData, err := json.Marshal(utxo)
        if err != nil {
                return fmt.Errorf("failed to marshal UTXO: %v", err)
        }

        // Generate key
        utxoKey := fmt.Sprintf("%s%s_%d", UTXOPrefix, utxo.TxID, utxo.TxOutputIdx)

        // Save UTXO
        if err := s.db.Put([]byte(utxoKey), utxoData, nil); err != nil {
                return fmt.Errorf("failed to save UTXO: %v", err)
        }

        return nil
}

// GetUTXO retrieves a single UTXO by its ID and output index
func (s *LevelDBStorage) GetUTXO(txID string, outputIdx int) (*models.UTXO, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        // Generate key
        utxoKey := fmt.Sprintf("%s%s_%d", UTXOPrefix, txID, outputIdx)

        // Get UTXO data
        utxoData, err := s.db.Get([]byte(utxoKey), nil)
        if err != nil {
                if err == leveldb.ErrNotFound {
                        return nil, fmt.Errorf("UTXO not found: %s_%d", txID, outputIdx)
                }
                return nil, fmt.Errorf("failed to get UTXO: %v", err)
        }

        // Parse UTXO
        var utxo models.UTXO
        if err := json.Unmarshal(utxoData, &utxo); err != nil {
                return nil, fmt.Errorf("failed to unmarshal UTXO: %v", err)
        }

        return &utxo, nil
}

// GetUTXOsForAddress retrieves all UTXOs owned by an address
func (s *LevelDBStorage) GetUTXOsForAddress(address string) ([]*models.UTXO, error) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        utxos := []*models.UTXO{}
        iter := s.db.NewIterator(util.BytesPrefix([]byte(UTXOPrefix)), nil)
        defer iter.Release()

        for iter.Next() {
                var utxo models.UTXO
                if err := json.Unmarshal(iter.Value(), &utxo); err != nil {
                        return nil, fmt.Errorf("failed to unmarshal UTXO: %v", err)
                }
                
                if utxo.Owner == address && !utxo.IsSpent {
                        utxos = append(utxos, &utxo)
                }
        }

        if err := iter.Error(); err != nil {
                return nil, fmt.Errorf("error iterating UTXOs: %v", err)
        }

        return utxos, nil
}

// GetBalanceFromUTXOs calculates the balance of an address from UTXOs
func (s *LevelDBStorage) GetBalanceFromUTXOs(address string) (float64, error) {
        utxos, err := s.GetUTXOsForAddress(address)
        if err != nil {
                return 0, err
        }

        var balance float64
        for _, utxo := range utxos {
                balance += utxo.Amount
        }

        return balance, nil
}

// GetAllBalancesFromUTXOs retrieves balances for all addresses from UTXOs
func (s *LevelDBStorage) GetAllBalancesFromUTXOs() (map[string]float64, error) {
        // Try to get the full UTXO set first (more efficient)
        utxoSet, err := s.GetUTXOSet()
        if err == nil {
                return utxoSet.GetAllAddressBalances(), nil
        }

        // If that fails, iterate through all UTXOs
        s.mu.RLock()
        defer s.mu.RUnlock()

        balances := make(map[string]float64)
        iter := s.db.NewIterator(util.BytesPrefix([]byte(UTXOPrefix)), nil)
        defer iter.Release()

        for iter.Next() {
                var utxo models.UTXO
                if err := json.Unmarshal(iter.Value(), &utxo); err != nil {
                        return nil, fmt.Errorf("failed to unmarshal UTXO: %v", err)
                }
                
                if !utxo.IsSpent {
                        balances[utxo.Owner] += utxo.Amount
                }
        }

        if err := iter.Error(); err != nil {
                return nil, fmt.Errorf("error iterating UTXOs: %v", err)
        }

        return balances, nil
}

// MarkUTXOSpent marks a UTXO as spent
func (s *LevelDBStorage) MarkUTXOSpent(txID string, outputIdx int) error {
        // Get the UTXO
        utxo, err := s.GetUTXO(txID, outputIdx)
        if err != nil {
                return err
        }

        // Mark as spent
        utxo.IsSpent = true

        // Save the updated UTXO
        return s.SaveUTXO(utxo)
}