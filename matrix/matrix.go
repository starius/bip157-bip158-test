// Package matrix builds cross-implementation conformance tables from harness
// run reports.
package matrix

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bip157-bip158-test/suite/scenario"
	"github.com/bip157-bip158-test/suite/score"
)

// Run identifies one implementation report directory.
type Run struct {
	Implementation string
	Environment    string
	Dir            string
}

// Table is the normalized matrix used for Markdown and JSON output.
type Table struct {
	Implementations []string   `json:"implementations"`
	Columns         []Column   `json:"columns"`
	Rows            []Row      `json:"rows"`
	Colors          []RunColor `json:"colors"`
}

// Column identifies one implementation/environment run in the matrix.
type Column struct {
	Implementation string `json:"implementation"`
	Environment    string `json:"environment,omitempty"`
	Label          string `json:"label"`
}

// RunColor records one implementation's aggregate conformance color.
type RunColor struct {
	Implementation string      `json:"implementation"`
	Environment    string      `json:"environment,omitempty"`
	Label          string      `json:"label"`
	Color          score.Color `json:"color"`
}

// Row contains one scenario across all loaded implementations.
type Row struct {
	ID      string                    `json:"id"`
	Title   string                    `json:"title"`
	Level   score.Level               `json:"level"`
	Results map[string]Implementation `json:"results"`
}

// Implementation is the cell value for one implementation/scenario pair.
type Implementation struct {
	Status   score.Status `json:"status"`
	Evidence string       `json:"evidence,omitempty"`
}

// Load reads run.json from each report directory and returns a table with rows
// ordered by scenario ID. Rows include the catalog even when all runs skipped a
// scenario, so missing coverage is visible in the matrix.
func Load(runs []Run) (Table, error) {
	defs := scenario.Catalog()
	rows := make(map[string]Row, len(defs)+1)
	for _, def := range defs {
		rows[def.ID] = Row{
			ID:      def.ID,
			Title:   def.Title,
			Level:   def.Level,
			Results: map[string]Implementation{},
		}
	}
	rows["adapter.honest_wallet_receive_spend"] = Row{
		ID:      "adapter.honest_wallet_receive_spend",
		Title:   "honest peer wallet receive and spend",
		Level:   score.Must,
		Results: map[string]Implementation{},
	}

	table := Table{
		Implementations: make([]string, 0, len(runs)),
		Columns:         make([]Column, 0, len(runs)),
		Colors:          make([]RunColor, 0, len(runs)),
	}
	for _, run := range runs {
		if run.Implementation == "" {
			return Table{}, fmt.Errorf("implementation name is empty")
		}
		label := runLabel(run)
		summary, err := readSummary(filepath.Join(run.Dir, "run.json"))
		if err != nil {
			return Table{}, fmt.Errorf("%s: %w", label, err)
		}
		table.Implementations = append(table.Implementations, label)
		table.Columns = append(table.Columns, Column{
			Implementation: run.Implementation,
			Environment:    run.Environment,
			Label:          label,
		})
		table.Colors = append(table.Colors, RunColor{
			Implementation: run.Implementation,
			Environment:    run.Environment,
			Label:          label,
			Color:          summary.Color,
		})
		for _, result := range summary.Results {
			row := rows[result.ID]
			if row.ID == "" {
				row = Row{
					ID:      result.ID,
					Title:   result.Title,
					Level:   result.Level,
					Results: map[string]Implementation{},
				}
			}
			if row.Title == "" {
				row.Title = result.Title
			}
			if row.Level == "" {
				row.Level = result.Level
			}
			row.Results[label] = Implementation{
				Status:   result.Status,
				Evidence: result.Evidence,
			}
			rows[result.ID] = row
		}
	}

	table.Rows = make([]Row, 0, len(rows))
	for _, row := range rows {
		table.Rows = append(table.Rows, row)
	}
	sort.Slice(table.Rows, func(i, j int) bool {
		return table.Rows[i].ID < table.Rows[j].ID
	})
	return table, nil
}

// Markdown renders a compact matrix with one status cell per implementation.
func Markdown(table Table) string {
	var b strings.Builder
	b.WriteString("# BIP157/BIP158 Implementation Matrix\n\n")
	if len(table.Colors) > 0 {
		b.WriteString("| Run | Overall |\n")
		b.WriteString("| --- | --- |\n")
		for _, color := range table.Colors {
			b.WriteString("| `" + escape(color.Label) + "` | `" + string(color.Color) + "` |\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("| Test | BIP Status |")
	for _, implementation := range table.Implementations {
		b.WriteString(" " + escape(implementation) + " |")
	}
	b.WriteString("\n| --- | --- |")
	for range table.Implementations {
		b.WriteString(" --- |")
	}
	b.WriteString("\n")

	for _, row := range table.Rows {
		b.WriteString("| `" + escape(row.ID) + "` | `" + levelForMatrix(row.Level) + "` |")
		for _, implementation := range table.Implementations {
			cell, ok := row.Results[implementation]
			if !ok {
				b.WriteString(" `missing` |")
				continue
			}
			b.WriteString(" `" + string(cell.Status) + "` |")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// Write saves the matrix in Markdown and JSON form.
func Write(dir string, table Table) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create matrix dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "matrix.md"), []byte(Markdown(table)), 0o644); err != nil {
		return fmt.Errorf("write matrix markdown: %w", err)
	}
	data, err := json.MarshalIndent(table, "", "  ")
	if err != nil {
		return fmt.Errorf("encode matrix json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "matrix.json"), data, 0o644); err != nil {
		return fmt.Errorf("write matrix json: %w", err)
	}
	return nil
}

func runLabel(run Run) string {
	if run.Environment == "" {
		return run.Implementation
	}
	return run.Implementation + "@" + run.Environment
}

func readSummary(path string) (score.Summary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return score.Summary{}, err
	}
	var summary score.Summary
	if err := json.Unmarshal(data, &summary); err != nil {
		return score.Summary{}, err
	}
	return summary, nil
}

func levelForMatrix(level score.Level) string {
	switch level {
	case score.Must:
		return "MUST"
	case score.Should:
		return "SHOULD"
	default:
		return "OTHER"
	}
}

func escape(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}
