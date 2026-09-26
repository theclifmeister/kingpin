package game

import (
	"errors"

	"github.com/theclifmeister/kingpin/internal/content"
)

// Ambitions (#347, docs/ambitions.md): the endings surfaced early as
// plans, each a read of the world with its steps and how far along each
// one is. They are pure queries, as Eligible and the tiers are: nothing
// here writes the world, no sim reads them, nothing ends a run for them,
// and the score is the account as it always was. An ambition is done
// exactly when its ending's own condition holds (Retire's account and
// quiet days, the businessman's streak at legit_days, the reign
// stamped, an identity owned), so a full bar is never a promise the
// ending does not keep (TestAmbitionProgressAgreesWithTheEnding). The
// one thing kept on the world is the plan the player pinned (World.
// Ambition), which only the UI and the alerts read.

// ErrNoAmbition is what pinning an ambition the game does not have
// returns.
var ErrNoAmbition = errors.New("no such ambition")

// The units a step's Have and Need are counted in, for the words.
const (
	UnitCash    = "cash"    // dirty or clean money: the account
	UnitClean   = "clean"   // a node of the tree, its cost in clean cash against the clean pile; Have is the cost once owned
	UnitDirty   = "dirty"   // a node of the tree paid in dirty cash, against the dirty pile (#478)
	UnitDays    = "days"    // a streak
	UnitPoints  = "points"  // goodwill against pressure
	UnitIncome  = "income"  // a day's legit income against the street's night
	UnitCorners = "corners" // corners held against the share's line
	UnitCount   = "count"   // factions, cities
)

// AmbitionStep is one step of a plan: what is read (Have) against what
// the ending needs (Need), in Unit, and whether it is met.
type AmbitionStep struct {
	ID   string
	Have float64
	Need float64
	Unit string
	Done bool
}

// Frac is how far along the step is: 1 once done, else Have over Need
// short of 1, 0 with nothing to read.
func (s AmbitionStep) Frac() float64 {
	if s.Done {
		return 1
	}
	if s.Need <= 0 || s.Have <= 0 {
		return 0
	}
	return min(almost, s.Have/s.Need)
}

// almost is the most a plan not done reads: a full bar is the ending's
// condition and nothing short of it.
const almost = 0.99

// Ambition is one plan as the world stands: its id (content.AmbitionIDs),
// the ending it is the plan for ("" for the milestone), its steps in
// order, and whether it is done.
type Ambition struct {
	ID     string
	Ending string
	Steps  []AmbitionStep
	Done   bool
}

// Progress is the plan's bar: 1 exactly when it is done, else its
// least-done step short of 1. A plan is done when every step is, so
// the step furthest off is how far along it is; the mean read `Retire
// clean 94%` on the ambitions screen the morning the plan's line said
// `the account 88%` (#502), and `50%` with nothing offshore (#465).
func (a Ambition) Progress() float64 {
	if a.Done {
		return 1
	}
	if len(a.Steps) == 0 {
		return 0
	}
	least := 1.0
	for _, s := range a.Steps {
		least = min(least, s.Frac())
	}
	return min(almost, least)
}

// Next is the first step not met, nil once the plan is done.
func (a Ambition) Next() *AmbitionStep {
	if a.Done {
		return nil
	}
	for i := range a.Steps {
		if !a.Steps[i].Done {
			return &a.Steps[i]
		}
	}
	return nil
}

// Reached is how many steps are met in order, the first unmet one
// stopping the count: the plan's milestone, which the alerts key on, so a
// later step met out of turn is no milestone until the ones before it
// are.
func (a Ambition) Reached() int {
	for i, s := range a.Steps {
		if !s.Done {
			return i
		}
	}
	return len(a.Steps)
}

// AmbitionTerms is what the plans are read against: the owners'
// thresholds, which the caller passes from the sims' tuning as Retire
// takes its terms, and the two numbers only a sim can work out (the
// fronts' own income, laundering.Sim.LegitIncome, and what the street
// sold for last night, the tick's PlayerSold). A zero threshold is an
// ending boxed in the file, and its plan is left out.
type AmbitionTerms struct {
	RetireCash, RetireDays int     // laundering.toml [offshore]
	LegitDays              int     // laundering.toml [businessman]
	LegitIncome, Street    int     // the fronts' own income a day; the street's revenue last night
	DominantDays           int     // rivals.toml [endings]
	KingpinShare           float64 // ... more than this share of home's corners
	Tree                   content.UpgradesConfig
	TwoCities              content.TwoCitiesConfig
}

// Ambitions reads every plan the file does not box, in the panel's
// order. It writes nothing (TestAmbitionsNeverWriteTheWorld).
func Ambitions(w *World, t AmbitionTerms) []Ambition {
	var out []Ambition
	for _, id := range content.AmbitionIDs {
		if a, ok := ambition(w, id, t); ok {
			out = append(out, a)
		}
	}
	return out
}

// AmbitionOf is the one plan by id, false for one the game lacks or the
// file boxes.
func AmbitionOf(w *World, id string, t AmbitionTerms) (Ambition, bool) {
	return ambition(w, id, t)
}

func ambition(w *World, id string, t AmbitionTerms) (Ambition, bool) {
	a := Ambition{ID: id, Ending: content.AmbitionEnding[id]}
	switch id {
	case content.AmbitionRetire:
		if t.RetireCash <= 0 {
			return a, false
		}
		a.Steps = []AmbitionStep{
			{ID: "offshore", Have: float64(w.Offshore), Need: float64(t.RetireCash), Unit: UnitCash, Done: w.Offshore >= t.RetireCash},
			{ID: "quiet", Have: float64(w.QuietDays), Need: float64(t.RetireDays), Unit: UnitDays, Done: w.QuietDays >= t.RetireDays},
		}
		// Retire's own condition (World.CanRetire, the run aside).
		a.Done = w.Offshore >= t.RetireCash && w.QuietDays >= t.RetireDays
	case content.AmbitionLegit:
		if t.LegitDays <= 0 {
			return a, false
		}
		home := w.Home()
		var goodwill, pressure float64
		if home != nil {
			goodwill, pressure = home.Goodwill, home.Pressure
		}
		a.Steps = []AmbitionStep{
			{ID: "income", Have: float64(t.LegitIncome), Need: float64(t.Street), Unit: UnitIncome, Done: t.LegitIncome > 0 && t.LegitIncome > t.Street},
			{ID: "goodwill", Have: goodwill, Need: pressure, Unit: UnitPoints, Done: goodwill > pressure},
			{ID: "streak", Have: float64(w.LegitDays), Need: float64(t.LegitDays), Unit: UnitDays, Done: w.LegitDays >= t.LegitDays},
		}
		// Nothing sold (or no pressure) needs nothing over it: the step is
		// done on any income (goodwill), and its Need stays the zero it
		// is, so it reads `against $0 last night` on a lie-low night, not
		// the $1 it was bumped to (#502). Frac reads a done step as 1.
		// The laundering sim's own count at its own line.
		a.Done = w.LegitDays >= t.LegitDays
	case content.AmbitionCity:
		if t.DominantDays <= 0 {
			return a, false
		}
		a.Steps = cityTaken(w, t)
		// The rivals sim's stamp: the reign on (World.CanCrown, the run aside).
		a.Done = w.Reign > 0
		if a.Done {
			a.Steps[2].Done = true
		}
	case content.AmbitionVanish:
		fx := FoldEffects(w, t.Tree)
		// The tree's own chain (#478: the lawyer on call was missing,
		// and the plan read half done with the first node unbought).
		ids := content.AmbitionSteps[content.AmbitionVanish]
		lawyer, retainer, identity := nodeStep(w, t.Tree, ids[0]), nodeStep(w, t.Tree, ids[1]), nodeStep(w, t.Tree, ids[2])
		identity.Done = fx.Identities > 0 // the identity's own read (World.CanVanish)
		retainer.Done = retainer.Done || identity.Done
		lawyer.Done = lawyer.Done || retainer.Done
		a.Steps = []AmbitionStep{lawyer, retainer, identity}
		a.Done = identity.Done
	case content.AmbitionTwoCities:
		if t.TwoCities.Cities <= 0 {
			return a, false
		}
		a.Steps = twoCities(w, t.TwoCities)
		a.Done = a.Steps[2].Done
	default:
		return a, false
	}
	return a, true
}

// cityTaken is the kingpin's steps: more than the share of home's
// corners (the rivals sim's HoldsTheCity), every faction at the table
// gone or paying (World.Dominant), and the days since the city became
// yours toward dominant_days while both hold (DominantSince). The streak
// reads one short of the line until the rivals sim stamps the reign, so
// the plan is done on the stamp and nothing else.
func cityTaken(w *World, t AmbitionTerms) []AmbitionStep {
	home := w.Home()
	held, corners := 0, 0
	if home != nil {
		held, corners = w.HeldIn(home.ID), len(home.Corners)
	}
	line := t.KingpinShare * float64(corners)
	share := AmbitionStep{ID: "share", Have: float64(held), Need: float64(int(line) + 1), Unit: UnitCorners, Done: corners > 0 && float64(held) > line}
	down := 0
	for _, r := range w.Rivals {
		// A seat that stood down before it arrived counts as gone, as
		// Dominant reads it (#472: it read 3 of 4 for good).
		if r != nil && (r.Gone() || (r.Arrived > 0 && w.DealWith(r.Faction(), DealHomage) != nil)) {
			down++
		}
	}
	dominant := w.Dominant()
	factions := AmbitionStep{ID: "factions", Have: float64(down), Need: float64(len(w.Rivals)), Unit: UnitCount, Done: dominant}
	streak := AmbitionStep{ID: "streak", Need: float64(t.DominantDays), Unit: UnitDays}
	if dominant && share.Done {
		streak.Have = float64(min(w.Day-w.DominantSince(), t.DominantDays-1))
		if w.Reign > 0 {
			streak.Have = float64(max(w.Day-w.DominantSince(), t.DominantDays))
		}
	}
	return []AmbitionStep{share, factions, streak}
}

// nodeStep is a node of the tree as a step: owned, or its cost against
// the pile it is paid from, clean (UnitClean) or dirty (UnitDirty, the
// lawyer on call, #478).
func nodeStep(w *World, tree content.UpgradesConfig, id string) AmbitionStep {
	s := AmbitionStep{ID: id, Unit: UnitClean}
	pile := w.Player.CleanCash
	if u := tree.Upgrade(id); u != nil {
		s.Need = float64(u.Cost)
		if !u.Clean {
			s.Unit, pile = UnitDirty, w.Player.DirtyCash
		}
	}
	if w.Owns(id) {
		s.Have, s.Done = s.Need, true
		return s
	}
	s.Have = float64(min(pile, int(s.Need)))
	return s
}

// twoCities is the milestone's steps: the cities where you hold the
// share of the corners, those of them a lieutenant or a captain runs, and those of
// them whose corners at the share have been yours days days.
func twoCities(w *World, t content.TwoCitiesConfig) []AmbitionStep {
	ground, run, held := 0, 0, 0
	for _, cid := range w.CityOrder {
		c := w.Cities[cid]
		if c == nil || len(c.Corners) == 0 {
			continue
		}
		line := t.Share * float64(len(c.Corners))
		n, old := 0, 0
		for _, k := range c.Corners {
			if k.Held() {
				n++
				if w.Day-k.Since >= t.Days {
					old++
				}
			}
		}
		if float64(n) < line {
			continue
		}
		ground++
		if w.Crew.Lieutenant(cid) == nil && w.Crew.Captain(cid) == nil {
			continue // nobody runs it for you: a lieutenant or a captain (#346)
		}
		run++
		if float64(old) >= line {
			held++
		}
	}
	need := float64(t.Cities)
	return []AmbitionStep{
		{ID: "ground", Have: float64(ground), Need: need, Unit: UnitCount, Done: ground >= t.Cities},
		{ID: "lieutenants", Have: float64(run), Need: need, Unit: UnitCount, Done: run >= t.Cities},
		{ID: "held", Have: float64(held), Need: need, Unit: UnitCount, Done: held >= t.Cities},
	}
}

// DominantSince is the day the city became yours (#43, #227): the
// latest of the days each faction at the table fell (Absorbed,
// Fragmented) or bowed (its homage deal's Since); 0 with no table. With
// Dominant() true it is the first day of the reign's count; the rivals
// sim's detector and the city plan read it alike.
func (w *World) DominantSince() int {
	since := 0
	for _, r := range w.Rivals {
		if r == nil {
			continue
		}
		since = max(since, r.Absorbed, r.Fragmented)
		if d := w.DealWith(r.Faction(), DealHomage); d != nil {
			since = max(since, d.Since)
		}
	}
	return since
}

// PinAmbition makes the ambition the plan (#347): the dashboard shows
// it with its next step and its milestones are alerts. "" unpins. It is
// the player's and nobody else's: no sim reads it.
func (w *World) PinAmbition(id string) error {
	if id == "" {
		w.Ambition = ""
		return nil
	}
	for _, a := range content.AmbitionIDs {
		if a == id {
			w.Ambition = id
			return nil
		}
	}
	return ErrNoAmbition
}
