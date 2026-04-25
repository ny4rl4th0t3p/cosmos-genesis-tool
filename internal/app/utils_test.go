package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/vesting_account"
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

// stubVestingAcct is a minimal VestingAccount implementation for tests.
type stubVestingAcct struct {
	amount     int64
	delegateTo string
}

func (stubVestingAcct) Address() string      { return "" }
func (s stubVestingAcct) Amount() int64      { return s.amount }
func (s stubVestingAcct) DelegateTo() string { return s.delegateTo }

// ensure stubVestingAcct satisfies the interface at compile time
var _ vesting_account.VestingAccount = stubVestingAcct{}

// testAccAddr returns a raw sdk.AccAddress for test index i.
func testAccAddr(i byte) sdk.AccAddress {
	raw := make([]byte, 20)
	raw[19] = i
	return sdk.AccAddress(raw)
}

// --- moduleAddress ---

func TestModuleAddress_ReturnsCorrectPrefix(t *testing.T) {
	addr, err := moduleAddress(testHRP, "gov")
	require.NoError(t, err)
	assert.Contains(t, addr, testHRP+"1")
}

func TestModuleAddress_Deterministic(t *testing.T) {
	a1, err := moduleAddress(testHRP, "mint")
	require.NoError(t, err)
	a2, err := moduleAddress(testHRP, "mint")
	require.NoError(t, err)
	assert.Equal(t, a1, a2)
}

func TestModuleAddress_DifferentModulesDifferentAddresses(t *testing.T) {
	gov, err := moduleAddress(testHRP, "gov")
	require.NoError(t, err)
	mint, err := moduleAddress(testHRP, "mint")
	require.NoError(t, err)
	assert.NotEqual(t, gov, mint)
}

func TestModuleAddress_DifferentHRPsDifferentAddresses(t *testing.T) {
	cosmos, err := moduleAddress("cosmos", "gov")
	require.NoError(t, err)
	osmo, err := moduleAddress("osmo", "gov")
	require.NoError(t, err)
	assert.NotEqual(t, cosmos, osmo)
}

// --- updateBalances ---

func TestUpdateBalances_NewAddress(t *testing.T) {
	accAddr := testAccAddr(1)
	coins := sdk.NewCoins(sdk.NewInt64Coin("uatom", 1000))
	balance := banktypes.Balance{Address: accAddr.String(), Coins: coins}
	bankState := &banktypes.GenesisState{}

	err := updateBalances(accAddr, balance, coins, bankState, false)
	require.NoError(t, err)
	require.Len(t, bankState.Balances, 1)
	assert.Equal(t, coins, bankState.Balances[0].Coins)
}

func TestUpdateBalances_ExistingAddressWithAppend(t *testing.T) {
	accAddr := testAccAddr(2)
	initial := sdk.NewCoins(sdk.NewInt64Coin("uatom", 500))
	bankState := &banktypes.GenesisState{
		Balances: []banktypes.Balance{{Address: accAddr.String(), Coins: initial}},
	}

	extra := sdk.NewCoins(sdk.NewInt64Coin("uatom", 300))
	err := updateBalances(accAddr, banktypes.Balance{Address: accAddr.String(), Coins: extra}, extra, bankState, true)
	require.NoError(t, err)
	require.Len(t, bankState.Balances, 1)
	assert.Equal(t, int64(800), bankState.Balances[0].Coins.AmountOf("uatom").Int64())
}

func TestUpdateBalances_ExistingAddressWithoutAppend_Errors(t *testing.T) {
	accAddr := testAccAddr(3)
	coins := sdk.NewCoins(sdk.NewInt64Coin("uatom", 1000))
	bankState := &banktypes.GenesisState{
		Balances: []banktypes.Balance{{Address: accAddr.String(), Coins: coins}},
	}

	err := updateBalances(accAddr, banktypes.Balance{Address: accAddr.String(), Coins: coins}, coins, bankState, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestUpdateBalances_MultipleDifferentAddresses(t *testing.T) {
	addr1, addr2 := testAccAddr(4), testAccAddr(5)
	coins := sdk.NewCoins(sdk.NewInt64Coin("uatom", 100))
	bankState := &banktypes.GenesisState{}

	require.NoError(t, updateBalances(addr1, banktypes.Balance{Address: addr1.String(), Coins: coins}, coins, bankState, false))
	require.NoError(t, updateBalances(addr2, banktypes.Balance{Address: addr2.String(), Coins: coins}, coins, bankState, false))
	assert.Len(t, bankState.Balances, 2)
}

// --- updateModuleState ---

func TestUpdateModuleState_ModuleNotFound(t *testing.T) {
	ec := encoding.NewEncodingConfig()
	appGenState := map[string]json.RawMessage{}
	var state banktypes.GenesisState
	err := updateModuleState(ec.Codec, appGenState, "bank", &state, func() error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bank module not found")
}

func TestUpdateModuleState_UnmarshalError(t *testing.T) {
	ec := encoding.NewEncodingConfig()
	appGenState := map[string]json.RawMessage{"bank": json.RawMessage(`not valid json`)}
	var state banktypes.GenesisState
	err := updateModuleState(ec.Codec, appGenState, "bank", &state, func() error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal bank genesis state")
}

func TestUpdateModuleState_UpdaterError(t *testing.T) {
	ec := encoding.NewEncodingConfig()
	original := banktypes.DefaultGenesisState()
	bz, err := ec.Codec.MarshalJSON(original)
	require.NoError(t, err)
	appGenState := map[string]json.RawMessage{"bank": bz}

	var state banktypes.GenesisState
	sentinelErr := errors.New("updater failed")
	err = updateModuleState(ec.Codec, appGenState, "bank", &state, func() error { return sentinelErr })
	require.ErrorIs(t, err, sentinelErr)
}

func TestUpdateModuleState_HappyPath(t *testing.T) {
	ec := encoding.NewEncodingConfig()
	original := banktypes.DefaultGenesisState()
	original.Params.DefaultSendEnabled = true
	bz, err := ec.Codec.MarshalJSON(original)
	require.NoError(t, err)
	appGenState := map[string]json.RawMessage{"bank": bz}

	var state banktypes.GenesisState
	err = updateModuleState(ec.Codec, appGenState, "bank", &state, func() error {
		state.Params.DefaultSendEnabled = false
		return nil
	})
	require.NoError(t, err)

	var result banktypes.GenesisState
	require.NoError(t, ec.Codec.UnmarshalJSON(appGenState["bank"], &result))
	assert.False(t, result.Params.DefaultSendEnabled)
}

// --- createVestingAccount ---

func TestCreateVestingAccount_DelayedVesting(t *testing.T) {
	acc, bal, err := createVestingAccount(
		stubVestingAcct{amount: 1_000_000},
		testAccAddr(30), 0, time.Now().Unix(),
		"uatom", 100_000,
	)
	require.NoError(t, err)
	_, ok := acc.(*authvesting.DelayedVestingAccount)
	assert.True(t, ok, "expected DelayedVestingAccount")
	assert.Equal(t, int64(1_000_000), bal.Coins.AmountOf("uatom").Int64())
}

func TestCreateVestingAccount_ContinuousVesting(t *testing.T) {
	now := time.Now().Unix()
	acc, _, err := createVestingAccount(
		stubVestingAcct{amount: 1_000_000},
		testAccAddr(31), now, now+86400,
		"uatom", 100_000,
	)
	require.NoError(t, err)
	_, ok := acc.(*authvesting.ContinuousVestingAccount)
	assert.True(t, ok, "expected ContinuousVestingAccount")
}

func TestCreateVestingAccount_BothZeroErrors(t *testing.T) {
	_, _, err := createVestingAccount(
		stubVestingAcct{amount: 1_000_000},
		testAccAddr(32), 0, 0,
		"uatom", 100_000,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid vesting")
}

func TestCreateVestingAccount_NoDelegation_DelegatedVestingEmpty(t *testing.T) {
	acc, _, err := createVestingAccount(
		stubVestingAcct{amount: 1_000_000}, // DelegateTo() == ""
		testAccAddr(33), 0, time.Now().Unix(),
		"uatom", 100_000,
	)
	require.NoError(t, err)
	dva, ok := acc.(*authvesting.DelayedVestingAccount)
	require.True(t, ok)
	assert.True(t, dva.DelegatedVesting.IsZero())
}

func TestCreateVestingAccount_WithDelegation_SetsDelegatedVesting(t *testing.T) {
	acc, _, err := createVestingAccount(
		stubVestingAcct{amount: 1_000_000, delegateTo: "some-validator"},
		testAccAddr(34), 0, time.Now().Unix(),
		"uatom", 100_000,
	)
	require.NoError(t, err)
	dva, ok := acc.(*authvesting.DelayedVestingAccount)
	require.True(t, ok)
	// DelegatedVesting = amount - nonStakedPortion
	assert.Equal(t, int64(900_000), dva.DelegatedVesting.AmountOf("uatom").Int64())
}

// --- allocateDelegatedFunds ---

func TestAllocateDelegatedFunds_NoDelegation(t *testing.T) {
	addr := testAccAddr(40)
	coins := sdk.NewCoins(sdk.NewInt64Coin("uatom", 1_000_000))
	balances := banktypes.Balance{Address: addr.String(), Coins: coins}
	bankState := &banktypes.GenesisState{}

	err := allocateDelegatedFunds(
		stubVestingAcct{amount: 1_000_000}, // DelegateTo() == ""
		addr, addr, balances,
		encoding.EncodingConfig{}, bankState,
		false, "uatom", 100_000,
	)
	require.NoError(t, err)
	require.Len(t, bankState.Balances, 1)
	assert.Equal(t, int64(1_000_000), bankState.Balances[0].Coins.AmountOf("uatom").Int64())
}

func TestAllocateDelegatedFunds_WithDelegation(t *testing.T) {
	viper.Set("chain.address_prefix", testHRP)
	t.Cleanup(func() { viper.Set("chain.address_prefix", nil) })

	ec := encoding.NewEncodingConfig()
	addr := testAccAddr(41)

	// The bonded pool is always pre-populated by appendModuleAccounts before claims are added.
	bondedPoolAddr, err := moduleAddress(testHRP, "bonded_tokens_pool")
	require.NoError(t, err)
	bankState := &banktypes.GenesisState{
		Balances: []banktypes.Balance{
			{Address: bondedPoolAddr, Coins: sdk.NewCoins(sdk.NewInt64Coin("uatom", 2_000_000))},
		},
	}

	coins := sdk.NewCoins(sdk.NewInt64Coin("uatom", 1_000_000))
	balances := banktypes.Balance{Address: addr.String(), Coins: coins}

	err = allocateDelegatedFunds(
		stubVestingAcct{amount: 1_000_000, delegateTo: "cosmosvaloper1anything"},
		addr, addr, balances,
		ec, bankState,
		true, "uatom", 100_000,
	)
	require.NoError(t, err)
	require.Len(t, bankState.Balances, 2)

	balanceOf := func(address string) int64 {
		for _, b := range bankState.Balances {
			if b.Address == address {
				return b.Coins.AmountOf("uatom").Int64()
			}
		}
		return 0
	}
	// staked portion (900K) added to existing 2M bonded pool
	assert.Equal(t, int64(2_900_000), balanceOf(bondedPoolAddr))
	// liquid portion stays with the account
	assert.Equal(t, int64(100_000), balanceOf(addr.String()))
}

// --- saveGenesis ---

func TestSaveGenesis_WritesReadableFile(t *testing.T) {
	viper.Set(GenesisTimeKey, int64(1_700_000_000))
	t.Cleanup(func() { viper.Set(GenesisTimeKey, nil) })

	ec := encoding.NewEncodingConfig()
	bankDefault := banktypes.DefaultGenesisState()
	bz, err := ec.Codec.MarshalJSON(bankDefault)
	require.NoError(t, err)
	appGenState := map[string]json.RawMessage{"bank": bz}

	appGenesis := &genutiltypes.AppGenesis{ChainID: "test-chain-1"}
	path := filepath.Join(t.TempDir(), "genesis.json")

	require.NoError(t, saveGenesis(appGenState, appGenesis, path))

	_, err = os.Stat(path)
	require.NoError(t, err, "genesis file should exist")

	// File is valid JSON parseable by GenesisStateFromGenFile.
	readState, readGenesis, err := genutiltypes.GenesisStateFromGenFile(path)
	require.NoError(t, err)
	assert.Equal(t, "test-chain-1", readGenesis.ChainID)
	assert.Contains(t, readState, "bank")
}

func TestSaveGenesis_StampsGenesisTime(t *testing.T) {
	const genesisUnix = int64(1_700_000_000)
	viper.Set(GenesisTimeKey, genesisUnix)
	t.Cleanup(func() { viper.Set(GenesisTimeKey, nil) })

	appGenesis := &genutiltypes.AppGenesis{}
	path := filepath.Join(t.TempDir(), "genesis.json")
	require.NoError(t, saveGenesis(map[string]json.RawMessage{}, appGenesis, path))

	_, readGenesis, err := genutiltypes.GenesisStateFromGenFile(path)
	require.NoError(t, err)
	assert.Equal(t, time.Unix(genesisUnix, 0).UTC(), readGenesis.GenesisTime)
}

// --- LoadGenesis ---

func TestLoadGenesis_ReadsStateAndMetadata(t *testing.T) {
	ec := encoding.NewEncodingConfig()
	bankDefault := banktypes.DefaultGenesisState()
	bz, err := ec.Codec.MarshalJSON(bankDefault)
	require.NoError(t, err)
	appStateJSON, err := json.Marshal(map[string]json.RawMessage{"bank": bz})
	require.NoError(t, err)

	appGenesis := &genutiltypes.AppGenesis{
		ChainID:  "load-test-1",
		AppState: appStateJSON,
	}
	path := filepath.Join(t.TempDir(), "genesis.json")
	require.NoError(t, appGenesis.SaveAs(path))

	viper.Set(ChainIDKey, "load-test-1")
	viper.Set(NameKey, "testapp")
	viper.Set(VersionKey, "v1.0.0")
	viper.Set(GenesisTimeKey, int64(1_700_000_000))
	viper.Set(InitialHeightKey, int64(1))
	t.Cleanup(func() {
		viper.Set(ChainIDKey, nil)
		viper.Set(NameKey, nil)
		viper.Set(VersionKey, nil)
		viper.Set(GenesisTimeKey, nil)
		viper.Set(InitialHeightKey, nil)
	})

	loadedEC, clientCtx, appState, loadedGenesis, err := LoadGenesis(path)
	require.NoError(t, err)
	assert.NotNil(t, loadedEC.Codec)
	assert.NotNil(t, clientCtx.Codec)
	assert.Contains(t, appState, "bank")
	assert.Equal(t, "load-test-1", loadedGenesis.ChainID)
	assert.Equal(t, "testapp", loadedGenesis.AppName)
}

func TestLoadGenesis_NonExistentFile_ReturnsError(t *testing.T) {
	_, _, _, _, err := LoadGenesis("/nonexistent/genesis.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read genesis file")
}
