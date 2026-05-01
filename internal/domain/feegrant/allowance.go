package feegrant

import (
	"errors"
	"fmt"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

var ErrInvalidFeeAllowance = errors.New("invalid fee allowance")

type FeeAllowance struct {
	granter    string
	grantee    string
	spendLimit int64 // base denom amount; 0 = no spend limit
	expiry     int64 // unix timestamp; 0 = no expiry
}

func (a FeeAllowance) Granter() string   { return a.granter }
func (a FeeAllowance) Grantee() string   { return a.grantee }
func (a FeeAllowance) SpendLimit() int64 { return a.spendLimit }
func (a FeeAllowance) Expiry() int64     { return a.expiry }

func NewFeeAllowance(granter, grantee string, spendLimit, expiry int64, enc encoding.EncodingConfig) (*FeeAllowance, error) {
	a := FeeAllowance{granter: granter, grantee: grantee, spendLimit: spendLimit, expiry: expiry}
	if err := a.validate(enc); err != nil {
		return nil, err
	}
	return &a, nil
}

func (a FeeAllowance) validate(enc encoding.EncodingConfig) error {
	codec := enc.TxConfig.SigningContext().AddressCodec()
	if _, err := codec.StringToBytes(a.granter); err != nil {
		return fmt.Errorf("%w: invalid granter %q: %w", ErrInvalidFeeAllowance, a.granter, err)
	}
	if _, err := codec.StringToBytes(a.grantee); err != nil {
		return fmt.Errorf("%w: invalid grantee %q: %w", ErrInvalidFeeAllowance, a.grantee, err)
	}
	if a.spendLimit < 0 {
		return fmt.Errorf("%w: spend_limit must be >= 0, got %d", ErrInvalidFeeAllowance, a.spendLimit)
	}
	if a.expiry < 0 {
		return fmt.Errorf("%w: expiry must be >= 0, got %d", ErrInvalidFeeAllowance, a.expiry)
	}
	return nil
}
