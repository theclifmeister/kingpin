package crew

import (
	"math"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Veterans (#346): at crew.toml [traits] days of service a member shows
// one trait, drawn off the traits stream (never the home city's) from
// the [trait.*] tables, weighted by their role and by what they lived
// through. A trait is a small fold of an existing crew effect word for
// that member alone (loyalty_loss_mul on their drift, skim_chance_mul
// on their roll to skim, deterrence against a skimmer; the heat sim
// reads sharp and heat off their corner, the market sim buyer_gap_mul),
// and every fold of no trait is the old number, so a run with the table
// boxed (days = 0, harness.NoTraits) is byte-for-byte the run before the
// feature: nothing draws and nothing is written.

// Trait is the tuning of a trait by name, the zero Trait for none: what
// the crew screen explains a trait with.
func (s *Sim) Trait(name string) content.Trait { return s.cfg.TraitOf(name) }

// TraitDays is how many days of service a trait takes to show; 0 is
// never.
func (s *Sim) TraitDays() int { return s.cfg.Traits.Days }

// traits has every member at trait_days of service who has shown
// nothing yet show a trait, in roster order, each draw off the traits
// stream.
func (s *Sim) traits(n *night) {
	tun := s.cfg.Traits
	if !tun.On() {
		return
	}
	t, c := n.t, n.c
	var rng game.Rand
	for i := range c.Members {
		m := &c.Members[i]
		if m.Trait != "" || t.Day-m.Hired < tun.Days {
			continue
		}
		if rng == nil {
			rng = t.Sub(game.StreamTraits)
		}
		name := s.drawTrait(rng, *m)
		if name == "" {
			continue // nothing in the file fits the role
		}
		m.Trait = name
		t.Emit(events.CrewTrait{Day: t.Day, ID: m.ID, Name: m.Name, Role: m.Role, Trait: name, Days: t.Day - m.Hired, Good: s.cfg.Trait[name].Good})
	}
}

// drawTrait is one draw from the traits the member's role and life
// weigh, "" with none that fits; the names walk in the file's fixed
// order, so the stream reads the same trait on every machine.
func (s *Sim) drawTrait(rng game.Rand, m game.CrewMember) string {
	names := s.cfg.TraitNames()
	total := 0.0
	for _, n := range names {
		total += s.cfg.Trait[n].WeightFor(m.Role, m.Lived)
	}
	if total <= 0 {
		return ""
	}
	r := rng.Float64() * total
	for _, n := range names {
		wt := s.cfg.Trait[n].WeightFor(m.Role, m.Lived)
		if wt <= 0 {
			continue
		}
		if r < wt {
			return n
		}
		r -= wt
	}
	for i := len(names) - 1; i >= 0; i-- { // the float's last crumb
		if s.cfg.Trait[names[i]].WeightFor(m.Role, m.Lived) > 0 {
			return names[i]
		}
	}
	return ""
}

// lived records what a member lived through (#346), each word once, for
// the trait's draw; only while traits are on, so a boxed run writes
// nothing.
func (s *Sim) lived(m *game.CrewMember, word string) {
	if !s.cfg.Traits.On() || m.HasLived(word) {
		return
	}
	m.Lived = append(m.Lived, word)
}

// deterrence is what the crew at work deter a skimmer with, in
// enforcers: the enforcers themselves plus each trait's deterrence
// (a hothead's) of the members at work.
func (s *Sim) deterrence(c *game.CrewState, enforcers int) float64 {
	d := float64(enforcers)
	for _, m := range c.Members {
		if m.Trait != "" && m.Working() {
			d += s.cfg.TraitOf(m.Trait).Deterrence
		}
	}
	return d
}

// skimOdds is a member's chance to skim tonight under the skim line:
// skim_chance, times the tree's skim_chance_mul, times what the
// deterrence leaves of it, times their trait's skim_chance_mul.
func (s *Sim) skimOdds(m game.CrewMember, fx game.Effects, deter float64) float64 {
	return s.cfg.Crew.SkimChance * fx.SkimChanceMul * deter * s.cfg.TraitOf(m.Trait).SkimMul()
}

// SkimOdds is m's chance to skim a night if their loyalty were under
// the skim line, at this morning's crew: what the pane shows, and the
// roll the skim makes.
func (s *Sim) SkimOdds(w *game.World, m game.CrewMember) float64 {
	deter := math.Pow(1-s.cfg.Role[game.RoleEnforcer].Deterrence, s.deterrence(&w.Crew, w.Crew.Role(game.RoleEnforcer)))
	return s.skimOdds(m, game.FoldEffects(w, s.tree), deter)
}
