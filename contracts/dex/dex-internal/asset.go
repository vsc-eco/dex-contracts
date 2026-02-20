package dexinternal

import (
	"dex/sdk"
	"fmt"
	"strconv"
	"strings"

	"github.com/CosmWasm/tinyjson"
	jwriter "github.com/CosmWasm/tinyjson/jwriter"
)

const (
	intentTransferType  = "transfer.allow"
	intentContractIdKey = "contract_id"
	intentAmountKey     = "limit"
	intentTokenKey      = "token"
)

// TODO: some sort of balance check for non-native assets?
// include marshaller but not unmarshaller, to avoid unmarshalling into an empty interface
type Asset interface {
	Name() string
	MappingContract() string
	DrawAsset(amount int64, mEnv MaybeEnv) error
	// transfers balance held by the contract to another magi account
	TransferAsset(to string, amount int64) error
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
		if len(mappingContract) != 1 {
			return nil, fmt.Errorf("must provide exactly 1 mapping contract for mapped asset: %s, received %d", normalizedName, len(mappingContract))
		}
		return &MappedAsset{
			Asset:             normalizedName,
			MappingContractId: mappingContract[0],
		}, nil
	}
}

func AssetFromJson(s string) (Asset, error) {
	if strings.Contains(s, "mapping_contract_id") {
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
func (ha *HiveAsset) DrawAsset(amount int64, me MaybeEnv) error {
	sdk.HiveDrawFrom(me.UseEnv().Sender.Address, amount, sdk.Asset(ha.Asset))
	return nil
}

func (ha *HiveAsset) TransferAsset(to string, amount int64) error {
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

func (ma *MappedAsset) DrawAsset(amount int64, mEnv MaybeEnv) error {
	env := mEnv.UseEnv()
	input := MappingContractInput{
		Amount: amount,
		To:     "contract:" + env.ContractId,
		From:   env.Sender.Address.String(),
	}
	payload, err := tinyjson.Marshal(input)
	if err != nil {
		return fmt.Errorf("invalid input to contract")
	}
	// error in the contract call will abort the whole transaction
	sdk.ContractCall(ma.MappingContractId, "transferFrom", string(payload), &sdk.ContractCallOptions{})
	return nil
}

func (ma *MappedAsset) TransferAsset(to string, amount int64) error {
	input := MappingContractInput{
		Amount: amount,
		To:     to,
	}
	payload, err := tinyjson.Marshal(input)
	if err != nil {
		return fmt.Errorf("invalid input to contract")
	}
	// error in the contract call will abort the whole transaction
	// sdk.ContractCall(ma.mappingContractId, "unmap", string(payload), &sdk.ContractCallOptions{
	sdk.ContractCall(ma.MappingContractId, "transfer", string(payload), &sdk.ContractCallOptions{
		Intents: []sdk.Intent{
			{
				Type: intentTransferType,
				Args: map[string]string{
					intentAmountKey:     strconv.FormatInt(amount, 10),
					intentTokenKey:      ma.Asset,
					intentContractIdKey: ma.MappingContractId,
				},
			},
		},
	})
	return nil
}
