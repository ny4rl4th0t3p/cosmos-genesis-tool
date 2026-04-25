package app

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

func newBankStateManager(t *testing.T) StateManager {
	t.Helper()
	ec := encoding.NewEncodingConfig()
	clientCtx := client.Context{}.WithCodec(ec.Codec)
	bankDefault := banktypes.DefaultGenesisState()
	bz, err := ec.Codec.MarshalJSON(bankDefault)
	require.NoError(t, err)
	return StateManager{
		encodingConfig: ec,
		clientCtx:      clientCtx,
		appGenState:    map[string]json.RawMessage{"bank": bz},
	}
}

func TestSetDenominationMetadata_EmptyBase_NoOp(t *testing.T) {
	asm := newBankStateManager(t)
	original := make([]byte, len(asm.appGenState["bank"]))
	copy(original, asm.appGenState["bank"])

	require.NoError(t, asm.setDenominationMetadata())
	assert.Equal(t, string(original), string(asm.appGenState["bank"]))
}

func TestSetDenominationMetadata_BaseSet_MetadataWritten(t *testing.T) {
	viper.Set("denom.base", "uatom")
	viper.Set("denom.display", "atom")
	viper.Set("denom.symbol", "ATOM")
	viper.Set("denom.description", "The ATOM token")
	viper.Set("denom.exponent", uint32(6))
	t.Cleanup(func() {
		viper.Set("denom.base", nil)
		viper.Set("denom.display", nil)
		viper.Set("denom.symbol", nil)
		viper.Set("denom.description", nil)
		viper.Set("denom.exponent", nil)
	})

	asm := newBankStateManager(t)
	require.NoError(t, asm.setDenominationMetadata())

	bankState := banktypes.GetGenesisStateFromAppState(asm.clientCtx.Codec, asm.appGenState)
	require.Len(t, bankState.DenomMetadata, 1)
	meta := bankState.DenomMetadata[0]
	assert.Equal(t, "uatom", meta.Base)
	assert.Equal(t, "atom", meta.Display)
	assert.Equal(t, "ATOM", meta.Symbol)
	assert.Equal(t, "The ATOM token", meta.Description)
	require.Len(t, meta.DenomUnits, 2)
	assert.Equal(t, "uatom", meta.DenomUnits[0].Denom)
	assert.Equal(t, uint32(0), meta.DenomUnits[0].Exponent)
	assert.Equal(t, "atom", meta.DenomUnits[1].Denom)
	assert.Equal(t, uint32(6), meta.DenomUnits[1].Exponent)
}

func TestSetDenominationMetadata_BaseEqualsDisplay_SingleDenomUnit(t *testing.T) {
	viper.Set("denom.base", "uatom")
	viper.Set("denom.display", "uatom")
	t.Cleanup(func() {
		viper.Set("denom.base", nil)
		viper.Set("denom.display", nil)
	})

	asm := newBankStateManager(t)
	require.NoError(t, asm.setDenominationMetadata())

	bankState := banktypes.GetGenesisStateFromAppState(asm.clientCtx.Codec, asm.appGenState)
	require.Len(t, bankState.DenomMetadata, 1)
	assert.Len(t, bankState.DenomMetadata[0].DenomUnits, 1)
}

// writeMinimalGenesis creates a temp genesis file containing only a bank state with the given supply.
func writeMinimalGenesis(t *testing.T, ec encoding.EncodingConfig, supply sdk.Coins) string {
	t.Helper()
	bankState := banktypes.DefaultGenesisState()
	bankState.Supply = supply
	bankBz, err := ec.Codec.MarshalJSON(bankState)
	require.NoError(t, err)
	appStateJSON, err := json.Marshal(map[string]json.RawMessage{"bank": bankBz})
	require.NoError(t, err)
	appGenesis := genutiltypes.AppGenesis{AppState: appStateJSON}
	path := filepath.Join(t.TempDir(), "genesis.json")
	require.NoError(t, appGenesis.SaveAs(path))
	return path
}

func TestValidateSupply_Match_NoError(t *testing.T) {
	ec := encoding.NewEncodingConfig()
	path := writeMinimalGenesis(t, ec, sdk.NewCoins(sdk.NewInt64Coin("uatom", 1_000_000)))

	viper.Set("genesis.output", path)
	viper.Set("default_bond_denom", "uatom")
	viper.Set("accounts.total_supply", int64(1_000_000))
	t.Cleanup(func() {
		viper.Set("genesis.output", nil)
		viper.Set("default_bond_denom", nil)
		viper.Set("accounts.total_supply", nil)
	})

	asm := StateManager{clientCtx: client.Context{}.WithCodec(ec.Codec)}
	require.NoError(t, asm.validateSupply())
}

func TestValidateSupply_Mismatch_ReturnsError(t *testing.T) {
	ec := encoding.NewEncodingConfig()
	path := writeMinimalGenesis(t, ec, sdk.NewCoins(sdk.NewInt64Coin("uatom", 1_000_000)))

	viper.Set("genesis.output", path)
	viper.Set("default_bond_denom", "uatom")
	viper.Set("accounts.total_supply", int64(9_999_999))
	t.Cleanup(func() {
		viper.Set("genesis.output", nil)
		viper.Set("default_bond_denom", nil)
		viper.Set("accounts.total_supply", nil)
	})

	asm := StateManager{clientCtx: client.Context{}.WithCodec(ec.Codec)}
	err := asm.validateSupply()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "total supply mismatch")
}
