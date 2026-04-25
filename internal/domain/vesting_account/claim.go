package vesting_account

import (
	"errors"
	"fmt"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

var ErrInvalidClaim = errors.New("invalid claim")

type Claim struct {
	address    string
	amount     int64
	delegateTo string
}

func (claim Claim) Address() string {
	return claim.address
}

func (claim Claim) Amount() int64 {
	return claim.amount
}

func (claim Claim) DelegateTo() string {
	return claim.delegateTo
}

func NewClaim(address string, amount int64, delegate string, encodingConfig encoding.EncodingConfig) (*Claim, error) {
	claim := Claim{address: address, amount: amount, delegateTo: delegate}
	if err := claim.Validate(encodingConfig); err != nil {
		return nil, err
	}
	return &claim, nil
}

func (claim Claim) Validate(encodingConfig encoding.EncodingConfig) error {
	if err := claim.validateAddress(encodingConfig); err != nil {
		return err
	}
	return claim.validateAmount()
}

func (claim Claim) validateAddress(encodingConfig encoding.EncodingConfig) error {
	if _, err := encodingConfig.TxConfig.SigningContext().AddressCodec().StringToBytes(claim.address); err != nil {
		return fmt.Errorf("%w: invalid address %q: %w", ErrInvalidClaim, claim.address, err)
	}
	return nil
}

func (claim Claim) validateAmount() error {
	if claim.amount < 1 {
		return fmt.Errorf("%w: amount must be > 0, got %d", ErrInvalidClaim, claim.amount)
	}
	return nil
}
