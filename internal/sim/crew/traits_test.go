package crew_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/gametest"
)

// TestTraitShowsAtItsDays (#346): a member shows a trait the night they
// reach crew.toml [traits] days of service and not a night before, the
// report hears of it once, and the draw is the role's: an accountant
// only ever shows a trait whose role table names accountants (or that
// has none), over many seeds. What they lived through weighs the draw
// only once they have lived it, and it is written only while traits
// are on.
func TestTraitShowsAtItsDays(t *testing.T) {
	cfg := content.MustLoad()
	days := cfg.Crew.Traits.Days
	seen := map[string]int{}
	for seed := uint64(1); seed <= 40; seed++ {
		w, s := world(t, cfg, 100_000)
		w.Seed = seed
		w.Crew.Members = []game.CrewMember{{ID: 101, Name: "Books", Role: game.RoleAccountant, Skill: 50, Loyalty: 90, Nerve: 90, Wage: 50}}
		w.Day = days - 2
		gametest.Step(w, s)
		if m := w.Crew.Member(101); m.Trait != "" {
			t.Fatalf("seed %d: %s showed %s on day %d, before %d days of service", seed, m.Name, m.Trait, w.Day, days)
		}
		shown := 0
		for _, e := range gametest.Step(w, s).Events() {
			if ev, ok := e.(events.CrewTrait); ok {
				shown++
				if ev.Days != days || ev.Trait == "" {
					t.Fatalf("seed %d: %+v", seed, ev)
				}
			}
		}
		m := w.Crew.Member(101)
		tr, ok := cfg.Crew.Trait[m.Trait]
		if shown != 1 || !ok {
			t.Fatalf("seed %d: %d traits shown at %d days, the member has %q", seed, shown, days, m.Trait)
		}
		if len(tr.Role) > 0 && tr.Role[game.RoleAccountant] == 0 {
			t.Fatalf("seed %d: an accountant showed %s, a trait of %v", seed, m.Trait, tr.Role)
		}
		seen[m.Trait]++
		for _, e := range gametest.Step(w, s).Events() {
			if _, ok := e.(events.CrewTrait); ok {
				t.Fatalf("seed %d: a second trait for %s", seed, m.Name)
			}
		}
	}
	if len(seen) < 2 {
		t.Fatalf("forty accountants showed only %v", seen)
	}
	t.Logf("forty accountants at %d days: %v", days, seen)
}
