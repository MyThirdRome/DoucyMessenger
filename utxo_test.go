package main

import (
	"fmt"
	"github.com/doucya/config"
	"github.com/doucya/core"
	"github.com/doucya/storage"
	"github.com/doucya/utils"
	"log"
	"os"
	"testing"
)

// TestUTXOConversion tests the UTXO conversion functionality
func TestUTXOConversion(t *testing.T) {
	// Initialize logger
	if err := utils.InitLogger(utils.LogLevelInfo, true, false); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}
	defer utils.CloseLogger()

	// Database path
	dbPath := "./doucya_data"

	// Initialize storage
	store, err := storage.NewLevelDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Create blockchain config object
	blockchainConfig := &utils.Config{
		ValidatorMinDeposit: 50.0,
		SenderReward:        0.75,
		ReceiverReward:      0.25,
		ValidatorRewardMult: 1.5,
	}

	// Create blockchain
	blockchain, err := core.NewBlockchain(store, blockchainConfig)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}

	// Execute UTXO conversion directly
	err = blockchain.ConvertLegacyBalancesToUTXOs()
	if err != nil {
		t.Errorf("Error converting balances to UTXOs: %v", err)
	} else {
		fmt.Println("Successfully converted legacy balances to UTXOs!")
	}
}

// Helper main function to run the test directly
func main() {
	fmt.Println("=== Testing UTXO Conversion ===")
	result := testing.Verbose()
	fmt.Printf("Testing mode: verbose = %v\n", result)
	
	// Run the test
	TestUTXOConversion(&testing.T{})
	
	fmt.Println("=== Test Complete ===")
}