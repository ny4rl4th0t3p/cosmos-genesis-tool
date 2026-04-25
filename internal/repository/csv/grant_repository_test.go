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

func writeTempGrantCSV(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "grants.csv")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestGetGrants_Valid(t *testing.T) {
	content := testAddr(60) + ",2000000\n" + testAddr(61) + ",1000000\n"
	path := writeTempGrantCSV(t, content)

	repo := csv.NewCSVGrantRepository(path, map[string]bool{})
	grants, err := repo.GetGrants(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	require.Len(t, grants, 2)
	assert.Equal(t, testAddr(60), grants[0].Address())
	assert.Equal(t, int64(2000000), grants[0].Amount())
	assert.Empty(t, grants[0].DelegateTo())
}

func TestGetGrants_ModuleAddressFiltered(t *testing.T) {
	moduleAddr := testAddr(70)
	content := testAddr(60) + ",1000\n" + moduleAddr + ",9999\n"
	path := writeTempGrantCSV(t, content)

	repo := csv.NewCSVGrantRepository(path, map[string]bool{moduleAddr: true})
	grants, err := repo.GetGrants(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	require.Len(t, grants, 1)
}

func TestGetGrants_EmptyFile(t *testing.T) {
	path := writeTempGrantCSV(t, "")
	repo := csv.NewCSVGrantRepository(path, map[string]bool{})
	grants, err := repo.GetGrants(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	assert.Empty(t, grants)
}

func TestGetGrants_InvalidAmount(t *testing.T) {
	content := testAddr(60) + ",bad\n"
	path := writeTempGrantCSV(t, content)

	repo := csv.NewCSVGrantRepository(path, map[string]bool{})
	_, err := repo.GetGrants(context.Background(), testEncodingConfig)
	require.Error(t, err)
}

func TestGetGrants_ZeroAmount(t *testing.T) {
	content := testAddr(60) + ",0\n"
	path := writeTempGrantCSV(t, content)

	repo := csv.NewCSVGrantRepository(path, map[string]bool{})
	_, err := repo.GetGrants(context.Background(), testEncodingConfig)
	require.Error(t, err)
}

func TestGetGrants_WrongColumnCount(t *testing.T) {
	content := testAddr(60) + "\n" // single column → invalid
	path := writeTempGrantCSV(t, content)

	repo := csv.NewCSVGrantRepository(path, map[string]bool{})
	_, err := repo.GetGrants(context.Background(), testEncodingConfig)
	require.Error(t, err)
}

func TestGetGrants_MissingFile(t *testing.T) {
	repo := csv.NewCSVGrantRepository("/nonexistent/grants.csv", map[string]bool{})
	_, err := repo.GetGrants(context.Background(), testEncodingConfig)
	require.Error(t, err)
}

func TestGetGrants_ContextCancelled(t *testing.T) {
	path := writeTempGrantCSV(t, testAddr(60)+",1000\n")
	repo := csv.NewCSVGrantRepository(path, map[string]bool{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.GetGrants(ctx, testEncodingConfig)
	require.ErrorIs(t, err, context.Canceled)
}
