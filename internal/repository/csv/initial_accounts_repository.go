package csv

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/accounts"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

type InitialAccountsRepository struct {
	filePath        string
	moduleAddresses map[string]bool
}

func NewCSVInitialAccountsRepository(filePath string, moduleAddresses map[string]bool) *InitialAccountsRepository {
	return &InitialAccountsRepository{filePath: filePath, moduleAddresses: moduleAddresses}
}

func (na *InitialAccountsRepository) GetInitialAccounts(
	ctx context.Context, encodingConfig encoding.EncodingConfig,
) ([]accounts.InitialAccount, error) {
	var initialAccounts []accounts.InitialAccount
	err := readCSVRecords(ctx, na.filePath, na.moduleAddresses, 0, func(record []string) error {
		accountRecord, err := parseInitialAccountRecord(record, encodingConfig)
		if err != nil {
			return err
		}
		initialAccounts = append(initialAccounts, *accountRecord)
		return nil
	})
	return initialAccounts, err
}

func parseInitialAccountRecord(record []string, encodingConfig encoding.EncodingConfig) (*accounts.InitialAccount, error) {
	address := strings.TrimSpace(record[0])
	var amount int64
	if len(record) == 2 && record[1] != "" {
		parsedAmount, err := strconv.ParseInt(strings.TrimSpace(record[1]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid amount '%s': %w", record[1], err)
		}
		amount = parsedAmount
	}
	acc, err := accounts.NewInitialAccount(address, amount, encodingConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create initial account for record %v: %w", record, err)
	}
	return acc, nil
}
