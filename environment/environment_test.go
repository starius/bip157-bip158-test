package environment

import "testing"

func TestLookupDefaultAndKnownEnvironments(t *testing.T) {
	def, err := Lookup("")
	if err != nil {
		t.Fatalf("default lookup: %v", err)
	}
	if def.ID != IPv4 {
		t.Fatalf("default environment = %s, want %s", def.ID, IPv4)
	}

	want := map[ID]bool{
		IPv4:  false,
		IPv6:  false,
		TorV3: true,
		I2P:   true,
		CJDNS: true,
	}
	for id, overlay := range want {
		def, err := Lookup(string(id))
		if err != nil {
			t.Fatalf("lookup %s: %v", id, err)
		}
		if def.Overlay != overlay {
			t.Fatalf("%s overlay = %v, want %v", id, def.Overlay, overlay)
		}
	}
}

func TestLookupRejectsUnknownEnvironment(t *testing.T) {
	if _, err := Lookup("bogus"); err == nil {
		t.Fatalf("unknown environment unexpectedly succeeded")
	}
}

func TestIDsCoversAllDefinitions(t *testing.T) {
	ids := IDs()
	defs := All()
	if len(ids) != len(defs) {
		t.Fatalf("ids = %d, definitions = %d", len(ids), len(defs))
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate environment id %s", id)
		}
		seen[id] = true
	}
}
