package models

import (
        "encoding/json"
        "time"
)

// Validator represents a validator in the PoS system
type Validator struct {
        Address       string    `json:"address"`
        Deposit       float64   `json:"deposit"`
        StartTime     time.Time `json:"start_time"`
        LastRewardTime time.Time `json:"last_reward_time"`
        IsActive      bool      `json:"is_active"`
}

// NewValidator creates a new validator
func NewValidator(address string, deposit float64, startTime time.Time) *Validator {
        return &Validator{
                Address:       address,
                Deposit:       deposit,
                StartTime:     startTime,
                LastRewardTime: startTime,
                IsActive:      true,
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
}

// GetMonthlyReward calculates the monthly reward based on the APY
func (v *Validator) GetMonthlyReward(apy float64) float64 {
        monthlyRate := apy / 12
        return v.Deposit * monthlyRate
}

// IsEligible checks if the validator is eligible to validate based on the minimum deposit
func (v *Validator) IsEligible(minDeposit float64) bool {
        return v.Deposit >= minDeposit && v.IsActive
}

// GetAddress returns the validator's address
func (v *Validator) GetAddress() string {
        return v.Address
}

// GetStake returns the validator's deposit (stake)
func (v *Validator) GetStake() float64 {
        return v.Deposit
}

// GetCreationTime returns the validator's start time
func (v *Validator) GetCreationTime() time.Time {
        return v.StartTime
}

// GetLastRewardTime returns the validator's last reward time
func (v *Validator) GetLastRewardTime() time.Time {
        return v.LastRewardTime
}

// MarshalJSON customizes the JSON marshaling for Validator
func (v *Validator) MarshalJSON() ([]byte, error) {
        type ValidatorAlias Validator
        return json.Marshal(&struct {
                StartTime     string `json:"start_time"`
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
                StartTime     string `json:"start_time"`
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