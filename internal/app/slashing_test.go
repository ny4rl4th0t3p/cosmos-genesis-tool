package app

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/validator"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

func slashingAppState(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	ec := encoding.NewEncodingConfig()
	gs := slashingtypes.DefaultGenesisState()
	bz, err := ec.Codec.MarshalJSON(gs)
	require.NoError(t, err)
	return map[string]json.RawMessage{"slashing": bz}
}

func slashingStateManager(t *testing.T, validators []validator.Validator, repoErr error) StateManager {
	t.Helper()
	ec := encoding.NewEncodingConfig()
	return StateManager{
		encodingConfig:      ec,
		validatorRepository: stubValidatorRepo{validators: validators, err: repoErr},
	}
}

func readSlashingState(t *testing.T, appGenState map[string]json.RawMessage, ec encoding.EncodingConfig) *slashingtypes.GenesisState {
	t.Helper()
	var gs slashingtypes.GenesisState
	require.NoError(t, ec.Codec.UnmarshalJSON(appGenState["slashing"], &gs))
	return &gs
}

func TestSetSlashingState_ValidatorRepoError(t *testing.T) {
	sentinel := errors.New("repo fail")
	asm := slashingStateManager(t, nil, sentinel)
	err := asm.setSlashingState(slashingAppState(t))
	require.ErrorIs(t, err, sentinel)
}

func TestSetSlashingState_NoValidators_EmptySigningInfos(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	asm := slashingStateManager(t, nil, nil)
	appGenState := slashingAppState(t)
	require.NoError(t, asm.setSlashingState(appGenState))

	gs := readSlashingState(t, appGenState, asm.encodingConfig)
	assert.Empty(t, gs.SigningInfos)
	assert.Empty(t, gs.MissedBlocks)
}

func TestSetSlashingState_SingleValidator_SigningInfoPopulated(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	v := testValidator(t, 1)
	asm := slashingStateManager(t, []validator.Validator{v}, nil)
	appGenState := slashingAppState(t)
	require.NoError(t, asm.setSlashingState(appGenState))

	gs := readSlashingState(t, appGenState, asm.encodingConfig)
	require.Len(t, gs.SigningInfos, 1)
	require.Len(t, gs.MissedBlocks, 1)

	info := gs.SigningInfos[0]
	assert.NotEmpty(t, info.Address)
	assert.Contains(t, info.Address, testHRP+valconsHRPSuffix)
	assert.Equal(t, info.Address, info.ValidatorSigningInfo.Address)
	assert.False(t, info.ValidatorSigningInfo.Tombstoned)
	assert.Zero(t, info.ValidatorSigningInfo.MissedBlocksCounter)
	assert.Empty(t, gs.MissedBlocks[0].MissedBlocks)
}

func TestSetSlashingState_MultipleValidators(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	v1 := testValidator(t, 2)
	v2 := testValidator(t, 3)
	asm := slashingStateManager(t, []validator.Validator{v1, v2}, nil)
	appGenState := slashingAppState(t)
	require.NoError(t, asm.setSlashingState(appGenState))

	gs := readSlashingState(t, appGenState, asm.encodingConfig)
	assert.Len(t, gs.SigningInfos, 2)
	assert.Len(t, gs.MissedBlocks, 2)
}

func TestSetSlashingState_ViperParams_Applied(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	viper.Set("slashing.signed_blocks_window", int64(10000))
	viper.Set("slashing.min_signed_per_window", "0.05")
	viper.Set("slashing.downtime_jail_duration_seconds", int64(600))
	viper.Set("slashing.slash_fraction_double_sign", "0.05")
	viper.Set("slashing.slash_fraction_downtime", "0.0001")
	t.Cleanup(func() {
		viper.Set("chain.address_prefix", nil)
		viper.Set("slashing.signed_blocks_window", nil)
		viper.Set("slashing.min_signed_per_window", nil)
		viper.Set("slashing.downtime_jail_duration_seconds", nil)
		viper.Set("slashing.slash_fraction_double_sign", nil)
		viper.Set("slashing.slash_fraction_downtime", nil)
	})

	asm := slashingStateManager(t, nil, nil)
	appGenState := slashingAppState(t)
	require.NoError(t, asm.setSlashingState(appGenState))

	gs := readSlashingState(t, appGenState, asm.encodingConfig)
	assert.Equal(t, int64(10000), gs.Params.SignedBlocksWindow)
	assert.Equal(t, "0.050000000000000000", gs.Params.MinSignedPerWindow.String())
	assert.Equal(t, 600*time.Second, gs.Params.DowntimeJailDuration)
	assert.Equal(t, "0.050000000000000000", gs.Params.SlashFractionDoubleSign.String())
	assert.Equal(t, "0.000100000000000000", gs.Params.SlashFractionDowntime.String())
}
