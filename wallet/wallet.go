package wallet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/doucya/utils"
)

// Wallet represents a wallet in the blockchain
type Wallet struct {
	PrivateKey *ecdsa.PrivateKey `json:"-"` // Not serialized directly
	PublicKey  *ecdsa.PublicKey  `json:"-"` // Not serialized directly
	PrivKeyStr string            `json:"private_key"`
	PubKeyStr  string            `json:"public_key"`
	Address    string            `json:"address"`
}

// NewWallet creates a new wallet
func NewWallet() *Wallet {
	// Generate a new ECDSA key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil
	}

	// Extract public key
	publicKey := &privateKey.PublicKey

	// Create wallet
	wallet := &Wallet{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}

	// Generate address
	wallet.Address = GenerateAddress(publicKey)

	// Store string representations
	wallet.PrivKeyStr = wallet.GetPrivateKeyString()
	wallet.PubKeyStr = wallet.GetPublicKeyString()

	return wallet
}

// LoadWalletFromPrivateKey loads a wallet from a private key string
func LoadWalletFromPrivateKey(privateKeyStr string) (*Wallet, error) {
	// Decode private key
	privateKeyBytes, err := hex.DecodeString(privateKeyStr)
	if err != nil {
		return nil, fmt.Errorf("invalid private key format: %v", err)
	}

	// Convert bytes to ECDSA private key
	curve := elliptic.P256()
	privateKey := new(ecdsa.PrivateKey)
	privateKey.D = new(big.Int).SetBytes(privateKeyBytes)
	privateKey.PublicKey.Curve = curve

	// Calculate public key point
	privateKey.PublicKey.X, privateKey.PublicKey.Y = curve.ScalarBaseMult(privateKeyBytes)

	// Create wallet
	wallet := &Wallet{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
	}

	// Generate address
	wallet.Address = GenerateAddress(&privateKey.PublicKey)

	// Store string representations
	wallet.PrivKeyStr = privateKeyStr
	wallet.PubKeyStr = wallet.GetPublicKeyString()

	return wallet, nil
}

// GetPrivateKeyString returns the private key as a hex string
func (w *Wallet) GetPrivateKeyString() string {
	if w.PrivKeyStr != "" {
		return w.PrivKeyStr
	}
	return hex.EncodeToString(w.PrivateKey.D.Bytes())
}

// GetPublicKeyString returns the public key as a hex string
func (w *Wallet) GetPublicKeyString() string {
	if w.PubKeyStr != "" {
		return w.PubKeyStr
	}
	pubKeyBytes := elliptic.Marshal(w.PublicKey.Curve, w.PublicKey.X, w.PublicKey.Y)
	return hex.EncodeToString(pubKeyBytes)
}

// GetAddress returns the wallet's address
func (w *Wallet) GetAddress() string {
	return w.Address
}

// GetPrivateKey returns the wallet's private key
func (w *Wallet) GetPrivateKey() *ecdsa.PrivateKey {
	return w.PrivateKey
}

// Sign signs data with the wallet's private key
func (w *Wallet) Sign(data []byte) ([]byte, error) {
	return utils.SignData(w.PrivateKey, data)
}

// Verify verifies a signature with the wallet's public key
func (w *Wallet) Verify(data, signature []byte) bool {
	return utils.VerifySignature(w.PublicKey, data, signature)
}

// MarshalJSON implements custom JSON marshaling for Wallet
func (w *Wallet) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		PrivateKey string `json:"private_key"`
		PublicKey  string `json:"public_key"`
		Address    string `json:"address"`
	}{
		PrivateKey: w.GetPrivateKeyString(),
		PublicKey:  w.GetPublicKeyString(),
		Address:    w.Address,
	})
}

// UnmarshalJSON implements custom JSON unmarshaling for Wallet
func (w *Wallet) UnmarshalJSON(data []byte) error {
	aux := struct {
		PrivateKey string `json:"private_key"`
		PublicKey  string `json:"public_key"`
		Address    string `json:"address"`
	}{}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	w.PrivKeyStr = aux.PrivateKey
	w.PubKeyStr = aux.PublicKey
	w.Address = aux.Address

	// Reconstruct private and public keys from strings
	if len(aux.PrivateKey) > 0 {
		wallet, err := LoadWalletFromPrivateKey(aux.PrivateKey)
		if err != nil {
			return err
		}
		w.PrivateKey = wallet.PrivateKey
		w.PublicKey = wallet.PublicKey
	}

	return nil
}

// GetMessageSignatureKey returns a key for signature in messages
func (w *Wallet) GetMessageSignatureKey() string {
	// Create a hash of the address
	hash := sha256.Sum256([]byte(w.Address))
	return hex.EncodeToString(hash[:])
}
