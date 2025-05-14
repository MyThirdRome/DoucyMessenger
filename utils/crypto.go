package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
)

// SignData signs data with a private key
func SignData(privateKey *ecdsa.PrivateKey, data []byte) ([]byte, error) {
	if privateKey == nil {
		return nil, errors.New("nil private key")
	}

	// Create a hash of the data
	hash := sha256.Sum256(data)

	// Sign the hash with the private key
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, hash[:])
	if err != nil {
		return nil, fmt.Errorf("error signing data: %v", err)
	}

	// Serialize the signature
	signature := append(r.Bytes(), s.Bytes()...)
	return signature, nil
}

// VerifySignature verifies a signature
func VerifySignature(publicKey *ecdsa.PublicKey, data, signature []byte) bool {
	if publicKey == nil {
		return false
	}

	// Create a hash of the data
	hash := sha256.Sum256(data)

	// Signature should be the concatenation of two big ints
	sigLen := len(signature)
	if sigLen%2 != 0 {
		return false
	}

	// Split the signature into r and s
	r := new(big.Int).SetBytes(signature[:sigLen/2])
	s := new(big.Int).SetBytes(signature[sigLen/2:])

	// Verify the signature
	return ecdsa.Verify(publicKey, hash[:], r, s)
}

// PublicKeyToBytes converts a public key to bytes
func PublicKeyToBytes(publicKey *ecdsa.PublicKey) []byte {
	if publicKey == nil {
		return nil
	}
	return elliptic.Marshal(publicKey.Curve, publicKey.X, publicKey.Y)
}

// PublicKeyFromBytes creates a public key from bytes
func PublicKeyFromBytes(data []byte) (*ecdsa.PublicKey, error) {
	if len(data) == 0 {
		return nil, errors.New("empty public key data")
	}

	curve := elliptic.P256()
	x, y := elliptic.Unmarshal(curve, data)
	if x == nil {
		return nil, errors.New("invalid public key data")
	}

	return &ecdsa.PublicKey{
		Curve: curve,
		X:     x,
		Y:     y,
	}, nil
}

// PrivateKeyToBytes converts a private key to bytes
func PrivateKeyToBytes(privateKey *ecdsa.PrivateKey) []byte {
	if privateKey == nil {
		return nil
	}
	return privateKey.D.Bytes()
}

// PrivateKeyFromBytes creates a private key from bytes
func PrivateKeyFromBytes(data []byte) (*ecdsa.PrivateKey, error) {
	if len(data) == 0 {
		return nil, errors.New("empty private key data")
	}

	curve := elliptic.P256()
	privateKey := new(ecdsa.PrivateKey)
	privateKey.D = new(big.Int).SetBytes(data)
	privateKey.PublicKey.Curve = curve

	// Calculate public key
	privateKey.PublicKey.X, privateKey.PublicKey.Y = curve.ScalarBaseMult(data)

	return privateKey, nil
}

// PrivateKeyFromHex creates a private key from a hex string
func PrivateKeyFromHex(hexString string) (*ecdsa.PrivateKey, error) {
	data, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string: %v", err)
	}
	return PrivateKeyFromBytes(data)
}

// PublicKeyFromHex creates a public key from a hex string
func PublicKeyFromHex(hexString string) (*ecdsa.PublicKey, error) {
	data, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string: %v", err)
	}
	return PublicKeyFromBytes(data)
}

// GenerateKeyPair generates a new ECDSA key pair
func GenerateKeyPair() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating key pair: %v", err)
	}
	return privateKey, &privateKey.PublicKey, nil
}

// HashToHex converts a hash to a hex string
func HashToHex(hash [32]byte) string {
	return hex.EncodeToString(hash[:])
}

// HashData creates a SHA-256 hash of data
func HashData(data []byte) [32]byte {
	return sha256.Sum256(data)
}
