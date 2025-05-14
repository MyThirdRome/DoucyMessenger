package p2p

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/doucya/core"
)

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
			fmt.Printf("Failed to connect to bootstrap node %s: %v\n", addr, err)
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

// AddPeer adds a new peer to the server
func (s *Server) AddPeer(addr string) error {
	s.peersMutex.Lock()
	defer s.peersMutex.Unlock()

	// Check if peer already exists
	if _, exists := s.peers[addr]; exists {
		return nil // Already connected
	}

	// Create and connect peer
	peer, err := NewPeer(addr, s)
	if err != nil {
		return fmt.Errorf("failed to create peer: %v", err)
	}

	err = peer.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to peer: %v", err)
	}

	// Add to peers map
	s.peers[addr] = peer

	// Start peer handlers
	go peer.HandleMessages()

	// Exchange node information
	s.exchangeNodeInfo(peer)

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
	default:
		fmt.Printf("Unknown message type: %s\n", message.Type)
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
	message := &Message{
		Type: msgType,
		Data: data,
	}

	return json.Marshal(message)
}

// exchangeNodeInfo exchanges node information with a peer
func (s *Server) exchangeNodeInfo(peer *Peer) {
	// Get current blockchain height
	height, err := s.blockchain.GetHeight()
	if err != nil {
		fmt.Printf("Failed to get blockchain height: %v\n", err)
		return
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
		fmt.Printf("Failed to create node info message: %v\n", err)
		return
	}

	err = peer.SendMessage(messageBytes)
	if err != nil {
		fmt.Printf("Failed to send node info to peer %s: %v\n", peer.GetAddr(), err)
	}

	// Request peer list
	s.requestPeerList(peer)
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
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.syncWithPeers()
		case <-s.quit:
			return
		}
	}
}

// syncWithPeers synchronizes with all peers
func (s *Server) syncWithPeers() {
	// Get current blockchain height
	height, err := s.blockchain.GetHeight()
	if err != nil {
		fmt.Printf("Failed to get blockchain height: %v\n", err)
		return
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
		fmt.Printf("Failed to broadcast node info: %v\n", err)
	}
}

// Message handlers

// handleNodeInfo handles a node info message
func (s *Server) handleNodeInfo(peer *Peer, message *Message) {
	var nodeInfo NodeInfo
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

// handlePeerList handles a peer list message
func (s *Server) handlePeerList(peer *Peer, message *Message) {
	var peerList []string
	err := json.Unmarshal(message.Data, &peerList)
	if err != nil {
		fmt.Printf("Failed to unmarshal peer list: %v\n", err)
		return
	}

	// Connect to new peers
	for _, addr := range peerList {
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
		fmt.Printf("Failed to create get blocks message: %v\n", err)
		return
	}

	// Send message
	err = peer.SendMessage(messageBytes)
	if err != nil {
		fmt.Printf("Failed to send get blocks to peer %s: %v\n", peer.GetAddr(), err)
	}
}
