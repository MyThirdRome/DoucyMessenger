package consensus

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"sort"
	"time"

	"github.com/doucya/core"
)

// PoSConsensus implements the Proof of Stake consensus mechanism
type PoSConsensus struct {
	MinDeposit            float64
	AnnualDepositIncrease float64 // 10% increase per year
	ValidatorRewardRate   float64 // 150% of user rewards
	ValidatorAPY          float64 // 17% annual yield
}

// NewPoSConsensus creates a new PoS consensus instance
func NewPoSConsensus(minDeposit, annualIncrease, rewardRate, apy float64) *PoSConsensus {
	return &PoSConsensus{
		MinDeposit:            minDeposit,
		AnnualDepositIncrease: annualIncrease,
		ValidatorRewardRate:   rewardRate,
		ValidatorAPY:          apy,
	}
}

// SelectValidator selects a validator to create the next block
func (pos *PoSConsensus) SelectValidator(validators map[string]*core.Validator, lastBlockHash string) (string, error) {
	if len(validators) == 0 {
		return "", errors.New("no validators available")
	}

	// Filter eligible validators
	eligibleValidators := make([]*core.Validator, 0, len(validators))
	for _, v := range validators {
		if v.IsEligible(pos.MinDeposit) {
			eligibleValidators = append(eligibleValidators, v)
		}
	}

	if len(eligibleValidators) == 0 {
		return "", errors.New("no eligible validators")
	}

	// Sort validators by stake (highest first) to create a deterministic list
	sort.Slice(eligibleValidators, func(i, j int) bool {
		return eligibleValidators[i].Deposit > eligibleValidators[j].Deposit
	})

	// Create a seed from last block hash and current time
	timestamp := time.Now().Unix()
	seed := lastBlockHash + string(timestamp)
	seedHash := sha256.Sum256([]byte(seed))
	seedInt := new(big.Int).SetBytes(seedHash[:])

	// Calculate total stake
	totalStake := 0.0
	for _, v := range eligibleValidators {
		totalStake += v.Deposit
	}

	// Select validator based on stake weight
	target := new(big.Float).SetInt(seedInt)
	target.Mod(target, big.NewFloat(totalStake))
	targetValue, _ := target.Float64()

	currentSum := 0.0
	for _, v := range eligibleValidators {
		currentSum += v.Deposit
		if currentSum >= targetValue {
			return v.Address, nil
		}
	}

	// Fallback to the first validator if something goes wrong
	return eligibleValidators[0].Address, nil
}

// VerifyBlock verifies that a block was created by the correct validator
func (pos *PoSConsensus) VerifyBlock(block *core.Block, validators map[string]*core.Validator) bool {
	// Verify block signature
	if !block.VerifyBlock() {
		return false
	}

	// If it's the genesis block, no need to verify validator
	if block.Height == 0 {
		return true
	}

	// Check if the validator exists
	validator, exists := validators[block.Validator]
	if !exists {
		return false
	}

	// Check if validator was eligible at the time
	if !validator.IsEligible(pos.MinDeposit) {
		return false
	}

	// In a real implementation, we would verify that this validator was the one
	// selected for this block based on the previous block's hash and stake distribution

	return true
}

// UpdateMinDeposit updates the minimum deposit based on time passed
func (pos *PoSConsensus) UpdateMinDeposit(genesisTime time.Time) {
	// Calculate years since genesis
	yearsSinceGenesis := time.Since(genesisTime).Hours() / (24 * 365.25)
	
	// Increase by compound interest formula: P * (1 + r)^t
	// where P is initial deposit, r is annual rate, t is time in years
	increaseRate := 1.0 + pos.AnnualDepositIncrease
	compoundFactor := pow(increaseRate, yearsSinceGenesis)
	
	pos.MinDeposit = 50.0 * compoundFactor // Initial deposit was 50 DOU
}

// CalculateValidatorRewards calculates monthly rewards for a validator
func (pos *PoSConsensus) CalculateValidatorRewards(deposit float64) float64 {
	// Monthly APY is annual rate divided by 12
	monthlyRate := pos.ValidatorAPY / 12
	return deposit * monthlyRate
}

// Helper function to calculate power with floating point exponent
func pow(base, exponent float64) float64 {
	if exponent == 0 {
		return 1.0
	}
	if exponent == 1 {
		return base
	}
	
	// Split into integer and fractional parts
	intPart := int(exponent)
	fracPart := exponent - float64(intPart)
	
	// Calculate integer power
	result := 1.0
	for i := 0; i < intPart; i++ {
		result *= base
	}
	
	// Approximate fractional power if needed
	if fracPart > 0 {
		// Using simplified approximation
		fracResult := 1.0 + fracPart*(base-1)
		result *= fracResult
	}
	
	return result
}

// HashToHex converts a hash to a hex string
func HashToHex(hash [32]byte) string {
	return hex.EncodeToString(hash[:])
}
