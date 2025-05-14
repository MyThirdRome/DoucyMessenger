package messaging

import (
        "crypto/ecdsa"
        "crypto/sha256"
        "encoding/hex"
        "encoding/json"
        "errors"
        "fmt"
        "time"

        "github.com/doucya/utils"
        "github.com/doucya/wallet"
)

// Message represents a message in the blockchain
type Message struct {
        ID        string    `json:"id"`
        Sender    string    `json:"sender"`
        Receiver  string    `json:"receiver"`
        Content   string    `json:"content"`
        Timestamp time.Time `json:"timestamp"`
        Signature string    `json:"signature"`
        Encrypted bool      `json:"encrypted"`
}

// GetID returns the message ID
func (m *Message) GetID() string {
        return m.ID
}

// GetSender returns the sender address
func (m *Message) GetSender() string {
        return m.Sender
}

// GetReceiver returns the receiver address
func (m *Message) GetReceiver() string {
        return m.Receiver
}

// GetContent returns the message content
func (m *Message) GetContent() string {
        return m.Content
}

// GetTimestamp returns the message timestamp
func (m *Message) GetTimestamp() time.Time {
        return m.Timestamp
}

// GetSignature returns the message signature
func (m *Message) GetSignature() string {
        return m.Signature
}

// IsEncrypted returns whether the message is encrypted
func (m *Message) IsEncrypted() bool {
        return m.Encrypted
}

// NewMessage creates a new message
func NewMessage(sender, receiver, content string) *Message {
        message := &Message{
                Sender:    sender,
                Receiver:  receiver,
                Content:   content,
                Timestamp: time.Now(),
                Encrypted: true, // Messages are encrypted by default
        }

        // Generate message ID based on its contents
        message.ID = message.generateID()

        // Encrypt content if receiver is specified
        if receiver != "" && wallet.ValidateAddress(receiver) {
                message.Content = encryptContent(content, receiver)
        }

        return message
}

// generateID generates a unique ID for the message
func (m *Message) generateID() string {
        // Create a hash of message details
        msgData := struct {
                Sender    string
                Receiver  string
                Content   string
                Timestamp time.Time
        }{
                Sender:    m.Sender,
                Receiver:  m.Receiver,
                Content:   m.Content,
                Timestamp: m.Timestamp,
        }

        data, err := json.Marshal(msgData)
        if err != nil {
                return utils.GenerateRandomID() // Fallback
        }

        hash := sha256.Sum256(data)
        return hex.EncodeToString(hash[:])
}

// Sign signs the message with the given private key
func (m *Message) Sign(key interface{}) error {
        privateKey, ok := key.(*ecdsa.PrivateKey)
        if !ok || privateKey == nil {
                return errors.New("invalid or nil private key")
        }

        // Sign the message ID
        signature, err := utils.SignData(privateKey, []byte(m.ID))
        if err != nil {
                return err
        }

        m.Signature = hex.EncodeToString(signature)
        return nil
}

// Verify verifies the message's signature
func (m *Message) Verify(key interface{}) bool {
        publicKey, ok := key.(*ecdsa.PublicKey)
        if !ok || publicKey == nil || m.Signature == "" {
                return false
        }

        // Decode signature
        signature, err := hex.DecodeString(m.Signature)
        if err != nil {
                return false
        }

        // Verify signature
        return utils.VerifySignature(publicKey, []byte(m.ID), signature)
}

// Decrypt decrypts the message content with the given private key
func (m *Message) Decrypt(privateKeyHex string) (string, error) {
        if !m.Encrypted {
                return m.Content, nil
        }

        // Validate private key format
        _, err := utils.PrivateKeyFromHex(privateKeyHex)
        if err != nil {
                return "", fmt.Errorf("invalid private key: %v", err)
        }

        // In a real implementation, we would use the private key to decrypt the content
        // Here, we're using a simple encryption for demonstration
        decrypted := decryptContent(m.Content, privateKeyHex)
        return decrypted, nil
}

// MarshalJSON customizes the JSON marshaling for Message
func (m *Message) MarshalJSON() ([]byte, error) {
        type MessageAlias Message
        return json.Marshal(&struct {
                Timestamp string `json:"timestamp"`
                *MessageAlias
        }{
                Timestamp:    m.Timestamp.Format(time.RFC3339),
                MessageAlias: (*MessageAlias)(m),
        })
}

// UnmarshalJSON customizes the JSON unmarshaling for Message
func (m *Message) UnmarshalJSON(data []byte) error {
        type MessageAlias Message
        aux := &struct {
                Timestamp string `json:"timestamp"`
                *MessageAlias
        }{
                MessageAlias: (*MessageAlias)(m),
        }

        if err := json.Unmarshal(data, &aux); err != nil {
                return err
        }

        timestamp, err := time.Parse(time.RFC3339, aux.Timestamp)
        if err != nil {
                return err
        }
        m.Timestamp = timestamp

        return nil
}

// Helper functions for encryption/decryption

// encryptContent encrypts message content using receiver's address
// This is a simplified encryption; in a real implementation, we would use
// the receiver's public key for asymmetric encryption
func encryptContent(content, receiverAddr string) string {
        // For simplicity, we're using XOR with a hash of the receiver's address
        // In a real implementation, use proper encryption like AES or RSA
        
        // Create a hash of the receiver's address
        hash := sha256.Sum256([]byte(receiverAddr))
        
        // XOR the content with the hash
        result := make([]byte, len(content))
        for i := 0; i < len(content); i++ {
                result[i] = content[i] ^ hash[i%32]
        }
        
        return hex.EncodeToString(result)
}

// decryptContent decrypts message content using private key
func decryptContent(encryptedContent, privateKeyHex string) string {
        // For simplicity, we're reversing the XOR operation
        // In a real implementation, use proper decryption
        
        // Decode hex string
        content, err := hex.DecodeString(encryptedContent)
        if err != nil {
                return "Error: Could not decrypt message"
        }
        
        // Create a hash of the private key
        hash := sha256.Sum256([]byte(privateKeyHex))
        
        // XOR the content with the hash
        result := make([]byte, len(content))
        for i := 0; i < len(content); i++ {
                result[i] = content[i] ^ hash[i%32]
        }
        
        return string(result)
}
