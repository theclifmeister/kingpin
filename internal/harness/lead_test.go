package harness

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// TestLeadDoesNotMoveTheRun (#354): the morning's lead is a report. The
// boss on two seeds, the table and the weather in, is played twice, on
// the file's [digest] and on one that leads with a single line and
// weighs every kind the other way round; with the report and the
// journal's digest lines set aside, the worlds are the same every day,
// so the lead rolls no dice and nothing reads it. (TestSeedDigest moved
// for the lead's words alone: with World.Report and World.Journal set
// aside it was the list before on every day.)
func TestLeadDoesNotMoveTheRun(t *testing.T) {
	t.Parallel()
	flipped := content.MustLoad()
	flipped.Headlines.Digest.Lines = 1
	for i, k := range content.DigestKinds {
		flipped.Headlines.Digest.Weights[k] = float64(i + 1)
	}
	for _, seed := range []uint64{7, 3} {
		a, b := leadRun(t, content.MustLoad(), seed), leadRun(t, flipped, seed)
		if len(a) != len(b) {
			t.Fatalf("seed %d: %d days against %d", seed, len(a), len(b))
		}
		for d := range a {
			if a[d] != b[d] {
				t.Fatalf("seed %d: the world moved with the lead on day %d", seed, d+1)
			}
		}
	}
}

// leadRun is the boss's first 120 days on seed, the world's digest each
// morning with the report and the lead's journal lines set aside, and
// fails when no morning had a lead.
func leadRun(t *testing.T, cfg *content.Config, seed uint64) []string {
	t.Helper()
	w := sim.NewWorld(cfg, seed)
	_, sims, err := sim.Default(cfg)
	if err != nil {
		t.Fatal(err)
	}
	clock := game.NewClock(nil, sims...)
	policy := Boss(cfg, 40, "")
	var out []string
	led := 0
	for day := 1; day <= 120 && w.Over == nil; day++ {
		policy(w)
		clock.EndDay(w)
		led += len(w.Report.Lead)
		rep, journal := w.Report, w.Journal
		var kept []game.Headline
		for _, h := range journal {
			if h.Source != "digest" {
				kept = append(kept, h)
			}
		}
		w.Report, w.Journal = nil, kept
		out = append(out, digest(w))
		w.Report, w.Journal = rep, journal
	}
	if led == 0 {
		t.Fatalf("seed %d: no morning had a lead", seed)
	}
	return out
}
