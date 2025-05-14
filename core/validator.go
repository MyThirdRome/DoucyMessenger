package core

import (
        "encoding/json"
        "fmt"
        "math"
        "time"
)

// ValidatorConstants defines the constants for validators
const (
        // BaseMinimumDeposit is the base minimum deposit required (50 DOU)
        BaseMinimumDeposit = 50.0
        
        // AnnualDepositIncrease is the annual increase rate for minimum deposit (10%)
        AnnualDepositIncrease = 0.10
        
        // ValidatorYearlyReward is the annual percentage yield for validators (17%)
        ValidatorYearlyReward = 0.17
        
        // MessageRewardMultiplier is the multiplier for message rewards for validators (150%)
        MessageRewardMultiplier = 1.50
)

// Validator represents a validator in the PoS system
type Validator struct {
        Address        string    `json:"address"`
        Deposit        float64   `json:"deposit"`
        StartTime      time.Time `json:"start_time"`
        LastRewardTime time.Time `json:"last_reward_time"`
        IsActive       bool      `json:"is_active"`
        TotalRewards   float64   `json:"total_rewards"`
        MessageCount   int       `json:"message_count"`
}

// NewValidator creates a new validator
func NewValidator(address string, deposit float64, startTime time.Time) *Validator {
        return &Validator{
                Address:        address,
                Deposit:        deposit,
                StartTime:      startTime,
                LastRewardTime: startTime,
                IsActive:       true,
                TotalRewards:   0,
                MessageCount:   0,
        }
}

// IncreaseDeposit increases the validator's deposit
func (v *Validator) IncreaseDeposit(amount float64) {
        v.Deposit += amount
}

// DecreaseDeposit decreases the validator's deposit
func (v *Validator) DecreaseDeposit(amount float64) {
        v.Deposit -= amount
        if v.Deposit < 0 {
                v.Deposit = 0
        }
        
        // Check if validator is still eligible
        if !v.IsEligible(GetMinimumDepositAtTime(time.Now())) {
                v.IsActive = false
        }
}

// GetMinimumDepositAtTime calculates the minimum deposit required at a given time
// This increases by 10% annually from the base of 50 DOU
func GetMinimumDepositAtTime(currentTime time.Time) float64 {
        // Start date (project launch date)
        startDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
        
        // Calculate years since launch (fractional)
        yearsSinceLaunch := currentTime.Sub(startDate).Hours() / (24 * 365.25)
        
        // Calculate compound increase: baseAmount * (1 + rate)^years
        minimumDeposit := BaseMinimumDeposit * math.Pow(1+AnnualDepositIncrease, yearsSinceLaunch)
        
        // Round to 2 decimal places
        return math.Round(minimumDeposit*100) / 100
}

// GetMonthlyReward calculates the monthly reward based on the 17% APY
func (v *Validator) GetMonthlyReward() float64 {
        monthlyRate := ValidatorYearlyReward / 12
        return v.Deposit * monthlyRate
}

// ProcessMonthlyReward processes the monthly APY reward if eligible
func (v *Validator) ProcessMonthlyReward(currentTime time.Time) (float64, error) {
        // Check if a month has passed since last reward
        if currentTime.Sub(v.LastRewardTime).Hours() < 24*30 {
                return 0, fmt.Errorf("not eligible for monthly reward yet")
        }
        
        // Check if validator is active
        if !v.IsActive {
                return 0, fmt.Errorf("validator is not active")
        }
        
        // Calculate reward
        reward := v.GetMonthlyReward()
        v.TotalRewards += reward
        v.LastRewardTime = currentTime
        
        return reward, nil
}

// CalculateMessageReward applies the 150% multiplier to standard message rewards
func (v *Validator) CalculateMessageReward(baseReward float64) float64 {
        return baseReward * MessageRewardMultiplier
}

// IsEligible checks if the validator is eligible based on the minimum deposit
func (v *Validator) IsEligible(minDeposit float64) bool {
        return v.Deposit >= minDeposit && v.IsActive
}

// IncrementMessageCount increments the number of messages processed by this validator
func (v *Validator) IncrementMessageCount() {
        v.MessageCount++
}

// GetMessageCount returns the number of messages processed by this validator
func (v *Validator) GetMessageCount() int {
        return v.MessageCount
}

// MarshalJSON customizes the JSON marshaling for Validator
func (v *Validator) MarshalJSON() ([]byte, error) {
        type ValidatorAlias Validator
        return json.Marshal(&struct {
                StartTime      string `json:"start_time"`
                LastRewardTime string `json:"last_reward_time"`
                *ValidatorAlias
        }{
                StartTime:      v.StartTime.Format(time.RFC3339),
                LastRewardTime: v.LastRewardTime.Format(time.RFC3339),
                ValidatorAlias: (*ValidatorAlias)(v),
        })
}

// UnmarshalJSON customizes the JSON unmarshaling for Validator
func (v *Validator) UnmarshalJSON(data []byte) error {
        type ValidatorAlias Validator
        aux := &struct {
                StartTime      string `json:"start_time"`
                LastRewardTime string `json:"last_reward_time"`
                *ValidatorAlias
        }{
                ValidatorAlias: (*ValidatorAlias)(v),
        }
        
        if err := json.Unmarshal(data, &aux); err != nil {
                return err
        }
        
        startTime, err := time.Parse(time.RFC3339, aux.StartTime)
        if err != nil {
                return err
        }
        v.StartTime = startTime
        
        lastRewardTime, err := time.Parse(time.RFC3339, aux.LastRewardTime)
        if err != nil {
                return err
        }
        v.LastRewardTime = lastRewardTime
        
        return nil
}
