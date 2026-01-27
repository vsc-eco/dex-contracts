package dexinternal

import "dex/sdk"

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
	AmountIn     int64   `json:"amount_in"`
	AssetOut     string  `json:"asset_out"`
	MinAmountOut *int64  `json:"min_amount_out,omitempty"`
	Recipient    string  `json:"recipient"`
	Beneficiary  *string `json:"beneficiary,omitempty"`
	RefBps       *int    `json:"ref_bps,omitempty"`
}

//tinyjson:json
type AddLiquidityParams struct {
	Amount0   uint64 `json:"amount0"`
	Amount1   uint64 `json:"amount1"`
	Recipient string `json:"recipient"`
}

//tinyjson:json
type RemoveLiquidityParams struct {
	LpAmount  uint64 `json:"lp_amount"`
	Recipient string `json:"recipient"`
}

//tinyjson:json
type PoolInfo struct {
	Asset0   string `json:"asset0"`
	Asset1   string `json:"asset1"`
	Reserve0 uint64 `json:"reserve0"`
	Reserve1 uint64 `json:"reserve1"`
	Fee      uint64 `json:"fee"`
	TotalLp  uint64 `json:"total_lp"`
}

//tinyjson:json
type TokenInfo struct {
	MappingContract string `json:"mapping_contract,omitempty"`
	Chain           string `json:"string"`
	Description     string `json:"description,omitempty"`
}

//tinyjson:json
type SwapResult struct {
	AmountOut uint64   `json:"amount_out"`
	PoolState PoolInfo `json:"pool_state"` // Current pool state after swap
}

// type for the ENV that only queries the env if actually needed, saving gas if not
type MaybeEnv struct {
	Env *sdk.Env
}

//tinyjson:json
type MappingContractInput struct {
	Amount  int64  `json:"amount"`
	Address string `json:"address"`
}
