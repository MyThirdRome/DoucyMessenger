package utils

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"math/big"
)

// RandomReader provides a reader for cryptographically secure random numbers
var RandomReader io.Reader = rand.Reader

// GenerateRandomBytes generates random bytes of the specified length
func GenerateRandomBytes(n int) ([]byte, error) {
	bytes := make([]byte, n)
	_, err := RandomReader.Read(bytes)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

// GenerateRandomString generates a random hex string of the specified length
func GenerateRandomString(length int) string {
	bytes, err := GenerateRandomBytes(length / 2)
	if err != nil {
		// Fallback to a less secure method if crypto/rand fails
		return fallbackRandomString(length)
	}
	return hex.EncodeToString(bytes)
}

// GenerateRandomID generates a random ID
func GenerateRandomID() string {
	return GenerateRandomString(32)
}

// fallbackRandomString is a fallback for when crypto/rand fails
// This is less secure but ensures the application doesn't crash
func fallbackRandomString(length int) string {
	const chars = "0123456789abcdef"
	result := make([]byte, length)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result[i] = chars[num.Int64()]
	}
	return string(result)
}

// GenerateRandomInt generates a random integer between min and max
func GenerateRandomInt(min, max int64) (int64, error) {
	// Calculate range size
	delta := max - min
	if delta <= 0 {
		return min, nil
	}

	// Generate random number in range
	n, err := rand.Int(rand.Reader, big.NewInt(delta))
	if err != nil {
		return 0, err
	}

	return min + n.Int64(), nil
}

// ShuffleSlice shuffles a slice using Fisher-Yates algorithm
func ShuffleSlice(slice []string) []string {
	// Make a copy to avoid modifying the original
	result := make([]string, len(slice))
	copy(result, slice)

	for i := len(result) - 1; i > 0; i-- {
		// Generate random index
		j, err := GenerateRandomInt(0, int64(i+1))
		if err != nil {
			continue
		}

		// Swap elements
		result[i], result[j] = result[j], result[i]
	}

	return result
}
