package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/doucya/config"
	"github.com/doucya/core"
	"github.com/doucya/messaging"
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
	fmt.Println("Welcome to DoucyA Blockchain CLI!")
	fmt.Println("Type 'help' for a list of commands")

	for cli.running {
		fmt.Print("> ")
		if !cli.scanner.Scan() {
			break
		}

		cmd := cli.scanner.Text()
		cli.processCommand(cmd)
	}
}

// InitializeNode initializes a new node
func (cli *CLI) InitializeNode() error {
	// Create a wallet for this node if it doesn't exist
	address, err := cli.createNewAddress()
	if err != nil {
		return fmt.Errorf("failed to create node wallet: %v", err)
	}

	fmt.Printf("Node initialized with address: %s\n", address)
	return nil
}

// processCommand processes a CLI command
func (cli *CLI) processCommand(cmd string) {
	args := strings.Fields(cmd)
	if len(args) == 0 {
		return
	}

	switch args[0] {
	case "help":
		cli.printHelp()
	case "exit", "quit":
		cli.running = false
	case "createaddress":
		address, err := cli.createNewAddress()
		if err != nil {
			fmt.Printf("Error creating address: %v\n", err)
			return
		}
		fmt.Printf("New address created: %s\n", address)
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
	default:
		fmt.Printf("Unknown command: %s\n", args[0])
		fmt.Println("Type 'help' for a list of commands")
	}
}

func (cli *CLI) printHelp() {
	fmt.Println("DoucyA Blockchain CLI Commands:")
	fmt.Println("  help                                - Show this help message")
	fmt.Println("  exit, quit                          - Exit the CLI")
	fmt.Println("  createaddress                       - Create a new wallet address")
	fmt.Println("  listaddresses                       - List all wallet addresses")
	fmt.Println("  getbalance ADDRESS                  - Get balance for an address")
	fmt.Println("  send FROM TO AMOUNT                 - Send coins from one address to another")
	fmt.Println("  message FROM TO MESSAGE             - Send a message from one address to another")
	fmt.Println("  readmessages ADDRESS                - Read all messages sent to an address")
	fmt.Println("  whitelist FROM TO                   - Whitelist an address to receive messages without fees")
	fmt.Println("  startvalidating ADDRESS             - Start validating with an address (requires 50 DOU deposit)")
	fmt.Println("  stopvalidating ADDRESS              - Stop validating with an address")
	fmt.Println("  creategroup ADDRESS NAME [public|private] - Create a new group")
	fmt.Println("  joingroup ADDRESS GROUP_ID          - Join an existing group")
	fmt.Println("  createchannel ADDRESS NAME [public|private] - Create a new channel")
	fmt.Println("  status                              - Show blockchain status")
	fmt.Println("  peers                               - Show connected peers")
	fmt.Println("  addpeer ADDRESS                     - Add a new peer by address")
}

func (cli *CLI) createNewAddress() (string, error) {
	w := wallet.NewWallet()
	address := w.GetAddress()
	
	// Store wallet
	err := cli.blockchain.AddWallet(w)
	if err != nil {
		return "", err
	}
	
	fmt.Printf("Your private key: %s\n", w.GetPrivateKeyString())
	fmt.Println("IMPORTANT: Save this private key securely. It cannot be recovered if lost.")
	
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
	err := cli.p2pServer.AddPeer(addr)
	if err != nil {
		fmt.Printf("Failed to add peer: %v\n", err)
		return
	}

	fmt.Printf("Added peer: %s\n", addr)
}
