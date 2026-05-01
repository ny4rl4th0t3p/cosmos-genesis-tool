package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	feegranttypes "cosmossdk.io/x/feegrant"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/viper"
)

func (asm StateManager) setFeegrantState(ctx context.Context, appGenState map[string]json.RawMessage) error {
	if !viper.IsSet("feegrant.file_name") {
		return nil
	}

	allowances, err := asm.feeAllowanceRepository.GetFeeAllowances(ctx, asm.encodingConfig)
	if err != nil {
		return fmt.Errorf("failed to read fee allowances: %w", err)
	}
	if len(allowances) == 0 {
		return nil
	}

	denom := viper.GetString("default_bond_denom")

	var genAllowances []feegranttypes.Grant
	for _, a := range allowances {
		basic := &feegranttypes.BasicAllowance{}
		if a.SpendLimit() > 0 {
			basic.SpendLimit = sdk.NewCoins(sdk.NewInt64Coin(denom, a.SpendLimit()))
		}
		if a.Expiry() > 0 {
			t := time.Unix(a.Expiry(), 0).UTC()
			basic.Expiration = &t
		}
		anyAllowance, err := codectypes.NewAnyWithValue(basic)
		if err != nil {
			return fmt.Errorf("failed to pack fee allowance for %s→%s: %w", a.Granter(), a.Grantee(), err)
		}
		genAllowances = append(genAllowances, feegranttypes.Grant{
			Granter:   a.Granter(),
			Grantee:   a.Grantee(),
			Allowance: anyAllowance,
		})
	}

	var feegrantGenState feegranttypes.GenesisState
	return updateModuleState(asm.encodingConfig.Codec, appGenState, "feegrant", &feegrantGenState, func() error {
		feegrantGenState.Allowances = append(feegrantGenState.Allowances, genAllowances...)
		return nil
	})
}
