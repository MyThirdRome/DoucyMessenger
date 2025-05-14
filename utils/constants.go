package utils

// Constants for blockchain parameters
const (
	// Network
	DefaultPort = "8000"
	DefaultHost = "0.0.0.0"
	
	// Blockchain
	MinValidatorDeposit       = 50.0
	AnnualDepositIncrease     = 0.10  // 10%
	ValidatorRewardMultiplier = 1.5   // 150%
	ValidatorAPY              = 0.17  // 17%
	
	// Message Rewards
	SenderReward   = 0.75
	ReceiverReward = 0.25
	
	// Rate Limiting
	MessageRateLimitTotal = 200 // Maximum messages per hour
	MessageRateLimitAddr  = 30  // Maximum messages to a single address per hour
	
	// Addresses
	AddressPrefix          = "Dou"
	AddressSuffix          = "cyA"
	AddressMiddleLength    = 10
	GroupAddressMiddleLength = 6
	
	// Genesis
	GenesisReward         = 15000.0
	MaxGenesisNodeCount   = 10
	
	// Database
	DefaultDBPath = "./doucya_data"
)

// TransactionType represents the type of a transaction
type TransactionType string

// Transaction types
const (
	TransactionTypeCoin        TransactionType = "COIN"
	TransactionTypeMessage     TransactionType = "MESSAGE"
	TransactionTypeGenesis     TransactionType = "GENESIS"
	TransactionTypeValidatorIn TransactionType = "VALIDATOR_IN"
	TransactionTypeValidatorOut TransactionType = "VALIDATOR_OUT"
)

// GroupType represents the type of a group or channel
type GroupType string

// Group types
const (
	GroupTypePublic  GroupType = "PUBLIC"
	GroupTypePrivate GroupType = "PRIVATE"
)
