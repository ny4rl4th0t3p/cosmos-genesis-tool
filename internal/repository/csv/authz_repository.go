package csv

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	domainauthz "github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/domain/authz"
	"github.com/ny4rl4th0t3p/cosmos-genesis-tool/internal/encoding"
)

type AuthzGrantRepository struct {
	filePath        string
	moduleAddresses map[string]bool
}

func NewCSVAuthzGrantRepository(filePath string, moduleAddresses map[string]bool) *AuthzGrantRepository {
	return &AuthzGrantRepository{filePath: filePath, moduleAddresses: moduleAddresses}
}

func (r *AuthzGrantRepository) GetAuthzGrants(ctx context.Context, enc encoding.EncodingConfig) ([]domainauthz.AuthzGrant, error) {
	var grants []domainauthz.AuthzGrant
	err := readCSVRecords(ctx, r.filePath, r.moduleAddresses, -1, func(record []string) error {
		g, err := parseAuthzGrantRecord(record, enc)
		if err != nil {
			return err
		}
		grants = append(grants, *g)
		return nil
	})
	return grants, err
}

func parseAuthzGrantRecord(record []string, enc encoding.EncodingConfig) (*domainauthz.AuthzGrant, error) {
	if len(record) < 3 || len(record) > 4 {
		return nil, fmt.Errorf("expected 3 or 4 fields, got %d", len(record))
	}
	granter := strings.TrimSpace(record[0])
	grantee := strings.TrimSpace(record[1])
	msgTypeURL := strings.TrimSpace(record[2])

	var expiry int64
	if len(record) == 4 {
		v, err := strconv.ParseInt(strings.TrimSpace(record[3]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid expiry %q: %w", record[3], err)
		}
		expiry = v
	}

	return domainauthz.NewAuthzGrant(granter, grantee, msgTypeURL, expiry, enc)
}
