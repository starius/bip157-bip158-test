// Package score implements the suite's green/orange/red result aggregation.
package score

import "sort"

// Level is the normative strength attached to a scenario.
type Level string

const (
	// Must corresponds to BIP requirements that use MUST, MUST NOT, REQUIRED,
	// SHALL, or SHALL NOT, plus gross correctness failures.
	Must Level = "MUST"
	// Should corresponds to BIP requirements that use SHOULD or SHOULD NOT.
	Should Level = "SHOULD"
	// Info records observations that do not affect conformance color.
	Info Level = "INFO"
)

// Status is the machine-readable scenario result.
type Status string

const (
	Pass        Status = "pass"
	Fail        Status = "fail"
	Unsupported Status = "unsupported"
	Skipped     Status = "skipped"
)

// Color is the conformance summary color.
type Color string

const (
	Green  Color = "green"
	Orange Color = "orange"
	Red    Color = "red"
)

// Result is one scenario's outcome.
type Result struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Level    Level  `json:"level"`
	Status   Status `json:"status"`
	Evidence string `json:"evidence,omitempty"`
}

// Summary is the aggregate result emitted by a conformance run.
type Summary struct {
	Color   Color    `json:"color"`
	Results []Result `json:"results"`
}

// Summarize applies the suite color policy to a list of scenario results.
func Summarize(results []Result) Summary {
	color := Green
	for _, result := range results {
		switch {
		case result.Level == Must && result.Status != Pass && result.Status != Skipped:
			color = Red
		case color == Green && result.Level == Should && result.Status != Pass && result.Status != Skipped:
			color = Orange
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})

	return Summary{
		Color:   color,
		Results: results,
	}
}
