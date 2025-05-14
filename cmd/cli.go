package cmd

import (
        "bufio"
        "encoding/json"
        "fmt"
        "os"
        "strconv"
        "strings"
        "time"

        "github.com/doucya/config"
        "github.com/doucya/core"
        "github.com/doucya/messaging"
        "github.com/doucya/models"
        "github.com/doucya/p2p"
        "github.com/doucya/storage"
        "github.com/doucya/wallet"
)

// CLI represents the command line interface for interacting with the DoucyA blockchain
type CLI struct {
        blockchain *core.Blockchain
        p2pServer  *p2p.Server
        storage    *storage.LevelDBStorage
        config     *config.Config
        running    bool
        scanner    *bufio.Scanner
}

// NewCLI creates a new CLI instance
func NewCLI(blockchain *core.Blockchain, p2pServer *p2p.Server, storage *storage.LevelDBStorage, config *config.Config) *CLI {
        return &CLI{
                blockchain: blockchain,
                p2pServer:  p2pServer,
                storage:    storage,
                config:     config,
                scanner:    bufio.NewScanner(os.Stdin),
        }
}

// Start begins the CLI interactive mode
func (cli *CLI) Start() {
        cli.running = true
        fmt.Println("\nWelcome to DoucyA Blockchain CLI!")
        fmt.Println("Type 'help' for a list of commands")

        // Initialize a scanner for reading stdin in a more reliable way
        scanner := bufio.NewScanner(os.Stdin)
        
        // Print initial prompt
        fmt.Print("> ")
        
        // Main CLI loop
        for cli.running && scanner.Scan() {
                // Get the command
                cmd := scanner.Text()
                
                // Skip empty commands
                if cmd == "" {
                        fmt.Print("> ")
                        continue
                }
                
                // Process the command
                cli.processCommand(cmd)
                
                // Print prompt after each command if we're still running
                if cli.running {
                        fmt.Print("> ")
                }
        }
        
        // Check for errors
        if err := scanner.Err(); err != nil {
                fmt.Printf("Scanner error: %v\n", err)
        }
}

// InitializeNode initializes a new node without creating a wallet automatically
func (cli *CLI) InitializeNode() error {
        // Check if the blockchain is properly initialized
        fmt.Println("DoucyA Blockchain node initialized successfully")
        fmt.Println("Use 'createwallet' to create a new wallet or 'importwallet' to import an existing one")
        fmt.Println("First 10 wallets will receive 15,000 DOU genesis allocation")
        return nil
}

// processCommand processes a CLI command
func (cli *CLI) processCommand(cmd string) {
        // Clean the command and extract args
        cleanCmd := strings.TrimSpace(cmd)
        args := strings.Fields(cleanCmd)
        if len(args) == 0 {
                return
        }

        switch args[0] {
        case "help":
                cli.printHelp()
        case "exit", "quit":
                cli.running = false
        case "createwallet", "createaddress":
                // Create a new wallet 
                _, err := cli.CreateNewWallet()
                if err != nil {
                        fmt.Printf("Error creating wallet: %v\n", err)
                }
        case "importwallet":
                if len(args) < 2 {
                        fmt.Println("Usage: importwallet PRIVATE_KEY")
                        return
                }
                address, err := cli.ImportWallet(args[1])
                if err != nil {
                        fmt.Printf("Error importing wallet: %v\n", err)
                } else {
                        fmt.Printf("Wallet imported successfully with address: %s\n", address)
                }
        case "listaddresses":
                cli.listAddresses()
        case "getbalance":
                if len(args) < 2 {
                        fmt.Println("Usage: getbalance ADDRESS")
                        return
                }
                cli.getBalance(args[1])
        case "send":
                if len(args) < 4 {
                        fmt.Println("Usage: send FROM TO AMOUNT")
                        return
                }
                amount, err := strconv.ParseFloat(args[3], 64)
                if err != nil {
                        fmt.Printf("Invalid amount: %v\n", err)
                        return
                }
                cli.sendCoins(args[1], args[2], amount)
        case "message":
                if len(args) < 3 {
                        fmt.Println("Usage: message FROM TO MESSAGE")
                        return
                }
                message := strings.Join(args[3:], " ")
                cli.sendMessage(args[1], args[2], message)
        case "readmessages":
                if len(args) < 2 {
                        fmt.Println("Usage: readmessages ADDRESS")
                        return
                }
                cli.readMessages(args[1])
        case "whitelist":
                if len(args) < 3 {
                        fmt.Println("Usage: whitelist FROM TO")
                        return
                }
                cli.whitelistAddress(args[1], args[2])
        case "startvalidating":
                if len(args) < 2 {
                        fmt.Println("Usage: startvalidating ADDRESS")
                        return
                }
                cli.startValidating(args[1])
        case "stopvalidating":
                if len(args) < 2 {
                        fmt.Println("Usage: stopvalidating ADDRESS")
                        return
                }
                cli.stopValidating(args[1])
        case "creategroup":
                if len(args) < 3 {
                        fmt.Println("Usage: creategroup ADDRESS NAME [public|private]")
                        return
                }
                isPublic := true
                if len(args) > 3 && args[3] == "private" {
                        isPublic = false
                }
                cli.createGroup(args[1], args[2], isPublic)
        case "joingroup":
                if len(args) < 3 {
                        fmt.Println("Usage: joingroup ADDRESS GROUP_ID")
                        return
                }
                cli.joinGroup(args[1], args[2])
        case "createchannel":
                if len(args) < 3 {
                        fmt.Println("Usage: createchannel ADDRESS NAME [public|private]")
                        return
                }
                isPublic := true
                if len(args) > 3 && args[3] == "private" {
                        isPublic = false
                }
                cli.createChannel(args[1], args[2], isPublic)
        case "status":
                cli.showStatus()
        case "peers":
                cli.showPeers()
        case "addpeer":
                if len(args) < 2 {
                        fmt.Println("Usage: addpeer ADDRESS")
                        return
                }
                cli.addPeer(args[1])
        case "sync":
                cli.syncWithPeers()
        case "reply":
                if len(args) < 4 {
                        fmt.Println("Usage: reply FROM TO MESSAGE_ID MESSAGE")
                        return
                }
                message := strings.Join(args[4:], " ")
                cli.replyToMessage(args[1], args[2], args[3], message)
        case "validators":
                cli.listValidators()
        case "messagerewards":
                if len(args) < 2 {
                        fmt.Println("Usage: messagerewards ADDRESS")
                        return
                }
                cli.showMessageRewards(args[1])
        case "stakerewards":
                if len(args) < 2 {
                        fmt.Println("Usage: stakerewards ADDRESS")
                        return
                }
                cli.showStakeRewards(args[1])
        case "increasestake":
                if len(args) < 3 {
                        fmt.Println("Usage: increasestake ADDRESS AMOUNT")
                        return
                }
                amount, err := strconv.ParseFloat(args[2], 64)
                if err != nil {
                        fmt.Printf("Invalid amount: %v\n", err)
                        return
                }
                cli.increaseStake(args[1], amount)
        case "decrypt":
                if len(args) < 4 {
                        fmt.Println("Usage: decrypt ADDRESS MESSAGE_ID PRIVATE_KEY")
                        return
                }
                cli.decryptMessage(args[1], args[2], args[3])
        default:
                fmt.Printf("Unknown command: %s\n", args[0])
                fmt.Println("Type 'help' for a list of commands")
        }
}

func (cli *CLI) printHelp() {
        fmt.Println("DoucyA Blockchain CLI Commands:")
        fmt.Println("=== General Commands ===")
        fmt.Println("  help                                - Show this help message")
        fmt.Println("  exit, quit                          - Exit the CLI")
        fmt.Println("  status                              - Show blockchain status")
        
        fmt.Println("\n=== Wallet Commands ===")
        fmt.Println("  createwallet                        - Create a new wallet")
        fmt.Println("  importwallet PRIVATE_KEY            - Import a wallet from a private key")
        fmt.Println("  listaddresses                       - List all wallet addresses")
        fmt.Println("  getbalance ADDRESS                  - Get balance for an address")
        fmt.Println("  send FROM TO AMOUNT                 - Send coins from one address to another")
        
        fmt.Println("\n=== Messaging Commands ===")
        fmt.Println("  message FROM TO MESSAGE             - Send a message from one address to another")
        fmt.Println("  reply FROM TO MESSAGE_ID MESSAGE    - Reply to a specific message")
        fmt.Println("  readmessages ADDRESS                - Read all messages sent to an address")
        fmt.Println("  decrypt ADDRESS MESSAGE_ID PRIVATE_KEY - Decrypt an encrypted message")
        fmt.Println("  whitelist FROM TO                   - Whitelist an address to receive messages without fees")
        fmt.Println("  messagerewards ADDRESS              - Show message rewards for an address")
        
        fmt.Println("\n=== Validator Commands ===")
        fmt.Println("  startvalidating ADDRESS             - Start validating with an address (requires 50 DOU deposit)")
        fmt.Println("  stopvalidating ADDRESS              - Stop validating with an address")
        fmt.Println("  validators                          - List all active validators")
        fmt.Println("  stakerewards ADDRESS                - Show staking rewards for a validator")
        fmt.Println("  increasestake ADDRESS AMOUNT        - Increase stake for a validator")
        
        fmt.Println("\n=== Group and Channel Commands ===")
        fmt.Println("  creategroup ADDRESS NAME [public|private] - Create a new group")
        fmt.Println("  joingroup ADDRESS GROUP_ID          - Join an existing group")
        fmt.Println("  createchannel ADDRESS NAME [public|private] - Create a new channel")
        
        fmt.Println("\n=== Network Commands ===")
        fmt.Println("  peers                               - Show connected peers")
        fmt.Println("  addpeer ADDRESS                     - Add a new peer by address")
        fmt.Println("  sync                                - Manually trigger synchronization with all peers")
}

// CreateNewWallet creates a new wallet and returns its address
func (cli *CLI) CreateNewWallet() (string, error) {
        w := wallet.NewWallet()
        address := w.GetAddress()
        
        // Store wallet
        err := cli.blockchain.AddWallet(w)
        if err != nil {
                return "", err
        }
        
        // Prepare output with clean formatting
        output := fmt.Sprintf("Your private key: %s\n", w.GetPrivateKeyString())
        output += "IMPORTANT: Save this private key securely. It cannot be recovered if lost.\n"
        output += fmt.Sprintf("Your wallet address: %s", address)
        
        // Print in one go to avoid output being mixed with other messages
        fmt.Println(output)
        
        // Check for node count directly from storage
        nodeCount, err := cli.storage.GetNodeCount()
        if err != nil {
                fmt.Printf("Warning: Failed to check node count: %v\n", err)
                return address, nil
        }
        
        if nodeCount < 10 {
                // Check for existing balance
                existingBalance, err := cli.storage.GetBalance(address)
                if err != nil {
                        existingBalance = 0
                }
                
                if existingBalance == 0 {
                        // We're one of the first 10 wallets, give it 15,000 DOU
                        fmt.Println("---------------------------------------------")
                        fmt.Println("🎉 Congratulations! This wallet is one of the first 10 on the DoucyA network.")
                        fmt.Println("📊 Current node count: ", nodeCount)
                        fmt.Println("💰 You receive 15,000 DOU genesis allocation.")
                        fmt.Println("---------------------------------------------")
                        
                        // Get last block
                        lastBlock, err := cli.storage.GetLastBlock()
                        if err != nil {
                                fmt.Printf("Warning: Failed to get last block: %v\n", err)
                                return address, nil
                        }
                        
                        // Create genesis transaction for this wallet
                        tx := models.NewTransaction("", address, 15000.0, models.TransactionTypeGenesis)
                        
                        // Create a new block with this transaction
                        block := models.NewBlock([]*models.Transaction{tx}, lastBlock)
                        
                        // Save the block to storage
                        if err := cli.storage.SaveBlock(block); err != nil {
                                fmt.Printf("Warning: Failed to save block with genesis allocation: %v\n", err)
                                return address, nil
                        }
                        
                        // Update wallet balance
                        if err := cli.storage.UpdateBalance(address, 15000.0); err != nil {
                                fmt.Printf("Warning: Failed to update balance: %v\n", err)
                                return address, nil
                        }
                        
                        // Increment node count since we added a new funded wallet
                        if err := cli.storage.IncrementNodeCount(); err != nil {
                                fmt.Printf("Warning: Failed to increment node count: %v\n", err)
                        }
                }
        }
        
        return address, nil
}

// ImportWallet imports a wallet from a private key
func (cli *CLI) ImportWallet(privateKey string) (string, error) {
        // Load wallet from private key
        w, err := wallet.LoadWalletFromPrivateKey(privateKey)
        if err != nil {
                return "", fmt.Errorf("failed to load wallet: %v", err)
        }
        
        // Check if wallet with this address already exists
        addresses, err := cli.blockchain.GetAddresses()
        if err != nil {
                return "", fmt.Errorf("failed to check addresses: %v", err)
        }
        
        address := w.GetAddress()
        for _, addr := range addresses {
                if addr == address {
                        return address, fmt.Errorf("wallet with address %s already exists", address)
                }
        }
        
        // Store wallet
        err = cli.blockchain.AddWallet(w)
        if err != nil {
                return "", fmt.Errorf("failed to store wallet: %v", err)
        }
        
        // Check for node count directly from storage
        nodeCount, err := cli.storage.GetNodeCount()
        if err != nil {
                fmt.Printf("Warning: Failed to check node count: %v\n", err)
                return address, nil
        }
        
        if nodeCount < 10 {
                // Check for existing balance
                existingBalance, err := cli.storage.GetBalance(address)
                if err != nil {
                        existingBalance = 0
                }
                
                if existingBalance == 0 {
                        // We're one of the first 10 wallets, give it 15,000 DOU
                        fmt.Println("---------------------------------------------")
                        fmt.Println("🎉 Congratulations! This wallet is one of the first 10 on the DoucyA network.")
                        fmt.Println("📊 Current node count: ", nodeCount)
                        fmt.Println("💰 You receive 15,000 DOU genesis allocation.")
                        fmt.Println("---------------------------------------------")
                        
                        // Get last block
                        lastBlock, err := cli.storage.GetLastBlock()
                        if err != nil {
                                fmt.Printf("Warning: Failed to get last block: %v\n", err)
                                return address, nil
                        }
                        
                        // Create genesis transaction for this wallet
                        tx := models.NewTransaction("", address, 15000.0, models.TransactionTypeGenesis)
                        
                        // Create a new block with this transaction
                        block := models.NewBlock([]*models.Transaction{tx}, lastBlock)
                        
                        // Save the block to storage
                        if err := cli.storage.SaveBlock(block); err != nil {
                                fmt.Printf("Warning: Failed to save block with genesis allocation: %v\n", err)
                                return address, nil
                        }
                        
                        // Update wallet balance
                        if err := cli.storage.UpdateBalance(address, 15000.0); err != nil {
                                fmt.Printf("Warning: Failed to update balance: %v\n", err)
                                return address, nil
                        }
                        
                        // Increment node count since we added a new funded wallet
                        if err := cli.storage.IncrementNodeCount(); err != nil {
                                fmt.Printf("Warning: Failed to increment node count: %v\n", err)
                        }
                }
        }
        
        return address, nil
}

func (cli *CLI) listAddresses() {
        addresses, err := cli.blockchain.GetAddresses()
        if err != nil {
                fmt.Printf("Error retrieving addresses: %v\n", err)
                return
        }

        if len(addresses) == 0 {
                fmt.Println("No addresses found. Use 'createaddress' to create one.")
                return
        }

        fmt.Println("Addresses:")
        for _, addr := range addresses {
                fmt.Printf("  %s\n", addr)
        }
}

func (cli *CLI) getBalance(address string) {
        if !wallet.ValidateAddress(address) {
                fmt.Println("Invalid address format")
                return
        }

        balance, err := cli.blockchain.GetBalance(address)
        if err != nil {
                fmt.Printf("Error getting balance: %v\n", err)
                return
        }

        fmt.Printf("Balance of '%s': %.10f DOU\n", address, balance)
}

func (cli *CLI) sendCoins(from, to string, amount float64) {
        if !wallet.ValidateAddress(from) || !wallet.ValidateAddress(to) {
                fmt.Println("Invalid address format")
                return
        }

        err := cli.blockchain.SendCoins(from, to, amount)
        if err != nil {
                fmt.Printf("Failed to send coins: %v\n", err)
                return
        }

        fmt.Printf("Sent %.10f DOU from %s to %s\n", amount, from, to)
}

func (cli *CLI) sendMessage(from, to, content string) {
        if !wallet.ValidateAddress(from) || !wallet.ValidateAddress(to) {
                fmt.Println("Invalid address format")
                return
        }

        message := messaging.NewMessage(from, to, content)
        err := cli.blockchain.SendMessage(message)
        if err != nil {
                fmt.Printf("Failed to send message: %v\n", err)
                return
        }

        fmt.Printf("Message sent from %s to %s\n", from, to)
}

func (cli *CLI) readMessages(address string) {
        if !wallet.ValidateAddress(address) {
                fmt.Println("Invalid address format")
                return
        }

        messages, err := cli.blockchain.GetMessagesForAddress(address)
        if err != nil {
                fmt.Printf("Error retrieving messages: %v\n", err)
                return
        }

        if len(messages) == 0 {
                fmt.Println("No messages found for this address")
                return
        }

        fmt.Printf("Messages for %s:\n", address)
        for i, msg := range messages {
                fmt.Printf("%d. From: %s\n   Content: %s\n   Timestamp: %s\n", 
                        i+1, msg.Sender, msg.Content, msg.Timestamp.Format("2006-01-02 15:04:05"))
        }
}

func (cli *CLI) whitelistAddress(from, to string) {
        if !wallet.ValidateAddress(from) || !wallet.ValidateAddress(to) {
                fmt.Println("Invalid address format")
                return
        }

        err := cli.blockchain.WhitelistAddress(from, to)
        if err != nil {
                fmt.Printf("Failed to whitelist address: %v\n", err)
                return
        }

        fmt.Printf("Address %s whitelisted by %s\n", to, from)
}

func (cli *CLI) startValidating(address string) {
        if !wallet.ValidateAddress(address) {
                fmt.Println("Invalid address format")
                return
        }

        balance, err := cli.blockchain.GetBalance(address)
        if err != nil {
                fmt.Printf("Error checking balance: %v\n", err)
                return
        }

        minDeposit := cli.blockchain.GetValidatorMinDeposit()
        if balance < minDeposit {
                fmt.Printf("Insufficient balance. Required: %.10f DOU, Available: %.10f DOU\n", minDeposit, balance)
                return
        }

        err = cli.blockchain.RegisterValidator(address, minDeposit)
        if err != nil {
                fmt.Printf("Failed to register as validator: %v\n", err)
                return
        }

        fmt.Printf("Address %s is now a validator with deposit of %.10f DOU\n", address, minDeposit)
}

func (cli *CLI) stopValidating(address string) {
        if !wallet.ValidateAddress(address) {
                fmt.Println("Invalid address format")
                return
        }

        err := cli.blockchain.UnregisterValidator(address)
        if err != nil {
                fmt.Printf("Failed to unregister validator: %v\n", err)
                return
        }

        fmt.Printf("Address %s has stopped validating\n", address)
}

func (cli *CLI) createGroup(address, name string, isPublic bool) {
        if !wallet.ValidateAddress(address) {
                fmt.Println("Invalid address format")
                return
        }

        groupID, err := cli.blockchain.CreateGroup(address, name, isPublic)
        if err != nil {
                fmt.Printf("Failed to create group: %v\n", err)
                return
        }

        privacy := "public"
        if !isPublic {
                privacy = "private"
        }

        fmt.Printf("Created new %s group '%s' with ID: %s\n", privacy, name, groupID)
}

func (cli *CLI) joinGroup(address, groupID string) {
        if !wallet.ValidateAddress(address) {
                fmt.Println("Invalid address format")
                return
        }

        err := cli.blockchain.JoinGroup(address, groupID)
        if err != nil {
                fmt.Printf("Failed to join group: %v\n", err)
                return
        }

        fmt.Printf("Address %s joined group %s\n", address, groupID)
}

func (cli *CLI) createChannel(address, name string, isPublic bool) {
        if !wallet.ValidateAddress(address) {
                fmt.Println("Invalid address format")
                return
        }

        channelID, err := cli.blockchain.CreateChannel(address, name, isPublic)
        if err != nil {
                fmt.Printf("Failed to create channel: %v\n", err)
                return
        }

        privacy := "public"
        if !isPublic {
                privacy = "private"
        }

        fmt.Printf("Created new %s channel '%s' with ID: %s\n", privacy, name, channelID)
}

func (cli *CLI) showStatus() {
        height, err := cli.blockchain.GetHeight()
        if err != nil {
                fmt.Printf("Error getting blockchain height: %v\n", err)
                return
        }

        validators, err := cli.blockchain.GetValidators()
        if err != nil {
                fmt.Printf("Error getting validators: %v\n", err)
                return
        }

        fmt.Println("Blockchain Status:")
        fmt.Printf("  Current Height: %d\n", height)
        fmt.Printf("  Active Validators: %d\n", len(validators))
        fmt.Printf("  Minimum Validator Deposit: %.10f DOU\n", cli.blockchain.GetValidatorMinDeposit())
}

func (cli *CLI) showPeers() {
        peers := cli.p2pServer.GetPeers()
        
        if len(peers) == 0 {
                fmt.Println("No connected peers")
                return
        }

        fmt.Printf("Connected Peers (%d):\n", len(peers))
        for i, peer := range peers {
                fmt.Printf("  %d. %s\n", i+1, peer.GetAddr())
        }
}

func (cli *CLI) addPeer(addr string) {
        fmt.Printf("Connecting to peer %s...\n", addr)
        
        // Use a channel with buffer to avoid goroutine leak
        connectChannel := make(chan error, 1)
        
        // Create done channel to signal completion
        done := make(chan struct{})
        defer close(done)
        
        // Start connection in a goroutine
        go func() {
                select {
                case <-done:
                        // The function has returned, abort connection attempt
                        return
                default:
                        // Attempt to connect
                        err := cli.p2pServer.AddPeer(addr)
                        
                        // Try to send result, but don't block if function has already returned
                        select {
                        case connectChannel <- err:
                                // Sent successfully
                        case <-done:
                                // Function already returned
                        }
                }
        }()
        
        // Wait for result with a shorter timeout (3 seconds)
        var err error
        select {
        case err = <-connectChannel:
                // Connection attempt completed (success or failure)
        case <-time.After(3 * time.Second):
                err = fmt.Errorf("connection timed out after 3 seconds")
        }
        
        // Handle connection result
        if err != nil {
                fmt.Printf("Failed to add peer: %v\n", err)
                fmt.Println("Common issues:")
                fmt.Println("  - Peer may be offline or unreachable")
                fmt.Println("  - Network port may be blocked or closed")
                fmt.Println("  - Peer may be running a different protocol version")
                fmt.Println("Try connecting to a different peer or check your network settings.")
                return
        }
        
        fmt.Printf("Successfully connected to peer: %s\n", addr)
        
        // Show current peer count
        peers := cli.p2pServer.GetPeers()
        fmt.Printf("Current peers: %d\n", len(peers))
        
        fmt.Println("Synchronizing with network...")
        fmt.Println("Type 'peers' to see all connected peers.")
}

// replyToMessage sends a reply to a specific message
func (cli *CLI) replyToMessage(from, to, messageID, content string) {
        if !wallet.ValidateAddress(from) || !wallet.ValidateAddress(to) {
                fmt.Println("Invalid address format")
                return
        }

        // Create a new message as a reply
        message := messaging.NewReplyMessage(from, to, content, messageID)
        err := cli.blockchain.SendMessage(message)
        if err != nil {
                fmt.Printf("Failed to send reply: %v\n", err)
                return
        }

        fmt.Printf("Reply sent from %s to %s (in response to message %s)\n", from, to, messageID)
}

// listValidators lists all active validators
func (cli *CLI) listValidators() {
        validators, err := cli.blockchain.GetValidators()
        if err != nil {
                fmt.Printf("Error retrieving validators: %v\n", err)
                return
        }

        if len(validators) == 0 {
                fmt.Println("No active validators found.")
                return
        }

        fmt.Println("Active Validators:")
        for i, v := range validators {
                stake := v.GetStake()
                address := v.GetAddress()
                creationTime := v.GetCreationTime().Format("2006-01-02 15:04:05")
                
                fmt.Printf("%d. Address: %s\n", i+1, address)
                fmt.Printf("   Stake: %.10f DOU\n", stake)
                fmt.Printf("   Validating since: %s\n", creationTime)
                fmt.Printf("   Status: %s\n", v.GetValidatorStatus())
                
                // Calculate estimated monthly reward
                monthlyReward := v.GetMonthlyReward()
                fmt.Printf("   Est. Monthly Reward: %.10f DOU\n", monthlyReward)
                fmt.Println()
        }
}

// showMessageRewards displays message rewards for an address
func (cli *CLI) showMessageRewards(address string) {
        if !wallet.ValidateAddress(address) {
                fmt.Println("Invalid address format")
                return
        }

        // In a full implementation, we would get the reward history from the blockchain
        // For now, we'll show a basic view of pending/processed rewards
        rewards, err := cli.blockchain.GetMessageRewards(address)
        if err != nil {
                fmt.Printf("Error retrieving rewards: %v\n", err)
                return
        }

        if len(rewards) == 0 {
                fmt.Printf("No message rewards found for address %s\n", address)
                return
        }

        fmt.Printf("Message Rewards for %s:\n", address)
        
        var totalReceived float64
        var totalSent float64
        
        for i, reward := range rewards {
                rewardType := "Sent Message"
                if reward.Type == string(messaging.ReceiverRewardType) {
                        rewardType = "Response"
                        totalReceived += reward.Amount
                } else {
                        totalSent += reward.Amount
                }
                
                fmt.Printf("%d. Type: %s\n", i+1, rewardType)
                fmt.Printf("   Amount: %.10f DOU\n", reward.Amount)
                fmt.Printf("   Message ID: %s\n", reward.MessageID)
                fmt.Printf("   Timestamp: %s\n", reward.Timestamp.Format("2006-01-02 15:04:05"))
                fmt.Println()
        }
        
        fmt.Printf("Total Rewards from Sent Messages: %.10f DOU\n", totalSent)
        fmt.Printf("Total Rewards from Responses: %.10f DOU\n", totalReceived)
        fmt.Printf("Total Message Rewards: %.10f DOU\n", totalSent+totalReceived)
}

// showStakeRewards displays staking rewards for a validator
func (cli *CLI) showStakeRewards(address string) {
        if !wallet.ValidateAddress(address) {
                fmt.Println("Invalid address format")
                return
        }

        // Get validator info
        validator, err := cli.blockchain.GetValidator(address)
        if err != nil {
                fmt.Printf("Error retrieving validator: %v\n", err)
                return
        }

        if validator == nil {
                fmt.Printf("Address %s is not a validator\n", address)
                return
        }

        // In a full implementation, we would get the reward history from the blockchain
        fmt.Printf("Staking Rewards for %s:\n", address)
        fmt.Printf("Current Stake: %.10f DOU\n", validator.GetStake())
        fmt.Printf("Annual APY: %.2f%%\n", validator.YearlyAPY * 100)
        
        // Calculate earnings
        monthlyReward := validator.GetMonthlyReward()
        dailyReward := validator.GetDailyReward()
        annualReward := monthlyReward * 12
        
        fmt.Printf("Daily Earnings: %.10f DOU\n", dailyReward)
        fmt.Printf("Monthly Earnings: %.10f DOU\n", monthlyReward)
        fmt.Printf("Annual Earnings: %.10f DOU\n", annualReward)
        
        // Show reward history if available
        if len(validator.RewardHistories) > 0 {
                fmt.Println("\nReward History:")
                for i, reward := range validator.RewardHistories {
                        fmt.Printf("%d. Amount: %.10f DOU\n", i+1, reward.Amount)
                        fmt.Printf("   Type: %s\n", reward.Type)
                        fmt.Printf("   Date: %s\n", reward.Timestamp.Format("2006-01-02 15:04:05"))
                        fmt.Println()
                }
        }
}

// syncWithPeers manually triggers synchronization with peers
func (cli *CLI) syncWithPeers() {
        // First check if we have any connected peers
        peers := cli.p2pServer.GetPeers()
        
        if len(peers) == 0 {
                // No peers, try to connect to bootstrap nodes first
                fmt.Println("No peers connected. Attempting to connect to main bootstrap node...")
                
                // Get bootstrap nodes from config
                bootstrapNodes := cli.config.BootstrapNodes
                
                if len(bootstrapNodes) == 0 {
                        fmt.Println("No bootstrap nodes configured. Use 'addpeer ADDRESS' to connect to peers manually.")
                        return
                }
                
                // Connect to bootstrap nodes
                connectedAny := false
                for _, addr := range bootstrapNodes {
                        fmt.Printf("Connecting to bootstrap node %s...\n", addr)
                        err := cli.p2pServer.AddPeer(addr)
                        if err != nil {
                                fmt.Printf("Failed to connect to bootstrap node %s: %v\n", err)
                        } else {
                                fmt.Printf("Successfully connected to bootstrap node %s\n", addr)
                                connectedAny = true
                        }
                }
                
                if !connectedAny {
                        fmt.Println("Failed to connect to any bootstrap nodes. Use 'addpeer ADDRESS' to connect to peers manually.")
                        return
                }
                
                // Refresh peers list
                time.Sleep(time.Second) // brief delay to allow for connection to establish
                peers = cli.p2pServer.GetPeers()
                
                if len(peers) == 0 {
                        fmt.Println("Still no peers connected after bootstrap attempt. Try again later or use 'addpeer ADDRESS'.")
                        return
                }
        }
        
        fmt.Printf("Synchronizing with %d peers...\n", len(peers))
        
        // Request validator information from all peers
        for _, peer := range peers {
                fmt.Printf("Requesting validators from %s...\n", peer.GetAddr())
                
                // Create a get validators message
                message := &p2p.Message{
                        Type: p2p.MessageTypeGetValidators,
                        Data: json.RawMessage("{}"),
                }
                
                // Marshal message
                messageBytes, err := json.Marshal(message)
                if err != nil {
                        fmt.Printf("Failed to marshal validator request: %v\n", err)
                        continue
                }
                
                // Send message to peer
                err = peer.SendMessage(messageBytes)
                if err != nil {
                        fmt.Printf("Failed to send validator request to peer %s: %v\n", peer.GetAddr(), err)
                        continue
                }
                
                fmt.Printf("Successfully requested validators from peer: %s\n", peer.GetAddr())
                
                // Also request blockchain data
                height, err := cli.blockchain.GetHeight()
                if err != nil {
                        fmt.Printf("Failed to get blockchain height: %v\n", err)
                        continue
                }
                
                fmt.Printf("Requesting blocks from %s starting at height %d...\n", peer.GetAddr(), height)
                
                // Create get blocks message
                getBlocksMessage := &p2p.Message{
                        Type: p2p.MessageTypeGetBlocks,
                        Data: json.RawMessage(fmt.Sprintf(`{"from_height":%d}`, height)),
                }
                
                // Marshal message
                getBlocksBytes, err := json.Marshal(getBlocksMessage)
                if err != nil {
                        fmt.Printf("Failed to marshal get blocks message: %v\n", err)
                        continue
                }
                
                // Send message to peer
                err = peer.SendMessage(getBlocksBytes)
                if err != nil {
                        fmt.Printf("Failed to send get blocks message to peer %s: %v\n", peer.GetAddr(), err)
                        continue
                }
                
                // Also request peer list to discover more peers
                peerListMessage := &p2p.Message{
                        Type: p2p.MessageTypeGetPeers,
                        Data: json.RawMessage("{}"),
                }
                
                // Marshal message
                peerListBytes, err := json.Marshal(peerListMessage)
                if err != nil {
                        fmt.Printf("Failed to marshal get peers message: %v\n", err)
                        continue
                }
                
                // Send message to peer
                err = peer.SendMessage(peerListBytes)
                if err != nil {
                        fmt.Printf("Failed to send get peers message to peer %s: %v\n", peer.GetAddr(), err)
                        continue
                }
        }
        
        fmt.Println("Synchronization requests sent to all peers. Please wait for responses...")
        fmt.Println("Use 'status' to check the current blockchain state after synchronization.")
}

// increaseStake increases the stake for a validator
func (cli *CLI) increaseStake(address string, amount float64) {
        if !wallet.ValidateAddress(address) {
                fmt.Println("Invalid address format")
                return
        }

        // Check if address is a validator
        validator, err := cli.blockchain.GetValidator(address)
        if err != nil {
                fmt.Printf("Error retrieving validator: %v\n", err)
                return
        }

        if validator == nil {
                fmt.Printf("Address %s is not a validator. Use 'startvalidating' first.\n", address)
                return
        }

        // Check balance
        balance, err := cli.blockchain.GetBalance(address)
        if err != nil {
                fmt.Printf("Error checking balance: %v\n", err)
                return
        }

        if balance < amount {
                fmt.Printf("Insufficient balance. Required: %.10f DOU, Available: %.10f DOU\n", amount, balance)
                return
        }

        // Increase stake
        err = cli.blockchain.IncreaseValidatorStake(address, amount)
        if err != nil {
                fmt.Printf("Failed to increase stake: %v\n", err)
                return
        }

        newStake := validator.GetStake() + amount
        fmt.Printf("Stake increased by %.10f DOU. New stake: %.10f DOU\n", amount, newStake)
}

// decryptMessage decrypts a message with a private key
func (cli *CLI) decryptMessage(address, messageID, privateKeyHex string) {
        if !wallet.ValidateAddress(address) {
                fmt.Println("Invalid address format")
                return
        }

        // Get the message
        message, err := cli.blockchain.GetMessage(messageID)
        if err != nil {
                fmt.Printf("Error retrieving message: %v\n", err)
                return
        }

        if message == nil {
                fmt.Printf("Message with ID %s not found\n", messageID)
                return
        }

        // Check if message is for this address
        if message.GetReceiver() != address {
                fmt.Printf("Message %s is not addressed to %s\n", messageID, address)
                return
        }

        // Decrypt the message
        decrypted, err := message.Decrypt(privateKeyHex)
        if err != nil {
                fmt.Printf("Failed to decrypt message: %v\n", err)
                return
        }

        // Display decrypted message
        fmt.Printf("Decrypted Message (ID: %s):\n", messageID)
        fmt.Printf("From: %s\n", message.GetSender())
        fmt.Printf("Content: %s\n", decrypted)
        fmt.Printf("Timestamp: %s\n", message.GetTimestamp().Format("2006-01-02 15:04:05"))
}
