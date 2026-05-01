package repository

import (
	"context"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/accounts"
	domainauthz "github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/authz"
	domainfeegrant "github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/feegrant"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/validator"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/vesting_account"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

type ClaimRepository interface {
	GetClaims(ctx context.Context, encodingConfig encoding.EncodingConfig) ([]vesting_account.Claim, error)
}

type InitialAccountsRepository interface {
	GetInitialAccounts(ctx context.Context, encodingConfig encoding.EncodingConfig) ([]accounts.InitialAccount, error)
}

type GrantRepository interface {
	GetGrants(ctx context.Context, encodingConfig encoding.EncodingConfig) ([]vesting_account.Grant, error)
}

type ValidatorRepository interface {
	GetValidators(ctx context.Context) ([]validator.Validator, error)
}

type AuthzGrantRepository interface {
	GetAuthzGrants(ctx context.Context, encodingConfig encoding.EncodingConfig) ([]domainauthz.AuthzGrant, error)
}

type FeeAllowanceRepository interface {
	GetFeeAllowances(ctx context.Context, encodingConfig encoding.EncodingConfig) ([]domainfeegrant.FeeAllowance, error)
}
