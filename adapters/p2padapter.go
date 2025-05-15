package adapters

import (
	"github.com/doucya/interfaces"
	"github.com/doucya/p2p"
)

// P2PServerAdapter adapts the p2p.Server to the interfaces.P2PServer interface
type P2PServerAdapter struct {
	server *p2p.Server
}

// NewP2PServerAdapter creates a new adapter for the p2p.Server
func NewP2PServerAdapter(server *p2p.Server) interfaces.P2PServer {
	return &P2PServerAdapter{
		server: server,
	}
}

// BroadcastTransaction broadcasts a transaction to all connected peers
func (a *P2PServerAdapter) BroadcastTransaction(tx interface{}) error {
	return a.server.BroadcastTransaction(tx)
}

// BroadcastMessage broadcasts any type of message to all connected peers
func (a *P2PServerAdapter) BroadcastMessage(msgType interface{}, data interface{}) error {
	// Type assertion to convert the interface{} to p2p.MessageType
	messageType, ok := msgType.(p2p.MessageType)
	if !ok {
		messageType = p2p.MessageType(msgType.(string))
	}
	return a.server.BroadcastMessage(messageType, data)
}

// GetPeers returns the list of connected peers as []interface{}
func (a *P2PServerAdapter) GetPeers() []interface{} {
	peers := a.server.GetPeers()
	result := make([]interface{}, len(peers))
	for i, peer := range peers {
		result[i] = peer
	}
	return result
}

// Connect connects to a peer
func (a *P2PServerAdapter) Connect(address string) error {
	return a.server.AddPeer(address)
}

// Start starts the P2P server
func (a *P2PServerAdapter) Start() error {
	return a.server.Start()
}

// Stop stops the P2P server
func (a *P2PServerAdapter) Stop() error {
	a.server.Stop()
	return nil
}