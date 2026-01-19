package main

//tinyjson:json
type RegisterPoolParams struct {
	Asset0        string  `json:"asset0"`
	Asset1        string  `json:"asset1"`
	DexContractId string  `json:"dex_contract_id"`
	Asset0Chain   *string `json:"asset0_chain,omitempty"` // Optional: chain for asset0 (from mapping contract)
	Asset1Chain   *string `json:"asset1_chain,omitempty"` // Optional: chain for asset1 (from mapping contract)
}

//tinyjson:json
type DexInstruction struct {
	Type          string            `json:"type"`
	Version       string            `json:"version"`
	AssetIn       string            `json:"asset_in"`
	AssetOut      string            `json:"asset_out"`
	Recipient     string            `json:"recipient"`
	SlippageBps   *int              `json:"slippage_bps,omitempty"`
	MinAmountOut  *int64            `json:"min_amount_out,omitempty"`
	Beneficiary   *string           `json:"beneficiary,omitempty"`
	RefBps        *int              `json:"ref_bps,omitempty"`
	ReturnAddress *ReturnAddress    `json:"return_address,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	AmountIn      int64             `json:"amount_in"`
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
type SwapResult struct {
	AmountOut uint64   `json:"amount_out"`
	PoolState PoolInfo `json:"pool_state"`
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
type GetPoolParams struct {
	Asset0 string `json:"asset0"`
	Asset1 string `json:"asset1"`
}

//tinyjson:json
type ReturnAddress struct {
	Chain   string `json:"chain"`
	Address string `json:"address"`
}

//tinyjson:json
type FailureLog struct {
	Reason             string         `json:"reason"`
	FailedAtHop        int            `json:"failed_at_hop"` // 1 = first hop, 2 = second hop
	OriginalAsset      string         `json:"original_asset"`
	OriginalAmount     uint64         `json:"original_amount"`
	IntermediateAsset  string         `json:"intermediate_asset,omitempty"`
	IntermediateAmount uint64         `json:"intermediate_amount,omitempty"`
	ReturnAddress      *ReturnAddress `json:"return_address,omitempty"`
	Timestamp          string         `json:"timestamp"`
}

//tinyjson:json
type ReturnRequest struct {
	Chain   string
	Address string
	Asset   string
	Amount  uint64
	Log     FailureLog
}

//tinyjson:json
type SchemaReturn struct {
	SupportedChains     []string
	ReturnAddressChains []string
	Note                string
}
