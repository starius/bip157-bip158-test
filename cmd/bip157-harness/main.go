// Command bip157-harness runs the BIP157/BIP158 conformance suite.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/bip157-bip158-test/suite/harness"
)

func main() {
	var adapterURL string
	var dataDir string
	var outDir string
	var timeout time.Duration
	flag.StringVar(&adapterURL, "adapter-url", "", "base URL of the implementation adapter")
	flag.StringVar(&dataDir, "data-dir", "", "adapter data directory")
	flag.StringVar(&outDir, "out", "run-artifacts/latest", "directory for reports")
	flag.DurationVar(&timeout, "timeout", time.Minute, "per-scenario timeout")
	flag.Parse()

	summary, err := harness.Run(context.Background(), harness.Options{
		AdapterURL: adapterURL,
		DataDir:    dataDir,
		Timeout:    timeout,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "harness failed: %v\n", err)
		os.Exit(1)
	}
	if err := harness.WriteReports(outDir, summary); err != nil {
		fmt.Fprintf(os.Stderr, "write reports: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("overall=%s report=%s\n", summary.Color, outDir)
}
