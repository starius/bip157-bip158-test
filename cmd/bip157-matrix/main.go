// Command bip157-matrix combines per-implementation conformance reports into
// one cross-implementation table.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bip157-bip158-test/suite/matrix"
)

type runFlags []matrix.Run

func (r *runFlags) String() string {
	return fmt.Sprint([]matrix.Run(*r))
}

func (r *runFlags) Set(value string) error {
	name, dir, ok := strings.Cut(value, "=")
	if !ok || name == "" || dir == "" {
		return fmt.Errorf("run must be name[@environment]=dir")
	}
	implementation := name
	environment := ""
	if before, after, ok := strings.Cut(name, "@"); ok {
		implementation = before
		environment = after
	}
	if implementation == "" {
		return fmt.Errorf("implementation name is empty")
	}
	*r = append(*r, matrix.Run{
		Implementation: implementation,
		Environment:    environment,
		Dir:            dir,
	})
	return nil
}

func main() {
	var runs runFlags
	var out string
	flag.Var(&runs, "run", "implementation[@environment]=report-dir containing run.json; repeatable")
	flag.StringVar(&out, "out", "", "directory for matrix.md and matrix.json")
	flag.Parse()

	if len(runs) == 0 {
		fmt.Fprintln(os.Stderr, "at least one --run implementation=dir is required")
		os.Exit(2)
	}
	table, err := matrix.Load(runs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load matrix: %v\n", err)
		os.Exit(1)
	}
	if out == "" {
		fmt.Print(matrix.Markdown(table))
		return
	}
	if err := matrix.Write(out, table); err != nil {
		fmt.Fprintf(os.Stderr, "write matrix: %v\n", err)
		os.Exit(1)
	}
}
