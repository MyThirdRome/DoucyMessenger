package models

import (
        "crypto/sha256"
        "encoding/hex"
        "encoding/json"
        "time"
)

// Block represents a block in the blockchain
type Block struct {
        Height       int64         `json:"height"`
        Timestamp    time.Time     `json:"timestamp"`
        Hash         string        `json:"hash"`
        PrevHash     string        `json:"prev_hash"`
        Transactions []*Transaction `json:"transactions"`
        Validator    string        `json:"validator"`
        Signature    string        `json:"signature"`
}

// NewBlock creates a new block with the given transactions and previous block
func NewBlock(transactions []*Transaction, prevBlock *Block) *Block {
        block := &Block{
                Height:       prevBlock.Height + 1,
                Timestamp:    time.Now(),
                PrevHash:     prevBlock.Hash,
                Transactions: transactions,
                Validator:    "", // To be set by validator
                Signature:    "", // To be signed by validator
        }
        
        // Calculate hash
        block.Hash = block.CalculateHash()
        
        return block
}

// CreateGenesisBlock creates the genesis block
func CreateGenesisBlock() *Block {
        genesis := &Block{
                Height:       0,
                Timestamp:    time.Now(),
                Hash:         "",
                PrevHash:     "",
                Transactions: []*Transaction{},
                Validator:    "",
                Signature:    "",
        }
        
        // Calculate hash
        genesis.Hash = genesis.CalculateHash()
        
        return genesis
}

// CalculateHash calculates the hash of the block
func (b *Block) CalculateHash() string {
        // Marshal block data excluding the hash itself
        blockData := struct {
                Height       int64         `json:"height"`
                Timestamp    time.Time     `json:"timestamp"`
                PrevHash     string        `json:"prev_hash"`
                Transactions []*Transaction `json:"transactions"`
                Validator    string        `json:"validator"`
        }{
                Height:       b.Height,
                Timestamp:    b.Timestamp,
                PrevHash:     b.PrevHash,
                Transactions: b.Transactions,
                Validator:    b.Validator,
        }
        
        data, err := json.Marshal(blockData)
        if err != nil {
                return ""
        }
        
        hash := sha256.Sum256(data)
        return hex.EncodeToString(hash[:])
}

// VerifyBlock verifies the block's integrity and signature
func (b *Block) VerifyBlock() bool {
        // Verify hash
        if b.Hash != b.CalculateHash() {
                return false
        }
        
        // If it's the genesis block, no need to verify signature
        if b.Height == 0 {
                return true
        }
        
        // Verify signature
        if b.Validator == "" || b.Signature == "" {
                return false
        }
        
        // In a real implementation, we would verify the validator's signature here
        // using the validator's public key
        
        return true
}

// SignBlock signs the block with the validator's private key
func (b *Block) SignBlock(privateKeyHex string) error {
        // In a real implementation, we would sign the block hash with the private key
        // For simplicity, we'll just set a dummy signature
        b.Signature = "signed:" + b.Hash
        
        return nil
}

// GetHash returns the block hash
func (b *Block) GetHash() string {
        return b.Hash
}

// GetPrevHash returns the previous block hash
func (b *Block) GetPrevHash() string {
        return b.PrevHash
}

// GetTimestamp returns the block timestamp
func (b *Block) GetTimestamp() time.Time {
        return b.Timestamp
}

// GetTransactions returns the block's transactions
func (b *Block) GetTransactions() []*Transaction {
        return b.Transactions
}

// GetHeight returns the block height
func (b *Block) GetHeight() uint64 {
        return uint64(b.Height)
}

// MarshalJSON customizes the JSON marshaling for Block
func (b *Block) MarshalJSON() ([]byte, error) {
        type BlockAlias Block
        return json.Marshal(&struct {
                Timestamp string `json:"timestamp"`
                *BlockAlias
        }{
                Timestamp:  b.Timestamp.Format(time.RFC3339),
                BlockAlias: (*BlockAlias)(b),
        })
}

// UnmarshalJSON customizes the JSON unmarshaling for Block
func (b *Block) UnmarshalJSON(data []byte) error {
        type BlockAlias Block
        aux := &struct {
                Timestamp string `json:"timestamp"`
                *BlockAlias
        }{
                BlockAlias: (*BlockAlias)(b),
        }
        
        if err := json.Unmarshal(data, &aux); err != nil {
                return err
        }
        
        timestamp, err := time.Parse(time.RFC3339, aux.Timestamp)
        if err != nil {
                return err
        }
        b.Timestamp = timestamp
        
        return nil
}