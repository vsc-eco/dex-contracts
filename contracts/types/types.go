package types

import "github.com/vsc-eco/dex-contracts/sdk"

// DEX OPERATIONS

//tinyjson:json
type InitParams struct {
	Asset0                string `json:"asset0"`
	Asset1                string `json:"asset1"`
	FeeBps                uint64 `json:"fee_bps"`
	Asset0MappingContract string `json:"asset0_mapping_contract,omitempty"`
	Asset1MappingContract string `json:"asset1_mapping_contract,omitempty"`
	// RouterContract is the authorized router that can set PreDeposited flags.
	// If empty, PreDeposited is rejected for all callers.
	RouterContract string `json:"router_contract,omitempty"`
}

//tinyjson:json
type SwapParams struct {
	AssetIn      string  `json:"asset_in"`
	AmountIn     string  `json:"amount_in"`
	AssetOut     string  `json:"asset_out"`
	MinAmountOut *string `json:"min_amount_out,omitempty"`
	From         string  `json:"from,omitempty"`
	To           string  `json:"to"`
	Beneficiary  *string `json:"beneficiary,omitempty"`
	RefBps       *uint64 `json:"ref_bps,omitempty"`
	// PreDeposited indicates that mapped input tokens have already been
	// transferred into the pool by the router via ERC-20 allowances.
	// When true, the pool skips DrawAssetFrom for mapped assets.
	PreDeposited bool `json:"pre_deposited,omitempty"`
}

//tinyjson:json
type AddLiquidityParams struct {
	Amount0   string `json:"amount0"`
	Amount1   string `json:"amount1"`
	Recipient string `json:"recipient"`
	// Per-asset pre-deposit flags. When true, the corresponding asset was
	// already transferred into the pool by the router via ERC-20 allowances.
	PreDeposited0 bool `json:"pre_deposited_0,omitempty"`
	PreDeposited1 bool `json:"pre_deposited_1,omitempty"`
}

//tinyjson:json
type RemoveLiquidityParams struct {
	LpAmount  string `json:"lp_amount"`
	Recipient string `json:"recipient"`
}

//tinyjson:json
type SwapResult struct {
	AmountOut string   `json:"amount_out"`
	PoolState PoolInfo `json:"pool_state"` // Current pool state after swap
}

// ROUTER OPERATIONS

//tinyjson:json
type RegisterPoolParams struct {
	Asset0        string `json:"asset0"`
	Asset1        string `json:"asset1"`
	DexContractId string `json:"dex_contract_id"`
}

//tinyjson:json
type PoolInfo struct {
	Asset0   string `json:"asset0"`
	Asset1   string `json:"asset1"`
	Reserve0 string `json:"reserve0"`
	Reserve1 string `json:"reserve1"`
	Fee      uint64 `json:"fee"`
	TotalLp  string `json:"total_lp"`
}

//tinyjson:json
type TokenInfo struct {
	MappingContract string `json:"mapping_contract,omitempty"`
	Chain           string `json:"chain"` // HIVE, MAGI, BTC, ETH, etc.
	Description     string `json:"description,omitempty"`
	Decimals        int    `json:"decimals"`
}

// MappingContractInfoReturn is the response from calling getInfo on a mapping contract.
//
//tinyjson:json
type MappingContractInfoReturn struct {
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals string `json:"decimals"`
}

//tinyjson:json
type RegisterTokenParams struct {
	Name string `json:"name"`
	TokenInfo
}

//tinyjson:json
type GetPoolParams struct {
	Asset0 string `json:"asset0"`
	Asset1 string `json:"asset1"`
}

//tinyjson:json
type ReturnAddress struct {
	Chain   string `json:"chain"`
	Address string `json:"address"`
}

// ROUTER TYPES

//tinyjson:json
type DexInstruction struct {
	Type          string            `json:"type"`
	Version       string            `json:"version"`
	AssetIn       string            `json:"asset_in"`
	AssetOut      string            `json:"asset_out"`
	Recipient     string            `json:"recipient"`
	MinAmountOut  *string           `json:"min_amount_out,omitempty"`
	Beneficiary   *string           `json:"beneficiary,omitempty"`
	RefBps        *uint64           `json:"ref_bps,omitempty"`
	ReturnAddress *ReturnAddress    `json:"return_address,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	AmountIn      string            `json:"amount_in"`
	// DestinationChain specifies an external chain for settlement (e.g. "HIVE", "BTC").
	// When set to a non-MAGI chain, the router bridges swap output to the Recipient
	// address on that chain instead of settling on Magi.
	DestinationChain string `json:"destination_chain,omitempty"`
}

//tinyjson:json
type SchemaReturn struct {
	SupportedChains     []string              `json:"supported_chains"`
	ReturnAddressChains []string              `json:"return_address_chains"`
	Tokens              []RegisterTokenParams `json:"tokens"`
	Note                string                `json:"note"`
}

//tinyjson:json
type KeyList struct {
	Keys []string `json:"keys"`
}

//tinyjson:json
type KeyMap map[string]string

// type for the ENV that only queries the env if actually needed, saving gas if not
type MaybeEnv struct {
	env *sdk.Env
}

//tinyjson:json
type MappingContractInput struct {
	Amount string `json:"amount"`
	To     string `json:"to"`
	From   string `json:"from,omitempty"`
}

// UnmapParams is the input for the "unmap" action on mapping contracts,
// which sends mapped assets out to their native chain (e.g. BTC to Bitcoin mainnet).
//
//tinyjson:json
type UnmapParams struct {
	Amount    string `json:"amount"`
	To        string `json:"to"`
	From      string `json:"from,omitempty"`
	DeductFee bool   `json:"deduct_fee,omitempty"`
	MaxFee    *int64 `json:"max_fee,omitempty"`
}
