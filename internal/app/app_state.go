package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/cometbft/cometbft/config"
	"github.com/cosmos/cosmos-sdk/client"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/spf13/viper"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/repository"
)

type StateManager struct {
	claimRepository        repository.ClaimRepository
	grantRepository        repository.GrantRepository
	initialAccountsRepo    repository.InitialAccountsRepository
	validatorRepository    repository.ValidatorRepository
	authzGrantRepository   repository.AuthzGrantRepository
	feeAllowanceRepository repository.FeeAllowanceRepository
	accounts               *Accounts
	appGenState            map[string]json.RawMessage
	appGenesis             *genutiltypes.AppGenesis
	encodingConfig         encoding.EncodingConfig
	config                 *config.Config
	clientCtx              client.Context
}

func NewAppStateManager(
	claimRepository repository.ClaimRepository,
	grantRepository repository.GrantRepository,
	initialAccountsRepo repository.InitialAccountsRepository,
	validatorRepository repository.ValidatorRepository,
	authzGrantRepository repository.AuthzGrantRepository,
	feeAllowanceRepository repository.FeeAllowanceRepository,
	appGenState map[string]json.RawMessage,
	appGenesis *genutiltypes.AppGenesis,
	encodingConfig encoding.EncodingConfig,
	cfg *config.Config,
	clientCtx client.Context,
) *StateManager {
	return &StateManager{
		claimRepository:        claimRepository,
		grantRepository:        grantRepository,
		initialAccountsRepo:    initialAccountsRepo,
		validatorRepository:    validatorRepository,
		authzGrantRepository:   authzGrantRepository,
		feeAllowanceRepository: feeAllowanceRepository,
		accounts:               NewAccounts(claimRepository, grantRepository, initialAccountsRepo, validatorRepository),
		appGenState:            appGenState,
		appGenesis:             appGenesis,
		encodingConfig:         encodingConfig,
		config:                 cfg,
		clientCtx:              clientCtx,
	}
}

func (asm StateManager) SetupAppState(ctx context.Context) (map[string]int64, error) {
	outputPath := viper.GetString("genesis.output")

	slog.Info("Fixing governance parameters...")
	if err := asm.fixGovernanceParameters(asm.appGenState); err != nil {
		return nil, err
	}

	slog.Info("Fixing mint parameters...")
	if err := asm.fixMintParameters(asm.appGenState); err != nil {
		return nil, err
	}

	slog.Info("Saving initial app state...")
	if err := saveGenesis(asm.appGenState, asm.appGenesis, outputPath); err != nil {
		return nil, err
	}

	slog.Info("Appending module accounts...")
	if err := asm.accounts.appendModuleAccounts(ctx, asm.encodingConfig, asm.clientCtx); err != nil {
		return nil, err
	}

	slog.Info("Fetching validator shares...")
	shares, err := asm.accounts.fetchValidatorsShares(asm.encodingConfig)
	if err != nil {
		return nil, err
	}

	slog.Info("Appending validators...")
	validatorsReference, err := asm.accounts.appendValidators(ctx, asm.encodingConfig, asm.clientCtx)
	if err != nil {
		return nil, err
	}

	slog.Info("Appending initial accounts...")
	if err := asm.accounts.appendInitialAccounts(asm.encodingConfig, asm.clientCtx); err != nil {
		return nil, err
	}

	slog.Info("Appending claims and grants...")
	delegations, updatedAppState, updatedAppGenesis, err := asm.accounts.appendVestingAccounts(
		ctx, asm.encodingConfig, asm.clientCtx, validatorsReference)
	if err != nil {
		return nil, err
	}
	asm.appGenState = updatedAppState
	asm.appGenesis = updatedAppGenesis

	slog.Info("Configuring module states...")
	if err := asm.configureModuleStates(ctx, delegations, shares); err != nil {
		return nil, err
	}

	slog.Info("Validating total supply...")
	if err := asm.validateSupply(); err != nil {
		return nil, fmt.Errorf("supply validation failed: %w", err)
	}

	slog.Info("Saving final genesis file...")
	if err := saveGenesis(asm.appGenState, asm.appGenesis, outputPath); err != nil {
		return nil, err
	}

	slog.Info("SetupAppState completed successfully.")
	return shares, nil
}

func (asm StateManager) configureModuleStates(ctx context.Context, delegations []stakingtypes.Delegation, shares map[string]int64) error {
	slog.Info("Configuring staking parameters...")
	if err := asm.setStakingState(asm.appGenState, delegations, shares); err != nil {
		return err
	}

	slog.Info("Configuring denomination metadata...")
	if err := asm.setDenominationMetadata(); err != nil {
		return err
	}

	slog.Info("Configuring distribution parameters...")
	if err := asm.setDistribution(asm.appGenState, delegations); err != nil {
		return err
	}

	slog.Info("Configuring slashing parameters...")
	if err := asm.setSlashingState(asm.appGenState); err != nil {
		return err
	}

	slog.Info("Configuring authz grants...")
	if err := asm.setAuthzState(ctx, asm.appGenState); err != nil {
		return err
	}

	slog.Info("Configuring fee allowances...")
	if err := asm.setFeegrantState(ctx, asm.appGenState); err != nil {
		return err
	}

	return nil
}
