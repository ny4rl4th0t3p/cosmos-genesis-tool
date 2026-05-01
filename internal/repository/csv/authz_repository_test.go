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

func writeTempAuthzCSV(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "authz.csv")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestGetAuthzGrants_ThreeFields_NoExpiry(t *testing.T) {
	granter := testAddr(200)
	grantee := testAddr(201)
	content := granter + "," + grantee + ",/cosmos.bank.v1beta1.MsgSend\n"
	path := writeTempAuthzCSV(t, content)

	repo := csv.NewCSVAuthzGrantRepository(path, map[string]bool{})
	grants, err := repo.GetAuthzGrants(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, granter, grants[0].Granter())
	assert.Equal(t, grantee, grants[0].Grantee())
	assert.Equal(t, "/cosmos.bank.v1beta1.MsgSend", grants[0].MsgTypeURL())
	assert.Equal(t, int64(0), grants[0].Expiry())
}

func TestGetAuthzGrants_FourFields_WithExpiry(t *testing.T) {
	granter := testAddr(200)
	grantee := testAddr(201)
	content := granter + "," + grantee + ",/cosmos.staking.v1beta1.MsgDelegate,1900000000\n"
	path := writeTempAuthzCSV(t, content)

	repo := csv.NewCSVAuthzGrantRepository(path, map[string]bool{})
	grants, err := repo.GetAuthzGrants(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, int64(1900000000), grants[0].Expiry())
}

func TestGetAuthzGrants_MultipleRows(t *testing.T) {
	granter := testAddr(200)
	grantee1 := testAddr(201)
	grantee2 := testAddr(202)
	content := granter + "," + grantee1 + ",/cosmos.bank.v1beta1.MsgSend\n" +
		granter + "," + grantee2 + ",/cosmos.staking.v1beta1.MsgDelegate,1900000000\n"
	path := writeTempAuthzCSV(t, content)

	repo := csv.NewCSVAuthzGrantRepository(path, map[string]bool{})
	grants, err := repo.GetAuthzGrants(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	assert.Len(t, grants, 2)
}

func TestGetAuthzGrants_EmptyFile(t *testing.T) {
	path := writeTempAuthzCSV(t, "")
	repo := csv.NewCSVAuthzGrantRepository(path, map[string]bool{})
	grants, err := repo.GetAuthzGrants(context.Background(), testEncodingConfig)
	require.NoError(t, err)
	assert.Empty(t, grants)
}

func TestGetAuthzGrants_TooFewFields(t *testing.T) {
	content := testAddr(200) + "," + testAddr(201) + "\n"
	path := writeTempAuthzCSV(t, content)

	repo := csv.NewCSVAuthzGrantRepository(path, map[string]bool{})
	_, err := repo.GetAuthzGrants(context.Background(), testEncodingConfig)
	require.Error(t, err)
}

func TestGetAuthzGrants_TooManyFields(t *testing.T) {
	content := testAddr(200) + "," + testAddr(201) + ",/type,1900000000,extra\n"
	path := writeTempAuthzCSV(t, content)

	repo := csv.NewCSVAuthzGrantRepository(path, map[string]bool{})
	_, err := repo.GetAuthzGrants(context.Background(), testEncodingConfig)
	require.Error(t, err)
}

func TestGetAuthzGrants_BadExpiry(t *testing.T) {
	content := testAddr(200) + "," + testAddr(201) + ",/type,not-a-number\n"
	path := writeTempAuthzCSV(t, content)

	repo := csv.NewCSVAuthzGrantRepository(path, map[string]bool{})
	_, err := repo.GetAuthzGrants(context.Background(), testEncodingConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid expiry")
}

func TestGetAuthzGrants_EmptyMsgTypeURL(t *testing.T) {
	content := testAddr(200) + "," + testAddr(201) + ",\n"
	path := writeTempAuthzCSV(t, content)

	repo := csv.NewCSVAuthzGrantRepository(path, map[string]bool{})
	_, err := repo.GetAuthzGrants(context.Background(), testEncodingConfig)
	require.Error(t, err)
}

func TestGetAuthzGrants_InvalidGranterAddress(t *testing.T) {
	content := "bad-address," + testAddr(201) + ",/cosmos.bank.v1beta1.MsgSend\n"
	path := writeTempAuthzCSV(t, content)

	repo := csv.NewCSVAuthzGrantRepository(path, map[string]bool{})
	_, err := repo.GetAuthzGrants(context.Background(), testEncodingConfig)
	require.Error(t, err)
}

func TestGetAuthzGrants_MissingFile(t *testing.T) {
	repo := csv.NewCSVAuthzGrantRepository("/nonexistent/authz.csv", map[string]bool{})
	_, err := repo.GetAuthzGrants(context.Background(), testEncodingConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open")
}

func TestGetAuthzGrants_ContextCancelled(t *testing.T) {
	content := testAddr(200) + "," + testAddr(201) + ",/cosmos.bank.v1beta1.MsgSend\n"
	path := writeTempAuthzCSV(t, content)
	repo := csv.NewCSVAuthzGrantRepository(path, map[string]bool{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.GetAuthzGrants(ctx, testEncodingConfig)
	require.ErrorIs(t, err, context.Canceled)
}
