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

// EncryptMessage encrypts a message with a public key
func EncryptMessage(publicKey *ecdsa.PublicKey, message string) (string, error) {
        if publicKey == nil {
                return "", errors.New("nil public key")
        }

        // Generate random AES key
        aesKey := make([]byte, 32)
        if _, err := io.ReadFull(rand.Reader, aesKey); err != nil {
                return "", fmt.Errorf("error generating AES key: %v", err)
        }

        // Encrypt message with AES key
        block, err := aes.NewCipher(aesKey)
        if err != nil {
                return "", fmt.Errorf("error creating AES cipher: %v", err)
        }

        // Create a new GCM mode
        aesGCM, err := cipher.NewGCM(block)
        if err != nil {
                return "", fmt.Errorf("error creating GCM: %v", err)
        }

        // Create a nonce
        nonce := make([]byte, aesGCM.NonceSize())
        if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
                return "", fmt.Errorf("error generating nonce: %v", err)
        }

        // Encrypt message
        ciphertext := aesGCM.Seal(nonce, nonce, []byte(message), nil)

        // Encrypt AES key with public key
        // In a real implementation, we would use ECDH or similar to encrypt the AES key
        // For simplicity, we'll use a shared secret derived from the public key
        sharedSecret := sha256.Sum256(PublicKeyToBytes(publicKey))
        keyBlock, err := aes.NewCipher(sharedSecret[:])
        if err != nil {
                return "", fmt.Errorf("error creating key cipher: %v", err)
        }

        keyNonce := make([]byte, 12) // GCM nonce size
        if _, err := io.ReadFull(rand.Reader, keyNonce); err != nil {
                return "", fmt.Errorf("error generating key nonce: %v", err)
        }

        keyGCM, err := cipher.NewGCM(keyBlock)
        if err != nil {
                return "", fmt.Errorf("error creating key GCM: %v", err)
        }

        encryptedKey := keyGCM.Seal(keyNonce, keyNonce, aesKey, nil)

        // Combine encrypted key and ciphertext
        combined := append(encryptedKey, ciphertext...)

        // Base64 encode the result
        return base64.StdEncoding.EncodeToString(combined), nil
}

// DecryptMessage decrypts a message with a private key
func DecryptMessage(privateKey *ecdsa.PrivateKey, encryptedMessage string) (string, error) {
        if privateKey == nil {
                return "", errors.New("nil private key")
        }

        // Decode the base64 message
        combined, err := base64.StdEncoding.DecodeString(encryptedMessage)
        if err != nil {
                return "", fmt.Errorf("error decoding base64: %v", err)
        }

        // Split the combined data
        // In a real implementation, we would store the sizes to properly split
        // For simplicity, we'll use fixed sizes
        keySize := 12 + 32 + 16 // keyNonce (12) + AES key (32) + GCM tag (16)
        if len(combined) < keySize {
                return "", errors.New("encrypted message too short")
        }

        encryptedKey := combined[:keySize]
        ciphertext := combined[keySize:]

        // Decrypt the AES key
        // In a real implementation, we would use ECDH or similar
        // For simplicity, we'll use a shared secret derived from the public key
        sharedSecret := sha256.Sum256(PublicKeyToBytes(&privateKey.PublicKey))
        keyBlock, err := aes.NewCipher(sharedSecret[:])
        if err != nil {
                return "", fmt.Errorf("error creating key cipher: %v", err)
        }

        keyNonce := encryptedKey[:12]
        keyGCM, err := cipher.NewGCM(keyBlock)
        if err != nil {
                return "", fmt.Errorf("error creating key GCM: %v", err)
        }

        aesKey, err := keyGCM.Open(nil, keyNonce, encryptedKey[12:], nil)
        if err != nil {
                return "", fmt.Errorf("error decrypting AES key: %v", err)
        }

        // Use the AES key to decrypt the message
        block, err := aes.NewCipher(aesKey)
        if err != nil {
                return "", fmt.Errorf("error creating AES cipher: %v", err)
        }

        aesGCM, err := cipher.NewGCM(block)
        if err != nil {
                return "", fmt.Errorf("error creating GCM: %v", err)
        }

        nonceSize := aesGCM.NonceSize()
        if len(ciphertext) < nonceSize {
                return "", errors.New("ciphertext too short")
        }

        nonce := ciphertext[:nonceSize]
        plaintext, err := aesGCM.Open(nil, nonce, ciphertext[nonceSize:], nil)
        if err != nil {
                return "", fmt.Errorf("error decrypting message: %v", err)
        }

        return string(plaintext), nil
}

// SimpleEncrypt provides a simplified encryption using a password
// This is less secure than the public key encryption but easier to use
func SimpleEncrypt(message, password string) (string, error) {
        // Create a key from the password
        key := sha256.Sum256([]byte(password))

        // Create AES cipher
        block, err := aes.NewCipher(key[:])
        if err != nil {
                return "", fmt.Errorf("error creating cipher: %v", err)
        }

        // Create GCM mode
        gcm, err := cipher.NewGCM(block)
        if err != nil {
                return "", fmt.Errorf("error creating GCM: %v", err)
        }

        // Create nonce
        nonce := make([]byte, gcm.NonceSize())
        if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
                return "", fmt.Errorf("error creating nonce: %v", err)
        }

        // Encrypt message
        ciphertext := gcm.Seal(nonce, nonce, []byte(message), nil)

        // Encode to base64
        return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// SimpleDecrypt provides a simplified decryption using a password
func SimpleDecrypt(encryptedMessage, password string) (string, error) {
        // Decode base64
        ciphertext, err := base64.StdEncoding.DecodeString(encryptedMessage)
        if err != nil {
                return "", fmt.Errorf("error decoding base64: %v", err)
        }

        // Create key from password
        key := sha256.Sum256([]byte(password))

        // Create AES cipher
        block, err := aes.NewCipher(key[:])
        if err != nil {
                return "", fmt.Errorf("error creating cipher: %v", err)
        }

        // Create GCM mode
        gcm, err := cipher.NewGCM(block)
        if err != nil {
                return "", fmt.Errorf("error creating GCM: %v", err)
        }

        // Get nonce size
        nonceSize := gcm.NonceSize()
        if len(ciphertext) < nonceSize {
                return "", errors.New("ciphertext too short")
        }

        // Split nonce and ciphertext
        nonce := ciphertext[:nonceSize]
        ciphertext = ciphertext[nonceSize:]

        // Decrypt
        plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
        if err != nil {
                return "", fmt.Errorf("error decrypting: %v", err)
        }

        return string(plaintext), nil
}
