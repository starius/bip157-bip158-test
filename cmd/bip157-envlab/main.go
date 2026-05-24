// Command bip157-envlab prepares overlay-network lab manifests.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/bip157-bip158-test/suite/overlaylab"
)

func main() {
	var environmentID string
	var outDir string
	var peers int
	flag.StringVar(&environmentID, "environment", "tor-v3", "overlay environment: tor-v3, i2p, or cjdns")
	flag.StringVar(&outDir, "out", "run-artifacts/envlab", "directory for lab manifest and skeleton files")
	flag.IntVar(&peers, "peers", 2, "number of peerlab nodes to plan")
	flag.Parse()

	plan, err := overlaylab.Prepare(outDir, overlaylab.Spec{
		Environment: environmentID,
		PeerCount:   peers,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare lab: %v\n", err)
		os.Exit(1)
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode plan: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}
