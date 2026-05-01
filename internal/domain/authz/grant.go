package authz

import (
	"errors"
	"fmt"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

var ErrInvalidAuthzGrant = errors.New("invalid authz grant")

type AuthzGrant struct {
	granter    string
	grantee    string
	msgTypeURL string
	expiry     int64 // unix timestamp; 0 = no expiry
}

func (g AuthzGrant) Granter() string    { return g.granter }
func (g AuthzGrant) Grantee() string    { return g.grantee }
func (g AuthzGrant) MsgTypeURL() string { return g.msgTypeURL }
func (g AuthzGrant) Expiry() int64      { return g.expiry }

func NewAuthzGrant(granter, grantee, msgTypeURL string, expiry int64, enc encoding.EncodingConfig) (*AuthzGrant, error) {
	g := AuthzGrant{granter: granter, grantee: grantee, msgTypeURL: msgTypeURL, expiry: expiry}
	if err := g.validate(enc); err != nil {
		return nil, err
	}
	return &g, nil
}

func (g AuthzGrant) validate(enc encoding.EncodingConfig) error {
	codec := enc.TxConfig.SigningContext().AddressCodec()
	if _, err := codec.StringToBytes(g.granter); err != nil {
		return fmt.Errorf("%w: invalid granter %q: %w", ErrInvalidAuthzGrant, g.granter, err)
	}
	if _, err := codec.StringToBytes(g.grantee); err != nil {
		return fmt.Errorf("%w: invalid grantee %q: %w", ErrInvalidAuthzGrant, g.grantee, err)
	}
	if g.msgTypeURL == "" {
		return fmt.Errorf("%w: msg_type_url must not be empty", ErrInvalidAuthzGrant)
	}
	if g.expiry < 0 {
		return fmt.Errorf("%w: expiry must be >= 0, got %d", ErrInvalidAuthzGrant, g.expiry)
	}
	return nil
}
