package accounts

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

var ErrInvalidInitialAccount = errors.New("invalid initial account")

type InitialAccount struct {
	address string
	amount  int64
}

func (a InitialAccount) Address() string { return a.address }
func (a InitialAccount) Amount() int64   { return a.amount }

func NewInitialAccount(address string, amount int64, encodingConfig encoding.EncodingConfig) (*InitialAccount, error) {
	a := InitialAccount{address: address, amount: amount}
	if err := a.Validate(encodingConfig); err != nil {
		return nil, err
	}
	return &a, nil
}

func (a InitialAccount) Validate(encodingConfig encoding.EncodingConfig) error {
	if _, err := encodingConfig.TxConfig.SigningContext().AddressCodec().StringToBytes(a.address); err != nil {
		return fmt.Errorf("%w: invalid address %q: %w", ErrInvalidInitialAccount, a.address, err)
	}
	return nil
}

func (a InitialAccount) IsInRemainderAllowedList() bool {
	address := strings.TrimSpace(a.address)
	for _, allowed := range viper.GetStringSlice("accounts.remainder_allowlist") {
		if address == allowed {
			return true
		}
	}
	return false
}
