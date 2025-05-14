package models

import (
        "encoding/json"
        "time"
)

// ValidatorStatus represents the status of a validator
type ValidatorStatus string

const (
        ValidatorStatusActive     ValidatorStatus = "ACTIVE"
        ValidatorStatusInactive   ValidatorStatus = "INACTIVE"
        ValidatorStatusPenalized  ValidatorStatus = "PENALIZED"
        ValidatorStatusExited     ValidatorStatus = "EXITED"
)

// RewardHistory represents a validator reward entry
type RewardHistory struct {
        Amount    float64   `json:"amount"`
        Timestamp time.Time `json:"timestamp"`
        Type      string    `json:"type"` // "MONTHLY", "MESSAGE", etc.
}

// Validator represents a validator in the PoS system
type Validator struct {
        Address         string           `json:"address"`
        Deposit         float64          `json:"deposit"`
        StartTime       time.Time        `json:"start_time"`
        LastRewardTime  time.Time        `json:"last_reward_time"`
        Status          ValidatorStatus  `json:"status"`
        RewardHistories []RewardHistory  `json:"reward_histories"`
        MessageCount    int              `json:"message_count"`
        TotalEarnings   float64          `json:"total_earnings"`
        YearlyAPY       float64          `json:"yearly_apy"`
        MinimumStake    float64          `json:"minimum_stake"`
}

// GetDeposit returns the deposit amount of the validator
func (v *Validator) GetDeposit() float64 {
        return v.Deposit
}

// GetStartTime returns the start time of the validator
func (v *Validator) GetStartTime() time.Time {
        return v.StartTime
}

// GetTotalRewards returns the total rewards of the validator
func (v *Validator) GetTotalRewards() float64 {
        return v.TotalEarnings
}

// NewValidator creates a new validator
func NewValidator(address string, deposit float64, startTime time.Time) *Validator {
        return &Validator{
                Address:         address,
                Deposit:         deposit,
                StartTime:       startTime,
                LastRewardTime:  startTime,
                Status:          ValidatorStatusActive,
                RewardHistories: []RewardHistory{},
                MessageCount:    0,
                TotalEarnings:   0,
                YearlyAPY:       0.17, // 17% APY
                MinimumStake:    50.0, // 50 DOU minimum
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
        
        // Check if validator is still eligible after withdrawal
        if v.Deposit < v.MinimumStake {
                v.Status = ValidatorStatusInactive
        }
}

// UpdateLastRewardTime updates the validator's last reward time
func (v *Validator) UpdateLastRewardTime(rewardTime time.Time) {
        v.LastRewardTime = rewardTime
}

// AddRewardHistory adds a new reward to the validator's history
func (v *Validator) AddRewardHistory(amount float64, rewardType string) {
        reward := RewardHistory{
                Amount:    amount,
                Timestamp: time.Now(),
                Type:      rewardType,
        }
        
        v.RewardHistories = append(v.RewardHistories, reward)
        v.TotalEarnings += amount
        v.UpdateLastRewardTime(reward.Timestamp)
}

// IncrementMessageCount increments the number of messages processed by this validator
func (v *Validator) IncrementMessageCount() {
        v.MessageCount++
}

// GetMonthlyReward calculates the monthly reward based on the APY
func (v *Validator) GetMonthlyReward() float64 {
        monthlyRate := v.YearlyAPY / 12
        return v.Deposit * monthlyRate
}

// GetDailyReward calculates the daily reward based on the APY
func (v *Validator) GetDailyReward() float64 {
        dailyRate := v.YearlyAPY / 365
        return v.Deposit * dailyRate
}

// IsEligible checks if the validator is eligible to validate
func (v *Validator) IsEligible() bool {
        return v.Deposit >= v.MinimumStake && v.Status == ValidatorStatusActive
}

// GetAnnualizedReturn calculates the annualized return rate
func (v *Validator) GetAnnualizedReturn() float64 {
        if v.TotalEarnings == 0 {
                return 0
        }
        
        daysActive := time.Since(v.StartTime).Hours() / 24
        if daysActive < 1 {
                return 0
        }
        
        // Calculate annualized return
        dailyReturn := v.TotalEarnings / daysActive
        annualReturn := dailyReturn * 365
        
        return annualReturn / v.Deposit
}

// GetMessageRewardMultiplier returns the multiplier for message rewards
func (v *Validator) GetMessageRewardMultiplier() float64 {
        // Validators get 150% of user rewards
        return 1.5
}

// PenalizeValidator applies a penalty to the validator
func (v *Validator) PenalizeValidator(penaltyPercentage float64) float64 {
        penaltyAmount := v.Deposit * penaltyPercentage
        v.DecreaseDeposit(penaltyAmount)
        v.Status = ValidatorStatusPenalized
        
        return penaltyAmount
}

// RestoreValidator restores a penalized validator to active status
func (v *Validator) RestoreValidator() {
        if v.Status == ValidatorStatusPenalized && v.Deposit >= v.MinimumStake {
                v.Status = ValidatorStatusActive
        }
}

// ExitValidator makes a validator exit the network
func (v *Validator) ExitValidator() float64 {
        withdrawalAmount := v.Deposit
        v.Deposit = 0
        v.Status = ValidatorStatusExited
        
        return withdrawalAmount
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

// GetValidatorStatus returns the validator's status (original method)
func (v *Validator) GetValidatorStatus() ValidatorStatus {
        return v.Status
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