package csv

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/vesting_account"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

const expectedPreAllocationSize = 500000

type ClaimRepository struct {
	filePath        string
	moduleAddresses map[string]bool
}

func NewCSVClaimRepository(filePath string, moduleAddresses map[string]bool) *ClaimRepository {
	return &ClaimRepository{filePath: filePath, moduleAddresses: moduleAddresses}
}

func (r *ClaimRepository) GetClaims(ctx context.Context, encodingConfig encoding.EncodingConfig) ([]vesting_account.Claim, error) {
	claims := make([]vesting_account.Claim, 0, expectedPreAllocationSize)
	err := readCSVRecords(ctx, r.filePath, r.moduleAddresses, -1, func(record []string) error {
		claimRecord, err := parseCSVClaimRecord(record, encodingConfig)
		if err != nil {
			return err
		}
		claims = append(claims, *claimRecord)
		return nil
	})
	return claims, err
}

func parseCSVClaimRecord(record []string, encodingConfig encoding.EncodingConfig) (*vesting_account.Claim, error) {
	if len(record) < 2 || len(record) > 3 {
		return nil, fmt.Errorf("invalid record format: expected 2 or 3 fields, got %d", len(record))
	}
	address := strings.TrimSpace(record[0])
	delegateTo := ""
	if len(record) == 3 {
		delegateTo = record[2]
	}
	amount, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid amount '%s': %w", record[1], err)
	}
	newClaim, err := vesting_account.NewClaim(address, amount, delegateTo, encodingConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create claim for record %v: %w", record, err)
	}
	return newClaim, nil
}
