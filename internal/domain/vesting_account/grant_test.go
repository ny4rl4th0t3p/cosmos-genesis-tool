package vesting_account_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/vesting_account"
)

func TestNewGrant_Valid(t *testing.T) {
	grant, err := vesting_account.NewGrant(testAddr(50), 2000, testEncodingConfig)
	require.NoError(t, err)
	assert.Equal(t, testAddr(50), grant.Address())
	assert.Equal(t, int64(2000), grant.Amount())
}

func TestNewGrant_DelegateToAlwaysEmpty(t *testing.T) {
	grant, err := vesting_account.NewGrant(testAddr(50), 2000, testEncodingConfig)
	require.NoError(t, err)
	assert.Empty(t, grant.DelegateTo())
}

func TestNewGrant_InvalidAddress(t *testing.T) {
	_, err := vesting_account.NewGrant("not-valid", 1000, testEncodingConfig)
	require.ErrorIs(t, err, vesting_account.ErrInvalidGrant)
}

func TestNewGrant_EmptyAddress(t *testing.T) {
	_, err := vesting_account.NewGrant("", 1000, testEncodingConfig)
	require.ErrorIs(t, err, vesting_account.ErrInvalidGrant)
}

func TestNewGrant_ZeroAmount(t *testing.T) {
	_, err := vesting_account.NewGrant(testAddr(51), 0, testEncodingConfig)
	require.ErrorIs(t, err, vesting_account.ErrInvalidGrant)
}

func TestNewGrant_NegativeAmount(t *testing.T) {
	_, err := vesting_account.NewGrant(testAddr(51), -1, testEncodingConfig)
	require.ErrorIs(t, err, vesting_account.ErrInvalidGrant)
}

func TestNewGrant_MinimalValidAmount(t *testing.T) {
	grant, err := vesting_account.NewGrant(testAddr(52), 1, testEncodingConfig)
	require.NoError(t, err)
	assert.Equal(t, int64(1), grant.Amount())
}
