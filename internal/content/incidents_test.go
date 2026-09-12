package content

import (
	"strings"
	"testing"
)

// The incident table (#44): the shipped file validates, an effect key
// the world does not apply fails at start-up (rival_leader_killed waits
// for #43 and is one such), and the validation names what is wrong with
// a row.
func TestIncidentsRefuseUnknownKey(t *testing.T) {
	cfg := MustLoad()
	if len(cfg.Incidents.Table) < 11 {
		t.Fatalf("%d incidents in the table", len(cfg.Incidents.Table))
	}
	if err := cfg.Incidents.validate(cfg.City, cfg.Market, cfg.Routes, cfg.Names); err != nil {
		t.Fatal(err)
	}
	const head = `
[incidents]
min_gap = 8
max_gap = 26
`
	good := head + `
[[incident]]
id = "port_strike"
name = "Port strike"
route_mode = "boat"
report = "Shut for {{.Days}} days."
[incident.trigger]
city = "bayport"
[incident.effects]
route_closed = 6
market_shock = { product = "coke", mul = 1.6, days = 10 }
`
	var ic IncidentsConfig
	if err := decodeBytes("incidents.toml", []byte(good), &ic); err != nil {
		t.Fatal(err)
	}
	if err := ic.validate(cfg.City, cfg.Market, cfg.Routes, cfg.Names); err != nil {
		t.Fatal(err)
	}
	if ic.Table[0].Key() != "IncidentPortStrike" {
		t.Fatalf("key %q", ic.Table[0].Key())
	}
	for _, bad := range []struct{ name, src, want string }{
		{"a key not in the set", head + `
[[incident]]
id = "x"
name = "X"
report = "."
[incident.effects]
rival_leader_killed = true
`, "unknown key"},
		{"a typo", head + `
[[incident]]
id = "x"
name = "X"
report = "."
[incident.effects]
presure = 5
`, "unknown key"},
		{"no effects", head + `
[[incident]]
id = "x"
name = "X"
report = "."
`, "no effects"},
		{"a closure with no mode", head + `
[[incident]]
id = "x"
name = "X"
report = "."
[incident.effects]
route_closed = 3
`, "needs a route_mode"},
		{"an unknown city", head + `
[[incident]]
id = "x"
name = "X"
report = "."
[incident.trigger]
city = "atlantis"
[incident.effects]
pressure = 5
`, "unknown city"},
		{"an unknown product", head + `
[[incident]]
id = "x"
name = "X"
report = "."
[incident.effects]
market_shock = { product = "tea", mul = 2, days = 3 }
`, "unknown product"},
		{"an unknown name pool", head + `
[[incident]]
id = "x"
name = "X"
names = "senators"
report = "."
[incident.effects]
pressure = 5
`, "no name pool"},
		{"a bad pace", `
[incidents]
min_gap = 0
max_gap = 3
[[incident]]
id = "x"
name = "X"
report = "."
[incident.effects]
pressure = 5
`, "min_gap"},
	} {
		var ic IncidentsConfig
		err := decodeBytes("incidents.toml", []byte(bad.src), &ic)
		if err == nil {
			err = ic.validate(cfg.City, cfg.Market, cfg.Routes, cfg.Names)
		}
		if err == nil || !strings.Contains(err.Error(), bad.want) {
			t.Errorf("%s: got %v, want %q", bad.name, err, bad.want)
		}
	}
}
