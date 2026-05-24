// Package overlaylab prepares local overlay-network lab manifests.
//
// The conformance harness intentionally keeps overlay setup separate from the
// BIP157/BIP158 scenarios. Tor, I2P, and cjdns have different bootstrap and
// privilege requirements, so this package writes deterministic lab plans that
// can be executed by an operator or a future privileged runner without changing
// the scenario code.
package overlaylab

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bip157-bip158-test/suite/environment"
)

// Spec describes the overlay lab the caller wants to prepare.
type Spec struct {
	Environment string
	PeerCount   int
}

// Plan is the machine-readable description written by Prepare.
type Plan struct {
	Environment  string            `json:"environment"`
	AddressType  string            `json:"address_type"`
	Transport    string            `json:"transport"`
	PeerCount    int               `json:"peer_count"`
	Packages     []string          `json:"packages"`
	Commands     []Command         `json:"commands"`
	Files        map[string]string `json:"files"`
	Active       bool              `json:"active"`
	InactiveNote string            `json:"inactive_note"`
}

// Command is one shell command needed to activate a lab.
type Command struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Command     string `json:"command"`
}

// Build returns an overlay lab plan without writing it to disk.
func Build(spec Spec) (Plan, error) {
	if spec.PeerCount <= 0 {
		return Plan{}, fmt.Errorf("peer count must be positive")
	}
	env, err := environment.Lookup(spec.Environment)
	if err != nil {
		return Plan{}, err
	}
	if !env.Overlay {
		return Plan{}, fmt.Errorf("%s is not an overlay environment", env.ID)
	}

	plan := Plan{
		Environment:  string(env.ID),
		AddressType:  string(env.AddressType),
		Transport:    string(env.Transport),
		PeerCount:    spec.PeerCount,
		Files:        map[string]string{},
		InactiveNote: "lab preparation is available; runtime wiring is not active in the harness yet",
	}

	switch env.ID {
	case environment.TorV3:
		return torPlan(plan), nil
	case environment.I2P:
		return i2pPlan(plan), nil
	case environment.CJDNS:
		return cjdnsPlan(plan), nil
	default:
		return Plan{}, fmt.Errorf("no overlay lab plan for %s", env.ID)
	}
}

// Prepare writes a plan and deterministic config skeletons under dir.
func Prepare(dir string, spec Spec) (Plan, error) {
	plan, err := Build(spec)
	if err != nil {
		return Plan{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Plan{}, fmt.Errorf("create lab dir: %w", err)
	}
	for name, content := range plan.Files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return Plan{}, fmt.Errorf("create %s parent: %w", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return Plan{}, fmt.Errorf("write %s: %w", name, err)
		}
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return Plan{}, fmt.Errorf("encode manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644); err != nil {
		return Plan{}, fmt.Errorf("write manifest: %w", err)
	}
	return plan, nil
}

func torPlan(plan Plan) Plan {
	plan.Packages = []string{"tor", "python3", "chutney source"}
	plan.Commands = []Command{
		{
			Name:        "configure-chutney",
			Description: "configure a private Chutney network",
			Command:     `cd "$CHUTNEY_SOURCE" && CHUTNEY_DATA_DIR="$LAB_DIR/chutney" ./chutney configure networks/basic-min`,
		},
		{
			Name:        "start-chutney",
			Description: "start the private Tor network",
			Command:     `cd "$CHUTNEY_SOURCE" && CHUTNEY_DATA_DIR="$LAB_DIR/chutney" ./chutney start networks/basic-min`,
		},
	}
	plan.Files["tor/README.md"] = `# Tor v3 Lab

This directory is reserved for a private Chutney network and onion-service
metadata. The harness should expose each peerlab listener as a v3 onion service
before running BIP157/BIP158 scenarios with environment tor-v3.
`
	plan.Files["tor/onion-services.json"] = peerListJSON(plan.PeerCount, "onion-peer")
	return plan
}

func i2pPlan(plan Plan) Plan {
	plan.Packages = []string{"i2p", "i2pd", "openjdk_headless"}
	plan.Commands = []Command{{
		Name:        "start-i2pd-router",
		Description: "start isolated i2pd routers with SAM enabled",
		Command:     `i2pd --conf "$LAB_DIR/i2p/node-1/i2pd.conf" --datadir "$LAB_DIR/i2p/node-1"`,
	}}
	for i := 1; i <= plan.PeerCount; i++ {
		plan.Files[fmt.Sprintf("i2p/node-%d/i2pd.conf", i)] = fmt.Sprintf(`[sam]
enabled = true
address = 127.0.0.1
port = %d

[httpproxy]
enabled = false
`, 7656+i)
		plan.Files[fmt.Sprintf("i2p/node-%d/tunnels.conf", i)] = `[peerlab]
type = server
host = 127.0.0.1
port = 0
keys = peerlab.dat
`
	}
	return plan
}

func cjdnsPlan(plan Plan) Plan {
	plan.Packages = []string{"cjdns", "iproute2"}
	plan.Commands = []Command{
		{
			Name:        "generate-configs",
			Description: "generate per-node cjdroute configs",
			Command:     `for n in $(seq 1 "$PEER_COUNT"); do cjdroute --genconf > "$LAB_DIR/cjdns/node-$n/cjdroute.conf"; done`,
		},
		{
			Name:        "start-cjdroute",
			Description: "start cjdroute nodes in isolated namespaces or containers",
			Command:     `cjdroute < "$LAB_DIR/cjdns/node-1/cjdroute.conf"`,
		},
	}
	for i := 1; i <= plan.PeerCount; i++ {
		plan.Files[fmt.Sprintf("cjdns/node-%d/README.md", i)] = `# cjdns Node

Generate cjdroute.conf here, then wire connectTo entries between all peerlab
nodes. The harness should use the generated fc00::/8 cjdns IPv6 addresses as
adapter peer identities.
`
	}
	return plan
}

func peerListJSON(count int, prefix string) string {
	type peer struct {
		ID string `json:"id"`
	}
	peers := make([]peer, count)
	for i := range peers {
		peers[i] = peer{ID: fmt.Sprintf("%s-%d", prefix, i+1)}
	}
	data, _ := json.MarshalIndent(peers, "", "  ")
	return string(data) + "\n"
}
