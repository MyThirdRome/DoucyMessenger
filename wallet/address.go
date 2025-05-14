package wallet

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/doucya/utils"
)

const (
	// AddressPrefix is the prefix for all DoucyA addresses
	AddressPrefix = "Dou"
	
	// AddressSuffix is the suffix for all DoucyA addresses
	AddressSuffix = "cyA"
	
	// AddressMiddleLength is the length of the middle part of addresses
	AddressMiddleLength = 10
	
	// GroupAddressMiddleLength is the length of the middle part of group addresses
	GroupAddressMiddleLength = 6
)

// Regular expressions for address validation
var (
	// Regular expression for standard address format
	// Format: Dou + 10 alphanumeric characters (1-9, a-z) + cyA
	addressRegex = regexp.MustCompile(`^Dou[1-9a-z]{10}cyA$`)
	
	// Regular expression for group/channel address format
	// Format: Dou + 6 alphanumeric characters (1-9, a-z) + cyA
	groupAddressRegex = regexp.MustCompile(`^Dou[1-9a-z]{6}cyA$`)
)

// GenerateAddress generates a DoucyA address from a public key
func GenerateAddress(publicKey *ecdsa.PublicKey) string {
	// Convert public key to bytes
	pubKeyBytes := utils.PublicKeyToBytes(publicKey)
	
	// Hash the public key
	hashBytes := sha256.Sum256(pubKeyBytes)
	hash := hex.EncodeToString(hashBytes[:])
	
	// Take first AddressMiddleLength characters from hash and filter
	middle := filterAddressChars(hash, AddressMiddleLength)
	
	// Construct the address
	address := AddressPrefix + middle + AddressSuffix
	
	return address
}

// GenerateGroupAddress generates a DoucyA address for a group or channel
func GenerateGroupAddress() string {
	// Generate random bytes for the middle part
	randomBytes := make([]byte, 32)
	_, err := utils.RandomReader.Read(randomBytes)
	if err != nil {
		// Fallback to a deterministic but unique string if random fails
		randomBytes = []byte(utils.GenerateRandomString(32))
	}
	
	// Hash the random bytes
	hashBytes := sha256.Sum256(randomBytes)
	hash := hex.EncodeToString(hashBytes[:])
	
	// Take first GroupAddressMiddleLength characters from hash and filter
	middle := filterAddressChars(hash, GroupAddressMiddleLength)
	
	// Construct the address
	address := AddressPrefix + middle + AddressSuffix
	
	return address
}

// ValidateAddress checks if an address is valid
func ValidateAddress(address string) bool {
	// Check standard address format
	if addressRegex.MatchString(address) {
		return true
	}
	
	// Check group/channel address format
	if groupAddressRegex.MatchString(address) {
		return true
	}
	
	return false
}

// IsGroupAddress checks if an address is a group/channel address
func IsGroupAddress(address string) bool {
	return groupAddressRegex.MatchString(address)
}

// filterAddressChars filters characters from a string to create an address middle part
func filterAddressChars(input string, length int) string {
	var builder strings.Builder
	
	// Only allow characters 1-9 and a-z
	allowedChars := "123456789abcdefghijklmnopqrstuvwxyz"
	allowedSet := make(map[rune]bool)
	for _, char := range allowedChars {
		allowedSet[char] = true
	}
	
	// Collect allowed characters from the input
	for _, char := range input {
		if allowedSet[char] && builder.Len() < length {
			builder.WriteRune(char)
		}
	}
	
	result := builder.String()
	
	// If we don't have enough characters, pad with 'a'
	for builder.Len() < length {
		builder.WriteRune('a')
	}
	
	return result
}
