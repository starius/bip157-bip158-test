package overlaylab

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildRejectsNonOverlayEnvironment(t *testing.T) {
	if _, err := Build(Spec{Environment: "ipv4", PeerCount: 1}); err == nil {
		t.Fatalf("ipv4 unexpectedly produced an overlay plan")
	}
}

func TestBuildTorPlan(t *testing.T) {
	plan, err := Build(Spec{Environment: "tor-v3", PeerCount: 2})
	if err != nil {
		t.Fatalf("build tor plan: %v", err)
	}
	if plan.Environment != "tor-v3" || plan.Transport != "tor-v3" {
		t.Fatalf("unexpected tor plan: %+v", plan)
	}
	if len(plan.Commands) != 2 {
		t.Fatalf("tor commands = %d, want 2", len(plan.Commands))
	}
	if !strings.Contains(plan.Files["tor/onion-services.json"], "onion-peer-2") {
		t.Fatalf("tor peer list missing second peer: %s", plan.Files["tor/onion-services.json"])
	}
}

func TestPrepareWritesManifestAndSkeletons(t *testing.T) {
	dir := t.TempDir()
	plan, err := Prepare(dir, Spec{Environment: "i2p", PeerCount: 2})
	if err != nil {
		t.Fatalf("prepare i2p plan: %v", err)
	}
	if plan.Environment != "i2p" {
		t.Fatalf("environment = %s", plan.Environment)
	}
	for _, rel := range []string{
		"manifest.json",
		"i2p/node-1/i2pd.conf",
		"i2p/node-2/tunnels.conf",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
}

func TestBuildCJDNSPlan(t *testing.T) {
	plan, err := Build(Spec{Environment: "cjdns", PeerCount: 3})
	if err != nil {
		t.Fatalf("build cjdns plan: %v", err)
	}
	if plan.AddressType != "cjdns" {
		t.Fatalf("address type = %s", plan.AddressType)
	}
	if len(plan.Files) != 3 {
		t.Fatalf("cjdns files = %d, want 3", len(plan.Files))
	}
}
