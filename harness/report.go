// Package harness runs conformance scenarios and writes reports.
package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bip157-bip158-test/suite/scenario"
	"github.com/bip157-bip158-test/suite/score"
)

// WriteReports saves machine-readable JSON and a human-readable Markdown
// summary. Reports intentionally include skipped catalog entries so missing
// coverage is visible.
func WriteReports(dir string, summary score.Summary) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create report dir: %w", err)
	}
	jsonBytes, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json report: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run.json"), jsonBytes, 0o644); err != nil {
		return fmt.Errorf("write json report: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "summary.md"), []byte(markdown(summary)), 0o644); err != nil {
		return fmt.Errorf("write markdown report: %w", err)
	}
	return nil
}

func markdown(summary score.Summary) string {
	var b strings.Builder
	b.WriteString("# BIP157/BIP158 Conformance Report\n\n")
	b.WriteString("Overall: `" + string(summary.Color) + "`\n\n")
	b.WriteString("| Scenario | Level | Status | Evidence |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, result := range summary.Results {
		b.WriteString("| `" + result.ID + "` | `" + string(result.Level) + "` | `" + string(result.Status) + "` | " + escape(result.Evidence) + " |\n")
	}
	b.WriteString("\n## Catalog Coverage\n\n")
	b.WriteString(fmt.Sprintf("Catalog size: %d scenarios.\n", len(scenario.Catalog())))
	return b.String()
}

func escape(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}
