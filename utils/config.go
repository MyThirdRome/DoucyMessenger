package utils

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

// Config holds configuration parameters for the application
type Config struct {
	// Network settings
	ListenAddr     string   `json:"listen_addr"`
	BootstrapNodes []string `json:"bootstrap_nodes"`
	
	// Storage settings
	DBPath string `json:"db_path"`
	
	// Blockchain settings
	GenesisReward         float64 `json:"genesis_reward"`
	ValidatorMinDeposit   float64 `json:"validator_min_deposit"`
	ValidatorAPY          float64 `json:"validator_apy"`
	MessageRateLimitTotal int     `json:"message_rate_limit_total"`
	MessageRateLimitAddr  int     `json:"message_rate_limit_addr"`
	
	// Reward settings
	SenderReward         float64 `json:"sender_reward"`
	ReceiverReward       float64 `json:"receiver_reward"`
	ValidatorRewardMult  float64 `json:"validator_reward_mult"`
	
	// Application settings
	LogLevel string `json:"log_level"`
}

// LoadConfig loads the configuration from a JSON file
func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var config Config
	if err = json.Unmarshal(bytes, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveConfig saves the configuration to a JSON file
func SaveConfig(config *Config, path string) error {
	bytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, bytes, 0644)
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		ListenAddr:            DefaultHost + ":" + DefaultPort,
		BootstrapNodes:        []string{},
		DBPath:                DefaultDBPath,
		GenesisReward:         GenesisReward,
		ValidatorMinDeposit:   MinValidatorDeposit,
		ValidatorAPY:          ValidatorAPY,
		MessageRateLimitTotal: MessageRateLimitTotal,
		MessageRateLimitAddr:  MessageRateLimitAddr,
		SenderReward:          SenderReward,
		ReceiverReward:        ReceiverReward,
		ValidatorRewardMult:   ValidatorRewardMultiplier,
		LogLevel:              "info",
	}
}
