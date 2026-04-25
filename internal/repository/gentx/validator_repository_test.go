package gentx_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/repository/gentx"
)

const testHRP = "cosmos"

func TestMain(m *testing.M) {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount(testHRP, testHRP+"pub")
	cfg.SetBech32PrefixForValidator(testHRP+"valoper", testHRP+"valoperpub")
	cfg.SetBech32PrefixForConsensusNode(testHRP+"valcons", testHRP+"valconspub")
	cfg.Seal()
	os.Exit(m.Run())
}

// testOperatorAddr returns a cosmosvaloper bech32 address for index i.
func testOperatorAddr(i byte) string {
	raw := make([]byte, 20)
	raw[19] = i
	addr, err := bech32.ConvertAndEncode(testHRP+"valoper", raw)
	if err != nil {
		panic(err)
	}
	return addr
}

// testConsensusPubKeyB64 returns a base64-encoded 32-byte ed25519 pubkey for index i.
func testConsensusPubKeyB64(i byte) string {
	raw := make([]byte, 32)
	raw[0] = i + 1 // avoid all-zeros which is still valid but distinct per index
	return base64.StdEncoding.EncodeToString(raw)
}

// testOperatorPubKeyB64 returns a base64-encoded 33-byte secp256k1 operator pubkey for index i.
func testOperatorPubKeyB64(i byte) string {
	raw := make([]byte, 33)
	raw[0] = i + 1
	return base64.StdEncoding.EncodeToString(raw)
}

// gentxJSON builds a minimal valid gentx JSON string.
func gentxJSON(operatorAddr, consensusPubKeyB64, operatorPubKeyB64, moniker string) string {
	return fmt.Sprintf(`{
		"body": {
			"messages": [{
				"description": {
					"moniker": %q,
					"identity": "",
					"website": "",
					"security_contact": "",
					"details": ""
				},
				"validator_address": %q,
				"pubkey": {
					"@type": "/cosmos.crypto.ed25519.PubKey",
					"key": %q
				},
				"value": {"denom": "uatom", "amount": "1000000"},
				"commission": {
					"rate": "0.10",
					"max_rate": "0.20",
					"max_change_rate": "0.01"
				},
				"min_self_delegation": "1"
			}],
			"memo": "nodeid@localhost:26656"
		},
		"auth_info": {
			"signer_infos": [{
				"public_key": {
					"@type": "/cosmos.crypto.secp256k1.PubKey",
					"key": %q
				},
				"mode_info": {"single": {"mode": "SIGN_MODE_DIRECT"}},
				"sequence": "0"
			}],
			"fee": {}
		},
		"signatures": ["sig=="]
	}`, moniker, operatorAddr, consensusPubKeyB64, operatorPubKeyB64)
}

// writeTempGentxDir creates a temp directory containing the given JSON files.
func writeTempGentxDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
	return dir
}

func TestGetValidators_Valid_SingleFile(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	dir := writeTempGentxDir(t, map[string]string{
		"gentx-val1.json": gentxJSON(testOperatorAddr(1), testConsensusPubKeyB64(1), testOperatorPubKeyB64(1), "my-validator"),
	})

	repo := gentx.NewValidatorRepository(dir)
	validators, err := repo.GetValidators(context.Background())
	require.NoError(t, err)
	require.Len(t, validators, 1)
	assert.Equal(t, testOperatorAddr(1), validators[0].OperatorAddress())
	assert.Equal(t, "my-validator", validators[0].Name())
	assert.Equal(t, int64(1_000_000), validators[0].Amount())
	assert.Equal(t, testConsensusPubKeyB64(1), validators[0].PubKey())
	assert.Equal(t, testOperatorPubKeyB64(1), validators[0].OperatorPublicKey())
}

func TestGetValidators_AllFieldsParsed(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	json := fmt.Sprintf(`{
		"body": {
			"messages": [{
				"description": {
					"moniker": "full-validator",
					"identity": "ABCD1234",
					"website": "https://example.com",
					"security_contact": "security@example.com",
					"details": "A detailed validator"
				},
				"validator_address": %q,
				"pubkey": {
					"@type": "/cosmos.crypto.ed25519.PubKey",
					"key": %q
				},
				"value": {"denom": "uatom", "amount": "2000000"},
				"commission": {
					"rate": "0.05",
					"max_rate": "0.15",
					"max_change_rate": "0.01"
				},
				"min_self_delegation": "1"
			}],
			"memo": "testnode@localhost:26656"
		},
		"auth_info": {
			"signer_infos": [{
				"public_key": {
					"@type": "/cosmos.crypto.secp256k1.PubKey",
					"key": %q
				},
				"mode_info": {"single": {"mode": "SIGN_MODE_DIRECT"}},
				"sequence": "0"
			}],
			"fee": {}
		},
		"signatures": ["sig=="]
	}`, testOperatorAddr(2), testConsensusPubKeyB64(2), testOperatorPubKeyB64(2))

	dir := writeTempGentxDir(t, map[string]string{"gentx-full.json": json})

	repo := gentx.NewValidatorRepository(dir)
	validators, err := repo.GetValidators(context.Background())
	require.NoError(t, err)
	require.Len(t, validators, 1)
	v := validators[0]
	assert.Equal(t, "full-validator", v.Name())
	assert.Equal(t, "ABCD1234", v.Identity())
	assert.Equal(t, "https://example.com", v.Website())
	assert.Equal(t, "security@example.com", v.SecurityContact())
	assert.Equal(t, "A detailed validator", v.Details())
	assert.Equal(t, "0.05", v.CommissionRate())
	assert.Equal(t, "0.15", v.MaxRate())
	assert.Equal(t, "0.01", v.MaxChangeRate())
	assert.Equal(t, int64(2_000_000), v.Amount())
	assert.Equal(t, testOperatorPubKeyB64(2), v.OperatorPublicKey())
}

func TestGetValidators_MultipleFiles(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	dir := writeTempGentxDir(t, map[string]string{
		"gentx-val1.json": gentxJSON(testOperatorAddr(3), testConsensusPubKeyB64(3), testOperatorPubKeyB64(3), "validator-one"),
		"gentx-val2.json": gentxJSON(testOperatorAddr(4), testConsensusPubKeyB64(4), testOperatorPubKeyB64(4), "validator-two"),
	})

	repo := gentx.NewValidatorRepository(dir)
	validators, err := repo.GetValidators(context.Background())
	require.NoError(t, err)
	assert.Len(t, validators, 2)
}

func TestGetValidators_EmptyDirectory(t *testing.T) {
	repo := gentx.NewValidatorRepository(t.TempDir())
	_, err := repo.GetValidators(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no JSON files found")
}

func TestGetValidators_NonExistentDirectory(t *testing.T) {
	repo := gentx.NewValidatorRepository("/nonexistent/path/gentx")
	_, err := repo.GetValidators(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no JSON files found")
}

func TestGetValidators_MalformedJSON(t *testing.T) {
	dir := writeTempGentxDir(t, map[string]string{
		"gentx-bad.json": `{not valid json`,
	})

	repo := gentx.NewValidatorRepository(dir)
	_, err := repo.GetValidators(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse validators")
}

func TestGetValidators_InvalidAmount(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	json := fmt.Sprintf(`{
		"body": {
			"messages": [{
				"description": {"moniker": "v", "identity": "", "website": "", "security_contact": "", "details": ""},
				"validator_address": %q,
				"pubkey": {"@type": "/cosmos.crypto.ed25519.PubKey", "key": %q},
				"value": {"denom": "uatom", "amount": "not-a-number"},
				"commission": {"rate": "0.10", "max_rate": "0.20", "max_change_rate": "0.01"},
				"min_self_delegation": "1"
			}],
			"memo": ""
		},
		"auth_info": {"signer_infos": [], "fee": {}},
		"signatures": []
	}`, testOperatorAddr(5), testConsensusPubKeyB64(5))

	dir := writeTempGentxDir(t, map[string]string{"gentx-bad-amount.json": json})

	repo := gentx.NewValidatorRepository(dir)
	_, err := repo.GetValidators(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid amount")
}

func TestGetValidators_MissingSignerInfos_Errors(t *testing.T) {
	// operatorPublicKey is validated non-empty; absent signer_infos produces ErrInvalidValidator.
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	json := fmt.Sprintf(`{
		"body": {
			"messages": [{
				"description": {"moniker": "v", "identity": "", "website": "", "security_contact": "", "details": ""},
				"validator_address": %q,
				"pubkey": {"@type": "/cosmos.crypto.ed25519.PubKey", "key": %q},
				"value": {"denom": "uatom", "amount": "1000000"},
				"commission": {"rate": "0.10", "max_rate": "0.20", "max_change_rate": "0.01"},
				"min_self_delegation": "1"
			}],
			"memo": ""
		},
		"auth_info": {"signer_infos": [], "fee": {}},
		"signatures": []
	}`, testOperatorAddr(6), testConsensusPubKeyB64(6))

	dir := writeTempGentxDir(t, map[string]string{"gentx-no-signer.json": json})

	repo := gentx.NewValidatorRepository(dir)
	_, err := repo.GetValidators(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse validators")
}

func TestGetValidators_InvalidAddress(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	json := fmt.Sprintf(`{
		"body": {
			"messages": [{
				"description": {"moniker": "v", "identity": "", "website": "", "security_contact": "", "details": ""},
				"validator_address": "not-a-valid-bech32-address",
				"pubkey": {"@type": "/cosmos.crypto.ed25519.PubKey", "key": %q},
				"value": {"denom": "uatom", "amount": "1000000"},
				"commission": {"rate": "0.10", "max_rate": "0.20", "max_change_rate": "0.01"},
				"min_self_delegation": "1"
			}],
			"memo": ""
		},
		"auth_info": {
			"signer_infos": [{
				"public_key": {"@type": "/cosmos.crypto.secp256k1.PubKey", "key": %q},
				"mode_info": {"single": {"mode": "SIGN_MODE_DIRECT"}},
				"sequence": "0"
			}],
			"fee": {}
		},
		"signatures": []
	}`, testConsensusPubKeyB64(7), testOperatorPubKeyB64(7))

	dir := writeTempGentxDir(t, map[string]string{"gentx-bad-addr.json": json})

	repo := gentx.NewValidatorRepository(dir)
	_, err := repo.GetValidators(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse validators")
}
