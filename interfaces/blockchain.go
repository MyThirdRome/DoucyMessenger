package interfaces

// Blockchain defines the interface for blockchain methods
// needed by the P2P Server
type Blockchain interface {
	// ProcessTransaction processes a transaction received from the network
	ProcessTransaction(tx interface{}) error
	
	// GetCurrentBlock returns the latest block in the blockchain 
	GetCurrentBlock() interface{}
	
	// GetHeight returns the current blockchain height
	GetHeight() (uint64, error)
	
	// GetValidators returns the list of validators
	GetValidators() ([]*interface{}, error)
	
	// AddBlock adds a block to the blockchain
	AddBlock(block interface{}) error
}