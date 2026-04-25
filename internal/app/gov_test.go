package app

import (
	"encoding/json"
	"testing"
	"time"

	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

func govAppState(t *testing.T) (map[string]json.RawMessage, encoding.EncodingConfig) {
	t.Helper()
	ec := encoding.NewEncodingConfig()
	gs := govv1.DefaultGenesisState()
	bz, err := ec.Codec.MarshalJSON(gs)
	require.NoError(t, err)
	return map[string]json.RawMessage{"gov": bz}, ec
}

func readGovState(t *testing.T, appGenState map[string]json.RawMessage, ec encoding.EncodingConfig) *govv1.GenesisState {
	t.Helper()
	var gs govv1.GenesisState
	require.NoError(t, ec.Codec.UnmarshalJSON(appGenState["gov"], &gs))
	return &gs
}

func TestFixGovernanceParameters_NoViperKeys_NoChange(t *testing.T) {
	appGenState, ec := govAppState(t)
	asm := StateManager{encodingConfig: ec}
	require.NoError(t, asm.fixGovernanceParameters(appGenState))
	gs := readGovState(t, appGenState, ec)
	assert.NotNil(t, gs.Params)
}

func TestFixGovernanceParameters_MinDeposit(t *testing.T) {
	viper.Set("default_bond_denom", "uatom")
	viper.Set("gov.min_deposit_amount", int64(500_000))
	t.Cleanup(func() {
		viper.Set("default_bond_denom", nil)
		viper.Set("gov.min_deposit_amount", nil)
	})

	appGenState, ec := govAppState(t)
	asm := StateManager{encodingConfig: ec}
	require.NoError(t, asm.fixGovernanceParameters(appGenState))

	gs := readGovState(t, appGenState, ec)
	require.Len(t, gs.Params.MinDeposit, 1)
	assert.Equal(t, "uatom", gs.Params.MinDeposit[0].Denom)
	assert.Equal(t, int64(500_000), gs.Params.MinDeposit[0].Amount.Int64())
}

func TestFixGovernanceParameters_VotingPeriod(t *testing.T) {
	viper.Set("gov.voting_period", "72h")
	t.Cleanup(func() { viper.Set("gov.voting_period", nil) })

	appGenState, ec := govAppState(t)
	asm := StateManager{encodingConfig: ec}
	require.NoError(t, asm.fixGovernanceParameters(appGenState))

	gs := readGovState(t, appGenState, ec)
	require.NotNil(t, gs.Params.VotingPeriod)
	assert.Equal(t, 72*time.Hour, *gs.Params.VotingPeriod)
}

func TestFixGovernanceParameters_InvalidVotingPeriod_ReturnsError(t *testing.T) {
	viper.Set("gov.voting_period", "not-a-duration")
	t.Cleanup(func() { viper.Set("gov.voting_period", nil) })

	appGenState, ec := govAppState(t)
	asm := StateManager{encodingConfig: ec}
	err := asm.fixGovernanceParameters(appGenState)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid gov.voting_period")
}

func TestFixGovernanceParameters_ExpeditedParams(t *testing.T) {
	viper.Set("default_bond_denom", "uatom")
	viper.Set("gov.expedited_min_deposit_amount", int64(1_000_000))
	viper.Set("gov.expedited_voting_period", "24h")
	t.Cleanup(func() {
		viper.Set("default_bond_denom", nil)
		viper.Set("gov.expedited_min_deposit_amount", nil)
		viper.Set("gov.expedited_voting_period", nil)
	})

	appGenState, ec := govAppState(t)
	asm := StateManager{encodingConfig: ec}
	require.NoError(t, asm.fixGovernanceParameters(appGenState))

	gs := readGovState(t, appGenState, ec)
	require.Len(t, gs.Params.ExpeditedMinDeposit, 1)
	assert.Equal(t, int64(1_000_000), gs.Params.ExpeditedMinDeposit[0].Amount.Int64())
	require.NotNil(t, gs.Params.ExpeditedVotingPeriod)
	assert.Equal(t, 24*time.Hour, *gs.Params.ExpeditedVotingPeriod)
}
