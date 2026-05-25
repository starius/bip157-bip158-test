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
	var addressLab string
	var dataDir string
	var environmentID string
	var outDir string
	var proxyAddress string
	var torLab string
	var chutneyPath string
	var chutneyNet string
	var timeout time.Duration
	flag.StringVar(&adapterURL, "adapter-url", "", "base URL of the implementation adapter")
	flag.StringVar(&addressLab, "address-lab", "loopback", "address allocator: loopback, auto, or linux-iproute")
	flag.StringVar(&dataDir, "data-dir", "", "adapter data directory")
	flag.StringVar(&environmentID, "environment", "ipv4", "test environment: ipv4, ipv6, tor-v3, i2p, or cjdns")
	flag.StringVar(&outDir, "out", "run-artifacts/latest", "directory for reports")
	flag.StringVar(&proxyAddress, "proxy-address", "", "SOCKS, SAM, or overlay proxy address for the selected environment")
	flag.StringVar(&torLab, "tor-lab", "off", "Tor lab mode: off or chutney")
	flag.StringVar(&chutneyPath, "chutney-path", "", "path to the Chutney source tree; defaults to CHUTNEY_SOURCE")
	flag.StringVar(&chutneyNet, "chutney-network", "", "Chutney network name for tor-v3; defaults to hs-v3-min")
	flag.DurationVar(&timeout, "timeout", time.Minute, "per-scenario timeout")
	flag.Parse()

	summary, err := harness.Run(context.Background(), harness.Options{
		AdapterURL:   adapterURL,
		AddressLab:   addressLab,
		DataDir:      dataDir,
		Environment:  environmentID,
		ProxyAddress: proxyAddress,
		TorLab:       torLab,
		ChutneyPath:  chutneyPath,
		ChutneyNet:   chutneyNet,
		Timeout:      timeout,
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
