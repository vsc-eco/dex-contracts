package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// IdentityConfig matches go-vsc-node's identityConfig.json format.
type IdentityConfig struct {
	BlsPrivKeySeed string `json:"BlsPrivKeySeed"`
	HiveActiveKey  string `json:"HiveActiveKey"`
	HiveUsername   string `json:"HiveUsername"`
	Libp2pPrivKey  string `json:"Libp2pPrivKey"`
}

// HiveConfig matches go-vsc-node's hiveConfig.json format.
type HiveConfig struct {
	HiveURIs []string `json:"HiveURIs,omitempty"`
	HiveURI  string   `json:"HiveURI,omitempty"`
}

// ServiceConfig holds the GraphQL endpoint and VSC network parameters.
type ServiceConfig struct {
	GraphQLURL          string `json:"graphql_url"`
	VscNetID            string `json:"vsc_net_id"`
	HiveChainID         string `json:"hive_chain_id"`
	PollIntervalSeconds int    `json:"poll_interval_seconds"`
	PollTimeoutSeconds  int    `json:"poll_timeout_seconds"`
}

// PoolConfig describes a single pool to initialize.
type PoolConfig struct {
	ContractID           string `json:"contract_id"`
	Asset0               string `json:"asset0"`
	Asset1               string `json:"asset1"`
	FeeBps               uint64 `json:"fee_bps"`
	Asset0MappingContract string `json:"asset0_mapping_contract,omitempty"`
	Asset1MappingContract string `json:"asset1_mapping_contract,omitempty"`
	Asset0Chain          string `json:"asset0_chain"`
	Asset1Chain          string `json:"asset1_chain"`
}

// PoolsConfig is the top-level input specifying the router and all pools.
type PoolsConfig struct {
	RouterContractID string       `json:"router_contract_id"`
	Pools            []PoolConfig `json:"pools"`
}

// Contract payload types matching the on-chain structs.

type InitPoolPayload struct {
	Asset0                string `json:"asset0"`
	Asset1                string `json:"asset1"`
	FeeBps                uint64 `json:"fee_bps"`
	Asset0MappingContract string `json:"asset0_mapping_contract,omitempty"`
	Asset1MappingContract string `json:"asset1_mapping_contract,omitempty"`
	RouterContract        string `json:"router_contract,omitempty"`
}

type RegisterTokenPayload struct {
	Name            string `json:"name"`
	MappingContract string `json:"mapping_contract,omitempty"`
	Chain           string `json:"chain"`
	Description     string `json:"description,omitempty"`
}

type RegisterPoolPayload struct {
	Asset0        string `json:"asset0"`
	Asset1        string `json:"asset1"`
	DexContractId string `json:"dex_contract_id"`
}

func loadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	return nil
}

func loadIdentityConfig(path string) (*IdentityConfig, error) {
	var c IdentityConfig
	if err := loadJSON(path, &c); err != nil {
		return nil, err
	}
	if c.HiveActiveKey == "" || c.HiveActiveKey == "ADD_YOUR_PRIVATE_WIF" {
		return nil, fmt.Errorf("HiveActiveKey not set in %s", path)
	}
	if c.HiveUsername == "" || c.HiveUsername == "ADD_YOUR_USERNAME" {
		return nil, fmt.Errorf("HiveUsername not set in %s", path)
	}
	return &c, nil
}

func loadHiveConfig(path string) (*HiveConfig, error) {
	var c HiveConfig
	if err := loadJSON(path, &c); err != nil {
		return nil, err
	}
	// Migrate old single URI to array
	if c.HiveURI != "" && len(c.HiveURIs) == 0 {
		c.HiveURIs = []string{c.HiveURI}
	}
	if len(c.HiveURIs) == 0 {
		return nil, fmt.Errorf("no Hive URIs configured in %s", path)
	}
	return &c, nil
}

func loadServiceConfig(path string) (*ServiceConfig, error) {
	var c ServiceConfig
	if err := loadJSON(path, &c); err != nil {
		return nil, err
	}
	if c.GraphQLURL == "" {
		return nil, fmt.Errorf("graphql_url not set in %s", path)
	}
	if c.VscNetID == "" {
		return nil, fmt.Errorf("vsc_net_id not set in %s", path)
	}
	if c.HiveChainID == "" {
		return nil, fmt.Errorf("hive_chain_id not set in %s", path)
	}
	if c.PollIntervalSeconds <= 0 {
		c.PollIntervalSeconds = 5
	}
	if c.PollTimeoutSeconds <= 0 {
		c.PollTimeoutSeconds = 300
	}
	return &c, nil
}

func loadPoolsConfig(path string) (*PoolsConfig, error) {
	var c PoolsConfig
	if err := loadJSON(path, &c); err != nil {
		return nil, err
	}
	if c.RouterContractID == "" {
		return nil, fmt.Errorf("router_contract_id not set in %s", path)
	}
	if len(c.Pools) == 0 {
		return nil, fmt.Errorf("no pools configured in %s", path)
	}
	for i, p := range c.Pools {
		if p.ContractID == "" {
			return nil, fmt.Errorf("pool[%d]: contract_id required", i)
		}
		if p.Asset0 == "" || p.Asset1 == "" {
			return nil, fmt.Errorf("pool[%d]: asset0 and asset1 required", i)
		}
		if p.Asset0Chain == "" || p.Asset1Chain == "" {
			return nil, fmt.Errorf("pool[%d]: asset0_chain and asset1_chain required", i)
		}
	}
	return &c, nil
}
