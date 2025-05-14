package interfaces

import (
        "time"
)

// Block represents a blockchain block
type Block interface {
        GetHash() string
        GetPrevHash() string
        GetTimestamp() time.Time
        GetHeight() uint64
}

// Transaction represents a blockchain transaction
type Transaction interface {
        GetID() string
        GetSender() string
        GetReceiver() string
        GetAmount() float64
        GetTimestamp() time.Time
        GetSignature() string
}

// Message represents a message in the messaging system
type Message interface {
        GetID() string
        GetSender() string
        GetReceiver() string
        GetContent() string
        GetTimestamp() time.Time
        GetSignature() string
        IsEncrypted() bool
}

// Validator represents a blockchain validator
type Validator interface {
        GetAddress() string
        GetStake() float64
        GetCreationTime() time.Time
        GetLastRewardTime() time.Time
}

// Storage defines the interface for blockchain storage
type Storage interface {
        SaveBlock(block Block) error
        GetBlock(hash string) (Block, error)
        GetBlockByHeight(height uint64) (Block, error)
        GetLatestBlock() (Block, error)
        GetBlockHeight() (uint64, error)
        
        SaveTransaction(tx Transaction) error
        GetTransaction(id string) (Transaction, error)
        GetTransactionsByAddress(address string) ([]Transaction, error)
        
        SaveMessage(message Message) error
        GetMessage(id string) (Message, error)
        GetMessagesByAddress(address string) ([]Message, error)
        GetMessagesForAddress(address string) ([]Message, error)
        HasMessageFromTo(from, to string) (bool, error)
        
        SaveValidator(validator Validator) error
        GetValidator(address string) (Validator, error)
        GetValidators() ([]Validator, error)
        DeleteValidator(address string) error
        
        SaveWallet(address string, data []byte) error
        
        SaveGroup(group interface{}) error
        GetGroup(id string) (interface{}, error)
        AddMemberToGroup(groupID, address string) error
        
        SaveChannel(channel interface{}) error
        GetChannel(id string) (interface{}, error)
        
        SaveWhitelist(address string, whitelist []string) error
        
        GetBalance(address string) (float64, error)
        UpdateBalance(address string, amount float64) error
        
        SavePendingRewards(rewards interface{}) error
        GetPendingRewards() (interface{}, error)
        
        Close() error
}