package csv

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	domainfeegrant "github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/feegrant"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

type FeeAllowanceRepository struct {
	filePath        string
	moduleAddresses map[string]bool
}

func NewCSVFeeAllowanceRepository(filePath string, moduleAddresses map[string]bool) *FeeAllowanceRepository {
	return &FeeAllowanceRepository{filePath: filePath, moduleAddresses: moduleAddresses}
}

func (r *FeeAllowanceRepository) GetFeeAllowances(ctx context.Context, enc encoding.EncodingConfig) ([]domainfeegrant.FeeAllowance, error) {
	var allowances []domainfeegrant.FeeAllowance
	err := readCSVRecords(ctx, r.filePath, r.moduleAddresses, -1, func(record []string) error {
		a, err := parseFeeAllowanceRecord(record, enc)
		if err != nil {
			return err
		}
		allowances = append(allowances, *a)
		return nil
	})
	return allowances, err
}

func parseFeeAllowanceRecord(record []string, enc encoding.EncodingConfig) (*domainfeegrant.FeeAllowance, error) {
	if len(record) < 3 || len(record) > 4 {
		return nil, fmt.Errorf("expected 3 or 4 fields, got %d", len(record))
	}
	granter := strings.TrimSpace(record[0])
	grantee := strings.TrimSpace(record[1])

	spendLimit, err := strconv.ParseInt(strings.TrimSpace(record[2]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid spend_limit %q: %w", record[2], err)
	}

	var expiry int64
	if len(record) == 4 {
		v, err := strconv.ParseInt(strings.TrimSpace(record[3]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid expiry %q: %w", record[3], err)
		}
		expiry = v
	}

	return domainfeegrant.NewFeeAllowance(granter, grantee, spendLimit, expiry, enc)
}
