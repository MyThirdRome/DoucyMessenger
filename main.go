package main

import (
        "flag"
        "fmt"
        "log"
        "os"
        "os/signal"
        "syscall"

        "github.com/doucya/cmd"
        "github.com/doucya/config"
        "github.com/doucya/core"
        "github.com/doucya/models"
        "github.com/doucya/p2p"
        "github.com/doucya/utils"
        "github.com/doucya/storage"
)

var (
        // Constants
        minValidatorDeposit = 50.0
        senderReward        = 0.75
        receiverReward      = 0.25
        validatorRewardMult = 1.5
)

func main() {
        // Parse command line flags
        initNode := flag.Bool("init", false, "Initialize a new node")
        dbPath := flag.String("db", "./doucya_data", "Path to database directory")
        cliOnly := flag.Bool("cli", false, "Start in CLI-only mode")
        testWallet := flag.Bool("testwallet", false, "Test wallet creation with genesis allocation")
        p2pPort := flag.Int("port", 8333, "P2P network port")
        flag.Parse()

        // Load configuration
        cfg := config.NewConfig(*p2pPort, *dbPath)

        // Initialize storage
        store, err := storage.NewLevelDBStorage(*dbPath)
        if err != nil {
                log.Fatalf("Failed to initialize storage: %v", err)
        }
        defer store.Close()

        // Create blockchain config object
        blockchainConfig := &utils.Config{
                ValidatorMinDeposit: minValidatorDeposit,
                SenderReward:        senderReward,
                ReceiverReward:      receiverReward,
                ValidatorRewardMult: validatorRewardMult,
        }
        
        // Create blockchain
        blockchain, err := core.NewBlockchain(store, blockchainConfig)
        if err != nil {
                log.Fatalf("Failed to create blockchain: %v", err)
        }
        
        // Initialize P2P server
        p2pPortStr := fmt.Sprintf(":%d", *p2pPort)
        bootstrapNodes := []string{} // Empty list for now
        p2pServer := p2p.NewServer(p2pPortStr, blockchain, bootstrapNodes)

        // Check if we need to initialize a new node
        if *initNode {
                fmt.Println("Initializing new node...")
                
                // Create CLI and initialize node
                cli := cmd.NewCLI(blockchain, p2pServer, store, cfg)
                if err := cli.InitializeNode(); err != nil {
                        log.Fatalf("Failed to initialize node: %v", err)
                }
                
                // If we're in CLI-only mode, just exit after initialization
                if *cliOnly {
                        return
                }
        }
        
        // Test wallet creation with genesis allocation
        if *testWallet {
                // Create CLI
                cli := cmd.NewCLI(blockchain, p2pServer, store, cfg)
                
                // Create a new wallet
                fmt.Println("\n=== Testing Wallet Creation ===")
                address, err := cli.CreateNewWallet()
                if err != nil {
                        log.Fatalf("Failed to create wallet: %v", err)
                }
                
                // Check the wallet balance
                balance, err := store.GetBalance(address)
                if err != nil {
                        log.Fatalf("Failed to get wallet balance: %v", err)
                }
                
                // Get node count
                nodeCount, err := store.GetNodeCount()
                if err != nil {
                        log.Fatalf("Failed to get node count: %v", err)
                }
                
                fmt.Printf("\nWallet created with address: %s\n", address)
                fmt.Printf("Wallet balance: %.2f DOU\n", balance)
                fmt.Printf("Current node count: %d\n", nodeCount)
                fmt.Println("=== Test Complete ===\n")
                
                return
        }

        // If we're in CLI-only mode, start the CLI and return
        if *cliOnly {
                cli := cmd.NewCLI(blockchain, p2pServer, store, cfg)
                cli.Start()
                return
        }

        // Start P2P server
        go p2pServer.Start()

        // Start CLI in a goroutine
        cli := cmd.NewCLI(blockchain, p2pServer, store, cfg)
        
        // Display welcome message
        fmt.Println("\nDoucyA Blockchain - P2P Messaging Platform")
        fmt.Println("------------------------------------------")
        fmt.Println("• Create or import a wallet with 'createwallet' or 'importwallet'")
        fmt.Println("• Connect to peers with 'addpeer <address:port>'")
        fmt.Println("• Type 'help' for more commands")
        fmt.Println("------------------------------------------")
        
        go cli.Start()

        // Print node status
        printNodeStatus(store, p2pServer)

        // Wait for interrupt signal to gracefully shutdown
        c := make(chan os.Signal, 1)
        signal.Notify(c, os.Interrupt, syscall.SIGTERM)
        <-c

        fmt.Println("\nShutting down...")
}

// initializeNode initializes a new node
func initializeNode(store *storage.LevelDBStorage) error {
        // Check if we already have blocks
        lastBlock, err := store.GetLastBlock()
        if err != nil {
                return fmt.Errorf("failed to check last block: %v", err)
        }

        // Create genesis block if this is a new blockchain
        if lastBlock == nil {
                fmt.Println("Creating genesis block...")
                genesis := models.CreateGenesisBlock()
                if err := store.SaveBlock(genesis); err != nil {
                        return fmt.Errorf("failed to save genesis block: %v", err)
                }
                lastBlock = genesis
        }

        // Don't automatically create a wallet
        wallets, err := store.GetWallets()
        if err != nil {
                return fmt.Errorf("failed to check existing wallets: %v", err)
        }

        if len(wallets) > 0 {
                fmt.Println("Found existing wallets in storage")
                fmt.Println("You can use 'wallet' command to view wallet details")
        } else {
                fmt.Println("No wallets found in storage")
                fmt.Println("Use 'createwallet' to create a new wallet or 'importwallet' to import an existing one")
                fmt.Println("First 10 wallets on the network will receive 15,000 DOU genesis allocation")
        }

        // Check the current node count to inform the user
        nodeCount, err := store.GetNodeCount()
        if err != nil {
                fmt.Printf("Error getting node count: %v\n", err)
        } else {
                if nodeCount < 10 {
                        fmt.Println("---------------------------------------------")
                        fmt.Println("ℹ️ Currently there are " + fmt.Sprintf("%d", nodeCount) + " nodes on the network")
                        fmt.Println("ℹ️ First 10 wallets will receive 15,000 DOU genesis allocation")
                        fmt.Println("---------------------------------------------")
                } else {
                        fmt.Println("---------------------------------------------")
                        fmt.Println("ℹ️ Currently there are " + fmt.Sprintf("%d", nodeCount) + " nodes on the network")
                        fmt.Println("ℹ️ The 15,000 DOU genesis allocation has been fully distributed")
                        fmt.Println("💬 You can earn DOU by participating in messaging and validation.")
                        fmt.Println("---------------------------------------------")
                }
        }
        
        // Still increment the node count for accurate tracking
        if err := store.IncrementNodeCount(); err != nil {
                return fmt.Errorf("failed to increment node count: %v", err)
        }

        return nil
}

// printNodeStatus prints the status of the node
func printNodeStatus(store *storage.LevelDBStorage, p2pServer *p2p.Server) {
        lastBlock, err := store.GetLastBlock()
        if err != nil {
                fmt.Printf("Error getting last block: %v\n", err)
                return
        }

        if lastBlock == nil {
                fmt.Println("No blocks in the blockchain yet.")
                return
        }

        fmt.Println("\nBlockchain Status:")
        fmt.Printf("  Current Height: %d\n", lastBlock.Height)
        fmt.Printf("  Latest Block Hash: %s\n", lastBlock.Hash)

        // Calculate total circulating supply
        totalSupply, err := calculateTotalSupply(store)
        if err != nil {
                fmt.Printf("Error calculating total supply: %v\n", err)
        } else {
                fmt.Printf("  Circulating Supply: %.2f DOU\n", totalSupply)
        }

        // Get validators count only (not detailed info)
        validators, err := store.GetValidators()
        if err != nil {
                fmt.Printf("Error getting validators: %v\n", err)
        } else {
                fmt.Printf("  Active Validators: %d\n", len(validators))
        }

        // Get node count
        nodeCount, err := store.GetNodeCount()
        if err != nil {
                fmt.Printf("Error getting node count: %v\n", err)
        } else {
                fmt.Printf("  Total Nodes: %d", nodeCount)
                if nodeCount < 10 {
                        fmt.Printf(" (First 10 nodes receive 15,000 DOU each)\n")
                } else {
                        fmt.Println()
                }
        }
        
        // Get connected peers count if p2pServer is available
        fmt.Printf("  Connected Peers: %d\n", len(p2pServer.GetPeers()))
        
        fmt.Println("\nUse 'createwallet' to create a new wallet or 'importwallet' to import an existing one.")
        fmt.Println("Type 'help' for available commands.")
}

// calculateTotalSupply calculates the total amount of DOU in circulation
func calculateTotalSupply(store *storage.LevelDBStorage) (float64, error) {
        wallets, err := store.GetWallets()
        if err != nil {
                return 0, err
        }
        
        var totalSupply float64
        for _, w := range wallets {
                balance, err := store.GetBalance(w.GetAddress())
                if err != nil {
                        continue
                }
                totalSupply += balance
        }
        
        return totalSupply, nil
}