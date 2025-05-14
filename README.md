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

## Quick Installation Guide

### Step 1: Update system and install dependencies
```bash
sudo apt update
sudo apt upgrade -y
sudo apt install -y git build-essential golang-go libleveldb-dev
```

### Step 2: Clone the repository
```bash
mkdir -p ~/projects
cd ~/projects
git clone https://github.com/MyThirdRome/DoucyMessenger.git
cd DoucyMessenger
```

### Step 3: Build and initialize the blockchain
```bash
go mod tidy
go run main.go -init -port=8333
```

This will:
- Create a genesis block if needed
- Generate a new wallet address for this node
- Save the private key to node_private_key.txt
- Allocate 15,000 DOU if you're among the first 10 nodes

### Step 4: Run the blockchain
```bash
# Run the node
go run main.go
```

### Step 5: Connect to Other Nodes (for Synchronization)

Once in the CLI prompt, add peer nodes:

```
> addpeer OTHER_SERVER_IP:8333
```

## Setting Up for Auto-start (Optional)

If you want your blockchain node to run automatically at system startup:

```bash
# Create a systemd service file
sudo tee /etc/systemd/system/doucya.service > /dev/null << EOT
[Unit]
Description=DoucyA Blockchain
After=network.target

[Service]
User=$USER
WorkingDirectory=$HOME/projects/DoucyMessenger
ExecStart=/usr/bin/go run $HOME/projects/DoucyMessenger/main.go
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOT

# Enable and start the service
sudo systemctl daemon-reload
sudo systemctl enable doucya.service
sudo systemctl start doucya.service

# Check the status
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

## Node Synchronization

DoucyA Blockchain now uses a configurable node list for automatic synchronization:

### Using nodes.txt (Recommended)

The blockchain node will automatically connect to trusted nodes listed in the `nodes.txt` file:

1. Edit the `nodes.txt` file in the root directory
2. Add one node address per line (in format `host:port`)
3. Lines starting with `#` are treated as comments
4. The blockchain will automatically connect to all listed nodes at startup
5. Changes to `nodes.txt` take effect after restarting the node

Example `nodes.txt`:
```
# Main bootstrap node
185.251.25.31:8333

# Secondary nodes
example.com:8333
192.168.1.100:8333
```

### Manual Peer Connection

You can also manually connect to peers using the CLI:

1. Each node needs to connect to at least one other node in the network
2. Use the `addpeer` command to connect nodes
3. Type `sync` to manually trigger synchronization with all connected nodes
4. You can check synchronization status with the `status` command

Note: The first 10 nodes on the network receive 15,000 DOU each as genesis allocation

## License

[MIT License](LICENSE)
