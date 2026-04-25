package app

import (
	"encoding/json"

	"cosmossdk.io/math"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/spf13/viper"
)

func (asm StateManager) fixMintParameters(appGenState map[string]json.RawMessage) error {
	var mintGenState minttypes.GenesisState
	return updateModuleState(asm.encodingConfig.Codec, appGenState, "mint", &mintGenState, func() error {
		mintGenState.Params.MintDenom = viper.GetString("default_bond_denom")
		if v := viper.GetInt64("chain.blocks_per_year"); v > 0 {
			mintGenState.Params.BlocksPerYear = uint64(v)
		}
		if v := viper.GetString("chain.inflation_rate_change"); v != "" {
			mintGenState.Params.InflationRateChange = math.LegacyMustNewDecFromStr(v)
		}
		if v := viper.GetString("chain.inflation_max"); v != "" {
			mintGenState.Params.InflationMax = math.LegacyMustNewDecFromStr(v)
		}
		if v := viper.GetString("chain.inflation_min"); v != "" {
			mintGenState.Params.InflationMin = math.LegacyMustNewDecFromStr(v)
		}
		if v := viper.GetString("chain.goal_bonded"); v != "" {
			mintGenState.Params.GoalBonded = math.LegacyMustNewDecFromStr(v)
		}
		return nil
	})
}
