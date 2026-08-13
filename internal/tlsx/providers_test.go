package tlsx

import "testing"

func TestDNSProvidersHaveFields(t *testing.T) {
	seen := map[string]struct{}{}
	for _, p := range DNSProviders() {
		if p.ID == "" || p.Name == "" {
			t.Fatalf("empty provider %+v", p)
		}
		if _, ok := seen[p.ID]; ok {
			t.Fatalf("duplicate %s", p.ID)
		}
		seen[p.ID] = struct{}{}
		if len(p.Fields) == 0 {
			t.Fatalf("%s has no fields", p.ID)
		}
	}
	if _, ok := seen["cloudflare"]; !ok {
		t.Fatal("missing cloudflare")
	}
}

func TestParseExtraEnv(t *testing.T) {
	got := parseExtraEnv("FOO=bar\n# skip\nBAZ=qux")
	if got["FOO"] != "bar" || got["BAZ"] != "qux" {
		t.Fatalf("%v", got)
	}
}
