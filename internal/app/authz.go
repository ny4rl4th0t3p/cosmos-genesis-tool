package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	authztypes "github.com/cosmos/cosmos-sdk/x/authz"
	"github.com/spf13/viper"
)

func (asm StateManager) setAuthzState(ctx context.Context, appGenState map[string]json.RawMessage) error {
	if !viper.IsSet("authz.file_name") {
		return nil
	}

	grants, err := asm.authzGrantRepository.GetAuthzGrants(ctx, asm.encodingConfig)
	if err != nil {
		return fmt.Errorf("failed to read authz grants: %w", err)
	}
	if len(grants) == 0 {
		return nil
	}

	var genGrants []authztypes.GrantAuthorization
	for _, g := range grants {
		authorization := &authztypes.GenericAuthorization{Msg: g.MsgTypeURL()}
		anyAuth, err := codectypes.NewAnyWithValue(authorization)
		if err != nil {
			return fmt.Errorf("failed to pack authz authorization for %s→%s: %w", g.Granter(), g.Grantee(), err)
		}
		ga := authztypes.GrantAuthorization{
			Granter:       g.Granter(),
			Grantee:       g.Grantee(),
			Authorization: anyAuth,
		}
		if g.Expiry() > 0 {
			t := time.Unix(g.Expiry(), 0).UTC()
			ga.Expiration = &t
		}
		genGrants = append(genGrants, ga)
	}

	var authzGenState authztypes.GenesisState
	return updateModuleState(asm.encodingConfig.Codec, appGenState, "authz", &authzGenState, func() error {
		authzGenState.Authorization = append(authzGenState.Authorization, genGrants...)
		return nil
	})
}
