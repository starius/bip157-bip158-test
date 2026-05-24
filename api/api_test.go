package api

import (
	"testing"

	"github.com/bip157-bip158-test/suite/environment"
)

func TestEnvironmentFromDefinition(t *testing.T) {
	def, err := environment.Lookup("tor-v3")
	if err != nil {
		t.Fatalf("lookup tor-v3: %v", err)
	}
	got := EnvironmentFromDefinition(def)
	if got.ID != "tor-v3" {
		t.Fatalf("environment id = %s", got.ID)
	}
	if got.AddressType != "tor-v3" || got.Transport != "tor-v3" {
		t.Fatalf("unexpected tor metadata: %+v", got)
	}
	if !got.RequiresProxy {
		t.Fatalf("tor-v3 should require a proxy")
	}
}

func TestDefaultCapabilitiesAreConservative(t *testing.T) {
	caps := DefaultCapabilities()
	if len(caps.Environments) != len(environment.All()) {
		t.Fatalf("capabilities = %d, environments = %d", len(caps.Environments), len(environment.All()))
	}
	for _, cap := range caps.Environments {
		if cap.ID == "ipv4" && !cap.Supported {
			t.Fatalf("legacy adapters should default to ipv4 support")
		}
		if cap.ID != "ipv4" && cap.Supported {
			t.Fatalf("legacy adapter unexpectedly supports %s", cap.ID)
		}
	}
}
