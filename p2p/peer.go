package p2p

import (
        "bufio"
        "encoding/json"
        "fmt"
        "net"
        "strings"
        "time"
        
        "github.com/doucya/utils"
)

// min returns the smaller of x or y.
func min(x, y int) int {
        if x < y {
                return x
        }
        return y
}

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

// Connect connects to the peer with a proper protocol handshake
func (p *Peer) Connect() error {
        // Use a shorter timeout for connection to prevent CLI from hanging
        conn, err := net.DialTimeout("tcp", p.addr, 3*time.Second)
        if err != nil {
                return fmt.Errorf("failed to connect to peer %s: %v", p.addr, err)
        }
        
        p.conn = conn
        
        // Set deadlines to prevent blocking indefinitely
        conn.SetReadDeadline(time.Now().Add(5 * time.Second))
        conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
        
        // Create DoucyA protocol handshake with protocol identifier
        handshake := Message{
                Type: MessageTypeNodeInfo,
                Data: json.RawMessage(`{"version":"1.0","protocol":"doucyap2p","handshake":true}`),
        }
        
        handshakeBytes, err := json.Marshal(handshake)
        if err != nil {
                conn.Close()
                return fmt.Errorf("failed to create handshake: %v", err)
        }
        
        // Add a newline delimiter for proper message framing
        handshakeBytes = append(handshakeBytes, '\n')
        
        // Send the handshake
        _, err = conn.Write(handshakeBytes)
        if err != nil {
                conn.Close()
                return fmt.Errorf("failed to send handshake: %v", err)
        }
        
        // Reset deadlines to normal operation - we'll handle timeouts in the message read loop
        conn.SetReadDeadline(time.Time{})
        conn.SetWriteDeadline(time.Time{})
        
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

// SendMessage sends a raw message to the peer
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

// SendMessageObject sends a Message object to the peer
func (p *Peer) SendMessageObject(message *Message) error {
        // Convert message to JSON
        data, err := json.Marshal(message)
        if err != nil {
                return fmt.Errorf("failed to marshal message: %v", err)
        }
        
        // Send message
        return p.SendMessage(data)
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
                
                // Enhanced protocol message detection
                // Check if this is a valid DoucyA protocol message
                if len(data) < 2 {
                        utils.Debug("Ignoring too short message from peer %s", p.addr)
                        continue
                }
                
                // Check for HTTP and other common protocols to filter out non-DoucyA traffic
                firstChar := data[0]
                if firstChar == 'G' || firstChar == 'H' || firstChar == 'P' ||     // HTTP methods (GET, HEAD, POST)
                   firstChar == 'C' || firstChar == 'U' || firstChar == 'D' ||     // HTTP methods (CONNECT, UPDATE, DELETE)
                   firstChar == 'A' || firstChar == 'S' || firstChar == 'X' {      // Other protocols
                        utils.Debug("Ignoring non-protocol message from peer %s: starts with %c", p.addr, firstChar)
                        continue
                }
                
                // Check for valid JSON (typical DoucyA message starts with "{")
                if data[0] != '{' {
                        // Not starting with JSON object open brace
                        utils.Debug("Ignoring non-JSON message from peer %s", p.addr)
                        continue
                }
                
                // Basic JSON structure verification - at minimum need opening/closing braces
                if !strings.Contains(string(data), "}") {
                        utils.Debug("Ignoring malformed JSON from peer %s", p.addr)
                        continue
                }
                
                // Parse message with better error handling
                var message Message
                err := json.Unmarshal(data, &message)
                if err != nil {
                        // Log error but don't print the data which might be large
                        utils.Debug("Failed to parse message from peer %s (len: %d): %v", p.addr, len(data), err)
                        continue
                }
                
                // Validate message
                if message.Type == "" {
                        utils.Debug("Received message with empty type from peer %s", p.addr)
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
