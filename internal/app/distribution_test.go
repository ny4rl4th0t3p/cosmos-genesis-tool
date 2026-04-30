package app

import (
	"encoding/json"
	"errors"
	"testing"

	"cosmossdk.io/math"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/validator"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

func distributionStateManager(t *testing.T, validators []validator.Validator, repoErr error) StateManager {
	t.Helper()
	ec := encoding.NewEncodingConfig()
	return StateManager{
		encodingConfig:      ec,
		validatorRepository: stubValidatorRepo{validators: validators, err: repoErr},
	}
}

func TestSetDistribution_ValidatorRepoError(t *testing.T) {
	sentinel := errors.New("repo fail")
	asm := distributionStateManager(t, nil, sentinel)
	err := asm.setDistribution(map[string]json.RawMessage{}, nil)
	require.ErrorIs(t, err, sentinel)
}

func TestSetDistribution_NoValidators_EmptyRecords(t *testing.T) {
	asm := distributionStateManager(t, nil, nil)
	appGenState := map[string]json.RawMessage{}
	require.NoError(t, asm.setDistribution(appGenState, nil))

	require.Contains(t, appGenState, "distribution")
	var ds distributiontypes.GenesisState
	require.NoError(t, asm.encodingConfig.Codec.UnmarshalJSON(appGenState["distribution"], &ds))
	assert.Empty(t, ds.DelegatorStartingInfos)
	assert.Empty(t, ds.OutstandingRewards)
	assert.Empty(t, ds.ValidatorAccumulatedCommissions)
}

func TestSetDistribution_ValidatorSelfDelegation(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	v := testValidator(t, 1)
	asm := distributionStateManager(t, []validator.Validator{v}, nil)
	appGenState := map[string]json.RawMessage{}
	require.NoError(t, asm.setDistribution(appGenState, nil))

	var ds distributiontypes.GenesisState
	require.NoError(t, asm.encodingConfig.Codec.UnmarshalJSON(appGenState["distribution"], &ds))

	require.Len(t, ds.DelegatorStartingInfos, 1)
	assert.Equal(t, v.DelegatorAddress(), ds.DelegatorStartingInfos[0].DelegatorAddress)
	assert.Equal(t, v.OperatorAddress(), ds.DelegatorStartingInfos[0].ValidatorAddress)

	require.Len(t, ds.OutstandingRewards, 1)
	assert.Equal(t, v.OperatorAddress(), ds.OutstandingRewards[0].ValidatorAddress)

	require.Len(t, ds.ValidatorAccumulatedCommissions, 1)
	assert.Equal(t, v.OperatorAddress(), ds.ValidatorAccumulatedCommissions[0].ValidatorAddress)
}

func TestSetDistribution_WithDelegations_AddsExtraDelegatorInfos(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	v := testValidator(t, 2)
	delegatorAddr := testAccAddr(50).String()
	delegations := []stakingtypes.Delegation{
		{
			DelegatorAddress: delegatorAddr,
			ValidatorAddress: v.OperatorAddress(),
			Shares:           math.LegacyNewDec(500_000),
		},
	}

	asm := distributionStateManager(t, []validator.Validator{v}, nil)
	appGenState := map[string]json.RawMessage{}
	require.NoError(t, asm.setDistribution(appGenState, delegations))

	var ds distributiontypes.GenesisState
	require.NoError(t, asm.encodingConfig.Codec.UnmarshalJSON(appGenState["distribution"], &ds))

	// validator self-delegation + 1 external delegator
	require.Len(t, ds.DelegatorStartingInfos, 2)
	addresses := []string{
		ds.DelegatorStartingInfos[0].DelegatorAddress,
		ds.DelegatorStartingInfos[1].DelegatorAddress,
	}
	assert.Contains(t, addresses, v.DelegatorAddress())
	assert.Contains(t, addresses, delegatorAddr)
}

func TestSetDistribution_CommunityPool_SetsFeepoolAndBank(t *testing.T) {
	const poolAmt = int64(1_000_000)
	viper.Set("distribution.community_pool_amount", poolAmt)
	viper.Set("chain.address_prefix", testHRP)
	viper.Set("default_bond_denom", "uatom")
	t.Cleanup(func() {
		viper.Set("distribution.community_pool_amount", nil)
		viper.Set("chain.address_prefix", nil)
		viper.Set("default_bond_denom", nil)
	})

	ec := encoding.NewEncodingConfig()
	asm := StateManager{
		encodingConfig:      ec,
		validatorRepository: stubValidatorRepo{},
	}

	bankState := banktypes.DefaultGenesisState()
	bankStateBz, err := ec.Codec.MarshalJSON(bankState)
	require.NoError(t, err)
	appGenState := map[string]json.RawMessage{"bank": bankStateBz}

	require.NoError(t, asm.setDistribution(appGenState, nil))

	var ds distributiontypes.GenesisState
	require.NoError(t, ec.Codec.UnmarshalJSON(appGenState["distribution"], &ds))
	require.Len(t, ds.FeePool.CommunityPool, 1)
	assert.Equal(t, "uatom", ds.FeePool.CommunityPool[0].Denom)
	assert.Equal(t, math.LegacyNewDec(poolAmt), ds.FeePool.CommunityPool[0].Amount)

	var bs banktypes.GenesisState
	require.NoError(t, ec.Codec.UnmarshalJSON(appGenState["bank"], &bs))
	assert.Equal(t, math.NewInt(poolAmt), bs.Supply.AmountOf("uatom"))

	distAddr, err := moduleAddress(testHRP, "distribution")
	require.NoError(t, err)
	var distBalance math.Int
	for _, b := range bs.Balances {
		if b.Address == distAddr {
			distBalance = b.Coins.AmountOf("uatom")
		}
	}
	assert.Equal(t, math.NewInt(poolAmt), distBalance)
}

func TestSetDistribution_CommunityPool_Absent_BankUnchanged(t *testing.T) {
	asm := distributionStateManager(t, nil, nil)

	ec := encoding.NewEncodingConfig()
	bankState := banktypes.DefaultGenesisState()
	bankStateBz, err := ec.Codec.MarshalJSON(bankState)
	require.NoError(t, err)
	appGenState := map[string]json.RawMessage{"bank": bankStateBz}

	require.NoError(t, asm.setDistribution(appGenState, nil))

	var ds distributiontypes.GenesisState
	require.NoError(t, ec.Codec.UnmarshalJSON(appGenState["distribution"], &ds))
	assert.Empty(t, ds.FeePool.CommunityPool)

	var bs banktypes.GenesisState
	require.NoError(t, ec.Codec.UnmarshalJSON(appGenState["bank"], &bs))
	assert.True(t, bs.Supply.IsZero())
}

func TestSetDistribution_HistoricalRewardsReferenceCount(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	v := testValidator(t, 3)
	delegations := []stakingtypes.Delegation{
		{
			DelegatorAddress: testAccAddr(51).String(),
			ValidatorAddress: v.OperatorAddress(),
			Shares:           math.LegacyNewDec(1_000_000),
		},
	}

	asm := distributionStateManager(t, []validator.Validator{v}, nil)
	appGenState := map[string]json.RawMessage{}
	require.NoError(t, asm.setDistribution(appGenState, delegations))

	var ds distributiontypes.GenesisState
	require.NoError(t, asm.encodingConfig.Codec.UnmarshalJSON(appGenState["distribution"], &ds))

	// First historical record has refCount decremented from 2→1 when a delegation was added.
	require.Len(t, ds.ValidatorHistoricalRewards, 2)
	assert.Equal(t, uint32(1), ds.ValidatorHistoricalRewards[0].Rewards.ReferenceCount)
	assert.Equal(t, uint32(2), ds.ValidatorHistoricalRewards[1].Rewards.ReferenceCount)
}
