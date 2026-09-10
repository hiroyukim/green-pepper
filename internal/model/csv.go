package model

import (
	"encoding/csv"
	"fmt"
	"os"
)

// CSVData holds the parsed data file: column order plus one variable map per row.
type CSVData struct {
	Columns []string
	Rows    []map[string]string
}

// LoadCSV reads a CSV file whose header row names the template variables and
// whose remaining rows supply one value set per request execution.
func LoadCSV(path string) (*CSVData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("reading data file: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing data file: %w", err)
	}
	if len(records) < 1 {
		return nil, fmt.Errorf("data file %s: missing header row", path)
	}

	header := records[0]
	rows := make([]map[string]string, 0, len(records)-1)
	for i, rec := range records[1:] {
		if len(rec) != len(header) {
			return nil, fmt.Errorf("data file %s: row %d has %d columns, want %d", path, i+2, len(rec), len(header))
		}
		row := make(map[string]string, len(header))
		for j, col := range header {
			row[col] = rec[j]
		}
		rows = append(rows, row)
	}

	return &CSVData{Columns: header, Rows: rows}, nil
}
