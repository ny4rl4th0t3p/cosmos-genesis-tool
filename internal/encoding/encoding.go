package encoding

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
)

// bech32 address prefix must be set on sdk.Config before constructing this (see sdk.GetConfig().SetBech32Prefix...).
type EncodingConfig struct {
	InterfaceRegistry codectypes.InterfaceRegistry
	Codec             codec.Codec
	TxConfig          client.TxConfig
	Amino             *codec.LegacyAmino
}

var StandardModuleNames = []string{
	"bonded_tokens_pool",
	"not_bonded_tokens_pool",
	"gov",
	"distribution",
	"mint",
	"fee_collector",
}

// Call sdk.GetConfig().SetBech32Prefix* before calling this.
func NewEncodingConfig() EncodingConfig {
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(interfaceRegistry)
	authtypes.RegisterInterfaces(interfaceRegistry)
	vestingtypes.RegisterInterfaces(interfaceRegistry)

	cdc := codec.NewProtoCodec(interfaceRegistry)
	amino := codec.NewLegacyAmino()
	std.RegisterLegacyAminoCodec(amino)

	txConfig := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)

	return EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Codec:             cdc,
		TxConfig:          txConfig,
		Amino:             amino,
	}
}

func ModuleAddresses(hrp string, moduleNames []string) map[string]bool {
	addrs := make(map[string]bool, len(moduleNames))
	for _, name := range moduleNames {
		addr, err := bech32.ConvertAndEncode(hrp, authtypes.NewModuleAddress(name))
		if err == nil {
			addrs[addr] = true
		}
	}
	return addrs
}
