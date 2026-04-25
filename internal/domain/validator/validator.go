package validator

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"

	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/spf13/viper"
)

var ErrInvalidValidator = errors.New("invalid validator")

const (
	errMsgPubKeyEmpty            = "pubKey cannot be empty"
	errMsgOperatorAddressEmpty   = "operatorAddress cannot be empty"
	errMsgDelegatorAddressEmpty  = "delegatorAddress cannot be empty"
	errMsgCommissionRateEmpty    = "commissionRate cannot be empty"
	errMsgMaxRateEmpty           = "maxRate cannot be empty"
	errMsgMaxChangeRateEmpty     = "maxChangeRate cannot be empty"
	errMsgMinSelfDelegationEmpty = "minSelfDelegation cannot be empty"
	errMsgOperatorPkEmpty        = "operatorPublicKey cannot be empty"
	errMsgNameEmpty              = "name cannot be empty"
	errMsgAmountTooLow           = "amount must be greater than 0"
)

type Validator struct {
	operatorAddress   string
	delegatorAddress  string
	consensusAddress  []byte
	amount            int64
	pubKey            string
	pubKeyType        string
	name              string
	identity          string
	website           string
	securityContact   string
	details           string
	commissionRate    string
	maxRate           string
	maxChangeRate     string
	minSelfDelegation string
	memo              string
	denom             string
	operatorPublicKey string
}

func (v Validator) OperatorAddress() string   { return v.operatorAddress }
func (v Validator) DelegatorAddress() string  { return v.delegatorAddress }
func (v Validator) ConsensusAddress() []byte  { return v.consensusAddress }
func (v Validator) Amount() int64             { return v.amount }
func (v Validator) PubKey() string            { return v.pubKey }
func (v Validator) Name() string              { return v.name }
func (v Validator) Identity() string          { return v.identity }
func (v Validator) Website() string           { return v.website }
func (v Validator) SecurityContact() string   { return v.securityContact }
func (v Validator) Details() string           { return v.details }
func (v Validator) CommissionRate() string    { return v.commissionRate }
func (v Validator) MaxRate() string           { return v.maxRate }
func (v Validator) MaxChangeRate() string     { return v.maxChangeRate }
func (v Validator) MinSelfDelegation() string { return v.minSelfDelegation }
func (v Validator) OperatorPublicKey() string { return v.operatorPublicKey }

func validateNonEmptyField(value, errorMessage string) bool {
	if value == "" {
		slog.Error(errorMessage)
		return false
	}
	return true
}

func (v Validator) Validate() bool {
	if v.amount < 1 {
		slog.Error(errMsgAmountTooLow)
		return false
	}
	return validateNonEmptyField(v.pubKey, errMsgPubKeyEmpty) &&
		validateNonEmptyField(v.operatorAddress, errMsgOperatorAddressEmpty) &&
		validateNonEmptyField(v.delegatorAddress, errMsgDelegatorAddressEmpty) &&
		validateNonEmptyField(v.commissionRate, errMsgCommissionRateEmpty) &&
		validateNonEmptyField(v.maxRate, errMsgMaxRateEmpty) &&
		validateNonEmptyField(v.maxChangeRate, errMsgMaxChangeRateEmpty) &&
		validateNonEmptyField(v.minSelfDelegation, errMsgMinSelfDelegationEmpty) &&
		validateNonEmptyField(v.name, errMsgNameEmpty) &&
		validateNonEmptyField(v.operatorPublicKey, errMsgOperatorPkEmpty)
}

// deriveDelegatorAddress re-encodes an operator address to the account address (same bytes, account HRP).
func deriveDelegatorAddress(operatorAddress string) (string, error) {
	hrp := viper.GetString("chain.address_prefix")
	_, bz, err := bech32.DecodeAndConvert(operatorAddress)
	if err != nil {
		return "", fmt.Errorf("failed to decode bech32 address: %w", err)
	}
	delegatorAddress, err := bech32.ConvertAndEncode(hrp, bz)
	if err != nil {
		return "", fmt.Errorf("failed to encode bech32 address for hrp %s: %w", hrp, err)
	}
	return delegatorAddress, nil
}

func NewValidatorFromFields(
	address, pubKey, pubKeyType, name, identity, website, securityContact, details,
	commissionRate, maxRate, maxChangeRate, minSelfDelegation, memo, denom, operatorPublicKey string,
	amount int64,
) (*Validator, error) {
	delegatorAddress, err := deriveDelegatorAddress(address)
	if err != nil {
		return nil, err
	}
	consensusAddress, err := deriveConsensusAddress(pubKey)
	if err != nil {
		return nil, err
	}

	v := &Validator{
		operatorAddress:   address,
		consensusAddress:  consensusAddress,
		delegatorAddress:  delegatorAddress,
		pubKey:            pubKey,
		pubKeyType:        pubKeyType,
		name:              name,
		identity:          identity,
		website:           website,
		securityContact:   securityContact,
		details:           details,
		commissionRate:    commissionRate,
		maxRate:           maxRate,
		maxChangeRate:     maxChangeRate,
		minSelfDelegation: minSelfDelegation,
		memo:              memo,
		denom:             denom,
		amount:            amount,
		operatorPublicKey: operatorPublicKey,
	}

	if !v.Validate() {
		return nil, ErrInvalidValidator
	}
	return v, nil
}

func deriveConsensusAddress(pubKeyBase64 string) ([]byte, error) {
	pubKeyBytes, err := base64.StdEncoding.DecodeString(pubKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode pubkey: %w", err)
	}
	hash := sha256.Sum256(pubKeyBytes)
	return hash[:20], nil // CometBFT address = first 20 bytes of SHA256(pubkey)
}
