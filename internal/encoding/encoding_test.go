package encoding_test

import (
	"os"
	"strings"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
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

func TestNewEncodingConfig_FieldsNotNil(t *testing.T) {
	cfg := encoding.NewEncodingConfig()
	assert.NotNil(t, cfg.InterfaceRegistry)
	assert.NotNil(t, cfg.Codec)
	assert.NotNil(t, cfg.TxConfig)
	assert.NotNil(t, cfg.Amino)
}

func TestNewEncodingConfig_AddressCodecRoundTrip(t *testing.T) {
	cfg := encoding.NewEncodingConfig()
	codec := cfg.TxConfig.SigningContext().AddressCodec()

	raw := make([]byte, 20)
	raw[19] = 42

	addr, err := codec.BytesToString(raw)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(addr, testHRP+"1"), "expected cosmos bech32 prefix")

	decoded, err := codec.StringToBytes(addr)
	require.NoError(t, err)
	assert.Equal(t, raw, decoded)
}

func TestStandardModuleNames_Count(t *testing.T) {
	assert.Len(t, encoding.StandardModuleNames, 6)
}

func TestStandardModuleNames_ContainsExpected(t *testing.T) {
	expected := []string{
		"bonded_tokens_pool",
		"not_bonded_tokens_pool",
		"gov",
		"distribution",
		"mint",
		"fee_collector",
	}
	assert.Equal(t, expected, encoding.StandardModuleNames)
}

func TestModuleAddresses_Count(t *testing.T) {
	addrs := encoding.ModuleAddresses(testHRP, []string{"gov", "mint", "fee_collector"})
	assert.Len(t, addrs, 3)
}

func TestModuleAddresses_AllTrue(t *testing.T) {
	addrs := encoding.ModuleAddresses(testHRP, []string{"gov", "mint"})
	for addr, v := range addrs {
		assert.True(t, v, "expected true for %s", addr)
	}
}

func TestModuleAddresses_HaveCorrectPrefix(t *testing.T) {
	addrs := encoding.ModuleAddresses(testHRP, encoding.StandardModuleNames)
	for addr := range addrs {
		assert.True(t, strings.HasPrefix(addr, testHRP+"1"), "unexpected prefix in %s", addr)
	}
}

func TestModuleAddresses_Deterministic(t *testing.T) {
	first := encoding.ModuleAddresses(testHRP, []string{"gov"})
	second := encoding.ModuleAddresses(testHRP, []string{"gov"})
	assert.Equal(t, first, second)
}

func TestModuleAddresses_DifferentHRP(t *testing.T) {
	cosmosAddrs := encoding.ModuleAddresses("cosmos", []string{"gov"})
	osmoAddrs := encoding.ModuleAddresses("osmo", []string{"gov"})
	for ca := range cosmosAddrs {
		for oa := range osmoAddrs {
			assert.NotEqual(t, ca, oa, "different HRPs must produce different addresses")
		}
	}
}

func TestModuleAddresses_EmptyList(t *testing.T) {
	addrs := encoding.ModuleAddresses(testHRP, []string{})
	assert.Empty(t, addrs)
}
