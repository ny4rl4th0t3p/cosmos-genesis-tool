package csv

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// fieldsPerRecord maps to csv.Reader.FieldsPerRecord: -1 allows variable field counts,
// 0 infers the count from the first record, positive enforces a fixed count.
func readCSVRecords(
	ctx context.Context,
	filePath string,
	moduleAddresses map[string]bool,
	fieldsPerRecord int,
	processor func(record []string) error,
) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", filePath, err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			slog.Error("failed to close file", "file", filePath, "err", cerr)
		}
	}()

	reader := csv.NewReader(bufio.NewReader(file))
	if fieldsPerRecord != 0 {
		reader.FieldsPerRecord = fieldsPerRecord
	}

	var lineNum int
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		record, err := reader.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s line %d: error reading record: %w", filePath, lineNum+1, err)
		}
		lineNum++

		record[0] = strings.TrimSpace(record[0])
		if moduleAddresses[record[0]] {
			continue
		}

		if err := processor(record); err != nil {
			return fmt.Errorf("%s line %d: %w", filePath, lineNum, err)
		}
	}
}
