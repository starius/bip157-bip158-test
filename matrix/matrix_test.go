package matrix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bip157-bip158-test/suite/score"
)

func TestLoadAndMarkdown(t *testing.T) {
	dir := t.TempDir()
	writeRun(t, filepath.Join(dir, "a"), score.Summary{
		Color: score.Green,
		Results: []score.Result{{
			ID:     "bip158.coinbase_output_included",
			Title:  "coinbase output",
			Level:  score.Must,
			Status: score.Pass,
		}},
	})
	writeRun(t, filepath.Join(dir, "b"), score.Summary{
		Color: score.Red,
		Results: []score.Result{{
			ID:     "bip158.coinbase_output_included",
			Title:  "coinbase output",
			Level:  score.Must,
			Status: score.Fail,
		}},
	})

	table, err := Load([]Run{
		{Implementation: "impl-a", Environment: "ipv4", Dir: filepath.Join(dir, "a")},
		{Implementation: "impl-b", Environment: "tor-v3", Dir: filepath.Join(dir, "b")},
	})
	if err != nil {
		t.Fatalf("load matrix: %v", err)
	}
	if len(table.Implementations) != 2 {
		t.Fatalf("implementations = %d, want 2", len(table.Implementations))
	}
	if len(table.Columns) != 2 || table.Columns[0].Label != "impl-a@ipv4" {
		t.Fatalf("unexpected columns: %+v", table.Columns)
	}
	md := Markdown(table)
	for _, want := range []string{
		"| `impl-a@ipv4` | `green` |",
		"| `impl-b@tor-v3` | `red` |",
		"`bip158.coinbase_output_included`",
		"`MUST`",
		"`pass`",
		"`fail`",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q:\n%s", want, md)
		}
	}
}

func writeRun(t *testing.T, dir string, summary score.Summary) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data := `{"color":"` + string(summary.Color) + `","results":[`
	for i, result := range summary.Results {
		if i > 0 {
			data += ","
		}
		data += `{"id":"` + result.ID + `","title":"` + result.Title +
			`","level":"` + string(result.Level) + `","status":"` +
			string(result.Status) + `"}`
	}
	data += `]}`
	if err := os.WriteFile(filepath.Join(dir, "run.json"), []byte(data), 0o644); err != nil {
		t.Fatalf("write run: %v", err)
	}
}
