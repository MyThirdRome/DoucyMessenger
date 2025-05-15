package interfaces

// P2PServer defines the interface for the p2p networking
type P2PServer interface {
        // BroadcastTransaction broadcasts a transaction to all connected peers
        BroadcastTransaction(tx interface{}) error
        
        // BroadcastMessage broadcasts any type of message to all connected peers
        // This interface must match exactly the implementation in p2p/server.go
        BroadcastMessage(msgType interface{}, data interface{}) error
        
        // GetPeers returns the list of connected peers
        GetPeers() []interface{}
        
        // Connect connects to a peer
        Connect(address string) error
        
        // Start starts the P2P server
        Start() error
        
        // Stop stops the P2P server
        Stop() error
}