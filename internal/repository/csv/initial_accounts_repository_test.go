package csv_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/repository/csv"
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

func writeTempCSV(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.csv")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestGetInitialAccounts_Valid(t *testing.T) {
	content := testAddr(1) + ",1000000\n" + testAddr(2) + ",500000\n"
	path := writeTempCSV(t, content)

	repo := csv.NewCSVInitialAccountsRepository(path, map[string]bool{})
	accs, err := repo.GetInitialAccounts(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	require.Len(t, accs, 2)
	assert.Equal(t, testAddr(1), accs[0].Address())
	assert.Equal(t, int64(1000000), accs[0].Amount())
	assert.Equal(t, testAddr(2), accs[1].Address())
}

func TestGetInitialAccounts_ModuleAddressFiltered(t *testing.T) {
	moduleAddr := testAddr(10)
	content := testAddr(1) + ",1000\n" + moduleAddr + ",9999\n"
	path := writeTempCSV(t, content)

	moduleAddresses := map[string]bool{moduleAddr: true}
	repo := csv.NewCSVInitialAccountsRepository(path, moduleAddresses)
	accs, err := repo.GetInitialAccounts(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	require.Len(t, accs, 1)
	assert.Equal(t, testAddr(1), accs[0].Address())
}

func TestGetInitialAccounts_EmptyFile(t *testing.T) {
	path := writeTempCSV(t, "")
	repo := csv.NewCSVInitialAccountsRepository(path, map[string]bool{})
	accs, err := repo.GetInitialAccounts(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	assert.Empty(t, accs)
}

func TestGetInitialAccounts_InvalidAmount(t *testing.T) {
	content := testAddr(1) + ",not-a-number\n"
	path := writeTempCSV(t, content)

	repo := csv.NewCSVInitialAccountsRepository(path, map[string]bool{})
	_, err := repo.GetInitialAccounts(context.Background(), testEncodingConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid amount")
}

func TestGetInitialAccounts_InvalidAddress(t *testing.T) {
	content := "not-an-address,1000\n"
	path := writeTempCSV(t, content)

	repo := csv.NewCSVInitialAccountsRepository(path, map[string]bool{})
	_, err := repo.GetInitialAccounts(context.Background(), testEncodingConfig)
	require.Error(t, err)
}

func TestGetInitialAccounts_MissingFile(t *testing.T) {
	repo := csv.NewCSVInitialAccountsRepository("/nonexistent/path/accounts.csv", map[string]bool{})
	_, err := repo.GetInitialAccounts(context.Background(), testEncodingConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open")
}

func TestGetInitialAccounts_ContextCancelled(t *testing.T) {
	path := writeTempCSV(t, testAddr(1)+",1000\n")
	repo := csv.NewCSVInitialAccountsRepository(path, map[string]bool{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.GetInitialAccounts(ctx, testEncodingConfig)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestGetInitialAccounts_WhitespaceTrimmed(t *testing.T) {
	addr := testAddr(3)
	content := " " + addr + " , 750000 \n"
	path := writeTempCSV(t, content)

	repo := csv.NewCSVInitialAccountsRepository(path, map[string]bool{})
	accs, err := repo.GetInitialAccounts(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	require.Len(t, accs, 1)
	assert.Equal(t, addr, accs[0].Address())
	assert.Equal(t, int64(750000), accs[0].Amount())
}

func TestGetInitialAccounts_AddressOnlyNoAmount(t *testing.T) {
	// single-column row → amount 0
	content := testAddr(5) + "\n"
	path := writeTempCSV(t, content)

	repo := csv.NewCSVInitialAccountsRepository(path, map[string]bool{})
	accs, err := repo.GetInitialAccounts(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	require.Len(t, accs, 1)
	assert.Equal(t, int64(0), accs[0].Amount())
}
