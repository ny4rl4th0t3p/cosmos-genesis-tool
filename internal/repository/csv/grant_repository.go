package csv

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/vesting_account"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

type GrantRepository struct {
	filePath        string
	moduleAddresses map[string]bool
}

func NewCSVGrantRepository(filePath string, moduleAddresses map[string]bool) *GrantRepository {
	return &GrantRepository{filePath: filePath, moduleAddresses: moduleAddresses}
}

func (r *GrantRepository) GetGrants(ctx context.Context, encodingConfig encoding.EncodingConfig) ([]vesting_account.Grant, error) {
	var grants []vesting_account.Grant
	err := readCSVRecords(ctx, r.filePath, r.moduleAddresses, 0, func(record []string) error {
		grantRecord, err := parseGrantRecord(record, encodingConfig)
		if err != nil {
			return err
		}
		grants = append(grants, *grantRecord)
		return nil
	})
	return grants, err
}

func parseGrantRecord(record []string, encodingConfig encoding.EncodingConfig) (*vesting_account.Grant, error) {
	if len(record) != 2 {
		return nil, fmt.Errorf("invalid record format: expected 2 fields, got %d", len(record))
	}
	address := strings.TrimSpace(record[0])
	amount, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid amount '%s': %w", record[1], err)
	}
	newGrant, err := vesting_account.NewGrant(address, amount, encodingConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create grant for record %v: %w", record, err)
	}
	return newGrant, nil
}
