package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"cosmossdk.io/core/address"
	"github.com/cometbft/cometbft/crypto/ed25519"
	comettypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/x/genutil/types"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/repository"
)

const (
	// microTokensPerToken converts micro-denomination tokens to whole tokens for consensus voting power.
	microTokensPerToken = 1_000_000

	defaultBlockMaxBytes           = 22020096
	defaultBlockMaxGas             = -1
	defaultEvidenceMaxAgeNumBlocks = 100000
	defaultEvidenceMaxAgeDuration  = 48 * time.Hour
	defaultEvidenceMaxBytes        = 1048576
)

type Consensus struct {
	appGenesis *types.AppGenesis
	codec      address.Codec
	repo       repository.ValidatorRepository
	shares     map[string]int64
}

func NewConsensus(
	repo repository.ValidatorRepository, appGenesis *types.AppGenesis, codec address.Codec, shares map[string]int64,
) *Consensus {
	return &Consensus{
		appGenesis: appGenesis,
		codec:      codec,
		repo:       repo,
		shares:     shares,
	}
}

func (c *Consensus) SetParams() error {
	validators, err := c.repo.GetValidators(context.Background())
	if err != nil {
		return err
	}
	var genValidators []comettypes.GenesisValidator
	for i := range validators {
		pubKeyBytes, err := base64.StdEncoding.DecodeString(validators[i].PubKey())
		if err != nil {
			return fmt.Errorf("failed to decode pubkey: %w", err)
		}
		pubKey := ed25519.PubKey(pubKeyBytes)
		if len(pubKey) != ed25519.PubKeySize {
			return fmt.Errorf("invalid pubkey length: expected %d, got %d", ed25519.PubKeySize, len(pubKeyBytes))
		}

		tokens := (validators[i].Amount() + c.shares[validators[i].Name()]) / microTokensPerToken

		gVal := comettypes.GenesisValidator{
			Address: validators[i].ConsensusAddress(),
			PubKey:  pubKey,
			Power:   tokens,
			Name:    validators[i].Name(),
		}
		genValidators = append(genValidators, gVal)
	}

	consensusParams := comettypes.ConsensusParams{
		Block: comettypes.BlockParams{
			MaxBytes: defaultBlockMaxBytes,
			MaxGas:   defaultBlockMaxGas,
		},
		Evidence: comettypes.EvidenceParams{
			MaxAgeNumBlocks: defaultEvidenceMaxAgeNumBlocks,
			MaxAgeDuration:  defaultEvidenceMaxAgeDuration,
			MaxBytes:        defaultEvidenceMaxBytes,
		},
		Validator: comettypes.ValidatorParams{
			PubKeyTypes: []string{"ed25519"},
		},
		Version: comettypes.VersionParams{
			App: 0,
		},
		ABCI: comettypes.ABCIParams{
			VoteExtensionsEnableHeight: 0,
		},
	}

	c.appGenesis.Consensus = &types.ConsensusGenesis{
		Validators: genValidators,
		Params:     &consensusParams,
	}
	return nil
}
