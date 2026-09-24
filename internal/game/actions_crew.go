package game

import (
	"errors"
	"math"

	"github.com/theclifmeister/kingpin/internal/events"
)

// Hire signs the candidate with id, paying the fee in dirty cash. maxCrew
// caps the roster. The crew sim reports the signing at end of day.
func (w *World) Hire(id, maxCrew int) (CrewMember, error) {
	if w.Over != nil {
		return CrewMember{}, ErrGameOver
	}
	idx := -1
	for i, c := range w.Crew.Candidates {
		if c.ID == id {
			idx = i
		}
	}
	if idx < 0 {
		return CrewMember{}, ErrNoCandidate
	}
	if len(w.Crew.Members) >= maxCrew {
		return CrewMember{}, ErrCrewFull
	}
	c := w.Crew.Candidates[idx]
	if err := w.payDirty(c.Fee); err != nil {
		return CrewMember{}, err
	}
	c.Hired = w.Day
	w.Crew.Candidates = append(w.Crew.Candidates[:idx], w.Crew.Candidates[idx+1:]...)
	w.Crew.Members = append(w.Crew.Members, c)
	w.Crew.HiredToday = append(w.Crew.HiredToday, c)
	return c, nil
}

// Fire removes the member with id and pulls them off their corner, and
// off their route (#46). The rest of the crew take it badly when the
// crew sim steps.
func (w *World) Fire(id int) (CrewMember, error) {
	if w.Over != nil {
		return CrewMember{}, ErrGameOver
	}
	for i, m := range w.Crew.Members {
		if m.ID == id {
			w.Recall(id)
			w.unseat(id)
			if m.Runs() {
				w.DropStanding(m.City)
			}
			w.Crew.Members = append(w.Crew.Members[:i], w.Crew.Members[i+1:]...)
			w.Crew.FiredToday = append(w.Crew.FiredToday, m)
			return m, nil
		}
	}
	return CrewMember{}, ErrNoMember
}

// SetPay sets the pay dial for the whole crew. It persists until changed.
// A run that is over, or a dial off the three positions, is refused, as
// SetRoute refuses them (#281).
func (w *World) SetPay(p events.Pay) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if p < events.PayStingy || p > events.PayGenerous {
		return ErrBadDial
	}
	w.Crew.Pay = p
	return nil
}

// Bail puts cost in clean cash down for the jailed member with id
// (#46): they walk tomorrow, with their loyalty up when the crew sim
// releases them. Clean cash only, the whole of it: a bail bondsman
// takes a cheque. The crew sim reports it tonight (Crew.BailedToday).
func (w *World) Bail(id, cost int) (CrewMember, error) {
	if w.Over != nil {
		return CrewMember{}, ErrGameOver
	}
	m := w.Crew.Member(id)
	if m == nil {
		return CrewMember{}, ErrNoMember
	}
	if !m.Jailed(w.Day) {
		return CrewMember{}, ErrNotJailed
	}
	if m.Bailed {
		return CrewMember{}, ErrBailed
	}
	if err := w.payClean(cost); err != nil {
		return CrewMember{}, err
	}
	w.Stats.Bails++
	w.Stats.BailCash += cost
	m.Bailed = true
	m.JailedUntil = w.Day + 1
	w.Crew.BailedToday = append(w.Crew.BailedToday, Payoff{ID: m.ID, Name: m.Name, Cost: cost, Clean: cost})
	return *m, nil
}

// Investigate pays cost to have the crew looked into tonight: the crew sim
// rolls whether it names the informant, if there is one, and everyone's
// loyalty suffers when it names nobody. One a day.
func (w *World) Investigate(cost int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	if len(w.Crew.Members) == 0 {
		return ErrNoCrew
	}
	if w.Today.Investigation != nil {
		return ErrInvestigating
	}
	clean, ok := w.spend(cost)
	if !ok {
		return &ShortError{Need: cost, Have: w.Cash()}
	}
	w.Today.Investigation = &InvestigationOrder{Cost: cost, Clean: clean}
	return nil
}

// PayOff hands the member with id cost in cash for loyalty points, at
// once. It buys loyalty, not silence: an informant stays one.
func (w *World) PayOff(id, cost int, loyalty float64) (CrewMember, error) {
	if w.Over != nil {
		return CrewMember{}, ErrGameOver
	}
	m := w.Crew.Member(id)
	if m == nil {
		return CrewMember{}, ErrNoMember
	}
	clean, ok := w.spend(cost)
	if !ok {
		return CrewMember{}, &ShortError{Need: cost, Have: w.Cash()}
	}
	m.Loyalty = math.Min(100, m.Loyalty+loyalty)
	w.Crew.PaidOffToday = append(w.Crew.PaidOffToday, Payoff{ID: m.ID, Name: m.Name, Cost: cost, Clean: clean})
	return *m, nil
}

var (
	ErrNotLieutenant = errors.New("only a lieutenant can run a city")
	ErrCityRun       = errors.New("somebody already runs that city")
)

// Assign gives a lieutenant on the payroll a city to run. From the next
// crew step they post the idle crew on its corners, sell its stash at
// their dial and keep their cut. One lieutenant per city; assigning to
// another city moves them.
func (w *World) Assign(id int, city string) error {
	if w.Over != nil {
		return ErrGameOver
	}
	m := w.Crew.Member(id)
	if m == nil {
		return ErrNoMember
	}
	if !m.Lieutenant() {
		return ErrNotLieutenant
	}
	if w.Cities[city] == nil {
		return ErrNoCity
	}
	if lt := w.Crew.Lieutenant(city); lt != nil && lt.ID != id {
		return ErrCityRun
	}
	if m.City == city {
		return nil
	}
	w.DropStanding(m.City)
	m.City = city
	m.Assigned = w.Day
	return nil
}

var (
	ErrCaptainRuns   = errors.New("a lieutenant runs a city: they are nobody's captain")
	ErrNotTrusted    = errors.New("not trusted enough to be captain")
	ErrCaptained     = errors.New("somebody is already captain there")
	ErrBadBudget     = errors.New("a budget is zero or more")
	ErrNotCaptain    = errors.New("they are nobody's captain")
	ErrCaptainAbsent = errors.New("they are in no state to take it on")
)

// NameCaptain makes the member with id captain of a city (#346): from
// tonight they post its idle runners, pull a suspected skimmer off a
// corner there and pay off a member near the quit line out of budget
// a night. One a city; naming them for another city moves them. The
// caller passes what trust takes, the loyalty and the days on the
// payroll (the crew's tuning, as Hire takes the cap). A lieutenant
// runs a city and is never one.
func (w *World) NameCaptain(id int, city string, budget int, loyalty float64, days int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	m := w.Crew.Member(id)
	if m == nil {
		return ErrNoMember
	}
	if m.Lieutenant() {
		return ErrCaptainRuns
	}
	if w.Cities[city] == nil {
		return ErrNoCity
	}
	if budget < 0 {
		return ErrBadBudget
	}
	if !m.Working() {
		return ErrCaptainAbsent
	}
	if m.Captain != city && (m.Loyalty < loyalty || w.Day-m.Hired < days) {
		return ErrNotTrusted
	}
	if c := w.Crew.Captain(city); c != nil && c.ID != id {
		return ErrCaptained
	}
	m.Captain, m.Budget = city, budget
	return nil
}

// DropCaptain takes the captaincy off the member with id (#346): they
// go back to being one of the crew, and the city's care stops tonight.
func (w *World) DropCaptain(id int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	m := w.Crew.Member(id)
	if m == nil {
		return ErrNoMember
	}
	if m.Captain == "" {
		return ErrNotCaptain
	}
	m.Captain, m.Budget = "", 0
	return nil
}

// Unassign takes a lieutenant off their city. The crew they posted stay
// where they are; the standing orders go.
func (w *World) Unassign(id int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	m := w.Crew.Member(id)
	if m == nil {
		return ErrNoMember
	}
	if !m.Lieutenant() {
		return ErrNotLieutenant
	}
	w.DropStanding(m.City)
	m.City = ""
	return nil
}
