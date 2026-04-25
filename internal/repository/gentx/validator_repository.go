package gentx

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/validator"
)

// Extend the structure to match full JSON schema of the file
var validatorJSON struct {
	Body struct {
		Messages []struct {
			Description struct {
				Moniker         string `json:"moniker"`
				Identity        string `json:"identity"`
				Website         string `json:"website"`
				SecurityContact string `json:"security_contact"`
				Details         string `json:"details"`
			} `json:"description"`
			ValidatorAddress string `json:"validator_address"`
			PubKey           struct {
				Type string `json:"@type"`
				Key  string `json:"key"`
			} `json:"pubkey"`
			Value struct {
				Denom  string `json:"denom"`
				Amount string `json:"amount"`
			} `json:"value"`
			Commission struct {
				Rate          string `json:"rate"`
				MaxRate       string `json:"max_rate"`
				MaxChangeRate string `json:"max_change_rate"`
			} `json:"commission"`
			MinSelfDelegation string `json:"min_self_delegation"`
		} `json:"messages"`
		Memo string `json:"memo"`
	} `json:"body"`
	AuthInfo struct {
		SignerInfos []struct {
			PublicKey struct {
				Type string `json:"@type"`
				Key  string `json:"key"`
			} `json:"public_key"`
			ModeInfo struct {
				Single struct {
					Mode string `json:"mode"`
				} `json:"single"`
			} `json:"mode_info"`
			Sequence string `json:"sequence"`
		} `json:"signer_infos"`
		Fee struct{} `json:"fee"`
	} `json:"auth_info"`
	Signatures []string `json:"signatures"`
}

type ValidatorRepository struct {
	gentTxFilesDir string
}

func NewValidatorRepository(jsonDir string) *ValidatorRepository {
	return &ValidatorRepository{gentTxFilesDir: jsonDir}
}

func (repo *ValidatorRepository) GetValidators(_ context.Context) ([]validator.Validator, error) {
	var validators []validator.Validator

	slog.Debug("Scanning JSON files in directory", slog.String("directory", repo.gentTxFilesDir))

	files, err := filepath.Glob(filepath.Join(repo.gentTxFilesDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON directory: %w", err)
	}

	if len(files) == 0 {
		slog.Warn("No JSON files found in directory", slog.String("directory", repo.gentTxFilesDir))
		return nil, fmt.Errorf("no JSON files found in directory '%s'", repo.gentTxFilesDir)
	}

	slog.Debug("Found JSON files", slog.Any("files", files))

	for _, file := range files {
		slog.Debug("Processing JSON file", slog.String("file", file))

		fileValidators, err := parseValidatorsFromFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to parse validators from file '%s': %w", file, err)
		}

		slog.Debug("Appending validators from file", slog.String("file", file), slog.Int("validator_count", len(fileValidators)))
		slog.Debug("Added validator:", slog.Any("validator", fileValidators[0]))
		validators = append(validators, fileValidators...)
	}

	slog.Debug("Successfully parsed all validators", slog.Int("total_validators", len(validators)))
	return validators, nil
}

func parseValidatorsFromFile(filePath string) ([]validator.Validator, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file '%s': %w", filePath, err)
	}
	defer func() {
		_ = file.Close()
	}()

	if err := json.NewDecoder(file).Decode(&validatorJSON); err != nil {
		return nil, fmt.Errorf("failed to decode JSON file '%s': %w", filePath, err)
	}

	var validators []validator.Validator
	for i := range validatorJSON.Body.Messages {
		msg := &validatorJSON.Body.Messages[i]
		slog.Debug("Parsing validator", slog.String("validator_address", msg.ValidatorAddress))

		amount, err := strconv.ParseInt(msg.Value.Amount, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid amount in JSON file '%s': %w", filePath, err)
		}

		var operatorPublicKey string
		if len(validatorJSON.AuthInfo.SignerInfos) > 0 {
			operatorPublicKey = validatorJSON.AuthInfo.SignerInfos[0].PublicKey.Key
		} else {
			slog.Warn("No signer_infos found in auth_info", slog.String("file", filePath))
		}

		v, err := validator.NewValidatorFromFields(
			strings.TrimSpace(msg.ValidatorAddress),
			msg.PubKey.Key,
			msg.PubKey.Type,
			msg.Description.Moniker,
			msg.Description.Identity,
			msg.Description.Website,
			msg.Description.SecurityContact,
			msg.Description.Details,
			msg.Commission.Rate,
			msg.Commission.MaxRate,
			msg.Commission.MaxChangeRate,
			msg.MinSelfDelegation,
			validatorJSON.Body.Memo,
			msg.Value.Denom,
			operatorPublicKey,
			amount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create validator from JSON file '%s': %w", filePath, err)
		}

		validators = append(validators, *v)
	}

	slog.Debug("Parsed all validators from file", slog.String("file", filePath), slog.Int("validator_count", len(validators)))
	return validators, nil
}
