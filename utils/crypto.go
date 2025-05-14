package utils

import (
        "crypto/aes"
        "crypto/cipher"
        "crypto/ecdsa"
        "crypto/elliptic"
        "crypto/rand"
        "crypto/sha256"
        "encoding/base64"
        "encoding/hex"
        "errors"
        "fmt"
        "io"
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

// AESEncryptMessage encrypts a message using AES-GCM
// Returns a base64-encoded encrypted message prefixed with the nonce
func AESEncryptMessage(message string, key []byte) (string, error) {
        // Create a new AES cipher block
        block, err := aes.NewCipher(key)
        if err != nil {
                return "", fmt.Errorf("failed to create AES cipher: %v", err)
        }

        // Create a GCM with the cipher block
        gcm, err := cipher.NewGCM(block)
        if err != nil {
                return "", fmt.Errorf("failed to create GCM: %v", err)
        }

        // Create a nonce
        nonce := make([]byte, gcm.NonceSize())
        if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
                return "", fmt.Errorf("failed to generate nonce: %v", err)
        }

        // Encrypt the message
        ciphertext := gcm.Seal(nonce, nonce, []byte(message), nil)

        // Encode the ciphertext as base64
        return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// AESDecryptMessage decrypts a message using AES-GCM
// Expects a base64-encoded encrypted message prefixed with the nonce
func AESDecryptMessage(encryptedMessage string, key []byte) (string, error) {
        // Decode the base64 ciphertext
        ciphertext, err := base64.StdEncoding.DecodeString(encryptedMessage)
        if err != nil {
                return "", fmt.Errorf("failed to decode base64: %v", err)
        }

        // Create a new AES cipher block
        block, err := aes.NewCipher(key)
        if err != nil {
                return "", fmt.Errorf("failed to create AES cipher: %v", err)
        }

        // Create a GCM with the cipher block
        gcm, err := cipher.NewGCM(block)
        if err != nil {
                return "", fmt.Errorf("failed to create GCM: %v", err)
        }

        // Check if the ciphertext is at least as long as the nonce
        nonceSize := gcm.NonceSize()
        if len(ciphertext) < nonceSize {
                return "", errors.New("ciphertext too short")
        }

        // Extract the nonce and actual ciphertext
        nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

        // Decrypt the message
        plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
        if err != nil {
                return "", fmt.Errorf("failed to decrypt message: %v", err)
        }

        return string(plaintext), nil
}

// DeriveSharedKey derives a shared encryption key from a private key and a recipient's public key
func DeriveSharedKey(privateKey *ecdsa.PrivateKey, publicKey *ecdsa.PublicKey) ([]byte, error) {
        // Compute shared secret
        x, _ := publicKey.Curve.ScalarMult(publicKey.X, publicKey.Y, privateKey.D.Bytes())
        
        // Use the shared secret as a seed for a key
        sharedSecret := x.Bytes()
        
        // Hash the shared secret to create a key of appropriate length for AES
        hash := sha256.Sum256(sharedSecret)
        return hash[:], nil
}

// GenerateEphemeralKey generates a new ephemeral key pair for one-time use
func GenerateEphemeralKey() (*ecdsa.PrivateKey, error) {
        return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

// EncryptWithPublicKey encrypts a message for a specific recipient using their public key
// This uses ECIES (Elliptic Curve Integrated Encryption Scheme)
func EncryptWithPublicKey(message string, recipientPubKey *ecdsa.PublicKey) (string, error) {
        // Generate an ephemeral key pair for this message
        ephemeralKey, err := GenerateEphemeralKey()
        if err != nil {
                return "", fmt.Errorf("failed to generate ephemeral key: %v", err)
        }
        
        // Derive shared key
        sharedKey, err := DeriveSharedKey(ephemeralKey, recipientPubKey)
        if err != nil {
                return "", fmt.Errorf("failed to derive shared key: %v", err)
        }
        
        // Encrypt the message with the shared key
        encryptedMessage, err := AESEncryptMessage(message, sharedKey)
        if err != nil {
                return "", fmt.Errorf("failed to encrypt message: %v", err)
        }
        
        // Convert ephemeral public key to hex for transmission
        ephemeralPublicKeyBytes := elliptic.Marshal(ephemeralKey.Curve, ephemeralKey.X, ephemeralKey.Y)
        ephemeralPublicKeyHex := hex.EncodeToString(ephemeralPublicKeyBytes)
        
        // Return the ephemeral public key and encrypted message, delimited by a colon
        return fmt.Sprintf("%s:%s", ephemeralPublicKeyHex, encryptedMessage), nil
}

// DecryptWithPrivateKey decrypts a message that was encrypted with the recipient's public key
func DecryptWithPrivateKey(encryptedData string, privateKey *ecdsa.PrivateKey) (string, error) {
        // Split the data into ephemeral public key and encrypted message
        parts := split(encryptedData, ":")
        if len(parts) != 2 {
                return "", errors.New("invalid encrypted data format")
        }
        
        ephemeralPublicKeyHex := parts[0]
        encryptedMessage := parts[1]
        
        // Decode the ephemeral public key
        ephemeralPublicKeyBytes, err := hex.DecodeString(ephemeralPublicKeyHex)
        if err != nil {
                return "", fmt.Errorf("failed to decode ephemeral public key: %v", err)
        }
        
        // Unmarshal the ephemeral public key
        x, y := elliptic.Unmarshal(privateKey.Curve, ephemeralPublicKeyBytes)
        if x == nil {
                return "", errors.New("failed to unmarshal ephemeral public key")
        }
        
        ephemeralPublicKey := &ecdsa.PublicKey{
                Curve: privateKey.Curve,
                X:     x,
                Y:     y,
        }
        
        // Derive shared key
        sharedKey, err := DeriveSharedKey(privateKey, ephemeralPublicKey)
        if err != nil {
                return "", fmt.Errorf("failed to derive shared key: %v", err)
        }
        
        // Decrypt the message with the shared key
        return AESDecryptMessage(encryptedMessage, sharedKey)
}

// Helper function to split a string
func split(s, sep string) []string {
        result := []string{}
        for i, j := 0, 0; i < len(s); i = j + len(sep) {
                j = indexOf(s[i:], sep)
                if j < 0 {
                        j = len(s)
                } else {
                        j += i
                }
                result = append(result, s[i:j])
        }
        return result
}

// Helper function to find the index of a substring
func indexOf(s, substr string) int {
        for i := 0; i <= len(s)-len(substr); i++ {
                if s[i:i+len(substr)] == substr {
                        return i
                }
        }
        return -1
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