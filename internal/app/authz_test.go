package app

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	authztypes "github.com/cosmos/cosmos-sdk/x/authz"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainauthz "github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/authz"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

type stubAuthzGrantRepo struct {
	grants []domainauthz.AuthzGrant
	err    error
}

func (s stubAuthzGrantRepo) GetAuthzGrants(_ context.Context, _ encoding.EncodingConfig) ([]domainauthz.AuthzGrant, error) {
	return s.grants, s.err
}

func makeAuthzGrant(t *testing.T, ec encoding.EncodingConfig, granterIdx, granteeIdx byte, msgType string, expiry int64) domainauthz.AuthzGrant {
	t.Helper()
	granter := testAccAddr(granterIdx).String()
	grantee := testAccAddr(granteeIdx).String()
	g, err := domainauthz.NewAuthzGrant(granter, grantee, msgType, expiry, ec)
	require.NoError(t, err)
	return *g
}

func authzAppState(t *testing.T, ec encoding.EncodingConfig) map[string]json.RawMessage {
	t.Helper()
	gs := authztypes.DefaultGenesisState()
	bz, err := ec.Codec.MarshalJSON(gs)
	require.NoError(t, err)
	return map[string]json.RawMessage{"authz": bz}
}

func readAuthzState(t *testing.T, appGenState map[string]json.RawMessage, ec encoding.EncodingConfig) *authztypes.GenesisState {
	t.Helper()
	var gs authztypes.GenesisState
	require.NoError(t, ec.Codec.UnmarshalJSON(appGenState["authz"], &gs))
	return &gs
}

func TestSetAuthzState_NoViperKey_Skipped(t *testing.T) {
	ec := encoding.NewEncodingConfig()
	appGenState := authzAppState(t, ec)
	asm := StateManager{encodingConfig: ec, authzGrantRepository: stubAuthzGrantRepo{}}

	require.NoError(t, asm.setAuthzState(context.Background(), appGenState))

	gs := readAuthzState(t, appGenState, ec)
	assert.Empty(t, gs.Authorization)
}

func TestSetAuthzState_EmptyGrants_Skipped(t *testing.T) {
	viper.Set("authz.file_name", "/tmp/authz.csv")
	t.Cleanup(func() { viper.Set("authz.file_name", nil) })

	ec := encoding.NewEncodingConfig()
	appGenState := authzAppState(t, ec)
	asm := StateManager{encodingConfig: ec, authzGrantRepository: stubAuthzGrantRepo{grants: nil}}

	require.NoError(t, asm.setAuthzState(context.Background(), appGenState))

	gs := readAuthzState(t, appGenState, ec)
	assert.Empty(t, gs.Authorization)
}

func TestSetAuthzState_RepoError_ReturnsError(t *testing.T) {
	viper.Set("authz.file_name", "/tmp/authz.csv")
	t.Cleanup(func() { viper.Set("authz.file_name", nil) })

	ec := encoding.NewEncodingConfig()
	appGenState := authzAppState(t, ec)
	sentinel := errors.New("repo fail")
	asm := StateManager{encodingConfig: ec, authzGrantRepository: stubAuthzGrantRepo{err: sentinel}}

	err := asm.setAuthzState(context.Background(), appGenState)
	require.ErrorIs(t, err, sentinel)
}

func TestSetAuthzState_PopulatedGrants_WrittenToGenesis(t *testing.T) {
	viper.Set("authz.file_name", "/tmp/authz.csv")
	t.Cleanup(func() { viper.Set("authz.file_name", nil) })

	ec := encoding.NewEncodingConfig()
	g1 := makeAuthzGrant(t, ec, 1, 2, "/cosmos.bank.v1beta1.MsgSend", 0)
	g2 := makeAuthzGrant(t, ec, 3, 4, "/cosmos.staking.v1beta1.MsgDelegate", 1900000000)

	appGenState := authzAppState(t, ec)
	asm := StateManager{encodingConfig: ec, authzGrantRepository: stubAuthzGrantRepo{grants: []domainauthz.AuthzGrant{g1, g2}}}

	require.NoError(t, asm.setAuthzState(context.Background(), appGenState))

	gs := readAuthzState(t, appGenState, ec)
	require.Len(t, gs.Authorization, 2)
	assert.Equal(t, testAccAddr(1).String(), gs.Authorization[0].Granter)
	assert.Equal(t, testAccAddr(2).String(), gs.Authorization[0].Grantee)
	assert.Nil(t, gs.Authorization[0].Expiration)
	require.NotNil(t, gs.Authorization[1].Expiration)
	assert.Equal(t, time.Unix(1900000000, 0).UTC(), *gs.Authorization[1].Expiration)
}

func TestSetAuthzState_GenericAuthorization_TypeURLContainsGeneric(t *testing.T) {
	viper.Set("authz.file_name", "/tmp/authz.csv")
	t.Cleanup(func() { viper.Set("authz.file_name", nil) })

	ec := encoding.NewEncodingConfig()
	g := makeAuthzGrant(t, ec, 1, 2, "/cosmos.bank.v1beta1.MsgSend", 0)

	appGenState := authzAppState(t, ec)
	asm := StateManager{encodingConfig: ec, authzGrantRepository: stubAuthzGrantRepo{grants: []domainauthz.AuthzGrant{g}}}

	require.NoError(t, asm.setAuthzState(context.Background(), appGenState))

	gs := readAuthzState(t, appGenState, ec)
	require.Len(t, gs.Authorization, 1)
	assert.Contains(t, gs.Authorization[0].Authorization.TypeUrl, "GenericAuthorization")
}
