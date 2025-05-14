package p2p

import (
        "encoding/json"
        "fmt"
        "math/rand"
        "net"
        "sync"
        "time"

        "github.com/doucya/core"
        "github.com/doucya/models"
        "github.com/doucya/utils"
)

// Initialize random number generator
func init() {
        // Seed the random number generator for better security
        rand.Seed(time.Now().UnixNano())
}

// Server represents the P2P server
type Server struct {
        listenAddr     string
        blockchain     *core.Blockchain
        bootstrapNodes []string
        peers          map[string]*Peer
        peersMutex     sync.RWMutex
        listener       net.Listener
        quit           chan struct{}
}

// NewServer creates a new P2P server
func NewServer(listenAddr string, blockchain *core.Blockchain, bootstrapNodes []string) *Server {
        return &Server{
                listenAddr:     listenAddr,
                blockchain:     blockchain,
                bootstrapNodes: bootstrapNodes,
                peers:          make(map[string]*Peer),
                quit:           make(chan struct{}),
        }
}

// Start starts the P2P server
func (s *Server) Start() error {
        // Start listening for connections
        listener, err := net.Listen("tcp", s.listenAddr)
        if err != nil {
                return fmt.Errorf("failed to start P2P server: %v", err)
        }
        s.listener = listener

        // Accept connections in a goroutine
        go s.acceptConnections()

        // Connect to bootstrap nodes
        for _, addr := range s.bootstrapNodes {
                err := s.AddPeer(addr)
                if err != nil {
                        utils.Warning("Failed to connect to bootstrap node %s: %v", addr, err)
                }
        }

        // Start periodic sync
        go s.periodicSync()

        return nil
}

// Stop stops the P2P server
func (s *Server) Stop() {
        // Signal quit
        close(s.quit)

        // Close listener
        if s.listener != nil {
                s.listener.Close()
        }

        // Close all peer connections
        s.peersMutex.Lock()
        for _, peer := range s.peers {
                peer.Close()
        }
        s.peersMutex.Unlock()
}

// AddPeer adds a new peer to the server with improved error handling and timeout
func (s *Server) AddPeer(addr string) error {
        // Check if address is valid
        if addr == "" {
                return fmt.Errorf("peer address cannot be empty")
        }
        
        // Lock for thread safety
        s.peersMutex.Lock()
        
        // Check if peer already exists
        if _, exists := s.peers[addr]; exists {
                s.peersMutex.Unlock()
                return nil // Already connected
        }
        
        // Create peer (without connecting)
        peer, err := NewPeer(addr, s)
        if err != nil {
                s.peersMutex.Unlock()
                return fmt.Errorf("failed to create peer: %v", err)
        }
        
        // Unlock before connect which might take time
        s.peersMutex.Unlock()
        
        // Connect with timeout
        done := make(chan struct{})
        defer close(done)
        
        connectChan := make(chan error, 1)
        go func() {
                select {
                case <-done:
                        // Function returned early, cancel connection
                        return
                default:
                        // Try to connect
                        err := peer.Connect()
                        
                        // Only send result if function hasn't returned
                        select {
                        case connectChan <- err:
                                // Result sent
                        case <-done:
                                // Function already returned
                        }
                }
        }()
        
        // Wait for connection with timeout (2 seconds)
        var connectErr error
        select {
        case connectErr = <-connectChan:
                // Connection completed (success or error)
        case <-time.After(2 * time.Second):
                connectErr = fmt.Errorf("connection timeout after 2 seconds")
        }
        
        if connectErr != nil {
                return fmt.Errorf("failed to connect to peer: %v", connectErr)
        }
        
        // Relock to update peers map
        s.peersMutex.Lock()
        defer s.peersMutex.Unlock()
        
        // Recheck if peer was added while we were connecting
        if _, exists := s.peers[addr]; exists {
                // Another thread already added this peer while we were connecting
                // Close our connection to avoid duplicate
                peer.Close()
                return nil
        }
        
        // Add to peers map
        s.peers[addr] = peer
        
        // Start peer handlers
        go peer.HandleMessages()
        
        // Exchange node information - async to avoid blocking
        go s.exchangeNodeInfo(peer)
        
        return nil
}

// RemovePeer removes a peer from the server
func (s *Server) RemovePeer(addr string) {
        s.peersMutex.Lock()
        defer s.peersMutex.Unlock()

        if peer, exists := s.peers[addr]; exists {
                peer.Close()
                delete(s.peers, addr)
        }
}

// GetPeers returns all connected peers
func (s *Server) GetPeers() []*Peer {
        s.peersMutex.RLock()
        defer s.peersMutex.RUnlock()

        peers := make([]*Peer, 0, len(s.peers))
        for _, peer := range s.peers {
                peers = append(peers, peer)
        }

        return peers
}

// RequestPeerList requests the peer list from a specific peer
func (s *Server) RequestPeerList(addr string) error {
        s.peersMutex.RLock()
        peer, exists := s.peers[addr]
        s.peersMutex.RUnlock()
        
        if !exists {
                return fmt.Errorf("peer %s not found", addr)
        }
        
        // Create a get peers message
        message := &Message{
                Type: MessageTypeGetPeers,
                Data: json.RawMessage("{}"),
        }
        
        // Marshal message
        messageBytes, err := json.Marshal(message)
        if err != nil {
                return fmt.Errorf("failed to marshal peer list request: %v", err)
        }
        
        // Send message to peer
        return peer.SendMessage(messageBytes)
}

// BroadcastMessage broadcasts a message to all peers
func (s *Server) BroadcastMessage(msgType MessageType, data interface{}) error {
        // Create message
        messageBytes, err := s.createMessageBytes(msgType, data)
        if err != nil {
                return fmt.Errorf("failed to create message: %v", err)
        }

        // Send to all peers
        s.peersMutex.RLock()
        defer s.peersMutex.RUnlock()

        for _, peer := range s.peers {
                go func(p *Peer) {
                        err := p.SendMessage(messageBytes)
                        if err != nil {
                                fmt.Printf("Failed to send message to peer %s: %v\n", p.GetAddr(), err)
                        }
                }(peer)
        }

        return nil
}

// HandleMessage handles a received message
func (s *Server) HandleMessage(peer *Peer, message *Message) {
        switch message.Type {
        case MessageTypeNodeInfo:
                s.handleNodeInfo(peer, message)
        case MessageTypePeerList:
                s.handlePeerList(peer, message)
        case MessageTypeBlock:
                s.handleBlock(peer, message)
        case MessageTypeTransaction:
                s.handleTransaction(peer, message)
        case MessageTypeGetBlocks:
                s.handleGetBlocks(peer, message)
        case MessageTypeGetPeers:
                s.handleGetPeers(peer)
        case MessageTypeMessageReward:
                s.handleMessageReward(peer, message)
        case MessageTypeValidatorReward:
                s.handleValidatorReward(peer, message)
        case MessageTypeSystemMessage:
                s.handleSystemMessage(peer, message)
        case MessageTypeValidator:
                s.handleValidator(peer, message)
        case MessageTypeGetValidators:
                s.handleGetValidators(peer)
        case MessageTypeValidatorList:
                s.handleValidatorList(peer, message)
        default:
                utils.Debug("Unknown message type: %s", message.Type)
        }
}

// handleValidator handles a validator message
func (s *Server) handleValidator(peer *Peer, message *Message) {
        var validator core.Validator
        if err := json.Unmarshal(message.Data, &validator); err != nil {
                utils.Error("Error unmarshaling validator: %v", err)
                return
        }

        // Convert to models.Validator
        modelValidator := &models.Validator{
                Address:         validator.Address,
                Deposit:         validator.Deposit,
                StartTime:       validator.StartTime,
                LastRewardTime:  validator.LastRewardTime,
                Status:          models.ValidatorStatusActive,
                MessageCount:    validator.MessageCount,
                TotalEarnings:   validator.TotalRewards,
                RewardHistories: []models.RewardHistory{},
                YearlyAPY:       0.17, // 17% APY
                MinimumStake:    50.0, // 50 DOU minimum
        }

        // Add the validator to the blockchain
        err := s.blockchain.SyncValidator(modelValidator)
        if err != nil {
                utils.Error("Failed to sync validator: %v", err)
                return
        }

        // Broadcast to other peers
        s.BroadcastMessage(MessageTypeValidator, validator)
}

// handleGetValidators handles a request for validators
func (s *Server) handleGetValidators(peer *Peer) {
        validators, err := s.blockchain.GetValidators()
        if err != nil {
                utils.Error("Error getting validators: %v", err)
                return
        }
        
        // Send each validator separately
        for _, validator := range validators {
            // Send each validator as a message
            err := s.BroadcastMessage(MessageTypeValidator, validator)
            if err != nil {
                utils.Error("Error broadcasting validator: %v", err)
            }
        }
}

// handleValidatorList handles a list of validators
func (s *Server) handleValidatorList(peer *Peer, message *Message) {
        var validators []*models.Validator
        if err := json.Unmarshal(message.Data, &validators); err != nil {
                utils.Error("Error unmarshaling validator list: %v", err)
                return
        }

        // Compare with our validators and request any we don't have
        ourValidators, err := s.blockchain.GetValidators()
        if err != nil {
                utils.Error("Error getting validators: %v", err)
                return
        }
        
        ourValidatorMap := make(map[string]bool)
        
        for _, v := range ourValidators {
                ourValidatorMap[v.Address] = true
        }
        
        for _, v := range validators {
                if _, exists := ourValidatorMap[v.Address]; !exists {
                        // Request detailed validator info
                        err := s.BroadcastMessage(MessageTypeGetValidators, nil)
                        if err != nil {
                                utils.Error("Error requesting validators: %v", err)
                        }
                        break // Just request all validators once
                }
        }
}

// Private methods

// acceptConnections accepts incoming connections
func (s *Server) acceptConnections() {
        for {
                conn, err := s.listener.Accept()
                if err != nil {
                        select {
                        case <-s.quit:
                                return // Server shutting down
                        default:
                                fmt.Printf("Error accepting connection: %v\n", err)
                                continue
                        }
                }

                // Create peer for this connection
                peer := &Peer{
                        conn:   conn,
                        addr:   conn.RemoteAddr().String(),
                        server: s,
                        send:   make(chan []byte, 100),
                        quit:   make(chan struct{}),
                }

                // Add to peers
                s.peersMutex.Lock()
                s.peers[peer.addr] = peer
                s.peersMutex.Unlock()

                // Start peer handlers
                go peer.HandleMessages()
        }
}

// createMessageBytes creates a byte slice for a message
func (s *Server) createMessageBytes(msgType MessageType, data interface{}) ([]byte, error) {
        // First marshal the data to JSON
        dataBytes, err := json.Marshal(data)
        if err != nil {
                return nil, fmt.Errorf("failed to marshal message data: %v", err)
        }

        // Create the message with the raw JSON data
        message := &Message{
                Type: msgType,
                Data: json.RawMessage(dataBytes),
        }

        return json.Marshal(message)
}

// exchangeNodeInfo exchanges node information with a peer
func (s *Server) exchangeNodeInfo(peer *Peer) error {
        // Get current blockchain height
        height, err := s.blockchain.GetHeight()
        if err != nil {
                utils.Error("Failed to get blockchain height: %v", err)
                return err
        }

        // Create node info
        nodeInfo := &NodeInfo{
                Version:      "1.0.0",
                BlockHeight:  height,
                PeerCount:    len(s.peers),
                ListenAddr:   s.listenAddr,
        }

        // Send message
        messageBytes, err := s.createMessageBytes(MessageTypeNodeInfo, nodeInfo)
        if err != nil {
                utils.Error("Failed to create node info message: %v", err)
                return err
        }

        err = peer.SendMessage(messageBytes)
        if err != nil {
                utils.Error("Failed to send node info to peer %s: %v", peer.GetAddr(), err)
                return err
        }

        return nil
}

// requestPeerList requests a list of peers from a peer
func (s *Server) requestPeerList(peer *Peer) {
        // Create message
        messageBytes, err := s.createMessageBytes(MessageTypeGetPeers, nil)
        if err != nil {
                fmt.Printf("Failed to create get peers message: %v\n", err)
                return
        }

        // Send message
        err = peer.SendMessage(messageBytes)
        if err != nil {
                fmt.Printf("Failed to send get peers to peer %s: %v\n", peer.GetAddr(), err)
        }
}

// periodicSync performs periodic synchronization with peers
func (s *Server) periodicSync() {
        // Sync every 5 minutes
        syncTicker := time.NewTicker(5 * time.Minute)
        // Check bootstrap connections every 3 minutes
        bootstrapTicker := time.NewTicker(3 * time.Minute)
        defer syncTicker.Stop()
        defer bootstrapTicker.Stop()

        for {
                select {
                case <-syncTicker.C:
                        // Sync with all peers
                        s.syncWithPeers()
                case <-bootstrapTicker.C:
                        // Ensure connection to bootstrap nodes
                        s.ensureBootstrapConnections()
                case <-s.quit:
                        return
                }
        }
}

// ensureBootstrapConnections ensures that we maintain connections to bootstrap nodes
func (s *Server) ensureBootstrapConnections() {
        for _, bootstrapAddr := range s.bootstrapNodes {
                // Check if we're already connected
                s.peersMutex.RLock()
                _, connected := s.peers[bootstrapAddr]
                s.peersMutex.RUnlock()
                
                // If not connected, try to connect
                if !connected {
                        utils.Info("Reconnecting to bootstrap node: %s", bootstrapAddr)
                        go s.AddPeer(bootstrapAddr) // Don't block on connection attempts
                }
        }
}

// syncWithPeers synchronizes with all peers
func (s *Server) syncWithPeers() {
        utils.Debug("Starting periodic synchronization...")
        
        // Get a safe copy of peers to iterate
        s.peersMutex.RLock()
        peers := make([]*Peer, 0, len(s.peers))
        for _, peer := range s.peers {
                peers = append(peers, peer)
        }
        s.peersMutex.RUnlock()
        
        if len(peers) == 0 {
                utils.Debug("No peers connected for synchronization")
                // Try to connect to bootstrap nodes if we have no peers
                s.ensureBootstrapConnections()
                return
        }
        
        utils.Debug("Synchronizing with %d peers", len(peers))
        
        // For each peer:
        // 1. Request latest blockchain info
        // 2. Request validator list
        // 3. Update peer list
        for _, peer := range peers {
                // Exchange node info (includes blockchain height)
                if err := s.exchangeNodeInfo(peer); err != nil {
                        utils.Warning("Failed to exchange node info with peer %s: %v", peer.GetAddr(), err)
                        continue
                }
                
                // Request validators
                s.requestValidators(peer)
                
                // Request peer list
                s.requestPeerList(peer)
        }
        
        utils.Debug("Synchronization completed")
}

// requestValidators requests validator information from a peer
func (s *Server) requestValidators(peer *Peer) {
        // Create a get validators message
        messageBytes, err := s.createMessageBytes(MessageTypeGetValidators, nil)
        if err != nil {
                utils.Error("Failed to create get validators message: %v", err)
                return
        }
        
        // Send message
        err = peer.SendMessage(messageBytes)
        if err != nil {
                utils.Error("Failed to send get validators to peer %s: %v", peer.GetAddr(), err)
        }
}

// broadcastNodeInfo broadcasts node information to all peers
func (s *Server) broadcastNodeInfo() error {
        // Get current blockchain height
        height, err := s.blockchain.GetHeight()
        if err != nil {
                utils.Error("Failed to get blockchain height: %v", err)
                return err
        }

        // Create node info
        nodeInfo := &NodeInfo{
                Version:      "1.0.0",
                BlockHeight:  height,
                PeerCount:    len(s.peers),
                ListenAddr:   s.listenAddr,
        }

        // Broadcast to all peers
        err = s.BroadcastMessage(MessageTypeNodeInfo, nodeInfo)
        if err != nil {
                utils.Error("Failed to broadcast node info: %v", err)
                return err
        }
        
        return nil
}

// We don't need this function anymore since we're using json.RawMessage
// Keeping it for backwards compatibility with a direct passthrough
func getBytesFromInterface(data json.RawMessage) ([]byte, error) {
        if len(data) == 0 {
                return nil, fmt.Errorf("data is empty")
        }
        
        return data, nil
}

// Message handlers

// handleNodeInfo handles a node info message
func (s *Server) handleNodeInfo(peer *Peer, message *Message) {
        var nodeInfo NodeInfo
        
        // The Data field is already a json.RawMessage which we can unmarshal directly
        err := json.Unmarshal(message.Data, &nodeInfo)
        if err != nil {
                fmt.Printf("Failed to unmarshal node info: %v\n", err)
                return
        }

        // Check if we need to sync blocks
        localHeight, err := s.blockchain.GetHeight()
        if err != nil {
                fmt.Printf("Failed to get local blockchain height: %v\n", err)
                return
        }

        if nodeInfo.BlockHeight > localHeight {
                // Request blocks from this peer
                s.requestBlocks(peer, localHeight+1)
        }

        // Update peer information
        peer.SetBlockHeight(nodeInfo.BlockHeight)
}

// handlePeerList handles a peer list message with spam protection
func (s *Server) handlePeerList(peer *Peer, message *Message) {
        var peerList []string
        
        err := json.Unmarshal(message.Data, &peerList)
        if err != nil {
                fmt.Printf("Failed to unmarshal peer list: %v\n", err)
                return
        }
        
        // Protect against spam - limit new peers to a reasonable number
        const maxNewPeers = 5
        peerListLen := len(peerList)
        
        if peerListLen > maxNewPeers {
                fmt.Printf("Received oversized peer list (%d entries), limiting to %d\n", 
                     peerListLen, maxNewPeers)
                // Trim the list to avoid connecting to too many peers at once
                // This helps prevent spam and DDoS attacks
                if peerListLen > 0 {
                        // Shuffle the list for randomness to avoid bias
                        for i := range peerList {
                                j := rand.Intn(i + 1)
                                peerList[i], peerList[j] = peerList[j], peerList[i]
                        }
                        peerList = peerList[:maxNewPeers]
                }
        }

        // Current peer count
        s.peersMutex.RLock()
        currentPeerCount := len(s.peers)
        s.peersMutex.RUnlock()
        
        // Don't add more peers if we already have enough
        // This prevents resource exhaustion
        const maxTotalPeers = 20 
        if currentPeerCount >= maxTotalPeers {
                fmt.Printf("Max peer count reached (%d), not adding more peers\n", maxTotalPeers)
                return
        }
        
        // Connect to new peers
        for _, addr := range peerList {
                // Skip invalid addresses
                if addr == "" {
                        continue
                }
                
                // Don't connect to self or the sending peer
                if addr != s.listenAddr && addr != peer.GetAddr() {
                        // Check if already connected
                        s.peersMutex.RLock()
                        _, exists := s.peers[addr]
                        s.peersMutex.RUnlock()

                        if !exists {
                                go s.AddPeer(addr)
                        }
                }
        }
}

// handleBlock handles a block message
func (s *Server) handleBlock(peer *Peer, message *Message) {
        var block core.Block
        
        err := json.Unmarshal(message.Data, &block)
        if err != nil {
                fmt.Printf("Failed to unmarshal block: %v\n", err)
                return
        }

        // Verify and add block to blockchain
        // This would typically involve:
        // 1. Validating the block
        // 2. Adding it to the blockchain if valid
        // 3. Updating state accordingly

        // For simplicity, we'll just print block info
        fmt.Printf("Received block: Height=%d, Hash=%s\n", block.Height, block.Hash)
}

// handleTransaction handles a transaction message
func (s *Server) handleTransaction(peer *Peer, message *Message) {
        var tx core.Transaction
        
        err := json.Unmarshal(message.Data, &tx)
        if err != nil {
                fmt.Printf("Failed to unmarshal transaction: %v\n", err)
                return
        }

        // Verify and add transaction to mempool
        // This would typically involve:
        // 1. Validating the transaction
        // 2. Adding it to the mempool if valid

        // For simplicity, we'll just print transaction info
        fmt.Printf("Received transaction: ID=%s, From=%s, To=%s, Amount=%.10f\n", 
                tx.ID, tx.Sender, tx.Receiver, tx.Amount)
}

// handleGetBlocks handles a get blocks message
func (s *Server) handleGetBlocks(peer *Peer, message *Message) {
        var startHeight int64
        
        err := json.Unmarshal(message.Data, &startHeight)
        if err != nil {
                fmt.Printf("Failed to unmarshal get blocks: %v\n", err)
                return
        }

        // TODO: Implement fetching blocks from storage and sending to peer
        fmt.Printf("Received request for blocks starting at height %d\n", startHeight)
}

// handleGetPeers handles a get peers message
func (s *Server) handleGetPeers(peer *Peer) {
        // Get list of peer addresses
        var peerList []string
        
        s.peersMutex.RLock()
        for addr := range s.peers {
                if addr != peer.GetAddr() {
                        peerList = append(peerList, addr)
                }
        }
        s.peersMutex.RUnlock()

        // Send peer list
        messageBytes, err := s.createMessageBytes(MessageTypePeerList, peerList)
        if err != nil {
                fmt.Printf("Failed to create peer list message: %v\n", err)
                return
        }

        err = peer.SendMessage(messageBytes)
        if err != nil {
                fmt.Printf("Failed to send peer list to peer %s: %v\n", peer.GetAddr(), err)
        }
}

// requestBlocks requests blocks from a peer starting at the given height
func (s *Server) requestBlocks(peer *Peer, startHeight int64) {
        // Create message
        messageBytes, err := s.createMessageBytes(MessageTypeGetBlocks, startHeight)
        if err != nil {
                utils.Error("Failed to create get blocks message: %v", err)
                return
        }

        // Send message
        err = peer.SendMessage(messageBytes)
        if err != nil {
                utils.Error("Failed to send get blocks to peer %s: %v", peer.GetAddr(), err)
        }
}

// handleMessageReward handles a message reward broadcast
func (s *Server) handleMessageReward(peer *Peer, message *Message) {
        var rewardData map[string]interface{}
        
        // Directly unmarshal the json.RawMessage
        if err := json.Unmarshal(message.Data, &rewardData); err != nil {
                fmt.Printf("Failed to unmarshal message reward: %v\n", err)
                return
        }
        
        // Log the message reward
        fmt.Printf("Received message reward: %v\n", rewardData)
        
        // In a full implementation, we would:
        // 1. Validate the reward
        // 2. Add it to the current block or mempool
        // 3. Update balances accordingly
}

// handleValidatorReward handles a validator reward message
func (s *Server) handleValidatorReward(peer *Peer, message *Message) {
        var rewardData map[string]interface{}
        
        // Directly unmarshal the json.RawMessage
        if err := json.Unmarshal(message.Data, &rewardData); err != nil {
                fmt.Printf("Failed to unmarshal validator reward: %v\n", err)
                return
        }
        
        // Log the validator reward
        fmt.Printf("Received validator reward: %v\n", rewardData)
        
        // In a full implementation, we would:
        // 1. Validate the reward
        // 2. Add it to the current block or mempool
        // 3. Update validator balances
}

// handleSystemMessage handles system-wide messages
func (s *Server) handleSystemMessage(peer *Peer, message *Message) {
        var systemData map[string]interface{}
        
        // Directly unmarshal the json.RawMessage
        if err := json.Unmarshal(message.Data, &systemData); err != nil {
                fmt.Printf("Failed to unmarshal system message: %v\n", err)
                return
        }
        
        // Handle system message based on its type
        if msgType, ok := systemData["type"].(string); ok {
                switch msgType {
                case "ANNOUNCEMENT":
                        fmt.Printf("System announcement: %s\n", systemData["message"])
                case "REWARD_RATE_CHANGE":
                        fmt.Printf("Reward rate changed: %v\n", systemData["new_rate"])
                case "VALIDATOR_MIN_STAKE_CHANGE":
                        fmt.Printf("Validator minimum stake changed: %v\n", systemData["new_min_stake"])
                default:
                        fmt.Printf("Unknown system message type: %s\n", msgType)
                }
        } else {
                fmt.Printf("System message without type: %v\n", systemData)
        }
}
