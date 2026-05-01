package csv_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/repository/csv"
)

func writeTempFeegrantCSV(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "feegrant.csv")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestGetFeeAllowances_ThreeFields_WithSpendLimit(t *testing.T) {
	granter := testAddr(210)
	grantee := testAddr(211)
	content := granter + "," + grantee + ",5000000\n"
	path := writeTempFeegrantCSV(t, content)

	repo := csv.NewCSVFeeAllowanceRepository(path, map[string]bool{})
	allowances, err := repo.GetFeeAllowances(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	require.Len(t, allowances, 1)
	assert.Equal(t, granter, allowances[0].Granter())
	assert.Equal(t, grantee, allowances[0].Grantee())
	assert.Equal(t, int64(5000000), allowances[0].SpendLimit())
	assert.Equal(t, int64(0), allowances[0].Expiry())
}

func TestGetFeeAllowances_ZeroSpendLimit_NoLimit(t *testing.T) {
	granter := testAddr(210)
	grantee := testAddr(211)
	content := granter + "," + grantee + ",0\n"
	path := writeTempFeegrantCSV(t, content)

	repo := csv.NewCSVFeeAllowanceRepository(path, map[string]bool{})
	allowances, err := repo.GetFeeAllowances(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	require.Len(t, allowances, 1)
	assert.Equal(t, int64(0), allowances[0].SpendLimit())
}

func TestGetFeeAllowances_FourFields_WithExpiry(t *testing.T) {
	granter := testAddr(210)
	grantee := testAddr(211)
	content := granter + "," + grantee + ",1000000,1900000000\n"
	path := writeTempFeegrantCSV(t, content)

	repo := csv.NewCSVFeeAllowanceRepository(path, map[string]bool{})
	allowances, err := repo.GetFeeAllowances(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	require.Len(t, allowances, 1)
	assert.Equal(t, int64(1900000000), allowances[0].Expiry())
}

func TestGetFeeAllowances_MultipleRows(t *testing.T) {
	granter := testAddr(210)
	grantee1 := testAddr(211)
	grantee2 := testAddr(212)
	content := granter + "," + grantee1 + ",1000000\n" +
		granter + "," + grantee2 + ",0,1900000000\n"
	path := writeTempFeegrantCSV(t, content)

	repo := csv.NewCSVFeeAllowanceRepository(path, map[string]bool{})
	allowances, err := repo.GetFeeAllowances(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	assert.Len(t, allowances, 2)
}

func TestGetFeeAllowances_EmptyFile(t *testing.T) {
	path := writeTempFeegrantCSV(t, "")
	repo := csv.NewCSVFeeAllowanceRepository(path, map[string]bool{})
	allowances, err := repo.GetFeeAllowances(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	assert.Empty(t, allowances)
}

func TestGetFeeAllowances_TooFewFields(t *testing.T) {
	content := testAddr(210) + "," + testAddr(211) + "\n"
	path := writeTempFeegrantCSV(t, content)

	repo := csv.NewCSVFeeAllowanceRepository(path, map[string]bool{})
	_, err := repo.GetFeeAllowances(context.Background(), testEncodingConfig)
	require.Error(t, err)
}

func TestGetFeeAllowances_TooManyFields(t *testing.T) {
	content := testAddr(210) + "," + testAddr(211) + ",1000,1900000000,extra\n"
	path := writeTempFeegrantCSV(t, content)

	repo := csv.NewCSVFeeAllowanceRepository(path, map[string]bool{})
	_, err := repo.GetFeeAllowances(context.Background(), testEncodingConfig)
	require.Error(t, err)
}

func TestGetFeeAllowances_BadSpendLimit(t *testing.T) {
	content := testAddr(210) + "," + testAddr(211) + ",not-a-number\n"
	path := writeTempFeegrantCSV(t, content)

	repo := csv.NewCSVFeeAllowanceRepository(path, map[string]bool{})
	_, err := repo.GetFeeAllowances(context.Background(), testEncodingConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid spend_limit")
}

func TestGetFeeAllowances_BadExpiry(t *testing.T) {
	content := testAddr(210) + "," + testAddr(211) + ",1000,not-a-number\n"
	path := writeTempFeegrantCSV(t, content)

	repo := csv.NewCSVFeeAllowanceRepository(path, map[string]bool{})
	_, err := repo.GetFeeAllowances(context.Background(), testEncodingConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid expiry")
}

func TestGetFeeAllowances_InvalidGranterAddress(t *testing.T) {
	content := "bad-address," + testAddr(211) + ",1000000\n"
	path := writeTempFeegrantCSV(t, content)

	repo := csv.NewCSVFeeAllowanceRepository(path, map[string]bool{})
	_, err := repo.GetFeeAllowances(context.Background(), testEncodingConfig)
	require.Error(t, err)
}

func TestGetFeeAllowances_MissingFile(t *testing.T) {
	repo := csv.NewCSVFeeAllowanceRepository("/nonexistent/feegrant.csv", map[string]bool{})
	_, err := repo.GetFeeAllowances(context.Background(), testEncodingConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open")
}

func TestGetFeeAllowances_ContextCancelled(t *testing.T) {
	content := testAddr(210) + "," + testAddr(211) + ",1000000\n"
	path := writeTempFeegrantCSV(t, content)
	repo := csv.NewCSVFeeAllowanceRepository(path, map[string]bool{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.GetFeeAllowances(ctx, testEncodingConfig)
	require.ErrorIs(t, err, context.Canceled)
}
