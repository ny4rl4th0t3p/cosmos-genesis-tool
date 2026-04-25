package accounts_test

import (
	"os"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/accounts"
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

func TestNewInitialAccount_Valid(t *testing.T) {
	acc, err := accounts.NewInitialAccount(testAddr(1), 1000, testEncodingConfig)
	require.NoError(t, err)
	assert.Equal(t, testAddr(1), acc.Address())
	assert.Equal(t, int64(1000), acc.Amount())
}

func TestNewInitialAccount_InvalidAddress(t *testing.T) {
	_, err := accounts.NewInitialAccount("not-a-valid-address", 1000, testEncodingConfig)
	require.ErrorIs(t, err, accounts.ErrInvalidInitialAccount)
}

func TestNewInitialAccount_EmptyAddress(t *testing.T) {
	_, err := accounts.NewInitialAccount("", 1000, testEncodingConfig)
	require.ErrorIs(t, err, accounts.ErrInvalidInitialAccount)
}

func TestNewInitialAccount_ZeroAmount(t *testing.T) {
	// Zero amount is allowed for initial accounts (filtered at write time by appendInitialAccounts)
	acc, err := accounts.NewInitialAccount(testAddr(2), 0, testEncodingConfig)
	require.NoError(t, err)
	assert.Equal(t, int64(0), acc.Amount())
}

func TestNewInitialAccount_NegativeAmount(t *testing.T) {
	// Negative amounts are permitted at construction — domain only validates address
	acc, err := accounts.NewInitialAccount(testAddr(3), -1, testEncodingConfig)
	require.NoError(t, err)
	assert.Equal(t, int64(-1), acc.Amount())
}

func TestIsInRemainderAllowedList_InList(t *testing.T) {
	addr := testAddr(10)
	viper.Set("accounts.remainder_allowlist", []string{addr, testAddr(11)})
	t.Cleanup(func() { viper.Set("accounts.remainder_allowlist", nil) })

	acc, err := accounts.NewInitialAccount(addr, 500, testEncodingConfig)
	require.NoError(t, err)
	assert.True(t, acc.IsInRemainderAllowedList())
}

func TestIsInRemainderAllowedList_NotInList(t *testing.T) {
	viper.Set("accounts.remainder_allowlist", []string{testAddr(20)})
	t.Cleanup(func() { viper.Set("accounts.remainder_allowlist", nil) })

	acc, err := accounts.NewInitialAccount(testAddr(21), 500, testEncodingConfig)
	require.NoError(t, err)
	assert.False(t, acc.IsInRemainderAllowedList())
}

func TestIsInRemainderAllowedList_EmptyList(t *testing.T) {
	viper.Set("accounts.remainder_allowlist", []string{})
	t.Cleanup(func() { viper.Set("accounts.remainder_allowlist", nil) })

	acc, err := accounts.NewInitialAccount(testAddr(30), 500, testEncodingConfig)
	require.NoError(t, err)
	assert.False(t, acc.IsInRemainderAllowedList())
}

func TestIsInRemainderAllowedList_MultipleEntries(t *testing.T) {
	addr := testAddr(40)
	other := testAddr(41)
	viper.Set("accounts.remainder_allowlist", []string{other, addr, testAddr(42)})
	t.Cleanup(func() { viper.Set("accounts.remainder_allowlist", nil) })

	acc, err := accounts.NewInitialAccount(addr, 1, testEncodingConfig)
	require.NoError(t, err)
	assert.True(t, acc.IsInRemainderAllowedList())
}
