package validator_test

import (
	"crypto/sha256"
	"encoding/base64"
	"os"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/validator"
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

// testOperatorAddr returns a valoper bech32 address for index i.
func testOperatorAddr(i byte) string {
	raw := make([]byte, 20)
	raw[19] = i
	addr, err := bech32.ConvertAndEncode(testHRP+"valoper", raw)
	if err != nil {
		panic(err)
	}
	return addr
}

// testAccountAddr returns a cosmos bech32 address for index i (same bytes as testOperatorAddr).
func testAccountAddr(i byte) string {
	raw := make([]byte, 20)
	raw[19] = i
	addr, err := bech32.ConvertAndEncode(testHRP, raw)
	if err != nil {
		panic(err)
	}
	return addr
}

// testPubKeyBase64 returns a deterministic base64-encoded pubkey for index i.
func testPubKeyBase64(i byte) string {
	raw := make([]byte, 32)
	raw[0] = i
	return base64.StdEncoding.EncodeToString(raw)
}

func validValidatorArgs(i byte) (address, pubKey, pubKeyType, name, identity, website, //nolint:gocritic // test helper intentionally returns many values to match NewValidatorFromFields signature
	securityContact, details, commissionRate, maxRate, maxChangeRate,
	minSelfDelegation, memo, denom, operatorPublicKey string, amount int64) {
	return testOperatorAddr(i), testPubKeyBase64(i), "ed25519",
		"validator-name", "", "", "", "",
		"0.1", "0.2", "0.05",
		"1", "", "uatom", base64.StdEncoding.EncodeToString(make([]byte, 33)),
		1_000_000
}

func TestNewValidatorFromFields_Valid(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	v, err := validator.NewValidatorFromFields(validValidatorArgs(1))
	require.NoError(t, err)
	assert.Equal(t, testOperatorAddr(1), v.OperatorAddress())
	assert.Equal(t, int64(1_000_000), v.Amount())
}

func TestNewValidatorFromFields_DelegatorAddressDerivedFromOperator(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	v, err := validator.NewValidatorFromFields(validValidatorArgs(2))
	require.NoError(t, err)
	// delegator address has account HRP, same underlying bytes as operator address
	assert.Equal(t, testAccountAddr(2), v.DelegatorAddress())
}

func TestNewValidatorFromFields_ConsensusAddressFromPubKey(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	pubKeyB64 := testPubKeyBase64(3)
	pubKeyBytes, _ := base64.StdEncoding.DecodeString(pubKeyB64)
	hash := sha256.Sum256(pubKeyBytes)
	expected := hash[:20]

	address, pubKey, pubKeyType, name, _, _, _, _, commissionRate, maxRate, maxChangeRate, minSD, memo, denom, opPK, amount := validValidatorArgs(3) //nolint:dogsled // 4 ignored fields are empty-string placeholders
	v, err := validator.NewValidatorFromFields(address, pubKey, pubKeyType, name, "", "", "", "",
		commissionRate, maxRate, maxChangeRate, minSD, memo, denom, opPK, amount)
	require.NoError(t, err)
	assert.Equal(t, expected, v.ConsensusAddress())
}

func TestNewValidatorFromFields_ZeroAmount(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	address, pubKey, pubKeyType, name, id, web, sec, det, cr, mr, mcr, msd, memo, denom, opPK, _ := validValidatorArgs(4)
	_, err := validator.NewValidatorFromFields(address, pubKey, pubKeyType, name, id, web, sec, det, cr, mr, mcr, msd, memo, denom, opPK, 0)
	require.ErrorIs(t, err, validator.ErrInvalidValidator)
}

func TestNewValidatorFromFields_EmptyName(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	address, pubKey, pubKeyType, _, id, web, sec, det, cr, mr, mcr, msd, memo, denom, opPK, amount := validValidatorArgs(5)
	_, err := validator.NewValidatorFromFields(address, pubKey, pubKeyType, "", id, web, sec, det, cr, mr, mcr, msd, memo, denom, opPK, amount)
	require.ErrorIs(t, err, validator.ErrInvalidValidator)
}

func TestNewValidatorFromFields_EmptyCommissionRate(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	address, pubKey, pubKeyType, name, id, web, sec, det, _, mr, mcr, msd, memo, denom, opPK, amount := validValidatorArgs(6)
	_, err := validator.NewValidatorFromFields(address, pubKey, pubKeyType, name, id, web, sec, det, "", mr, mcr, msd, memo, denom, opPK, amount)
	require.ErrorIs(t, err, validator.ErrInvalidValidator)
}

func TestNewValidatorFromFields_InvalidOperatorAddress(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	_, pubKey, pubKeyType, name, id, web, sec, det, cr, mr, mcr, msd, memo, denom, opPK, amount := validValidatorArgs(7)
	_, err := validator.NewValidatorFromFields("invalid-address", pubKey, pubKeyType, name, id, web, sec, det, cr, mr, mcr, msd, memo, denom, opPK, amount)
	require.Error(t, err)
}

func TestNewValidatorFromFields_InvalidBase64PubKey(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	address, _, pubKeyType, name, id, web, sec, det, cr, mr, mcr, msd, memo, denom, opPK, amount := validValidatorArgs(8)
	_, err := validator.NewValidatorFromFields(address, "not-base64!!!", pubKeyType, name, id, web, sec, det, cr, mr, mcr, msd, memo, denom, opPK, amount)
	require.Error(t, err)
}

func TestValidator_Accessors(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	v, err := validator.NewValidatorFromFields(
		testOperatorAddr(9), testPubKeyBase64(9), "ed25519",
		"my-node", "identity1", "https://example.com", "security@example.com", "a great validator",
		"0.05", "0.20", "0.01", "1", "some memo", "uatom",
		base64.StdEncoding.EncodeToString(make([]byte, 33)), 500_000,
	)
	require.NoError(t, err)

	assert.Equal(t, "my-node", v.Name())
	assert.Equal(t, "identity1", v.Identity())
	assert.Equal(t, "https://example.com", v.Website())
	assert.Equal(t, "security@example.com", v.SecurityContact())
	assert.Equal(t, "a great validator", v.Details())
	assert.Equal(t, "0.05", v.CommissionRate())
	assert.Equal(t, "0.20", v.MaxRate())
	assert.Equal(t, "0.01", v.MaxChangeRate())
	assert.Equal(t, "1", v.MinSelfDelegation())
}
