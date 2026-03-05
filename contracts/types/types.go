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
}

//tinyjson:json
type SwapParams struct {
	AssetIn      string  `json:"asset_in"`
	AmountIn     string  `json:"amount_in"`
	AssetOut     string  `json:"asset_out"`
	MinAmountOut *string `json:"min_amount_out,omitempty"`
	Recipient    string  `json:"recipient"`
	Beneficiary  *string `json:"beneficiary,omitempty"`
	RefBps       *uint64 `json:"ref_bps,omitempty"`
}

//tinyjson:json
type AddLiquidityParams struct {
	Amount0   string `json:"amount0"`
	Amount1   string `json:"amount1"`
	Recipient string `json:"recipient"`
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
}

//tinyjson:json
type FailureLog struct {
	Reason             string         `json:"reason"`
	FailedAtHop        int            `json:"failed_at_hop"` // 1 = first hop, 2 = second hop
	OriginalAsset      string         `json:"original_asset"`
	OriginalAmount     string         `json:"original_amount"`
	IntermediateAsset  string         `json:"intermediate_asset,omitempty"`
	IntermediateAmount string         `json:"intermediate_amount,omitempty"`
	ReturnAddress      *ReturnAddress `json:"return_address,omitempty"`
	Timestamp          string         `json:"timestamp"`
}

//tinyjson:json
type ReturnRequest struct {
	Chain   string
	Address string
	Asset   string
	Amount  string
	Log     FailureLog
}

//tinyjson:json
type SchemaReturn struct {
	SupportedChains     []string
	ReturnAddressChains []string
	Note                string
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
