package app

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"cosmossdk.io/math"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/validator"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

func TestApplyStakingParams_OnlyBondDenom_WhenNoViperKeys(t *testing.T) {
	params := stakingtypes.DefaultParams()
	originalUnbonding := params.UnbondingTime
	originalMaxVal := params.MaxValidators

	applyStakingParams(&params, "uatom")

	assert.Equal(t, "uatom", params.BondDenom)
	assert.Equal(t, originalUnbonding, params.UnbondingTime)
	assert.Equal(t, originalMaxVal, params.MaxValidators)
}

func TestApplyStakingParams_AllViperKeys(t *testing.T) {
	viper.Set("chain.unbonding_time_seconds", int64(86400))
	viper.Set("chain.max_validators", uint32(150))
	viper.Set("chain.max_entries", uint32(5))
	viper.Set("chain.historical_entries", uint32(5000))
	viper.Set("chain.min_commission_rate", "0.05")
	t.Cleanup(func() {
		viper.Set("chain.unbonding_time_seconds", nil)
		viper.Set("chain.max_validators", nil)
		viper.Set("chain.max_entries", nil)
		viper.Set("chain.historical_entries", nil)
		viper.Set("chain.min_commission_rate", nil)
	})

	params := stakingtypes.DefaultParams()
	applyStakingParams(&params, "ustake")

	assert.Equal(t, "ustake", params.BondDenom)
	assert.Equal(t, 86400*time.Second, params.UnbondingTime)
	assert.Equal(t, uint32(150), params.MaxValidators)
	assert.Equal(t, uint32(5), params.MaxEntries)
	assert.Equal(t, uint32(5000), params.HistoricalEntries)
	assert.Equal(t, "0.050000000000000000", params.MinCommissionRate.String())
}

func TestApplyStakingParams_PartialKeys_OnlyUpdatesSet(t *testing.T) {
	viper.Set("chain.max_validators", uint32(200))
	t.Cleanup(func() { viper.Set("chain.max_validators", nil) })

	params := stakingtypes.DefaultParams()
	originalEntries := params.MaxEntries
	originalUnbonding := params.UnbondingTime

	applyStakingParams(&params, "uatom")

	assert.Equal(t, uint32(200), params.MaxValidators)
	assert.Equal(t, originalEntries, params.MaxEntries)
	assert.Equal(t, originalUnbonding, params.UnbondingTime)
}

// --- setStakingState ---

func stakingAppState(t *testing.T, ec encoding.EncodingConfig) map[string]json.RawMessage {
	t.Helper()
	gs := stakingtypes.DefaultGenesisState()
	bz, err := ec.Codec.MarshalJSON(gs)
	require.NoError(t, err)
	return map[string]json.RawMessage{"staking": bz}
}

func TestSetStakingState_ValidatorRepoError(t *testing.T) {
	ec := encoding.NewEncodingConfig()
	sentinel := errors.New("repo fail")
	asm := StateManager{
		encodingConfig:      ec,
		validatorRepository: stubValidatorRepo{err: sentinel},
	}
	err := asm.setStakingState(stakingAppState(t, ec), nil, nil)
	require.ErrorIs(t, err, sentinel)
}

func TestSetStakingState_SingleValidator_InStakingState(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	viper.Set("default_bond_denom", "uatom")
	viper.Set("app.genesis_time", int64(0))
	t.Cleanup(func() {
		viper.Set("chain.address_prefix", nil)
		viper.Set("default_bond_denom", nil)
		viper.Set("app.genesis_time", nil)
	})

	ec := encoding.NewEncodingConfig()
	v := testValidator(t, 1) // amount = 1_000_000
	asm := StateManager{
		encodingConfig:      ec,
		validatorRepository: stubValidatorRepo{validators: []validator.Validator{v}},
	}
	appGenState := stakingAppState(t, ec)
	require.NoError(t, asm.setStakingState(appGenState, nil, nil))

	// Unmarshal into a raw map to read the hand-crafted validator objects.
	var stakingRaw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(appGenState["staking"], &stakingRaw))
	var vals []map[string]any
	require.NoError(t, json.Unmarshal(stakingRaw["validators"], &vals))

	require.Len(t, vals, 1)
	assert.Equal(t, v.OperatorAddress(), vals[0]["operator_address"])
	assert.Equal(t, "BOND_STATUS_BONDED", vals[0]["status"])
	assert.Equal(t, "1000000", vals[0]["tokens"])
}

func TestSetStakingState_SharesAddedToTokens(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	viper.Set("default_bond_denom", "uatom")
	viper.Set("app.genesis_time", int64(0))
	t.Cleanup(func() {
		viper.Set("chain.address_prefix", nil)
		viper.Set("default_bond_denom", nil)
		viper.Set("app.genesis_time", nil)
	})

	ec := encoding.NewEncodingConfig()
	v := testValidator(t, 2) // amount = 1_000_000
	shares := map[string]int64{"validator-2": 3_000_000}
	asm := StateManager{
		encodingConfig:      ec,
		validatorRepository: stubValidatorRepo{validators: []validator.Validator{v}},
	}
	appGenState := stakingAppState(t, ec)
	require.NoError(t, asm.setStakingState(appGenState, nil, shares))

	var stakingRaw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(appGenState["staking"], &stakingRaw))
	var vals []map[string]any
	require.NoError(t, json.Unmarshal(stakingRaw["validators"], &vals))

	require.Len(t, vals, 1)
	assert.Equal(t, "4000000", vals[0]["tokens"]) // 1M + 3M shares
}

func TestSetStakingState_DelegationsIncluded(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	viper.Set("default_bond_denom", "uatom")
	viper.Set("app.genesis_time", int64(0))
	t.Cleanup(func() {
		viper.Set("chain.address_prefix", nil)
		viper.Set("default_bond_denom", nil)
		viper.Set("app.genesis_time", nil)
	})

	ec := encoding.NewEncodingConfig()
	v := testValidator(t, 3)
	existingDelegation := stakingtypes.Delegation{
		DelegatorAddress: testAccAddr(60).String(),
		ValidatorAddress: v.OperatorAddress(),
		Shares:           math.LegacyNewDec(500_000),
	}
	asm := StateManager{
		encodingConfig:      ec,
		validatorRepository: stubValidatorRepo{validators: []validator.Validator{v}},
	}
	appGenState := stakingAppState(t, ec)
	require.NoError(t, asm.setStakingState(appGenState, []stakingtypes.Delegation{existingDelegation}, nil))

	// Unmarshal via codec to read delegations.
	var stakingRaw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(appGenState["staking"], &stakingRaw))
	var gs stakingtypes.GenesisState
	require.NoError(t, ec.Codec.UnmarshalJSON(appGenState["staking"], &gs))

	// existing delegation + validator self-delegation appended by setStakingState
	require.Len(t, gs.Delegations, 2)
}

func TestSetStakingState_GenutilCleared(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	viper.Set("default_bond_denom", "uatom")
	viper.Set("app.genesis_time", int64(0))
	t.Cleanup(func() {
		viper.Set("chain.address_prefix", nil)
		viper.Set("default_bond_denom", nil)
		viper.Set("app.genesis_time", nil)
	})

	ec := encoding.NewEncodingConfig()
	asm := StateManager{
		encodingConfig:      ec,
		validatorRepository: stubValidatorRepo{},
	}
	appGenState := stakingAppState(t, ec)
	appGenState["genutil"] = json.RawMessage(`{"gen_txs":["original"]}`)
	require.NoError(t, asm.setStakingState(appGenState, nil, nil))

	assert.Equal(t, json.RawMessage(`{"gen_txs":[]}`), appGenState["genutil"])
}
