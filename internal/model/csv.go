package model

import (
	"encoding/csv"
	"fmt"
	"io"
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

	data, err := ParseCSV(f)
	if err != nil {
		return nil, fmt.Errorf("data file %s: %w", path, err)
	}
	return data, nil
}

// ParseCSV parses CSV content (header row of variable names, followed by one
// row per request execution) from an arbitrary reader, e.g. an uploaded file.
func ParseCSV(r io.Reader) (*CSVData, error) {
	records, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing CSV: %w", err)
	}
	if len(records) < 1 {
		return nil, fmt.Errorf("missing header row")
	}

	header := records[0]
	rows := make([]map[string]string, 0, len(records)-1)
	for i, rec := range records[1:] {
		if len(rec) != len(header) {
			return nil, fmt.Errorf("row %d has %d columns, want %d", i+2, len(rec), len(header))
		}
		row := make(map[string]string, len(header))
		for j, col := range header {
			row[col] = rec[j]
		}
		rows = append(rows, row)
	}

	return &CSVData{Columns: header, Rows: rows}, nil
}
