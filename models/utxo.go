package models

import (
	"encoding/json"
)

// UTXO represents an Unspent Transaction Output
type UTXO struct {
	TxID         string  `json:"tx_id"`        // Transaction ID that created this UTXO
	TxOutputIdx  int     `json:"tx_output_idx"` // Output index in the transaction
	Amount       float64 `json:"amount"`       // Amount of coins
	Owner        string  `json:"owner"`        // Owner address
	IsSpent      bool    `json:"is_spent"`     // Whether the UTXO has been spent
	BlockHeight  int64   `json:"block_height"` // Height of the block containing this UTXO
}

// UTXOSet represents the collection of all UTXOs in the blockchain
type UTXOSet struct {
	UTXOs map[string]*UTXO `json:"utxos"` // Map of UTXO ID to UTXO
}

// NewUTXOSet creates a new empty UTXO set
func NewUTXOSet() *UTXOSet {
	return &UTXOSet{
		UTXOs: make(map[string]*UTXO),
	}
}

// AddUTXO adds a UTXO to the set
func (us *UTXOSet) AddUTXO(utxo *UTXO) {
	utxoKey := utxo.TxID + ":" + string(utxo.TxOutputIdx)
	us.UTXOs[utxoKey] = utxo
}

// GetUTXO gets a UTXO from the set
func (us *UTXOSet) GetUTXO(txID string, outputIdx int) *UTXO {
	utxoKey := txID + ":" + string(outputIdx)
	return us.UTXOs[utxoKey]
}

// MarkUTXOSpent marks a UTXO as spent
func (us *UTXOSet) MarkUTXOSpent(txID string, outputIdx int) {
	utxoKey := txID + ":" + string(outputIdx)
	if utxo, ok := us.UTXOs[utxoKey]; ok {
		utxo.IsSpent = true
	}
}

// GetUTXOsForAddress gets all UTXOs for an address
func (us *UTXOSet) GetUTXOsForAddress(address string) []*UTXO {
	var utxos []*UTXO
	for _, utxo := range us.UTXOs {
		if utxo.Owner == address && !utxo.IsSpent {
			utxos = append(utxos, utxo)
		}
	}
	return utxos
}

// GetBalanceForAddress calculates the balance for an address by summing all unspent UTXOs
func (us *UTXOSet) GetBalanceForAddress(address string) float64 {
	var balance float64
	for _, utxo := range us.UTXOs {
		if utxo.Owner == address && !utxo.IsSpent {
			balance += utxo.Amount
		}
	}
	return balance
}

// FindSpendableUTXOs finds the UTXOs that can be spent for the given amount
func (us *UTXOSet) FindSpendableUTXOs(address string, amount float64) ([]*UTXO, float64) {
	var unspentUTXOs []*UTXO
	var accumulated float64

	for _, utxo := range us.UTXOs {
		if utxo.Owner == address && !utxo.IsSpent {
			unspentUTXOs = append(unspentUTXOs, utxo)
			accumulated += utxo.Amount

			if accumulated >= amount {
				break
			}
		}
	}

	return unspentUTXOs, accumulated
}

// GetAllAddressBalances returns a map of all addresses and their balances
func (us *UTXOSet) GetAllAddressBalances() map[string]float64 {
	balances := make(map[string]float64)
	for _, utxo := range us.UTXOs {
		if !utxo.IsSpent {
			balances[utxo.Owner] += utxo.Amount
		}
	}
	return balances
}

// Marshal marshals the UTXO set to JSON
func (us *UTXOSet) Marshal() ([]byte, error) {
	return json.Marshal(us)
}

// Unmarshal unmarshals the UTXO set from JSON
func (us *UTXOSet) Unmarshal(data []byte) error {
	return json.Unmarshal(data, us)
}