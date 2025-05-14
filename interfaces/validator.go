package interfaces

import (
        "time"
)

// ValidatorStatus represents validator status strings
type ValidatorStatus string

// ValidatorInterface defines the interface for validator operations
type ValidatorInterface interface {
        GetAddress() string
        GetDeposit() float64
        GetStartTime() time.Time
        GetLastRewardTime() time.Time
        GetStatus() ValidatorStatus
        GetTotalRewards() float64
}