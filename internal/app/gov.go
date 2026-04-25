package app

import (
	"encoding/json"
	"fmt"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/spf13/viper"
)

func (asm StateManager) fixGovernanceParameters(appGenState map[string]json.RawMessage) error {
	var govGenState govv1.GenesisState
	return updateModuleState(asm.encodingConfig.Codec, appGenState, "gov", &govGenState, func() error {
		if govGenState.Params == nil {
			defaults := govv1.DefaultParams()
			govGenState.Params = &defaults
		}
		denom := viper.GetString("default_bond_denom")
		if v := viper.GetInt64("gov.min_deposit_amount"); v > 0 {
			govGenState.Params.MinDeposit = sdk.Coins{{Denom: denom, Amount: math.NewInt(v)}}
		}
		if v := viper.GetString("gov.voting_period"); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid gov.voting_period %q: %w", v, err)
			}
			govGenState.Params.VotingPeriod = &d
		}
		if v := viper.GetInt64("gov.expedited_min_deposit_amount"); v > 0 {
			govGenState.Params.ExpeditedMinDeposit = sdk.Coins{{Denom: denom, Amount: math.NewInt(v)}}
		}
		if v := viper.GetString("gov.expedited_voting_period"); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid gov.expedited_voting_period %q: %w", v, err)
			}
			govGenState.Params.ExpeditedVotingPeriod = &d
		}
		return nil
	})
}
