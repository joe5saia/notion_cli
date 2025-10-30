// Package render provides helpers for formatting CLI output.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

const (
	tabWriterMinWidth = 0
	tabWriterTabWidth = 2
	tabWriterPadding  = 2
	tabWriterFlags    = 0
)

// JSON writes the supplied value as indented JSON.
func JSON(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(v); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

// Table renders the provided headers and rows via a tabwriter.
func Table(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, tabWriterMinWidth, tabWriterTabWidth, tabWriterPadding, ' ', tabWriterFlags)
	if len(headers) > 0 {
		if err := writeRow(tw, headers); err != nil {
			return err
		}
	}
	for _, row := range rows {
		if err := writeRow(tw, row); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flush table: %w", err)
	}
	return nil
}

func writeRow(w io.Writer, columns []string) error {
	if len(columns) == 0 {
		if _, err := fmt.Fprintln(w); err != nil {
			return fmt.Errorf("write row: %w", err)
		}
		return nil
	}

	line := strings.Join(columns, "\t")
	if _, err := fmt.Fprintln(w, line); err != nil {
		return fmt.Errorf("write row: %w", err)
	}
	return nil
}
