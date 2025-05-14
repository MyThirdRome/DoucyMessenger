package p2p

import (
        "encoding/json"
        "time"
)

// MessageType represents the type of a P2P message
type MessageType string

const (
        MessageTypeNodeInfo        MessageType = "NODE_INFO"
        MessageTypePeerList        MessageType = "PEER_LIST"
        MessageTypeBlock           MessageType = "BLOCK"
        MessageTypeTransaction     MessageType = "TRANSACTION"
        MessageTypeGetBlocks       MessageType = "GET_BLOCKS"
        MessageTypeGetPeers        MessageType = "GET_PEERS"
        MessageTypeMessageReward   MessageType = "MESSAGE_REWARD"
        MessageTypeValidatorReward MessageType = "VALIDATOR_REWARD"
        MessageTypeSystemMessage   MessageType = "SYSTEM_MESSAGE"
)

// Message represents a P2P message
type Message struct {
        Type MessageType  `json:"type"`
        Data interface{} `json:"data"`
}

// NodeInfo represents node information shared during handshake
type NodeInfo struct {
        Version      string    `json:"version"`
        BlockHeight  int64     `json:"block_height"`
        PeerCount    int       `json:"peer_count"`
        ListenAddr   string    `json:"listen_addr"`
        Timestamp    time.Time `json:"timestamp"`
}

// MarshalJSON customizes the JSON marshaling for NodeInfo
func (ni *NodeInfo) MarshalJSON() ([]byte, error) {
        type NodeInfoAlias NodeInfo
        return json.Marshal(&struct {
                Timestamp string `json:"timestamp"`
                *NodeInfoAlias
        }{
                Timestamp:     time.Now().Format(time.RFC3339),
                NodeInfoAlias: (*NodeInfoAlias)(ni),
        })
}

// UnmarshalJSON customizes the JSON unmarshaling for NodeInfo
func (ni *NodeInfo) UnmarshalJSON(data []byte) error {
        type NodeInfoAlias NodeInfo
        aux := &struct {
                Timestamp string `json:"timestamp"`
                *NodeInfoAlias
        }{
                NodeInfoAlias: (*NodeInfoAlias)(ni),
        }
        
        if err := json.Unmarshal(data, &aux); err != nil {
                return err
        }
        
        timestamp, err := time.Parse(time.RFC3339, aux.Timestamp)
        if err != nil {
                timestamp = time.Now() // Default to current time on error
        }
        ni.Timestamp = timestamp
        
        return nil
}
