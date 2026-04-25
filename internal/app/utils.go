package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/client"
	sdkcodec "github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/spf13/viper"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/vesting_account"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

const (
	ChainIDKey       = "chain.id"
	NameKey          = "app.name"
	VersionKey       = "app.version"
	GenesisTimeKey   = "app.genesis_time"
	InitialHeightKey = "chain.initial_height"

	InvalidVestingErr = "invalid vesting parameters; must supply start and end time or end time"
)

// Set sdk.GetConfig() bech32 prefixes and sdk.DefaultBondDenom before calling this.
func LoadGenesis(path string) (encoding.EncodingConfig, client.Context, map[string]json.RawMessage, *genutiltypes.AppGenesis, error) {
	encodingConfig := encoding.NewEncodingConfig()

	appState, appGenesis, err := genutiltypes.GenesisStateFromGenFile(path)
	if err != nil {
		return encoding.EncodingConfig{}, client.Context{}, nil, nil, fmt.Errorf("failed to read genesis file %s: %w", path, err)
	}

	// Override genesis metadata from config; the baseline file values are ignored.
	appGenesis.GenesisTime = time.Unix(viper.GetInt64(GenesisTimeKey), 0).UTC()
	appGenesis.AppName = viper.GetString(NameKey)
	appGenesis.AppVersion = viper.GetString(VersionKey)
	appGenesis.ChainID = viper.GetString(ChainIDKey)
	appGenesis.InitialHeight = viper.GetInt64(InitialHeightKey)

	clientCtx := client.Context{}.
		WithCodec(encodingConfig.Codec).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithLegacyAmino(encodingConfig.Amino).
		WithTxConfig(encodingConfig.TxConfig).
		WithInput(os.Stdin).
		WithAccountRetriever(authtypes.AccountRetriever{}).
		WithHomeDir(".").
		WithViper("").
		WithChainID(viper.GetString(ChainIDKey))

	return encodingConfig, clientCtx, appState, appGenesis, nil
}

func moduleAddress(hrp, moduleName string) (string, error) {
	addr, err := bech32.ConvertAndEncode(hrp, authtypes.NewModuleAddress(moduleName))
	if err != nil {
		return "", fmt.Errorf("failed to compute module address for %s: %w", moduleName, err)
	}
	return addr, nil
}

func saveGenesis(appGenState map[string]json.RawMessage, appGenesis *genutiltypes.AppGenesis, outputPath string) error {
	appStateJSON, err := json.Marshal(appGenState)
	if err != nil {
		return errorsmod.Wrap(err, "failed to marshal app state")
	}
	appGenesis.AppState = appStateJSON
	appGenesis.GenesisTime = time.Unix(viper.GetInt64(GenesisTimeKey), 0).UTC()
	return appGenesis.SaveAs(outputPath)
}

func updateModuleState(
	cdc sdkcodec.Codec,
	appGenState map[string]json.RawMessage,
	moduleName string,
	state sdkcodec.ProtoMarshaler, //nolint:staticcheck // Cosmos SDK v0.50 still exposes this; proto.Message migration is a separate task
	updater func() error,
) error {
	raw, ok := appGenState[moduleName]
	if !ok {
		return fmt.Errorf("%s module not found in genesis state", moduleName)
	}
	if err := cdc.UnmarshalJSON(raw, state); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", moduleName, err)
	}
	if err := updater(); err != nil {
		return err
	}
	bz, err := cdc.MarshalJSON(state)
	if err != nil {
		return fmt.Errorf("failed to marshal %s genesis state: %w", moduleName, err)
	}
	appGenState[moduleName] = bz
	return nil
}

func AddCustomVestingGenesisAccount(
	vestingAccount vesting_account.VestingAccount,
	accAddr sdk.AccAddress,
	vestingStart, vestingEnd int64,
	encodingConfig encoding.EncodingConfig,
	accs authtypes.GenesisAccounts,
	bankGenState *banktypes.GenesisState,
	appendAcct bool,
) (authtypes.GenesisAccounts, error) {
	denom := viper.GetString("default_bond_denom")
	nonStakedPortion := getNonStakedPortion()

	genAccount, balances, err := createVestingAccount(vestingAccount, accAddr, vestingStart, vestingEnd, denom, nonStakedPortion)
	if err != nil {
		return nil, err
	}

	addr, err := encodingConfig.TxConfig.SigningContext().AddressCodec().StringToBytes(vestingAccount.Address())
	if err != nil {
		return nil, err
	}

	if err := allocateDelegatedFunds(
		vestingAccount, addr, accAddr, balances, encodingConfig, bankGenState, appendAcct, denom, nonStakedPortion,
	); err != nil {
		return nil, err
	}

	accs = append(accs, genAccount)
	bankGenState.Supply = bankGenState.Supply.Add(balances.Coins...)
	return accs, nil
}

func createVestingAccount(
	vestingAccount vesting_account.VestingAccount,
	accAddr sdk.AccAddress,
	vestingStart, vestingEnd int64,
	denom string,
	nonStakedPortion int64,
) (authtypes.GenesisAccount, banktypes.Balance, error) {
	coins, err := sdk.ParseCoinsNormalized(strconv.FormatInt(vestingAccount.Amount(), 10) + denom)
	if err != nil {
		return nil, banktypes.Balance{}, fmt.Errorf("failed to parse coins: %w", err)
	}

	balances := banktypes.Balance{Address: accAddr.String(), Coins: coins.Sort()}
	baseAccount := authtypes.NewBaseAccount(accAddr, nil, 0, 0)
	baseVestingAccount, err := authvesting.NewBaseVestingAccount(baseAccount, coins.Sort(), vestingEnd)
	if err != nil {
		return nil, banktypes.Balance{}, fmt.Errorf("failed to create base vesting account: %w", err)
	}
	if baseVestingAccount.OriginalVesting.IsAnyGT(balances.Coins) {
		return nil, banktypes.Balance{}, errors.New("vesting amount cannot be greater than total amount")
	}

	if vestingAccount.DelegateTo() != "" {
		baseVestingAccount.DelegatedVesting = baseVestingAccount.GetOriginalVesting().Sub(sdk.Coin{
			Denom:  denom,
			Amount: math.NewInt(nonStakedPortion),
		})
	}

	var genAccount authtypes.GenesisAccount
	switch {
	case vestingStart != 0 && vestingEnd != 0:
		genAccount = authvesting.NewContinuousVestingAccountRaw(baseVestingAccount, vestingStart)
	case vestingEnd != 0:
		genAccount = authvesting.NewDelayedVestingAccountRaw(baseVestingAccount)
	default:
		return nil, banktypes.Balance{}, errors.New(InvalidVestingErr)
	}
	if err := genAccount.Validate(); err != nil {
		return nil, banktypes.Balance{}, fmt.Errorf("failed to validate new genesis account: %w", err)
	}

	return genAccount, balances, nil
}

func allocateDelegatedFunds(
	vestingAccount vesting_account.VestingAccount,
	addr sdk.AccAddress,
	accAddr sdk.AccAddress,
	balances banktypes.Balance,
	encodingConfig encoding.EncodingConfig,
	bankGenState *banktypes.GenesisState,
	appendAcct bool,
	denom string,
	nonStakedPortion int64,
) error {
	if vestingAccount.DelegateTo() == "" {
		return updateBalances(addr, balances, balances.Coins, bankGenState, appendAcct)
	}

	hrp := viper.GetString("chain.address_prefix")
	bondedAddr, err := moduleAddress(hrp, "bonded_tokens_pool")
	if err != nil {
		return err
	}
	bondedModuleAddr, err := encodingConfig.TxConfig.SigningContext().AddressCodec().StringToBytes(bondedAddr)
	if err != nil {
		return err
	}
	unstakedCoin := sdk.Coin{Denom: denom, Amount: math.NewInt(nonStakedPortion)}
	stakedCoins := balances.Coins.Sub(unstakedCoin)
	if err := updateBalances(bondedModuleAddr, balances, stakedCoins, bankGenState, appendAcct); err != nil {
		return err
	}
	unstakedCoins := sdk.Coins{unstakedCoin}
	return updateBalances(addr, banktypes.Balance{Address: accAddr.String(), Coins: unstakedCoins}, unstakedCoins, bankGenState, true)
}

func updateBalances(
	accAddr sdk.AccAddress, balances banktypes.Balance, coins sdk.Coins, bankGenState *banktypes.GenesisState, appendAcct bool,
) error {
	for idx, acc := range bankGenState.Balances {
		if acc.Address == accAddr.String() {
			if !appendAcct {
				return fmt.Errorf("account %s already exists. Use `append` flag to append account at existing address", accAddr)
			}
			updatedCoins := acc.Coins.Add(coins...)
			bankGenState.Balances[idx] = banktypes.Balance{Address: accAddr.String(), Coins: updatedCoins.Sort()}
			return nil
		}
	}
	bankGenState.Balances = append(bankGenState.Balances, balances)
	return nil
}

func AddCustomModuleGenesisAccount(
	cdc sdkcodec.Codec,
	accAddr sdk.AccAddress,
	genesisFileURL,
	amountStr,
	moduleName string,
	permissions []string,
) error {
	coins, err := sdk.ParseCoinsNormalized(amountStr)
	if err != nil {
		return fmt.Errorf("failed to parse coins: %w", err)
	}
	balances := banktypes.Balance{Address: accAddr.String(), Coins: coins.Sort()}
	genAccount := authtypes.NewEmptyModuleAccount(moduleName, permissions...)
	if err := genAccount.Validate(); err != nil {
		return fmt.Errorf("failed to validate new genesis account: %w", err)
	}

	appState, appGenesis, err := genutiltypes.GenesisStateFromGenFile(genesisFileURL)
	if err != nil {
		return fmt.Errorf("failed to unmarshal genesis state: %w", err)
	}

	authGenState := authtypes.GetGenesisStateFromAppState(cdc, appState)
	accs, err := authtypes.UnpackAccounts(authGenState.Accounts)
	if err != nil {
		return fmt.Errorf("failed to get accounts from any: %w", err)
	}
	bankGenState := banktypes.GetGenesisStateFromAppState(cdc, appState)

	if accs.Contains(accAddr) {
		return fmt.Errorf("account %s already exists", accAddr)
	}

	accs = append(accs, genAccount)
	accs = authtypes.SanitizeGenesisAccounts(accs)
	genAccs, err := authtypes.PackAccounts(accs)
	if err != nil {
		return fmt.Errorf("failed to convert accounts into any's: %w", err)
	}
	authGenState.Accounts = genAccs
	authGenStateBz, err := cdc.MarshalJSON(&authGenState)
	if err != nil {
		return fmt.Errorf("failed to marshal auth genesis state: %w", err)
	}
	appState[authtypes.ModuleName] = authGenStateBz

	bankGenState.Balances = append(bankGenState.Balances, balances)
	bankGenState.Balances = banktypes.SanitizeGenesisBalances(bankGenState.Balances)
	bankGenState.Supply = bankGenState.Supply.Add(balances.Coins...)
	bankGenStateBz, err := cdc.MarshalJSON(bankGenState)
	if err != nil {
		return fmt.Errorf("failed to marshal bank genesis state: %w", err)
	}
	appState[banktypes.ModuleName] = bankGenStateBz

	appStateJSON, err := json.Marshal(appState)
	if err != nil {
		return fmt.Errorf("failed to marshal application genesis state: %w", err)
	}
	appGenesis.AppState = appStateJSON
	return genutil.ExportGenesisFile(appGenesis, genesisFileURL)
}
