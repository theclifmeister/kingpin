package content

import (
	"strings"
	"testing"
)

// Every file is strict: a misspelled key in any of them is refused on
// load rather than read as the zero it silently was, so a tuning edit
// that names no field cannot pass for a balance change.
func TestEveryFileRefusesAnUnknownKey(t *testing.T) {
	c := &Config{}
	targets := map[string]any{
		"market.toml": &c.Market, "city.toml": &c.City, "routes.toml": &c.Routes,
		"heat.toml": &c.Heat, "crew.toml": &c.Crew, "rivals.toml": &c.Rivals,
		"laundering.toml": &c.Laundering, "names.toml": &c.Names, "upgrades.toml": &c.Upgrades,
		"reputation.toml": &c.Reputation, "law.toml": &c.Law, "headlines.toml": &c.Headlines,
		"dilemmas.toml": &c.Dilemmas, "buyers.toml": &c.Buyers, "suppliers.toml": &c.Suppliers,
		"progression.toml": &c.Progression, "houses.toml": &c.Houses, "incidents.toml": &c.Incidents,
		"intel.toml": &c.Intel, "assets.toml": &c.Assets, "endings.toml": &c.Endings,
		"characters.toml": &c.Characters, "presets.toml": &c.Presets,
	}
	entries, err := files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		dst, ok := targets[e.Name()]
		if !ok {
			t.Errorf("%s is embedded but not in this test", e.Name())
			continue
		}
		b, err := files.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if err := decodeBytes(e.Name(), b, dst); err != nil {
			t.Fatalf("%s as shipped: %v", e.Name(), err)
		}
		bad := append([]byte("zz_misspelled = 1\n"), b...)
		if err := decodeBytes(e.Name(), bad, dst); err == nil || !strings.Contains(err.Error(), "zz_misspelled") {
			t.Errorf("%s took a misspelled key: %v", e.Name(), err)
		}
	}
}
