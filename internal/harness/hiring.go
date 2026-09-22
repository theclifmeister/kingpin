package harness

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The roster moves every crewed policy makes the same way (#275): who
// is the best of a role looking for work, who goes when the roster is
// full, who goes for skimming, where an enforcer stands. Each was a
// loop copied into staff, the road's crew step, Plant, Drive and
// hireChemist; the tie-breaks are the copies' own (the first of the
// best, the first of the worst, in the order the slices hold them),
// since which person a policy signs or fires is what the pinned numbers
// read.

// bestCandidate is the index of the most skilled candidate of role
// looking for work, the first of them on a tie, or -1 with none.
func bestCandidate(w *game.World, role string) int {
	best := -1
	for i, c := range w.Crew.Candidates {
		if c.Role == role && (best < 0 || c.Skill > w.Crew.Candidates[best].Skill) {
			best = i
		}
	}
	return best
}

// fireWorstRunner fires the least skilled runner on the payroll, the
// first of them on a tie, to make room on a full roster. It reports
// whether there was a runner to fire.
func fireWorstRunner(w *game.World) bool {
	worst := -1
	for i, m := range w.Crew.Members {
		if m.Role == game.RoleRunner && (worst < 0 || m.Skill < w.Crew.Members[worst].Skill) {
			worst = i
		}
	}
	if worst < 0 {
		return false
	}
	_, _ = w.Fire(w.Crew.Members[worst].ID)
	return true
}

// fireSkimmer fires the first member whose loyalty has sunk under the
// skim line: one a day, since each firing sours the rest.
func fireSkimmer(w *game.World, tun content.CrewTuning) {
	for _, m := range w.Crew.Members {
		if m.Loyalty < tun.SkimThreshold {
			_, _ = w.Fire(m.ID)
			return
		}
	}
}

// guardScore is where an idle enforcer stands: a corner the rival
// borders first, the busiest of those, then the riskiest.
func guardScore(w *game.World) func(game.Corner) float64 {
	return func(c game.Corner) float64 {
		if w.Contested(c) {
			return 10 + c.Demand
		}
		return c.Risk
	}
}
