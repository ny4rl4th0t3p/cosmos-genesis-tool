package vesting_account

type VestingAccount interface {
	Address() string
	Amount() int64
	DelegateTo() string
}
