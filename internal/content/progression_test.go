package content

import (
	"strings"
	"testing"
)

// The file in the box is a ladder (#147): five tiers (#48's Cartel the
// last), named, the
// checkpoints rising, a trigger on every tier past the first and none on
// the first; and the decode refuses a ladder that is not.
func TestProgressionReadsAsALadder(t *testing.T) {
	cfg := MustLoad()
	p := cfg.Progression
	if len(p.Tiers) != 5 {
		t.Fatalf("%d tiers, want 5", len(p.Tiers))
	}
	want := []string{"Corner", "Crew", "Territory", "Distribution", "Cartel"}
	for i, tier := range p.Tiers {
		if tier.Name != want[i] {
			t.Errorf("tier %d is %q, want %q", i+1, tier.Name, want[i])
		}
		if len(tier.Opens) == 0 || tier.Next == "" {
			t.Errorf("tier %q opens nothing or says nothing of the next", tier.ID)
		}
		// The stage's prose (#149): every tier past the first has it,
		// and the closing line is the last tier's alone.
		if i > 0 && (len(tier.Text) < StageTextMin || len(tier.Text) > StageTextMax) {
			t.Errorf("tier %q has %d text lines", tier.ID, len(tier.Text))
		}
		if (tier.Closing != "") != (i == len(p.Tiers)-1) {
			t.Errorf("tier %q: closing %q", tier.ID, tier.Closing)
		}
	}
	if days := p.Checkpoints(); len(days) != 5 || days[0] != 30 || days[1] != 70 || days[2] != 120 || days[3] != 200 || days[4] != 300 {
		t.Errorf("checkpoints %v, want 30, 70, 120, 200, 300", days)
	}
	if p.Tier(0) != nil || p.Tier(6) != nil || p.Tier(1).ID != "corner" || p.Tier(5).ID != "cartel" {
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
text = ["one", "two", "three"]
closing = "the end"
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
		{"too little text", `text = ["one", "two", "three"]`, `text = ["one"]`, "text lines"},
		{"no closing on the last", `closing = "the end"`, "", "closing line"},
		{"a closing on the first", `blurb = "first"`, "blurb = \"first\"\nclosing = \"early\"", "closing line"},
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
