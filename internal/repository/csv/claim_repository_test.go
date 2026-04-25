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

func writeTempClaimCSV(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "claims.csv")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestGetClaims_Valid_TwoFields(t *testing.T) {
	content := testAddr(1) + ",1000000\n" + testAddr(2) + ",500000\n"
	path := writeTempClaimCSV(t, content)

	repo := csv.NewCSVClaimRepository(path, map[string]bool{})
	claims, err := repo.GetClaims(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	require.Len(t, claims, 2)
	assert.Equal(t, testAddr(1), claims[0].Address())
	assert.Equal(t, int64(1000000), claims[0].Amount())
	assert.Empty(t, claims[0].DelegateTo())
}

func TestGetClaims_Valid_ThreeFields_WithDelegate(t *testing.T) {
	delegate := testAddr(99)
	content := testAddr(1) + ",5000000," + delegate + "\n"
	path := writeTempClaimCSV(t, content)

	repo := csv.NewCSVClaimRepository(path, map[string]bool{})
	claims, err := repo.GetClaims(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	assert.Equal(t, delegate, claims[0].DelegateTo())
}

func TestGetClaims_MixedFieldCounts(t *testing.T) {
	// first row has 3 fields, second has 2 — FieldsPerRecord=-1 allows this
	content := testAddr(1) + ",900000," + testAddr(99) + "\n" + testAddr(2) + ",1000000\n"
	path := writeTempClaimCSV(t, content)

	repo := csv.NewCSVClaimRepository(path, map[string]bool{})
	claims, err := repo.GetClaims(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	require.Len(t, claims, 2)
	assert.Equal(t, testAddr(99), claims[0].DelegateTo())
	assert.Empty(t, claims[1].DelegateTo())
}

func TestGetClaims_ModuleAddressFiltered(t *testing.T) {
	moduleAddr := testAddr(10)
	content := testAddr(1) + ",1000\n" + moduleAddr + ",9999\n"
	path := writeTempClaimCSV(t, content)

	repo := csv.NewCSVClaimRepository(path, map[string]bool{moduleAddr: true})
	claims, err := repo.GetClaims(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	assert.Equal(t, testAddr(1), claims[0].Address())
}

func TestGetClaims_EmptyFile(t *testing.T) {
	path := writeTempClaimCSV(t, "")
	repo := csv.NewCSVClaimRepository(path, map[string]bool{})
	claims, err := repo.GetClaims(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	assert.Empty(t, claims)
}

func TestGetClaims_InvalidAmount(t *testing.T) {
	content := testAddr(1) + ",not-a-number\n"
	path := writeTempClaimCSV(t, content)

	repo := csv.NewCSVClaimRepository(path, map[string]bool{})
	_, err := repo.GetClaims(context.Background(), testEncodingConfig)
	require.Error(t, err)
}

func TestGetClaims_ZeroAmount(t *testing.T) {
	content := testAddr(1) + ",0\n"
	path := writeTempClaimCSV(t, content)

	repo := csv.NewCSVClaimRepository(path, map[string]bool{})
	_, err := repo.GetClaims(context.Background(), testEncodingConfig)
	require.Error(t, err)
}

func TestGetClaims_TooFewFields(t *testing.T) {
	content := testAddr(1) + "\n"
	path := writeTempClaimCSV(t, content)

	repo := csv.NewCSVClaimRepository(path, map[string]bool{})
	_, err := repo.GetClaims(context.Background(), testEncodingConfig)
	require.Error(t, err)
}

func TestGetClaims_TooManyFields(t *testing.T) {
	content := testAddr(1) + ",1000,extra,extra2\n"
	path := writeTempClaimCSV(t, content)

	repo := csv.NewCSVClaimRepository(path, map[string]bool{})
	_, err := repo.GetClaims(context.Background(), testEncodingConfig)
	require.Error(t, err)
}

func TestGetClaims_MissingFile(t *testing.T) {
	repo := csv.NewCSVClaimRepository("/nonexistent/claims.csv", map[string]bool{})
	_, err := repo.GetClaims(context.Background(), testEncodingConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open")
}

func TestGetClaims_ContextCancelled(t *testing.T) {
	path := writeTempClaimCSV(t, testAddr(1)+",1000\n")
	repo := csv.NewCSVClaimRepository(path, map[string]bool{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.GetClaims(ctx, testEncodingConfig)
	require.ErrorIs(t, err, context.Canceled)
}

func TestGetClaims_InvalidAddress(t *testing.T) {
	content := "bad-address,1000\n"
	path := writeTempClaimCSV(t, content)

	repo := csv.NewCSVClaimRepository(path, map[string]bool{})
	_, err := repo.GetClaims(context.Background(), testEncodingConfig)
	require.Error(t, err)
}
