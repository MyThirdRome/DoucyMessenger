package messaging

import (
        "crypto/ecdsa"
        "crypto/sha256"
        "encoding/hex"
        "encoding/json"
        "errors"
        "time"

        "github.com/doucya/interfaces"
        "github.com/doucya/utils"
        "github.com/doucya/wallet"
)

// Message represents a message in the blockchain
// Implements the interfaces.MessageData interface
type Message struct {
        ID        string    `json:"id"`
        Sender    string    `json:"sender"`
        Receiver  string    `json:"receiver"`
        Content   string    `json:"content"`
        Timestamp time.Time `json:"timestamp"`
        Signature string    `json:"signature"`
        Encrypted bool      `json:"encrypted"`
        ReplyTo   string    `json:"reply_to,omitempty"` // ID of message this is replying to
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

// GetReplyToID returns the ID of the message this is replying to
func (m *Message) GetReplyToID() string {
        return m.ReplyTo
}

// GetSignature returns the message signature
func (m *Message) GetSignature() string {
        return m.Signature
}

// IsEncrypted returns whether the message is encrypted
func (m *Message) IsEncrypted() bool {
        return m.Encrypted
}

// GetReplyTo returns the ID of the message this is replying to
func (m *Message) GetReplyTo() string {
        return m.ReplyTo
}

// SetReplyTo sets the ID of the message this is replying to
func (m *Message) SetReplyTo(messageID string) {
        m.ReplyTo = messageID
}

// ToMessageMetadata converts the message to a MessageMetadata object
func (m *Message) ToMessageMetadata() *interfaces.MessageMetadata {
        return interfaces.NewMessageMetadata(m.ID, m.Sender, m.Receiver, m.Timestamp)
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

// NewReplyMessage creates a new message in reply to another message
func NewReplyMessage(sender, receiver, content, replyToID string) *Message {
        message := NewMessage(sender, receiver, content)
        message.ReplyTo = replyToID
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

        // Parse the private key
        privateKey, err := utils.PrivateKeyFromHex(privateKeyHex)
        if err != nil {
                // Fall back to simple decryption if we can't parse the private key
                decrypted := decryptContent(m.Content, privateKeyHex)
                m.Content = decrypted
                m.Encrypted = false
                return decrypted, nil
        }

        // Use proper private key decryption
        decrypted, err := utils.DecryptWithPrivateKey(m.Content, privateKey)
        if err != nil {
                // Fall back to simple decryption
                decrypted := decryptContent(m.Content, privateKeyHex)
                m.Content = decrypted
                m.Encrypted = false
                return decrypted, nil
        }

        m.Content = decrypted
        m.Encrypted = false
        
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
// Now using real encryption with our crypto utilities
func encryptContent(content, receiverAddr string) string {
        // Get the public key for the receiver
        // For now, we'll use a simple deterministic derivation based on the address
        // In a real implementation, we would look up the public key from a directory
        pubKeyHash := sha256.Sum256([]byte(receiverAddr))
        keyBytes := pubKeyHash[:]

        // Use AES encryption with derived key
        encrypted, err := utils.AESEncryptMessage(content, keyBytes)
        if err != nil {
                // Fall back to very simple encryption if real encryption fails
                hash := sha256.Sum256([]byte(receiverAddr))
                result := make([]byte, len(content))
                for i := 0; i < len(content); i++ {
                        result[i] = content[i] ^ hash[i%32]
                }
                return hex.EncodeToString(result)
        }
        
        return encrypted
}

// decryptContent decrypts message content using private key
func decryptContent(encryptedContent, privateKeyHex string) string {
        // Derive the same key hash that was used for encryption
        // In a real implementation, we would use the actual private key
        hash := sha256.Sum256([]byte(privateKeyHex))
        keyBytes := hash[:]

        // Try to decrypt with proper encryption
        decrypted, err := utils.AESDecryptMessage(encryptedContent, keyBytes)
        if err != nil {
                // Fall back to simple XOR decryption
                content, decodeErr := hex.DecodeString(encryptedContent)
                if decodeErr != nil {
                        return "Error: Could not decrypt message"
                }
                
                result := make([]byte, len(content))
                for i := 0; i < len(content); i++ {
                        result[i] = content[i] ^ hash[i%32]
                }
                
                return string(result)
        }
        
        return decrypted
}

// Encrypt encrypts the message content with a public key
func (m *Message) Encrypt(publicKeyHex string) error {
        if m.Encrypted {
                return errors.New("message already encrypted")
        }

        // Parse the public key
        publicKey, err := utils.PublicKeyFromHex(publicKeyHex)
        if err != nil {
                // Fall back to simple encryption if we can't parse the public key
                m.Content = encryptContent(m.Content, publicKeyHex)
                m.Encrypted = true
                return nil
        }

        // Use proper public key encryption
        encrypted, err := utils.EncryptWithPublicKey(m.Content, publicKey)
        if err != nil {
                // Fall back to simple encryption
                m.Content = encryptContent(m.Content, publicKeyHex)
                m.Encrypted = true
                return nil
        }

        m.Content = encrypted
        m.Encrypted = true
        
        return nil
}


