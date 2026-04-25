package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	stdmath "math"
	"sort"
	"strconv"
	"strings"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/client"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/spf13/viper"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/validator"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/repository"
)

const NonStakedPortion = 100000

func getNonStakedPortion() int64 {
	if v := viper.GetInt64("accounts.non_staked_portion"); v > 0 {
		return v
	}
	return NonStakedPortion
}

type Accounts struct {
	claimRepository     repository.ClaimRepository
	grantRepository     repository.GrantRepository
	initialAccountsRepo repository.InitialAccountsRepository
	validatorRepository repository.ValidatorRepository
}

func NewAccounts(
	claimRepository repository.ClaimRepository,
	grantRepository repository.GrantRepository,
	initialAccountsRepo repository.InitialAccountsRepository,
	validatorRepository repository.ValidatorRepository,
) *Accounts {
	return &Accounts{
		claimRepository:     claimRepository,
		grantRepository:     grantRepository,
		initialAccountsRepo: initialAccountsRepo,
		validatorRepository: validatorRepository,
	}
}

func (va Accounts) fetchValidatorsShares(encodingConfig encoding.EncodingConfig) (map[string]int64, error) {
	shares := map[string]int64{}
	claims, err := va.claimRepository.GetClaims(context.Background(), encodingConfig)
	if err != nil {
		return nil, err
	}
	nonStakedPortion := getNonStakedPortion()
	for _, claim := range claims {
		if claim.DelegateTo() != "" {
			delta := claim.Amount() - nonStakedPortion
			if delta > 0 && shares[claim.DelegateTo()] > stdmath.MaxInt64-delta {
				return nil, fmt.Errorf("share accumulation overflow for validator %q", claim.DelegateTo())
			}
			shares[claim.DelegateTo()] += delta
		}
	}
	return shares, nil
}

// Returns the updated in-memory genesis state to avoid a redundant disk reload by the caller.
func (va Accounts) appendVestingAccounts(
	ctx context.Context,
	encodingConfig encoding.EncodingConfig,
	clientCtx client.Context,
	validatorsReference map[string]ValidatorAddresses,
) (delegations []stakingtypes.Delegation, appState map[string]json.RawMessage, appGenesis *genutiltypes.AppGenesis, err error) {
	claims, err := va.claimRepository.GetClaims(ctx, encodingConfig)
	if err != nil {
		return nil, nil, nil, err
	}
	grants, err := va.grantRepository.GetGrants(ctx, encodingConfig)
	if err != nil {
		return nil, nil, nil, err
	}

	_, appGenesis, err = genutiltypes.GenesisStateFromGenFile(viper.GetString("genesis.output"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read genesis state: %w", err)
	}
	appState, err = genutiltypes.GenesisStateFromAppGenesis(appGenesis)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to unmarshal genesis state: %w", err)
	}

	authGenState := authtypes.GetGenesisStateFromAppState(clientCtx.Codec, appState)
	bankGenState := banktypes.GetGenesisStateFromAppState(clientCtx.Codec, appState)
	accs, err := authtypes.UnpackAccounts(authGenState.Accounts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to extract accounts: %w", err)
	}

	// --- claims: delayed vesting, optional immediate delegation ---
	sort.SliceStable(claims, func(i, j int) bool {
		return claims[i].Address() < claims[j].Address()
	})
	nonStakedPortion := getNonStakedPortion()
	for _, claim := range claims {
		addr, err := encodingConfig.TxConfig.SigningContext().AddressCodec().StringToBytes(claim.Address())
		if err != nil {
			return nil, nil, nil, err
		}
		accs, err = AddCustomVestingGenesisAccount(
			claim, addr, 0, viper.GetInt64("claims.vesting.end_date"),
			encodingConfig, accs, bankGenState, true,
		)
		if err != nil {
			slog.Error(err.Error())
			return nil, nil, nil, err
		}
		if strings.TrimSpace(claim.DelegateTo()) != "" {
			if _, ok := validatorsReference[claim.DelegateTo()]; !ok {
				return nil, nil, nil, fmt.Errorf("validator reference for '%s' does not exist", claim.DelegateTo())
			}
			delegations = append(delegations, stakingtypes.Delegation{
				DelegatorAddress: claim.Address(),
				ValidatorAddress: validatorsReference[claim.DelegateTo()].OperatorAddress,
				Shares:           math.LegacyNewDec(claim.Amount() - nonStakedPortion),
			})
		}
	}

	// --- grants: continuous vesting (start→end), never pre-delegated ---
	for _, grant := range grants {
		addr, err := encodingConfig.TxConfig.SigningContext().AddressCodec().StringToBytes(grant.Address())
		if err != nil {
			return nil, nil, nil, err
		}
		accs, err = AddCustomVestingGenesisAccount(
			grant, addr,
			viper.GetInt64("grants.vesting.start_date"),
			viper.GetInt64("grants.vesting.end_date"),
			encodingConfig, accs, bankGenState, false,
		)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	return delegations, appState, appGenesis, sealVestingState(clientCtx, accs, authGenState, bankGenState, appState, appGenesis)
}

func sealVestingState(
	clientCtx client.Context,
	accs authtypes.GenesisAccounts,
	authGenState authtypes.GenesisState,
	bankGenState *banktypes.GenesisState,
	appState map[string]json.RawMessage,
	appGenesis *genutiltypes.AppGenesis,
) error {
	accs = authtypes.SanitizeGenesisAccounts(accs)
	packedAccounts, err := authtypes.PackAccounts(accs)
	if err != nil {
		return fmt.Errorf("failed to pack accounts: %w", err)
	}
	authGenState.Accounts = packedAccounts
	bankGenState.Balances = banktypes.SanitizeGenesisBalances(bankGenState.Balances)

	authGenStateBz, err := clientCtx.Codec.MarshalJSON(&authGenState)
	if err != nil {
		return fmt.Errorf("failed to marshal auth genesis state: %w", err)
	}
	appState[authtypes.ModuleName] = authGenStateBz

	bankGenStateBz, err := clientCtx.Codec.MarshalJSON(bankGenState)
	if err != nil {
		return fmt.Errorf("failed to marshal bank genesis state: %w", err)
	}
	appState[banktypes.ModuleName] = bankGenStateBz

	appStateJSON, err := json.Marshal(appState)
	if err != nil {
		return fmt.Errorf("failed to marshal application genesis state: %w", err)
	}
	appGenesis.AppState = appStateJSON
	return genutil.ExportGenesisFile(appGenesis, viper.GetString("genesis.output"))
}

type ValidatorAddresses struct {
	OperatorAddress  string
	DelegatorAddress string
}

func (va Accounts) appendValidators(
	ctx context.Context,
	encodingConfig encoding.EncodingConfig,
	clientCtx client.Context,
) (map[string]ValidatorAddresses, error) {
	validators, err := va.validatorRepository.GetValidators(ctx)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(validators, func(i, j int) bool {
		return validators[i].Name() < validators[j].Name()
	})

	if err := addValidatorsToGenesis(encodingConfig, clientCtx, validators); err != nil {
		return nil, err
	}
	return buildValidatorReference(validators), nil
}

func addValidatorsToGenesis(encodingConfig encoding.EncodingConfig, clientCtx client.Context, validators []validator.Validator) error {
	for i := range validators {
		addr, err := encodingConfig.TxConfig.SigningContext().AddressCodec().StringToBytes(validators[i].DelegatorAddress())
		if err != nil {
			return err
		}
		if err := genutil.AddGenesisAccount(clientCtx.Codec, addr, true, viper.GetString("genesis.output"), "", "", 0, 0, ""); err != nil {
			return err
		}
	}
	return nil
}

func buildValidatorReference(validators []validator.Validator) map[string]ValidatorAddresses {
	ref := make(map[string]ValidatorAddresses, len(validators))
	for i := range validators {
		ref[validators[i].Name()] = ValidatorAddresses{
			OperatorAddress:  validators[i].OperatorAddress(),
			DelegatorAddress: validators[i].DelegatorAddress(),
		}
	}
	return ref
}

func (va Accounts) appendModuleAccounts(
	_ context.Context,
	encodingConfig encoding.EncodingConfig,
	clientCtx client.Context,
) error {
	hrp := viper.GetString("chain.address_prefix")
	denom := viper.GetString("default_bond_denom")

	type moduleEntry struct {
		address     string
		amount      int64
		permissions []string
	}

	validators, err := va.validatorRepository.GetValidators(context.Background())
	if err != nil {
		return err
	}
	var bondedTokens int64
	for i := range validators {
		if validators[i].Amount() > stdmath.MaxInt64-bondedTokens {
			return fmt.Errorf("bonded token overflow: cannot add %d to accumulated %d", validators[i].Amount(), bondedTokens)
		}
		bondedTokens += validators[i].Amount()
	}

	standardModules := []struct {
		name        string
		amount      int64
		permissions []string
	}{
		{"bonded_tokens_pool", bondedTokens, []string{authtypes.Burner, authtypes.Staking}},
		{"not_bonded_tokens_pool", 0, []string{authtypes.Burner, authtypes.Staking}},
		{"gov", 0, []string{authtypes.Burner}},
		{"distribution", 0, []string{}},
		{"mint", 0, []string{authtypes.Minter}},
		{"fee_collector", 0, []string{}},
	}

	modules := make(map[string]moduleEntry)
	for _, m := range standardModules {
		addr, err := moduleAddress(hrp, m.name)
		if err != nil {
			return err
		}
		modules[m.name] = moduleEntry{addr, m.amount, m.permissions}
	}

	// Extra modules from config (e.g. chain-specific modules like "meta")
	type extraModuleConfig struct {
		Name        string   `mapstructure:"name"`
		Permissions []string `mapstructure:"permissions"`
	}
	var extraModules []extraModuleConfig
	_ = viper.UnmarshalKey("modules.extra", &extraModules)
	for _, em := range extraModules {
		addr, err := moduleAddress(hrp, em.Name)
		if err != nil {
			return fmt.Errorf("failed to compute address for extra module %s: %w", em.Name, err)
		}
		modules[em.Name] = moduleEntry{addr, 0, em.Permissions}
	}

	moduleKeys := make([]string, 0, len(modules))
	for k := range modules {
		moduleKeys = append(moduleKeys, k)
	}
	sort.Strings(moduleKeys)

	for _, key := range moduleKeys {
		m := modules[key]
		addr, err := encodingConfig.TxConfig.SigningContext().AddressCodec().StringToBytes(m.address)
		if err != nil {
			return err
		}
		if err := AddCustomModuleGenesisAccount(
			clientCtx.Codec,
			addr,
			viper.GetString("genesis.output"),
			strconv.FormatInt(m.amount, 10)+denom,
			key,
			m.permissions,
		); err != nil {
			return err
		}
	}
	return nil
}

func (va Accounts) appendInitialAccounts(encodingConfig encoding.EncodingConfig, clientCtx client.Context) error {
	initialAccounts, err := va.initialAccountsRepo.GetInitialAccounts(context.Background(), encodingConfig)
	if err != nil {
		return err
	}
	if len(initialAccounts) == 0 {
		return fmt.Errorf("accounts.file_name CSV is empty; at least one account is required")
	}

	denom := viper.GetString("default_bond_denom")
	for _, acc := range initialAccounts {
		if acc.Amount() == 0 {
			continue
		}
		addr, err := encodingConfig.TxConfig.SigningContext().AddressCodec().StringToBytes(acc.Address())
		if err != nil {
			return err
		}
		amountStr := strconv.FormatInt(acc.Amount(), 10) + denom
		if err := genutil.AddGenesisAccount(
			clientCtx.Codec, addr, false, viper.GetString("genesis.output"), amountStr, "", 0, 0, "",
		); err != nil {
			return err
		}
	}
	return nil
}
