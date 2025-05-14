package config

import (
        "bufio"
        "encoding/json"
        "io/ioutil"
        "os"
        "strconv"
        "strings"
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

// LoadNodesFromFile loads a list of bootstrap nodes from a text file
func LoadNodesFromFile(filePath string) []string {
        // Check if file exists
        if _, err := os.Stat(filePath); os.IsNotExist(err) {
                // If file doesn't exist, return an empty list
                return []string{}
        }
        
        // Open and read the file
        file, err := os.Open(filePath)
        if err != nil {
                return []string{}
        }
        defer file.Close()
        
        var nodes []string
        scanner := bufio.NewScanner(file)
        
        // Read each line
        for scanner.Scan() {
                line := strings.TrimSpace(scanner.Text())
                
                // Skip empty lines and comments
                if line == "" || strings.HasPrefix(line, "#") {
                        continue
                }
                
                // Add the node address to the list
                nodes = append(nodes, line)
        }
        
        return nodes
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
        // Try to load bootstrap nodes from nodes.txt
        nodes := LoadNodesFromFile("nodes.txt")
        
        // If no nodes were loaded, use the default main node
        if len(nodes) == 0 {
                nodes = []string{"185.251.25.31:8333"} // Main network bootstrap node
        }
        
        return &Config{
                ListenAddr:            "0.0.0.0:8000",
                BootstrapNodes:        nodes,
                DBPath:                "./doucya_data",
                GenesisReward:         15000.0,
                ValidatorMinDeposit:   50.0,
                ValidatorAPY:          0.17,    // 17% annual yield
                MessageRateLimitTotal: 200,     // 200 messages per hour
                MessageRateLimitAddr:  30,      // 30 messages to a single address per hour
                SenderReward:          0.75,    // 0.75 DOU reward for sending
                ReceiverReward:        0.25,    // 0.25 DOU reward for receiving
                ValidatorRewardMult:   1.5,     // 150% of user rewards
                LogLevel:              "info",
        }
}

// NewConfig creates a new configuration with the specified settings
func NewConfig(p2pPort int, dbPath string) *Config {
        config := DefaultConfig()
        config.ListenAddr = "0.0.0.0:" + strconv.Itoa(p2pPort)
        config.DBPath = dbPath
        
        // Try to load nodes from nodes.txt (will already be loaded by DefaultConfig,
        // but we reload here in case the file was modified after the DefaultConfig call)
        nodes := LoadNodesFromFile("nodes.txt")
        if len(nodes) > 0 {
                config.BootstrapNodes = nodes
        }
        
        return config
}
