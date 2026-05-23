package scenario

import "testing"

func TestCatalogIncludesImplementationBaselines(t *testing.T) {
	catalog := Catalog()
	bySource := map[string]int{}
	for _, def := range catalog {
		bySource[def.Source]++
	}
	if bySource[SourceKyotoBaseline] != 8 {
		t.Fatalf("expected 8 Kyoto baseline scenarios, got %d", bySource[SourceKyotoBaseline])
	}
	if bySource[SourceNeutrinoBaseline] != 53 {
		t.Fatalf("expected 53 Neutrino baseline scenarios, got %d", bySource[SourceNeutrinoBaseline])
	}
	if bySource[SourceConformance] == 0 {
		t.Fatalf("expected stronger conformance scenarios")
	}
}

func TestCatalogIDsAreUnique(t *testing.T) {
	seen := map[string]struct{}{}
	for _, def := range Catalog() {
		if def.ID == "" {
			t.Fatalf("scenario has empty ID: %#v", def)
		}
		if _, ok := seen[def.ID]; ok {
			t.Fatalf("duplicate scenario ID %q", def.ID)
		}
		seen[def.ID] = struct{}{}
	}
}
