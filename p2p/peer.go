package p2p

import (
        "bufio"
        "encoding/json"
        "fmt"
        "net"
        "time"
)

// Peer represents a peer node in the network
type Peer struct {
        conn        net.Conn
        addr        string
        server      *Server
        send        chan []byte
        quit        chan struct{}
        blockHeight int64
}

// NewPeer creates a new peer
func NewPeer(addr string, server *Server) (*Peer, error) {
        return &Peer{
                addr:   addr,
                server: server,
                send:   make(chan []byte, 100),
                quit:   make(chan struct{}),
        }, nil
}

// Connect connects to the peer
func (p *Peer) Connect() error {
        conn, err := net.DialTimeout("tcp", p.addr, 5*time.Second)
        if err != nil {
                return fmt.Errorf("failed to connect to peer %s: %v", p.addr, err)
        }
        p.conn = conn
        return nil
}

// Close closes the connection to the peer
func (p *Peer) Close() {
        close(p.quit)
        if p.conn != nil {
                p.conn.Close()
        }
}

// GetAddr returns the peer's address
func (p *Peer) GetAddr() string {
        return p.addr
}

// SetBlockHeight sets the peer's blockchain height
func (p *Peer) SetBlockHeight(height int64) {
        p.blockHeight = height
}

// GetBlockHeight returns the peer's blockchain height
func (p *Peer) GetBlockHeight() int64 {
        return p.blockHeight
}

// SendMessage sends a message to the peer
func (p *Peer) SendMessage(data []byte) error {
        // Add newline as message delimiter
        data = append(data, '\n')
        
        // Set write deadline
        err := p.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
        if err != nil {
                return fmt.Errorf("failed to set write deadline: %v", err)
        }
        
        // Write data
        _, err = p.conn.Write(data)
        if err != nil {
                return fmt.Errorf("failed to send message: %v", err)
        }
        
        return nil
}

// HandleMessages handles incoming messages from the peer
func (p *Peer) HandleMessages() {
        // Start reader and writer goroutines
        go p.readMessages()
        go p.writeMessages()
}

// readMessages reads messages from the peer
func (p *Peer) readMessages() {
        scanner := bufio.NewScanner(p.conn)
        
        for scanner.Scan() {
                // Get message data
                data := scanner.Bytes()
                
                // Skip empty messages
                if len(data) == 0 {
                        continue
                }
                
                // Check if it's a protocol handshake or some other non-JSON data
                if len(data) > 0 && (data[0] == 'G' || data[0] == 'H' || data[0] == 'P') {
                        // Likely an HTTP request or other protocol, ignore
                        fmt.Printf("Ignoring non-protocol message from peer %s: starts with %c\n", p.addr, data[0])
                        continue
                }
                
                // Parse message
                var message Message
                err := json.Unmarshal(data, &message)
                if err != nil {
                        fmt.Printf("Failed to parse message from peer %s (len: %d): %v\n", p.addr, len(data), err)
                        continue
                }
                
                // Validate message
                if message.Type == "" {
                        fmt.Printf("Received message with empty type from peer %s\n", p.addr)
                        continue
                }
                
                // Handle message
                p.server.HandleMessage(p, &message)
        }
        
        // Check for scanner error
        if err := scanner.Err(); err != nil {
                fmt.Printf("Error reading from peer %s: %v\n", p.addr, err)
        }
        
        // Remove peer on error or disconnect
        p.server.RemovePeer(p.addr)
}

// writeMessages writes messages to the peer
func (p *Peer) writeMessages() {
        for {
                select {
                case data := <-p.send:
                        err := p.SendMessage(data)
                        if err != nil {
                                fmt.Printf("Failed to send message to peer %s: %v\n", p.addr, err)
                                p.server.RemovePeer(p.addr)
                                return
                        }
                case <-p.quit:
                        return
                }
        }
}
