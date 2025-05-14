# DoucyA Blockchain

A peer-to-peer messaging blockchain with DOU cryptocurrency built in Golang using PoS consensus and LevelDB storage.

## Features

- Custom cryptocurrency (DOU) with built-in messaging rewards
- Proof of Stake (PoS) consensus mechanism requiring 50 DOU minimum deposit
- Innovative messaging reward system (0.75 DOU for sender, 0.25 DOU for receiver upon response)
- Group/channel messaging support
- Encrypted private communications
- Spam protection through mutual address whitelisting
- Rate limiting (200 messages/hour, 30 messages to a single address)
- Privacy-focused interface

## Complete Installation Guide for DoucyA Blockchain

### Step 1: Update System and Install Prerequisites

```bash
# Update package list
sudo apt update

# Install essential tools
sudo apt install -y build-essential wget git curl libleveldb-dev
```

### Step 2: Install Go

```bash
# Download the latest Go version
wget https://golang.org/dl/go1.21.5.linux-amd64.tar.gz

# Extract Go to /usr/local
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
rm go1.21.5.linux-amd64.tar.gz

# Add Go to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.profile
source ~/.profile

# Verify Go installation
go version
```

### Step 3: Clone the DoucyA Repository

```bash
# Clone the repository
git clone https://github.com/MyThirdRome/DoucyMessenger.git
cd DoucyMessenger
```

### Step 4: Build the Application

```bash
# Get dependencies and build
go mod tidy
go build -o doucya_blockchain main.go
```

### Step 5: Initialize the Blockchain Node

```bash
# Initialize a new node
./doucya_blockchain -init
```

This will:
- Create a genesis block if needed
- Generate a new wallet address for this node
- Save the private key to node_private_key.txt
- Allocate 15,000 DOU if you're among the first 10 nodes

### Step 6: Run the Blockchain Node

```bash
# Run the node with default settings
./doucya_blockchain

# Or run with specific parameters
# ./doucya_blockchain -port 8333 -db ./doucya_data
```

### Step 7: Connect to Other Nodes (for Synchronization)

Once in the CLI prompt, add peer nodes:

```
> addpeer OTHER_SERVER_IP:8333
```

## Setting Up for Auto-start

### Create a Systemd Service

```bash
# Create a systemd service file
sudo nano /etc/systemd/system/doucya.service
```

Add the following content (adjust paths accordingly):

```
[Unit]
Description=DoucyA Blockchain
After=network.target

[Service]
User=your_username
WorkingDirectory=/home/your_username/DoucyMessenger
ExecStart=/home/your_username/DoucyMessenger/doucya_blockchain
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Then enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable doucya.service
sudo systemctl start doucya.service

# Check status
sudo systemctl status doucya.service
```

## Common CLI Commands

Once the node is running, you can use these commands:
- `createwallet` - Create a new wallet
- `importwallet PRIVATE_KEY` - Import an existing wallet
- `listaddresses` - List all wallet addresses
- `getbalance ADDRESS` - Check balance of an address
- `send FROM TO AMOUNT` - Send DOU from one address to another
- `message FROM TO MESSAGE` - Send a message
- `readmessages ADDRESS` - Read messages for an address
- `peers` - Show connected peers
- `status` - Show blockchain status
- `help` - Show all available commands

## Notes on Node Synchronization

To keep nodes synchronized across servers:

1. Each node needs to connect to at least one other node in the network
2. Use the `addpeer` command to connect nodes
3. Transactions and blocks will automatically propagate through the network
4. You can check synchronization status with the `status` command
5. The first 10 nodes on the network receive 15,000 DOU each as genesis allocation

## License

[MIT License](LICENSE)