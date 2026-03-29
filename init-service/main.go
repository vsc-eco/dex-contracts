package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	dataDir := flag.String("data-dir", "data", "directory containing config/ with all config files")
	poolsOnly := flag.Bool("pools-only", false, "skip router init and token registration, only init+register pools")
	flag.Parse()

	configDir := filepath.Join(*dataDir, "config")

	identity, err := loadIdentityConfig(filepath.Join(configDir, "identityConfig.json"))
	if err != nil {
		log.Fatalf("identity config: %v", err)
	}

	hiveConf, err := loadHiveConfig(filepath.Join(configDir, "hiveConfig.json"))
	if err != nil {
		log.Fatalf("hive config: %v", err)
	}

	serviceConf, err := loadServiceConfig(filepath.Join(configDir, "serviceConfig.json"))
	if err != nil {
		log.Fatalf("service config: %v", err)
	}

	poolsConf, err := loadPoolsConfig(filepath.Join(configDir, "poolsConfig.json"))
	if err != nil {
		log.Fatalf("pools config: %v", err)
	}

	bc := &Broadcaster{
		HiveURIs:  hiveConf.HiveURIs,
		ChainID:   serviceConf.HiveChainID,
		NetID:     serviceConf.VscNetID,
		Username:  identity.HiveUsername,
		ActiveKey: identity.HiveActiveKey,
	}

	pollInterval := time.Duration(serviceConf.PollIntervalSeconds) * time.Second
	pollTimeout := time.Duration(serviceConf.PollTimeoutSeconds) * time.Second

	if err := runInit(bc, serviceConf.GraphQLURL, poolsConf, *poolsOnly, pollInterval, pollTimeout); err != nil {
		log.Fatalf("initialization failed: %v", err)
	}

	log.Println("all initialization complete")
}

func runInit(bc *Broadcaster, gqlURL string, conf *PoolsConfig, poolsOnly bool, pollInterval, pollTimeout time.Duration) error {
	if !poolsOnly {
		// Step 1: Initialize the router contract
		if err := initRouter(bc, gqlURL, conf.RouterContractID, pollInterval, pollTimeout); err != nil {
			return fmt.Errorf("router init: %w", err)
		}

		// Step 2: Register all unique tokens on the router
		if err := registerTokens(bc, gqlURL, conf, pollInterval, pollTimeout); err != nil {
			return fmt.Errorf("token registration: %w", err)
		}
	}

	// Step 3: Initialize each pool and register it on the router
	for i, pool := range conf.Pools {
		log.Printf("--- pool %d/%d: %s (%s/%s) ---", i+1, len(conf.Pools), pool.ContractID, pool.Asset0, pool.Asset1)

		if err := initPool(bc, gqlURL, conf.RouterContractID, &pool, pollInterval, pollTimeout); err != nil {
			return fmt.Errorf("pool[%d] init: %w", i, err)
		}

		if err := registerPool(bc, gqlURL, conf.RouterContractID, &pool, pollInterval, pollTimeout); err != nil {
			return fmt.Errorf("pool[%d] register: %w", i, err)
		}
	}

	return nil
}

func broadcastAndWait(bc *Broadcaster, gqlURL string, contractID, action string, payload any, pollInterval, pollTimeout time.Duration) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling payload: %w", err)
	}

	txId, err := bc.CallContract(contractID, action, payloadJSON)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), pollTimeout)
	defer cancel()

	return WaitForConfirmation(ctx, gqlURL, txId, pollInterval)
}

func initRouter(bc *Broadcaster, gqlURL string, routerContractID string, pollInterval, pollTimeout time.Duration) error {
	log.Printf("=== initializing router %s ===", routerContractID)

	// Router init takes an optional version string; use default "1.0.0"
	payloadJSON := json.RawMessage(`"1.0.0"`)

	txId, err := bc.CallContract(routerContractID, "init", payloadJSON)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), pollTimeout)
	defer cancel()

	return WaitForConfirmation(ctx, gqlURL, txId, pollInterval)
}

func registerTokens(bc *Broadcaster, gqlURL string, conf *PoolsConfig, pollInterval, pollTimeout time.Duration) error {
	log.Println("=== registering tokens ===")

	type tokenKey struct {
		Name  string
		Chain string
	}

	seen := map[tokenKey]bool{}
	var tokens []RegisterTokenPayload

	for _, pool := range conf.Pools {
		k0 := tokenKey{pool.Asset0, pool.Asset0Chain}
		if !seen[k0] {
			seen[k0] = true
			tokens = append(tokens, RegisterTokenPayload{
				Name:            pool.Asset0,
				Chain:           pool.Asset0Chain,
				MappingContract: pool.Asset0MappingContract,
			})
		}

		k1 := tokenKey{pool.Asset1, pool.Asset1Chain}
		if !seen[k1] {
			seen[k1] = true
			tokens = append(tokens, RegisterTokenPayload{
				Name:            pool.Asset1,
				Chain:           pool.Asset1Chain,
				MappingContract: pool.Asset1MappingContract,
			})
		}
	}

	for _, tok := range tokens {
		log.Printf("  registering token: %s (chain: %s)", tok.Name, tok.Chain)
		if err := broadcastAndWait(bc, gqlURL, conf.RouterContractID, "register_token", tok, pollInterval, pollTimeout); err != nil {
			return fmt.Errorf("register_token %s: %w", tok.Name, err)
		}
	}

	return nil
}

func initPool(bc *Broadcaster, gqlURL string, routerContractID string, pool *PoolConfig, pollInterval, pollTimeout time.Duration) error {
	log.Printf("  initializing pool %s", pool.ContractID)

	payload := InitPoolPayload{
		Asset0:                pool.Asset0,
		Asset1:                pool.Asset1,
		FeeBps:                pool.FeeBps,
		Asset0MappingContract: pool.Asset0MappingContract,
		Asset1MappingContract: pool.Asset1MappingContract,
		RouterContract:        routerContractID,
	}

	return broadcastAndWait(bc, gqlURL, pool.ContractID, "init", payload, pollInterval, pollTimeout)
}

func registerPool(bc *Broadcaster, gqlURL string, routerContractID string, pool *PoolConfig, pollInterval, pollTimeout time.Duration) error {
	log.Printf("  registering pool %s on router", pool.ContractID)

	payload := RegisterPoolPayload{
		Asset0:        pool.Asset0,
		Asset1:        pool.Asset1,
		DexContractId: pool.ContractID,
	}

	return broadcastAndWait(bc, gqlURL, routerContractID, "register_pool", payload, pollInterval, pollTimeout)
}

func init() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lmsgprefix)
}
