package app

import (
	"fmt"

	"cosmossdk.io/math"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/spf13/viper"
)

func (asm StateManager) setDenominationMetadata() error {
	base := viper.GetString("denom.base")
	if base == "" {
		// No denom metadata configured; preserve baseline.
		return nil
	}

	bankGenState := banktypes.GetGenesisStateFromAppState(asm.clientCtx.Codec, asm.appGenState)

	display := viper.GetString("denom.display")
	symbol := viper.GetString("denom.symbol")
	description := viper.GetString("denom.description")
	exponent := viper.GetUint32("denom.exponent")
	aliases := viper.GetStringSlice("denom.aliases")

	denomUnits := []*banktypes.DenomUnit{
		{Denom: base, Exponent: 0, Aliases: aliases},
	}
	if display != "" && display != base {
		denomUnits = append(denomUnits, &banktypes.DenomUnit{Denom: display, Exponent: exponent})
	}

	metadata := banktypes.Metadata{
		Description: description,
		DenomUnits:  denomUnits,
		Base:        base,
		Display:     display,
		Name:        symbol,
		Symbol:      symbol,
	}
	bankGenState.DenomMetadata = []banktypes.Metadata{metadata}

	bankStateBz, err := asm.clientCtx.Codec.MarshalJSON(bankGenState)
	if err != nil {
		return fmt.Errorf("failed to marshal bank genesis state: %w", err)
	}
	asm.appGenState["bank"] = bankStateBz
	return nil
}

func (asm StateManager) validateSupply() error {
	bankGenState := banktypes.GetGenesisStateFromAppState(asm.clientCtx.Codec, asm.appGenState)
	supply := bankGenState.Supply.AmountOf(viper.GetString("default_bond_denom"))
	totalSupply := math.NewInt(viper.GetInt64("accounts.total_supply"))
	if !supply.Equal(totalSupply) {
		return fmt.Errorf("total supply mismatch: got %s, expected %s", supply, totalSupply)
	}
	return nil
}
