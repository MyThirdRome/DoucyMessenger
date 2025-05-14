package models

import (
        "crypto/ecdsa"
        "crypto/sha256"
        "encoding/hex"
        "encoding/json"
        "errors"
        "time"

        "github.com/doucya/messaging"
        "github.com/doucya/utils"
)

// TransactionType represents the type of a transaction
type TransactionType string

const (
        TransactionTypeCoin        TransactionType = "COIN"
        TransactionTypeMessage     TransactionType = "MESSAGE"
        TransactionTypeGenesis     TransactionType = "GENESIS"
        TransactionTypeValidatorIn TransactionType = "VALIDATOR_IN"
        TransactionTypeValidatorOut TransactionType = "VALIDATOR_OUT"
)

// Transaction represents a transaction in the blockchain
type Transaction struct {
        ID        string          `json:"id"`
        Timestamp time.Time       `json:"timestamp"`
        Sender    string          `json:"sender"`
        Receiver  string          `json:"receiver"`
        Amount    float64         `json:"amount"`
        Type      TransactionType `json:"type"`
        Message   *messaging.Message `json:"message,omitempty"`
        IsPenalty bool            `json:"is_penalty,omitempty"`
        Signature string          `json:"signature"`
}

// NewTransaction creates a new transaction
func NewTransaction(sender, receiver string, amount float64, txType TransactionType) *Transaction {
        tx := &Transaction{
                Timestamp: time.Now(),
                Sender:    sender,
                Receiver:  receiver,
                Amount:    amount,
                Type:      txType,
        }
        
        // Generate transaction ID based on its contents
        tx.ID = tx.generateID()
        
        return tx
}

// NewMessageTransaction creates a new message transaction
func NewMessageTransaction(sender, receiver string, message *messaging.Message, isPenalty bool) *Transaction {
        tx := &Transaction{
                Timestamp: time.Now(),
                Sender:    sender,
                Receiver:  receiver,
                Amount:    0, // No direct amount for message transactions
                Type:      TransactionTypeMessage,
                Message:   message,
                IsPenalty: isPenalty,
        }
        
        // Generate transaction ID based on its contents
        tx.ID = tx.generateID()
        
        return tx
}

// generateID generates a unique ID for the transaction
func (tx *Transaction) generateID() string {
        // Create a hash of transaction details
        txData := struct {
                Timestamp time.Time
                Sender    string
                Receiver  string
                Amount    float64
                Type      TransactionType
                MessageID string
                IsPenalty bool
        }{
                Timestamp: tx.Timestamp,
                Sender:    tx.Sender,
                Receiver:  tx.Receiver,
                Amount:    tx.Amount,
                Type:      tx.Type,
                IsPenalty: tx.IsPenalty,
        }
        
        if tx.Message != nil {
                txData.MessageID = tx.Message.ID
        }
        
        data, err := json.Marshal(txData)
        if err != nil {
                return ""
        }
        
        hash := sha256.Sum256(data)
        return hex.EncodeToString(hash[:])
}

// Sign signs the transaction with the given private key
func (tx *Transaction) Sign(privateKey *ecdsa.PrivateKey) error {
        if privateKey == nil {
                return errors.New("nil private key")
        }
        
        // In a real implementation, we would sign the transaction ID with the private key
        signature, err := utils.SignData(privateKey, []byte(tx.ID))
        if err != nil {
                return err
        }
        
        tx.Signature = hex.EncodeToString(signature)
        return nil
}

// Verify verifies the transaction's signature
func (tx *Transaction) Verify() bool {
        // Genesis transactions don't need verification
        if tx.Type == TransactionTypeGenesis {
                return true
        }
        
        // Transactions without a sender (system transactions) don't need verification
        if tx.Sender == "" {
                return true
        }
        
        // In a real implementation, we would verify the signature using the sender's public key
        // For simplicity, we'll just check if a signature exists
        return tx.Signature != ""
}

// GetID returns the transaction ID
func (tx *Transaction) GetID() string {
        return tx.ID
}

// GetSender returns the sender address
func (tx *Transaction) GetSender() string {
        return tx.Sender
}

// GetReceiver returns the receiver address
func (tx *Transaction) GetReceiver() string {
        return tx.Receiver
}

// GetAmount returns the transaction amount
func (tx *Transaction) GetAmount() float64 {
        return tx.Amount
}

// GetTimestamp returns the transaction timestamp
func (tx *Transaction) GetTimestamp() time.Time {
        return tx.Timestamp
}

// GetSignature returns the transaction signature
func (tx *Transaction) GetSignature() string {
        return tx.Signature
}

// GetType returns the transaction type
func (tx *Transaction) GetType() TransactionType {
        return tx.Type
}

// GetMessage returns the transaction message if it exists
func (tx *Transaction) GetMessage() *messaging.Message {
        return tx.Message
}

// MarshalJSON customizes the JSON marshaling for Transaction
func (tx *Transaction) MarshalJSON() ([]byte, error) {
        type TransactionAlias Transaction
        return json.Marshal(&struct {
                Timestamp string `json:"timestamp"`
                *TransactionAlias
        }{
                Timestamp:        tx.Timestamp.Format(time.RFC3339),
                TransactionAlias: (*TransactionAlias)(tx),
        })
}

// UnmarshalJSON customizes the JSON unmarshaling for Transaction
func (tx *Transaction) UnmarshalJSON(data []byte) error {
        type TransactionAlias Transaction
        aux := &struct {
                Timestamp string `json:"timestamp"`
                *TransactionAlias
        }{
                TransactionAlias: (*TransactionAlias)(tx),
        }
        
        if err := json.Unmarshal(data, &aux); err != nil {
                return err
        }
        
        timestamp, err := time.Parse(time.RFC3339, aux.Timestamp)
        if err != nil {
                return err
        }
        tx.Timestamp = timestamp
        
        return nil
}