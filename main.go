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
        "github.com/doucya/storage"
        "github.com/doucya/wallet"
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
        blockchain := core.NewBlockchain(store, blockchainConfig)
        
        // Initialize P2P server
        p2pServer := p2p.NewServer(*p2pPort, blockchain)

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
        go cli.Start()

        // Print node status
        printNodeStatus(store)

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

        // Create a wallet for this node
        w := wallet.NewWallet()
        if err := store.SaveWallet(w); err != nil {
                return fmt.Errorf("failed to save wallet: %v", err)
        }

        fmt.Printf("Created new wallet with address: %s\n", w.GetAddress())
        fmt.Printf("Private key: %s\n", w.GetPrivateKeyString())
        fmt.Println("IMPORTANT: Save this private key securely. It cannot be recovered if lost.")

        // Check if we're one of the first 10 nodes to get genesis allocation
        nodeCount, err := store.GetNodeCount()
        if err != nil {
                return fmt.Errorf("failed to get node count: %v", err)
        }

        if nodeCount < 10 {
                // We're one of the first 10 nodes, give us 15,000 DOU
                fmt.Println("Congratulations! You are one of the first 10 nodes.")
                fmt.Println("You receive 15,000 DOU.")

                // Create genesis transaction
                tx := models.NewTransaction("", w.GetAddress(), 15000.0, models.TransactionTypeGenesis)
                
                // Add to a new block
                block := models.NewBlock([]*models.Transaction{tx}, lastBlock)
                
                // Save block
                if err := store.SaveBlock(block); err != nil {
                        return fmt.Errorf("failed to save block with genesis allocation: %v", err)
                }
                
                // Update balance
                if err := store.UpdateBalance(w.GetAddress(), 15000.0); err != nil {
                        return fmt.Errorf("failed to update balance: %v", err)
                }

                // Increment node count
                if err := store.IncrementNodeCount(); err != nil {
                        return fmt.Errorf("failed to increment node count: %v", err)
                }
        } else {
                fmt.Println("You are not one of the first 10 nodes, so you don't receive the genesis allocation.")
        }

        return nil
}

// printNodeStatus prints the status of the node
func printNodeStatus(store *storage.LevelDBStorage) {
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

        // Get wallets
        wallets, err := store.GetWallets()
        if err != nil {
                fmt.Printf("Error getting wallets: %v\n", err)
        } else {
                fmt.Printf("  Wallets: %d\n", len(wallets))
                for _, w := range wallets {
                        balance, err := store.GetBalance(w.GetAddress())
                        if err != nil {
                                fmt.Printf("    %s: Error getting balance: %v\n", w.GetAddress(), err)
                        } else {
                                fmt.Printf("    %s: %.10f DOU\n", w.GetAddress(), balance)
                        }
                }
        }

        // Get validators
        validators, err := store.GetValidators()
        if err != nil {
                fmt.Printf("Error getting validators: %v\n", err)
        } else {
                fmt.Printf("  Validators: %d\n", len(validators))
                for _, v := range validators {
                        fmt.Printf("    %s: %.10f DOU\n", v.Address, v.Deposit)
                }
        }

        // Get node count
        nodeCount, err := store.GetNodeCount()
        if err != nil {
                fmt.Printf("Error getting node count: %v\n", err)
        } else {
                fmt.Printf("  Total Nodes: %d\n", nodeCount)
        }
}