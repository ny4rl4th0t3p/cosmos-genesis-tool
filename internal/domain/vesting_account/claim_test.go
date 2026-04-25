package vesting_account_test

import (
	"os"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/vesting_account"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

const testHRP = "cosmos"

var testEncodingConfig encoding.EncodingConfig

func TestMain(m *testing.M) {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount(testHRP, testHRP+"pub")
	cfg.SetBech32PrefixForValidator(testHRP+"valoper", testHRP+"valoperpub")
	cfg.SetBech32PrefixForConsensusNode(testHRP+"valcons", testHRP+"valconspub")
	cfg.Seal()
	testEncodingConfig = encoding.NewEncodingConfig()
	os.Exit(m.Run())
}

func testAddr(i byte) string {
	raw := make([]byte, 20)
	raw[19] = i
	addr, err := bech32.ConvertAndEncode(testHRP, raw)
	if err != nil {
		panic(err)
	}
	return addr
}

func TestNewClaim_Valid(t *testing.T) {
	claim, err := vesting_account.NewClaim(testAddr(1), 1000, "", testEncodingConfig)
	require.NoError(t, err)
	assert.Equal(t, testAddr(1), claim.Address())
	assert.Equal(t, int64(1000), claim.Amount())
	assert.Empty(t, claim.DelegateTo())
}

func TestNewClaim_WithDelegate(t *testing.T) {
	delegate := testAddr(99)
	claim, err := vesting_account.NewClaim(testAddr(1), 5000, delegate, testEncodingConfig)
	require.NoError(t, err)
	assert.Equal(t, delegate, claim.DelegateTo())
}

func TestNewClaim_InvalidAddress(t *testing.T) {
	_, err := vesting_account.NewClaim("bad-address", 1000, "", testEncodingConfig)
	require.ErrorIs(t, err, vesting_account.ErrInvalidClaim)
}

func TestNewClaim_EmptyAddress(t *testing.T) {
	_, err := vesting_account.NewClaim("", 1000, "", testEncodingConfig)
	require.ErrorIs(t, err, vesting_account.ErrInvalidClaim)
}

func TestNewClaim_ZeroAmount(t *testing.T) {
	_, err := vesting_account.NewClaim(testAddr(2), 0, "", testEncodingConfig)
	require.ErrorIs(t, err, vesting_account.ErrInvalidClaim)
}

func TestNewClaim_NegativeAmount(t *testing.T) {
	_, err := vesting_account.NewClaim(testAddr(2), -100, "", testEncodingConfig)
	require.ErrorIs(t, err, vesting_account.ErrInvalidClaim)
}

func TestNewClaim_MinimalValidAmount(t *testing.T) {
	claim, err := vesting_account.NewClaim(testAddr(3), 1, "", testEncodingConfig)
	require.NoError(t, err)
	assert.Equal(t, int64(1), claim.Amount())
}
