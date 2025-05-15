package adapters

import (
        "github.com/doucya/core"
        "github.com/doucya/models"
)

// BlockchainAdapter adapts the core.Blockchain to be used with the p2p server
type BlockchainAdapter struct {
        blockchain *core.Blockchain
}

// NewBlockchainAdapter creates a new adapter for the blockchain
func NewBlockchainAdapter(blockchain *core.Blockchain) *BlockchainAdapter {
        return &BlockchainAdapter{
                blockchain: blockchain,
        }
}

// ProcessTransaction processes a transaction received from the network
func (a *BlockchainAdapter) ProcessTransaction(tx interface{}) error {
        // This would call the appropriate method based on transaction type
        return nil // Placeholder
}

// GetCurrentBlockHeight returns the current blockchain height
func (a *BlockchainAdapter) GetCurrentBlockHeight() uint64 {
        if block := a.blockchain.GetCurrentBlock(); block != nil {
                // Convert int64 to uint64
                return uint64(block.Height)
        }
        return 0
}

// GetHeight returns the current blockchain height (alias for GetCurrentBlockHeight)
func (a *BlockchainAdapter) GetHeight() uint64 {
        return a.GetCurrentBlockHeight()
}

// GetValidators returns all validators in the blockchain
func (a *BlockchainAdapter) GetValidators() []interface{} {
        validators, _ := a.blockchain.GetValidators()
        result := make([]interface{}, 0, len(validators))
        for _, v := range validators {
                result = append(result, v)
        }
        return result
}

// GetLatestBlock returns the latest block in the blockchain
func (a *BlockchainAdapter) GetLatestBlock() interface{} {
        return a.blockchain.GetCurrentBlock()
}

// GetCurrentBlock returns the current block in the blockchain (alias for GetLatestBlock)
func (a *BlockchainAdapter) GetCurrentBlock() interface{} {
        return a.GetLatestBlock()
}

// GetStorage returns the storage interface
func (a *BlockchainAdapter) GetStorage() interface{} {
        return a.blockchain.GetStorage()
}

// AddBlock adds a block to the blockchain
func (a *BlockchainAdapter) AddBlock(block interface{}) error {
        if b, ok := block.(*models.Block); ok {
                return a.blockchain.AddBlock(b)
        }
        return nil // Handle other cases or return error
}

// GetBalance gets the balance for an address
func (a *BlockchainAdapter) GetBalance(address string) (float64, error) {
        return a.blockchain.GetBalance(address)
}

// UpdateBalance updates the balance for an address
func (a *BlockchainAdapter) UpdateBalance(address string, amount float64) error {
        return a.blockchain.UpdateBalance(address, amount)
}

// GetAllBalances gets all balances in the blockchain
func (a *BlockchainAdapter) GetAllBalances() (map[string]float64, error) {
        return a.blockchain.GetAllBalances()
}