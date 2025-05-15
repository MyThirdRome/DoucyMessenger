package test

import (
        "fmt"
        "log"

        "github.com/doucya/config"
        "github.com/doucya/core"
        "github.com/doucya/storage"
        "github.com/doucya/utils"
)

// TestUTXOConvert tests the UTXO conversion functionality directly
func main() {
        // Initialize logger
        if err := utils.InitLogger(utils.LogLevelInfo, true, false); err != nil {
                log.Printf("Failed to initialize logger: %v", err)
        }
        defer utils.CloseLogger()

        // Database path
        dbPath := "./doucya_data"

        // Log startup
        fmt.Println("Testing UTXO conversion functionality...")

        // Initialize storage
        store, err := storage.NewLevelDBStorage(dbPath)
        if err != nil {
                log.Fatalf("Failed to initialize storage: %v", err)
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
                log.Fatalf("Failed to create blockchain: %v", err)
        }

        // Print header for test
        fmt.Println("\n=== Executing UTXO Conversion ===")
        
        // Execute UTXO conversion directly
        err = blockchain.ConvertLegacyBalancesToUTXOs()
        if err != nil {
                fmt.Printf("Error converting balances to UTXOs: %v\n", err)
        } else {
                fmt.Println("Successfully converted legacy balances to UTXOs!")
        }

        fmt.Println("\n=== Test Complete ===")
}