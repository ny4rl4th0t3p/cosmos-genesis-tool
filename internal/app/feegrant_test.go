package app

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	feegranttypes "cosmossdk.io/x/feegrant"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainfeegrant "github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/feegrant"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

type stubFeeAllowanceRepo struct {
	allowances []domainfeegrant.FeeAllowance
	err        error
}

func (s stubFeeAllowanceRepo) GetFeeAllowances(_ context.Context, _ encoding.EncodingConfig) ([]domainfeegrant.FeeAllowance, error) {
	return s.allowances, s.err
}

func makeFeeAllowance(t *testing.T, ec encoding.EncodingConfig, granterIdx, granteeIdx byte, spendLimit, expiry int64) domainfeegrant.FeeAllowance {
	t.Helper()
	granter := testAccAddr(granterIdx).String()
	grantee := testAccAddr(granteeIdx).String()
	a, err := domainfeegrant.NewFeeAllowance(granter, grantee, spendLimit, expiry, ec)
	require.NoError(t, err)
	return *a
}

func feegrantAppState(t *testing.T, ec encoding.EncodingConfig) map[string]json.RawMessage {
	t.Helper()
	gs := feegranttypes.DefaultGenesisState()
	bz, err := ec.Codec.MarshalJSON(gs)
	require.NoError(t, err)
	return map[string]json.RawMessage{"feegrant": bz}
}

func readFeegrantState(t *testing.T, appGenState map[string]json.RawMessage, ec encoding.EncodingConfig) *feegranttypes.GenesisState {
	t.Helper()
	var gs feegranttypes.GenesisState
	require.NoError(t, ec.Codec.UnmarshalJSON(appGenState["feegrant"], &gs))
	return &gs
}

func TestSetFeegrantState_NoViperKey_Skipped(t *testing.T) {
	ec := encoding.NewEncodingConfig()
	appGenState := feegrantAppState(t, ec)
	asm := StateManager{encodingConfig: ec, feeAllowanceRepository: stubFeeAllowanceRepo{}}

	require.NoError(t, asm.setFeegrantState(context.Background(), appGenState))

	gs := readFeegrantState(t, appGenState, ec)
	assert.Empty(t, gs.Allowances)
}

func TestSetFeegrantState_EmptyAllowances_Skipped(t *testing.T) {
	viper.Set("feegrant.file_name", "/tmp/feegrant.csv")
	t.Cleanup(func() { viper.Set("feegrant.file_name", nil) })

	ec := encoding.NewEncodingConfig()
	appGenState := feegrantAppState(t, ec)
	asm := StateManager{encodingConfig: ec, feeAllowanceRepository: stubFeeAllowanceRepo{}}

	require.NoError(t, asm.setFeegrantState(context.Background(), appGenState))

	gs := readFeegrantState(t, appGenState, ec)
	assert.Empty(t, gs.Allowances)
}

func TestSetFeegrantState_RepoError_ReturnsError(t *testing.T) {
	viper.Set("feegrant.file_name", "/tmp/feegrant.csv")
	t.Cleanup(func() { viper.Set("feegrant.file_name", nil) })

	ec := encoding.NewEncodingConfig()
	appGenState := feegrantAppState(t, ec)
	sentinel := errors.New("repo fail")
	asm := StateManager{encodingConfig: ec, feeAllowanceRepository: stubFeeAllowanceRepo{err: sentinel}}

	err := asm.setFeegrantState(context.Background(), appGenState)
	require.ErrorIs(t, err, sentinel)
}

func TestSetFeegrantState_NonZeroSpendLimit_WrittenToGenesis(t *testing.T) {
	viper.Set("feegrant.file_name", "/tmp/feegrant.csv")
	viper.Set("default_bond_denom", "uatom")
	t.Cleanup(func() {
		viper.Set("feegrant.file_name", nil)
		viper.Set("default_bond_denom", nil)
	})

	ec := encoding.NewEncodingConfig()
	a := makeFeeAllowance(t, ec, 1, 2, 5_000_000, 0)

	appGenState := feegrantAppState(t, ec)
	asm := StateManager{encodingConfig: ec, feeAllowanceRepository: stubFeeAllowanceRepo{allowances: []domainfeegrant.FeeAllowance{a}}}

	require.NoError(t, asm.setFeegrantState(context.Background(), appGenState))

	gs := readFeegrantState(t, appGenState, ec)
	require.Len(t, gs.Allowances, 1)
	assert.Equal(t, testAccAddr(1).String(), gs.Allowances[0].Granter)
	assert.Equal(t, testAccAddr(2).String(), gs.Allowances[0].Grantee)
	assert.Contains(t, gs.Allowances[0].Allowance.TypeUrl, "BasicAllowance")
}

func TestSetFeegrantState_ZeroSpendLimit_NilSpendLimitInBasicAllowance(t *testing.T) {
	viper.Set("feegrant.file_name", "/tmp/feegrant.csv")
	viper.Set("default_bond_denom", "uatom")
	t.Cleanup(func() {
		viper.Set("feegrant.file_name", nil)
		viper.Set("default_bond_denom", nil)
	})

	ec := encoding.NewEncodingConfig()
	a := makeFeeAllowance(t, ec, 1, 2, 0, 0)

	appGenState := feegrantAppState(t, ec)
	asm := StateManager{encodingConfig: ec, feeAllowanceRepository: stubFeeAllowanceRepo{allowances: []domainfeegrant.FeeAllowance{a}}}

	require.NoError(t, asm.setFeegrantState(context.Background(), appGenState))

	gs := readFeegrantState(t, appGenState, ec)
	require.Len(t, gs.Allowances, 1)
	assert.Contains(t, gs.Allowances[0].Allowance.TypeUrl, "BasicAllowance")

	// Cached value is populated after UnmarshalJSON; zero spend limit → SpendLimit is nil/empty.
	basic, ok := gs.Allowances[0].Allowance.GetCachedValue().(*feegranttypes.BasicAllowance)
	require.True(t, ok)
	assert.Empty(t, basic.SpendLimit)
	assert.Nil(t, basic.Expiration)
}

func TestSetFeegrantState_WithExpiry_ExpirationSet(t *testing.T) {
	viper.Set("feegrant.file_name", "/tmp/feegrant.csv")
	viper.Set("default_bond_denom", "uatom")
	t.Cleanup(func() {
		viper.Set("feegrant.file_name", nil)
		viper.Set("default_bond_denom", nil)
	})

	ec := encoding.NewEncodingConfig()
	a := makeFeeAllowance(t, ec, 1, 2, 1_000_000, 1900000000)

	appGenState := feegrantAppState(t, ec)
	asm := StateManager{encodingConfig: ec, feeAllowanceRepository: stubFeeAllowanceRepo{allowances: []domainfeegrant.FeeAllowance{a}}}

	require.NoError(t, asm.setFeegrantState(context.Background(), appGenState))

	gs := readFeegrantState(t, appGenState, ec)
	require.Len(t, gs.Allowances, 1)

	basic, ok := gs.Allowances[0].Allowance.GetCachedValue().(*feegranttypes.BasicAllowance)
	require.True(t, ok)
	require.NotNil(t, basic.Expiration)
	assert.Equal(t, time.Unix(1900000000, 0).UTC(), *basic.Expiration)
}
