package content

import (
	"strings"
	"testing"
)

// The file in the box is a ladder (#147): four tiers, named, the
// checkpoints rising, a trigger on every tier past the first and none on
// the first; and the decode refuses a ladder that is not.
func TestProgressionReadsAsALadder(t *testing.T) {
	cfg := MustLoad()
	p := cfg.Progression
	if len(p.Tiers) != 4 {
		t.Fatalf("%d tiers, want 4", len(p.Tiers))
	}
	want := []string{"Corner", "Crew", "Territory", "Distribution"}
	for i, tier := range p.Tiers {
		if tier.Name != want[i] {
			t.Errorf("tier %d is %q, want %q", i+1, tier.Name, want[i])
		}
		if len(tier.Opens) == 0 || tier.Next == "" {
			t.Errorf("tier %q opens nothing or says nothing of the next", tier.ID)
		}
	}
	if days := p.Checkpoints(); len(days) != 4 || days[0] != 30 || days[1] != 70 || days[2] != 120 || days[3] != 200 {
		t.Errorf("checkpoints %v, want 30, 70, 120, 200", days)
	}
	if p.Tier(0) != nil || p.Tier(5) != nil || p.Tier(1).ID != "corner" {
		t.Errorf("Tier(n) does not count from 1")
	}

	good := `
[[tier]]
id = "a"
name = "A"
blurb = "first"
checkpoint = 10
opens = ["x"]
next = "y"

[[tier]]
id = "b"
name = "B"
blurb = "second"
checkpoint = 20
[tier.enter]
crew_min = 1
`
	var ok ProgressionConfig
	if err := decodeBytes("progression.toml", []byte(good), &ok); err != nil {
		t.Fatal(err)
	}
	if err := ok.validate(); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct{ name, from, to, want string }{
		{"out of order", "checkpoint = 20", "checkpoint = 10", "past the tier before"},
		{"no trigger past the first", "[tier.enter]\ncrew_min = 1\n", "", "needs a [tier.enter]"},
		{"a trigger on the first", `next = "y"`, "next = \"y\"\n[tier.enter]\ncrew_min = 1", "day 0"},
		{"a duplicate id", `id = "b"`, `id = "a"`, "twice"},
		{"an unknown trigger field", "crew_min = 1", "crew_mn = 1", "unknown key"},
	} {
		var cfg ProgressionConfig
		err := decodeBytes("progression.toml", []byte(strings.Replace(good, bad.from, bad.to, 1)), &cfg)
		if err == nil {
			err = cfg.validate()
		}
		if err == nil || !strings.Contains(err.Error(), bad.want) {
			t.Errorf("%s: accepted, or refused for another reason: %v", bad.name, err)
		}
	}
}
