package news_test

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/harness"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// The rich list (#392): a world over every line enters them one a
// morning, lowest first, each once; the rank is read off the figure;
// the headline is one of the RichListed templates and the TIER section
// says it; a run under the first line hears nothing.
func TestRichListOneAMorning(t *testing.T) {
	cfg := content.MustLoad()
	rl := cfg.Headlines.RichList
	if rl.Rank(100_000_000) != 1000 || rl.Rank(1_000_000_000) != 100 || rl.Rank(1_000_000_000_000) != 1 {
		t.Fatalf("ranks %d, %d, %d", rl.Rank(100_000_000), rl.Rank(1_000_000_000), rl.Rank(1_000_000_000_000))
	}
	w := sim.NewWorld(cfg, 7)
	w.Player.CleanCash = 2 * rl.Lines[len(rl.Lines)-1]
	res, err := harness.RunFrom(cfg, w, len(rl.Lines)+3, harness.Idle)
	if err != nil {
		t.Fatal(err)
	}
	var days, lines []int
	for _, e := range res.Events {
		if ev, ok := e.(events.RichListed); ok {
			days, lines = append(days, ev.Day), append(lines, ev.Line)
			if ev.Rank != rl.Rank(ev.NetWorth) {
				t.Fatalf("rank %d for %d", ev.Rank, ev.NetWorth)
			}
		}
	}
	if len(lines) != len(rl.Lines) {
		t.Fatalf("crossed %v on %v; want every line once", lines, days)
	}
	for i := range lines {
		if lines[i] != rl.Lines[i] || i > 0 && days[i] != days[i-1]+1 {
			t.Fatalf("crossed %v on %v; want the lines in order, one a morning", lines, days)
		}
	}
	found := false
	for _, l := range res.World.Journal {
		if l.Day == days[0] && strings.Contains(l.Text, "rich list") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no rich-list headline on day %d", days[0])
	}

	poor, _ := harness.Run(cfg, 7, 30, harness.Idle)
	for _, e := range poor.Events {
		if _, ok := e.(events.RichListed); ok {
			t.Fatal("an idle run made the rich list")
		}
	}
}
