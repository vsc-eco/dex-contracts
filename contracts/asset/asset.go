package asset

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/vsc-eco/dex-contracts/contracterrors"
	"github.com/vsc-eco/dex-contracts/contracts/types"
	"github.com/vsc-eco/dex-contracts/sdk"

	"github.com/CosmWasm/tinyjson"
	jwriter "github.com/CosmWasm/tinyjson/jwriter"
)

// TODO: some sort of balance check for non-native assets?
// include marshaller but not unmarshaller, to avoid unmarshalling into an empty interface
type Asset interface {
	Name() string
	MappingContract() string
	DrawAssetFrom(amount *big.Int, from sdk.Address, mEnv types.MaybeEnv) error
	// transfers balance held by the contract to another magi account
	TransferAsset(to string, amount *big.Int) error
	MarshalTinyJSON(w *jwriter.Writer)
}

var hiveAssets = map[string]bool{
	sdk.AssetHbd.String():        true,
	sdk.AssetHive.String():       true,
	sdk.AssetHiveCons.String():   true,
	sdk.AssetHbdSavings.String(): true,
}

func NewAsset(assetName string, mappingContract ...string) (Asset, error) {
	normalizedName := strings.ToLower(assetName)
	if hiveAssets[normalizedName] {
		return &HiveAsset{
			Asset: normalizedName,
		}, nil
	} else {
		if len(mappingContract) != 1 || mappingContract[0] == "" {
			return nil, fmt.Errorf("must provide exactly 1 non-empty mapping contract for mapped asset: %s", normalizedName)
		}
		return &MappedAsset{
			Asset:             normalizedName,
			MappingContractId: mappingContract[0],
		}, nil
	}
}

func AssetFromJson(s string) (Asset, error) {
	// Try mapped asset first — if JSON contains mapping_contract_id field, it's a mapped asset.
	// This is safe because the JSON is always contract-generated, never user input.
	if strings.Contains(s, `"mapping_contract_id"`) {
		var ma MappedAsset
		err := tinyjson.Unmarshal([]byte(s), &ma)
		if err != nil {
			return nil, err
		}
		return &ma, nil
	} else {
		var ha HiveAsset
		err := tinyjson.Unmarshal([]byte(s), &ha)
		if err != nil {
			return nil, err
		}
		return &ha, nil
	}
}

//tinyjson:json
type HiveAsset struct {
	Asset string
}

func (ha *HiveAsset) Name() string {
	return ha.Asset
}

func (ha *HiveAsset) MappingContract() string {
	return ""
}

// draws assets from the sender
func (ha *HiveAsset) DrawAssetFrom(amount *big.Int, from sdk.Address, me types.MaybeEnv) error {
	if from != me.UseEnv().Caller {
		return contracterrors.NewContractError(contracterrors.ErrAuth, "can only draw native assets from caller")
	}
	sdk.HiveDraw(amount, sdk.Asset(ha.Asset))
	return nil
}

func (ha *HiveAsset) TransferAsset(to string, amount *big.Int) error {
	sdk.HiveTransfer(sdk.Address(to), amount, sdk.Asset(ha.Asset))
	return nil
}

//tinyjson:json
type MappedAsset struct {
	Asset             string
	MappingContractId string
}

func (ma *MappedAsset) Name() string {
	return ma.Asset
}

func (ma *MappedAsset) MappingContract() string {
	return ma.MappingContractId
}

func (ma *MappedAsset) DrawAssetFrom(amount *big.Int, from sdk.Address, mEnv types.MaybeEnv) error {
	env := mEnv.UseEnv()
	if from != env.Caller {
		return contracterrors.NewContractError(contracterrors.ErrAuth, "can only draw mapped assets from caller")
	}
	input := types.MappingContractInput{
		Amount: amount.String(),
		To:     "contract:" + env.ContractId,
		From:   from.String(),
	}
	payload, err := tinyjson.Marshal(input)
	if err != nil {
		return fmt.Errorf("invalid input to contract")
	}
	// error in the contract call will abort the whole transaction
	sdk.ContractCall(ma.MappingContractId, "transferFrom", string(payload), &sdk.ContractCallOptions{})
	return nil
}

func (ma *MappedAsset) TransferAsset(to string, amount *big.Int) error {
	input := types.MappingContractInput{
		Amount: amount.String(),
		To:     to,
	}
	payload, err := tinyjson.Marshal(input)
	if err != nil {
		return fmt.Errorf("invalid input to contract")
	}
	// Pool owns these tokens — no allowance or intent needed for self-transfers.
	sdk.ContractCall(ma.MappingContractId, "transfer", string(payload), &sdk.ContractCallOptions{})
	return nil
}
