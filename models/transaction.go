package models

import (
        "crypto/ecdsa"
        "crypto/sha256"
        "encoding/hex"
        "encoding/json"
        "errors"
        "fmt"
        "time"

        "github.com/doucya/interfaces"
        "github.com/doucya/utils"
)

// TransactionType represents the type of a transaction
type TransactionType string

const (
        TransactionTypeCoin         TransactionType = "COIN"
        TransactionTypeMessage      TransactionType = "MESSAGE"
        TransactionTypeMessageReward TransactionType = "MESSAGE_REWARD"
        TransactionTypeGenesis      TransactionType = "GENESIS"
        TransactionTypeValidatorIn  TransactionType = "VALIDATOR_IN"
        TransactionTypeValidatorOut TransactionType = "VALIDATOR_OUT"
        TransactionTypeValidatorReward TransactionType = "VALIDATOR_REWARD"
)

// TxInput represents a transaction input
type TxInput struct {
        TxID        string `json:"tx_id"`        // Reference to the transaction containing the output
        OutputIndex int    `json:"output_index"` // Index of the output in the referenced transaction
        Owner       string `json:"owner"`        // Owner's address
        Signature   string `json:"signature"`    // Signature to verify ownership
}

// TxOutput represents a transaction output
type TxOutput struct {
        Value  float64 `json:"value"`  // Amount of coins
        Owner  string  `json:"owner"`  // Address of the recipient
}

// Transaction represents a transaction in the blockchain
type Transaction struct {
        ID          string          `json:"id"`
        Timestamp   time.Time       `json:"timestamp"`
        Sender      string          `json:"sender"`     // Kept for backwards compatibility
        Receiver    string          `json:"receiver"`   // Kept for backwards compatibility
        Amount      float64         `json:"amount"`     // Kept for backwards compatibility
        Type        TransactionType `json:"type"`
        MessageData *interfaces.MessageMetadata `json:"message_data,omitempty"`
        IsPenalty   bool            `json:"is_penalty,omitempty"`
        Signature   string          `json:"signature"`
        Data        map[string]interface{} `json:"data,omitempty"`
        
        // New UTXO fields
        Inputs      []*TxInput      `json:"inputs"`
        Outputs     []*TxOutput     `json:"outputs"`
}

// NewTransaction creates a new transaction (for backward compatibility)
func NewTransaction(sender, receiver string, amount float64, txType TransactionType) *Transaction {
        tx := &Transaction{
                Timestamp: time.Now(),
                Sender:    sender,
                Receiver:  receiver,
                Amount:    amount,
                Type:      txType,
                Data:      make(map[string]interface{}),
                Inputs:    []*TxInput{},
                Outputs:   []*TxOutput{},
        }
        
        // For genesis transactions, create a direct output to the receiver
        if txType == TransactionTypeGenesis {
                output := &TxOutput{
                        Value: amount,
                        Owner: receiver,
                }
                tx.Outputs = append(tx.Outputs, output)
        }
        
        // Generate transaction ID based on its contents
        tx.ID = tx.GenerateID()
        
        return tx
}

// NewUTXOTransaction creates a transaction with proper UTXO inputs and outputs
func NewUTXOTransaction(sender, receiver string, amount float64, txType TransactionType, utxoSet *UTXOSet) (*Transaction, error) {
        // Create transaction structure
        tx := &Transaction{
                Timestamp: time.Now(),
                Sender:    sender,
                Receiver:  receiver,
                Amount:    amount,
                Type:      txType,
                Data:      make(map[string]interface{}),
                Inputs:    []*TxInput{},
                Outputs:   []*TxOutput{},
        }
        
        // For genesis transactions, create a direct output to the receiver without inputs
        if txType == TransactionTypeGenesis {
                output := &TxOutput{
                        Value: amount,
                        Owner: receiver,
                }
                tx.Outputs = append(tx.Outputs, output)
        } else if sender != "" && sender != "SYSTEM" {
                // For normal transactions, find available UTXOs to use as inputs
                spendableUTXOs, accumulated := utxoSet.FindSpendableUTXOs(sender, amount)
                
                if accumulated < amount {
                        return nil, fmt.Errorf("not enough funds: got %.2f, need %.2f", accumulated, amount)
                }
                
                // Create inputs from the available UTXOs
                for _, utxo := range spendableUTXOs {
                        input := &TxInput{
                                TxID:        utxo.TxID,
                                OutputIndex: utxo.TxOutputIdx,
                                Owner:       sender,
                        }
                        tx.Inputs = append(tx.Inputs, input)
                }
                
                // Create output to receiver
                output := &TxOutput{
                        Value: amount,
                        Owner: receiver,
                }
                tx.Outputs = append(tx.Outputs, output)
                
                // If there's change, create output back to sender
                if accumulated > amount {
                        changeOutput := &TxOutput{
                                Value: accumulated - amount,
                                Owner: sender,
                        }
                        tx.Outputs = append(tx.Outputs, changeOutput)
                }
        }
        
        // Generate transaction ID based on its contents
        tx.ID = tx.GenerateID()
        
        return tx, nil
}

// NewMessageTransaction creates a new message transaction
func NewMessageTransaction(sender, receiver string, message interfaces.MessageData, isPenalty bool) *Transaction {
        // Create a light metadata object from the message 
        metadata := interfaces.NewMessageMetadata(
                message.GetID(),
                message.GetSender(),
                message.GetReceiver(),
                message.GetTimestamp(),
        )

        tx := &Transaction{
                Timestamp: time.Now(),
                Sender:    sender,
                Receiver:  receiver,
                Amount:    0, // No direct amount for message transactions
                Type:      TransactionTypeMessage,
                MessageData: metadata,
                IsPenalty: isPenalty,
                Data:      make(map[string]interface{}),
        }
        
        // Generate transaction ID based on its contents
        tx.ID = tx.GenerateID()
        
        return tx
}

// NewRewardTransaction creates a new reward transaction
func NewRewardTransaction(sender, receiver string, amount float64, messageID string) *Transaction {
        tx := &Transaction{
                Timestamp: time.Now(),
                Sender:    sender,
                Receiver:  receiver,
                Amount:    amount,
                Type:      TransactionTypeMessageReward,
                Data:      map[string]interface{}{"messageID": messageID},
        }
        
        // Generate transaction ID based on its contents
        tx.ID = tx.GenerateID()
        
        return tx
}

// GenerateID generates a unique ID for the transaction
func (tx *Transaction) GenerateID() string {
        // Create a hash of transaction details
        txData := struct {
                Timestamp time.Time
                Sender    string
                Receiver  string
                Amount    float64
                Type      TransactionType
                MessageID string
                IsPenalty bool
                Data      map[string]interface{}
        }{
                Timestamp: tx.Timestamp,
                Sender:    tx.Sender,
                Receiver:  tx.Receiver,
                Amount:    tx.Amount,
                Type:      tx.Type,
                IsPenalty: tx.IsPenalty,
                Data:      tx.Data,
        }
        
        if tx.MessageData != nil {
                txData.MessageID = tx.MessageData.GetID()
        }
        
        data, err := json.Marshal(txData)
        if err != nil {
                return utils.GenerateRandomID() // Fallback to random ID in case of error
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
        if tx.Sender == "" || tx.Sender == "SYSTEM" {
                return true
        }
        
        // In a real implementation, we would verify the signature using the sender's public key
        // For simplicity, we'll just check if a signature exists
        return tx.Signature != ""
}

// SetData sets additional data for the transaction
func (tx *Transaction) SetData(key string, value interface{}) {
        if tx.Data == nil {
                tx.Data = make(map[string]interface{})
        }
        tx.Data[key] = value
}

// GetData gets additional data from the transaction
func (tx *Transaction) GetData(key string) interface{} {
        if tx.Data == nil {
                return nil
        }
        return tx.Data[key]
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

// GetMessageData returns the transaction message metadata if it exists
func (tx *Transaction) GetMessageData() *interfaces.MessageMetadata {
        return tx.MessageData
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