package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// IdentityConfig matches go-vsc-node's identityConfig.json format.
type IdentityConfig struct {
	HiveActiveKey string `json:"HiveActiveKey"`
	HiveUsername  string `json:"HiveUsername"`
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
	ContractID            string `json:"contract_id"`
	Asset0                string `json:"asset0"`
	Asset1                string `json:"asset1"`
	FeeBps                uint64 `json:"fee_bps"`
	Asset0MappingContract string `json:"asset0_mapping_contract,omitempty"`
	Asset1MappingContract string `json:"asset1_mapping_contract,omitempty"`
	Asset0Chain           string `json:"asset0_chain"`
	Asset1Chain           string `json:"asset1_chain"`
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

// configTemplates maps each config file name to its empty-template contents.
// Optional fields (json `omitempty`) are included so users see every field
// they can set; remove or fill them in as needed.
var configTemplates = map[string]string{
	"identityConfig.json": `{
  "HiveActiveKey": "",
  "HiveUsername": "",
}
`,
	"hiveConfig.json": `{
  "HiveURIs": [""]
}
`,
	"serviceConfig.json": `{
  "graphql_url": "",
  "vsc_net_id": "",
  "hive_chain_id": "",
  "poll_interval_seconds": 0,
  "poll_timeout_seconds": 0
}
`,
	"poolsConfig.json": `{
  "router_contract_id": "",
  "pools": [
    {
      "contract_id": "",
      "asset0": "",
      "asset1": "",
      "fee_bps": 0,
      "asset0_mapping_contract": "",
      "asset1_mapping_contract": "",
      "asset0_chain": "",
      "asset1_chain": ""
    }
  ]
}
`,
}

// EnsureConfigTemplates creates any missing config files in configDir with
// empty-template contents. Returns paths of files that were newly created.
func EnsureConfigTemplates(configDir string) ([]string, error) {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating %s: %w", configDir, err)
	}

	var created []string
	for name, body := range configTemplates {
		path := filepath.Join(configDir, name)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat %s: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", path, err)
		}
		created = append(created, path)
	}

	return created, nil
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
