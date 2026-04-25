package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cosmossdk.io/math"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/spf13/viper"
)

func (asm StateManager) setStakingState(
	appGenState map[string]json.RawMessage,
	claimsDelegations []stakingtypes.Delegation,
	shares map[string]int64,
) error {
	validators, err := asm.validatorRepository.GetValidators(context.Background())
	if err != nil {
		return err
	}

	stakingStateRaw, ok := appGenState["staking"]
	if !ok {
		return fmt.Errorf("staking module not found in genesis state")
	}
	var stakingGenState stakingtypes.GenesisState
	if err := asm.encodingConfig.Codec.UnmarshalJSON(stakingStateRaw, &stakingGenState); err != nil {
		stakingGenState = *stakingtypes.DefaultGenesisState()
	}

	denom := viper.GetString("default_bond_denom")
	applyStakingParams(&stakingGenState.Params, denom)

	var lastTotalPower int64
	var lastValidatorPowers []stakingtypes.LastValidatorPower
	stakingValidators := make([]map[string]any, 0, len(validators))

	for i := range validators {
		tokens := validators[i].Amount() + shares[validators[i].Name()]
		power := tokens / microTokensPerToken
		lastTotalPower += power

		lastValidatorPowers = append(lastValidatorPowers, stakingtypes.LastValidatorPower{
			Address: validators[i].OperatorAddress(),
			Power:   power,
		})

		// commission update_time must predate genesis, otherwise the SDK blocks immediate rate changes.
		t := time.Unix(viper.GetInt64("app.genesis_time"), 0).AddDate(0, -1, 0).UTC()
		stakingValidators = append(stakingValidators, map[string]any{
			"operator_address": validators[i].OperatorAddress(),
			"consensus_pubkey": map[string]any{
				"@type": "/cosmos.crypto.ed25519.PubKey",
				"key":   validators[i].PubKey(),
			},
			"jailed":           false,
			"status":           "BOND_STATUS_BONDED",
			"tokens":           fmt.Sprintf("%d", tokens),
			"delegator_shares": fmt.Sprintf("%d.000000000000000000", tokens),
			"description": map[string]any{
				"moniker":          validators[i].Name(),
				"identity":         validators[i].Identity(),
				"website":          validators[i].Website(),
				"security_contact": validators[i].SecurityContact(),
				"details":          validators[i].Details(),
			},
			"unbonding_height": "0",
			"unbonding_time":   "1970-01-01T00:00:00Z",
			"commission": map[string]any{
				"commission_rates": map[string]string{
					"rate":            validators[i].CommissionRate(),
					"max_rate":        validators[i].MaxRate(),
					"max_change_rate": validators[i].MaxChangeRate(),
				},
				"update_time": t.Format("2006-01-02T15:04:05Z07:00"),
			},
			"min_self_delegation":         "1",
			"unbonding_on_hold_ref_count": "0",
			"unbonding_ids":               []any{},
		})

		claimsDelegations = append(claimsDelegations, stakingtypes.Delegation{
			DelegatorAddress: validators[i].DelegatorAddress(),
			ValidatorAddress: validators[i].OperatorAddress(),
			Shares:           math.LegacyNewDec(validators[i].Amount()),
		})
	}

	stakingGenState.LastTotalPower = math.NewInt(lastTotalPower)
	stakingGenState.LastValidatorPowers = lastValidatorPowers
	stakingGenState.Validators = []stakingtypes.Validator{}
	stakingGenState.Delegations = claimsDelegations
	stakingGenState.Exported = true // marks this as a pre-configured genesis, not a fresh initial state
	stakingGenState.UnbondingDelegations = []stakingtypes.UnbondingDelegation{}
	stakingGenState.Redelegations = []stakingtypes.Redelegation{}

	// Use cdc.MarshalJSON to get correct proto-JSON formatting (durations as strings, int64 quoted).
	stakingStateBz, err := asm.encodingConfig.Codec.MarshalJSON(&stakingGenState)
	if err != nil {
		return fmt.Errorf("failed to marshal staking genesis state: %w", err)
	}

	// Shallow-parse into raw-message map so proto-formatted values are not decoded and re-encoded;
	// then inject the handcrafted validator objects and marshal once.
	var stakingState map[string]json.RawMessage
	if err := json.Unmarshal(stakingStateBz, &stakingState); err != nil {
		return fmt.Errorf("failed to unmarshal staking state for validator injection: %w", err)
	}
	validatorsBz, err := json.Marshal(stakingValidators)
	if err != nil {
		return fmt.Errorf("failed to marshal validators: %w", err)
	}
	stakingState["validators"] = validatorsBz

	appGenState["staking"], err = json.Marshal(stakingState)
	if err != nil {
		return fmt.Errorf("failed to marshal updated staking genesis state: %w", err)
	}

	// The gentx transactions have been consumed into staking.validators.
	// Clear genutil.gen_txs so the chain does not re-process them on startup,
	// which would produce duplicate-validator errors.
	appGenState["genutil"] = json.RawMessage(`{"gen_txs":[]}`)

	return nil
}

func applyStakingParams(params *stakingtypes.Params, denom string) {
	params.BondDenom = denom
	if v := viper.GetInt64("chain.unbonding_time_seconds"); v > 0 {
		params.UnbondingTime = time.Duration(v) * time.Second
	}
	if v := viper.GetUint32("chain.max_validators"); v > 0 {
		params.MaxValidators = v
	}
	if v := viper.GetUint32("chain.max_entries"); v > 0 {
		params.MaxEntries = v
	}
	if v := viper.GetUint32("chain.historical_entries"); v > 0 {
		params.HistoricalEntries = v
	}
	if v := viper.GetString("chain.min_commission_rate"); v != "" {
		params.MinCommissionRate = math.LegacyMustNewDecFromStr(v)
	}
}
