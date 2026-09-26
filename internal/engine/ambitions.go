package engine

import (
	"fmt"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The ambitions (#347, docs/ambitions.md): the endings as plans with
// their progress, read off the world by game.Ambitions against the
// owners' thresholds. The session supplies the two numbers only a sim
// can work out, the fronts' own income and the street they are read
// against (the more of last night's and its average night over the
// run, #493), so a front end reads the plans and never works them out.

// AmbitionTerms is what the plans are read against this morning.
func (s *Session) AmbitionTerms() game.AmbitionTerms {
	off := s.set.Laundering.Offshore()
	end := s.cfg.Rivals.Endings
	return game.AmbitionTerms{
		RetireCash:   off.RetireCash,
		RetireDays:   off.RetireDays,
		LegitDays:    s.cfg.Laundering.Businessman.LegitDays,
		LegitIncome:  s.set.Laundering.LegitIncome(s.w),
		Street:       max(s.lastStreet(), s.w.StreetAverage(s.w.Day)),
		DominantDays: end.DominantDays,
		KingpinShare: end.KingpinShare,
		Tree:         s.cfg.Upgrades,
		TwoCities:    s.cfg.Ambitions.TwoCities,
	}
}

// Ambitions is every plan the file does not box, in the panel's order;
// nil before a run.
func (s *Session) Ambitions() []game.Ambition {
	if s.w == nil {
		return nil
	}
	return game.Ambitions(s.w, s.AmbitionTerms())
}

// Plan is the pinned plan as it stands, false with none pinned (or one
// the file boxes).
func (s *Session) Plan() (game.Ambition, bool) {
	if s.w == nil || s.w.Ambition == "" {
		return game.Ambition{}, false
	}
	return game.AmbitionOf(s.w, s.w.Ambition, s.AmbitionTerms())
}

// PinAmbition makes the ambition the plan, "" for none
// (World.PinAmbition).
func (s *Session) PinAmbition(id string) error { return s.w.PinAmbition(id) }

// lastStreet is what the street sold for last night: the tick's
// PlayerSold revenue when this session ended that night, else (a run
// loaded or attached since) the report's sales line, the street and the
// buyers net of the crew's cut, the nearest number the world keeps.
func (s *Session) lastStreet() int {
	if s.streetDay == s.w.Day && s.streetDay > 0 {
		return s.street
	}
	if r := s.w.Report; r != nil {
		return max(0, r.Flow.Line(game.FlowSales).Dirty)
	}
	return 0
}

// planAlert is the pinned plan's milestone (#347): once its first step
// is met, keyed by the plan and how many of its steps are met in order
// (Ambition.Reached), so a fast-forward stops once as each is met and
// once more when the plan is done (and again only if a step is lost and
// met again). Nothing with no plan pinned: the alerts before it.
func (s *Session) planAlert() []Alert {
	a, ok := s.Plan()
	if !ok {
		return nil
	}
	done := a.Reached()
	if done == 0 {
		return nil
	}
	return []Alert{{Kind: AlertPlan, Key: fmt.Sprintf("plan %s: %d of %d", a.ID, done, len(a.Steps)),
		Ambition: a.ID, Count: done, Steps: len(a.Steps), Ready: a.Done}}
}

// AmbitionView is one plan (#347): its id, name and the ending it is the
// plan for ("" for the milestone), whether it is pinned, the bar (1
// exactly when the ending's own condition holds), the next step's id
// ("" once done) and the steps.
type AmbitionView struct {
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	Ending   string             `json:"ending,omitempty"`
	Pinned   bool               `json:"pinned,omitempty"`
	Progress float64            `json:"progress"`
	Done     bool               `json:"done,omitempty"`
	Next     string             `json:"next,omitempty"`
	Steps    []AmbitionStepView `json:"steps"`
}

// AmbitionStepView is one step: the file's label, what is read against
// what the ending needs, in the unit (game.Unit*), and whether it is met.
type AmbitionStepView struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	Have  float64 `json:"have"`
	Need  float64 `json:"need"`
	Unit  string  `json:"unit"`
	Done  bool    `json:"done,omitempty"`
}

// ambitionViews is the plans as the view carries them.
func (s *Session) ambitionViews() []AmbitionView {
	var out []AmbitionView
	for _, a := range s.Ambitions() {
		out = append(out, AmbitionViewOf(s.cfg.Ambitions, a, s.w.Ambition == a.ID))
	}
	return out
}

// AmbitionViewOf is one plan in the view's words, named from the file.
func AmbitionViewOf(cfg content.AmbitionsConfig, a game.Ambition, pinned bool) AmbitionView {
	v := AmbitionView{ID: a.ID, Name: a.ID, Ending: a.Ending, Pinned: pinned, Progress: a.Progress(), Done: a.Done}
	row := cfg.Ambition(a.ID)
	if row != nil {
		v.Name = row.Name
	}
	if n := a.Next(); n != nil {
		v.Next = n.ID
	}
	for _, st := range a.Steps {
		label := st.ID
		if row != nil && row.Steps[st.ID] != "" {
			label = row.Steps[st.ID]
		}
		v.Steps = append(v.Steps, AmbitionStepView{ID: st.ID, Label: label, Have: st.Have, Need: st.Need, Unit: st.Unit, Done: st.Done})
	}
	return v
}
