package dexinternal

import (
	"dex/sdk"
)

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
	Chain           string `json:"string"`
	Description     string `json:"description,omitempty"`
}

//tinyjson:json
type SwapResult struct {
	AmountOut string   `json:"amount_out"`
	PoolState PoolInfo `json:"pool_state"` // Current pool state after swap
}

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
