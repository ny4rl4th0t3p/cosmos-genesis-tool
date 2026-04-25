package vesting_account

import (
	"errors"
	"fmt"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

var ErrInvalidGrant = errors.New("invalid grant")

type Grant struct {
	address string
	amount  int64
}

func (grant Grant) Address() string {
	return grant.address
}

func (grant Grant) Amount() int64 {
	return grant.amount
}

func (Grant) DelegateTo() string {
	return ""
}

func NewGrant(address string, amount int64, encodingConfig encoding.EncodingConfig) (*Grant, error) {
	grant := Grant{address: address, amount: amount}
	if err := grant.Validate(encodingConfig); err != nil {
		return nil, err
	}
	return &grant, nil
}

func (grant Grant) Validate(encodingConfig encoding.EncodingConfig) error {
	if err := grant.validateAddress(encodingConfig); err != nil {
		return err
	}
	return grant.validateAmount()
}

func (grant Grant) validateAddress(encodingConfig encoding.EncodingConfig) error {
	if _, err := encodingConfig.TxConfig.SigningContext().AddressCodec().StringToBytes(grant.address); err != nil {
		return fmt.Errorf("%w: invalid address %q: %w", ErrInvalidGrant, grant.address, err)
	}
	return nil
}

func (grant Grant) validateAmount() error {
	if grant.amount < 1 {
		return fmt.Errorf("%w: amount must be > 0, got %d", ErrInvalidGrant, grant.amount)
	}
	return nil
}
