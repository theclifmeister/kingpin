package engine_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// bare empties every stash, the road and the routine, so nothing is in
// stock or on its way.
func bare(w *game.World) {
	for _, cid := range w.CityOrder {
		for _, id := range w.Products {
			w.SetStock(cid, id, 0)
		}
	}
	w.Shipments, w.Supply, w.Standing = nil, nil, nil
}

// TestBrokeTonightAlert (#518): with nothing in stock or on the road and
// tonight's wages leaving the till under the cheapest unit, the morning
// carries a danger alert with the figures, on the market where a buy
// answers it; F will not start that night (Holds); and the night ended
// by hand is the broke ending the alert said. With the till over the
// line, or anything in the stash, there is no alert.
func TestBrokeTonightAlert(t *testing.T) {
	t.Parallel()
	s, w := crewRun(t)
	bare(w)
	w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 900, Name: "Brick", Role: game.RoleEnforcer, Skill: 40, Loyalty: 80, Nerve: 50, Wage: 200})
	w.Crew.NextID = 901
	w.Crew.Pay = events.PayGenerous
	wages := s.Rules().Crew.Wages(w, w.Crew.Pay)
	line := s.Sims().Crew.BrokeLine(w)
	if wages <= 0 || line <= 1 {
		t.Fatalf("wages %d, the broke line %.2f", wages, line)
	}
	w.Player.DirtyCash, w.Player.CleanCash = wages+1, 0
	got := ofKind(s, engine.AlertBroke)
	if len(got) != 1 {
		t.Fatalf("no broke alert with %d in hand, %d in wages, the line %.2f: %+v", w.Cash(), wages, line, s.Alerts())
	}
	a := got[0]
	if a.Amount != wages || a.Have != wages+1 || a.Gap != 1 || a.Line != line || !a.Danger() || a.Notice() || a.Act != (engine.Act{Screen: engine.ScreenMarket}) {
		t.Fatalf("the broke alert: %+v", a)
	}
	if s.Alerts()[0].Kind != engine.AlertBroke {
		t.Fatalf("the broke alert is not the loudest: %+v", s.Alerts()[0])
	}
	if h := s.Holds(); h == nil || h.Kind != engine.AlertBroke {
		t.Fatalf("F is not held: %+v", h)
	}
	day := w.Day
	if ran, st, _ := s.FastForward(7, nil); ran != 0 || st.Kind != engine.StopAlert || st.Alert.Kind != engine.AlertBroke || !st.Danger() || w.Day != day {
		t.Fatalf("F on the broke night: ran %d, %+v", ran, st)
	}
	// Over the line, no alert; stock in the stash, no alert.
	w.Player.DirtyCash = wages + int(line) + 10
	if got := ofKind(s, engine.AlertBroke); len(got) != 0 {
		t.Fatalf("a broke alert over the line: %+v", got)
	}
	w.Player.DirtyCash = wages + 1
	w.SetStock(w.Here().ID, w.Products[0], 1)
	if got := ofKind(s, engine.AlertBroke); len(got) != 0 {
		t.Fatalf("a broke alert with stock: %+v", got)
	}
	// The alert and the rule agree: the night ended by hand is broke.
	w.SetStock(w.Here().ID, w.Products[0], 0)
	s.EndDay()
	if w.Over == nil || w.Over.Cause != content.CauseBroke {
		t.Fatalf("the night the alert named ended %+v", w.Over)
	}
}

// TestWarrantHoldsFastForward (#519): a warrant out is the first alert,
// a danger, and F runs no night while it stands; once it is gone F runs.
func TestWarrantHoldsFastForward(t *testing.T) {
	t.Parallel()
	s, w := crewRun(t)
	s.EndDay()
	answered(w)
	w.Heat.WarrantDay = w.Day
	if as := s.Alerts(); len(as) == 0 || as[0].Kind != engine.AlertArrest || !as[0].Danger() {
		t.Fatalf("the warrant is not the first alert: %+v", as)
	}
	day := w.Day
	ran, st, _ := s.FastForward(7, nil)
	if ran != 0 || w.Day != day || st.Kind != engine.StopAlert || st.Alert.Kind != engine.AlertArrest || !st.Danger() {
		t.Fatalf("F with a warrant out: ran %d, %+v", ran, st)
	}
	w.Heat.WarrantDay = 0
	if ran, _, _ := s.FastForward(1, nil); ran != 1 {
		t.Fatalf("F with no warrant ran %d", ran)
	}
}

// TestADangerOutranksACard (#519): a card and an investigation on the
// same morning stop on the investigation, a danger, and the card is
// still there to answer; a task force formed outranks a card too. With
// no danger, the card stops as it did.
func TestADangerOutranksACard(t *testing.T) {
	t.Parallel()
	s, w := crewRun(t)
	answered(w)
	before := s.Alerts()
	w.Dilemmas.Pending = &game.Card{ID: "test", Day: w.Day}
	if st := s.Stop(nil, before); st.Kind != engine.StopCard {
		t.Fatalf("a card alone: %+v", st)
	}
	c := w.Here().Corners[0]
	w.Heat.Investigation = game.Investigation{City: w.Here().ID, Kind: game.LeadCorner, Target: c.ID, Opened: w.Day, Due: w.Day + 2}
	st := s.Stop([]events.Event{events.InvestigationOpened{Day: w.Day, City: w.Here().ID}}, before)
	if st.Kind != engine.StopAlert || st.Alert.Kind != engine.AlertInvestigation || !st.Danger() || w.Dilemmas.Pending == nil {
		t.Fatalf("a card and an investigation: %+v", st)
	}
	w.Heat.Investigation = game.Investigation{}
	st = s.Stop([]events.Event{events.TaskForceFormed{Day: w.Day, City: w.Here().ID}}, s.Alerts())
	if st.Kind != engine.StopEvent || !st.Danger() {
		t.Fatalf("a card and a task force: %+v", st)
	}
}

// TestQuietStreakStops (#519): the quiet streak lost stops F while
// retiring is the plan or the offshore account is open, and runs past
// otherwise.
func TestQuietStreakStops(t *testing.T) {
	t.Parallel()
	s, w := crewRun(t)
	answered(w)
	evs := []events.Event{events.QuietBroken{Day: w.Day, Days: 9, Cause: events.QuietHeat, City: w.Here().ID}}
	if st := s.Stop(evs, s.Alerts()); st.Kind == engine.StopEvent {
		t.Fatalf("a quiet streak lost with no plan to retire stopped: %+v", st)
	}
	if err := w.PinAmbition(content.AmbitionRetire); err != nil {
		t.Fatal(err)
	}
	if st := s.Stop(evs, s.Alerts()); st.Kind != engine.StopEvent {
		t.Fatalf("a quiet streak lost with retiring the plan: %+v", st)
	}
	_ = w.PinAmbition("")
	w.Offshore = 1000
	before := s.Alerts()
	if st := s.Stop(evs, before); st.Kind != engine.StopEvent {
		t.Fatalf("a quiet streak lost with the account open: %+v", st)
	}
}

// TestWarMuscleAlert (#520): an open war (declared, or over the war
// line) with fewer enforcers on the payroll than taken_out_muscle is an
// alert while it lasts, on the crew screen; the morning your held
// corners are down to one it is keyed again and a danger. With muscle
// enough, or no war, there is none.
func TestWarMuscleAlert(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	need := cfg.Rivals.Endings.TakenOutMuscle
	if need <= 0 {
		t.Skip("taken out is boxed")
	}
	s, w := crewRun(t)
	var kept []game.CrewMember
	for _, m := range w.Crew.Members {
		if m.Role != game.RoleEnforcer {
			kept = append(kept, m)
		}
	}
	w.Crew.Members = kept
	r := w.Rival()
	r.Arrived = 1
	home := w.Home()
	for i := range home.Corners[:2] {
		home.Corners[i].Owner, home.Corners[i].Runner, home.Corners[i].Faction = game.OwnerPlayer, game.You, ""
	}
	for i := 2; i < len(home.Corners); i++ {
		home.Corners[i].Owner, home.Corners[i].Runner = game.OwnerNone, 0
	}
	if got := ofKind(s, engine.AlertWarMuscle); len(got) != 0 {
		t.Fatalf("an alert with no war: %+v", got)
	}
	r.War = cfg.Rivals.Rivals.WarThreshold
	got := ofKind(s, engine.AlertWarMuscle)
	if len(got) != 1 || got[0].Level != r.Faction() || got[0].Have != 0 || got[0].Amount != need || got[0].Count != 2 || got[0].Danger() ||
		got[0].Act != (engine.Act{Screen: engine.ScreenCrew}) {
		t.Fatalf("the war short of muscle: %+v", got)
	}
	two := got[0].Key
	home.Corners[1].Owner, home.Corners[1].Runner = game.OwnerNone, 0
	got = ofKind(s, engine.AlertWarMuscle)
	if len(got) != 1 || got[0].Count != 1 || !got[0].Danger() || got[0].Key == two {
		t.Fatalf("the war short of muscle at the last corner: %+v", got)
	}
	for i := 0; i < need; i++ {
		w.Crew.Members = append(w.Crew.Members, game.CrewMember{ID: 900 + i, Name: "Brick", Role: game.RoleEnforcer, Skill: 40, Loyalty: 80, Nerve: 50, Wage: 50})
	}
	if got := ofKind(s, engine.AlertWarMuscle); len(got) != 0 {
		t.Fatalf("an alert with muscle enough: %+v", got)
	}
}

// TestFileAlertSaysOneBust (#519): the file alert carries the most pages
// one bust files (a raid's two), so a file two short says one bust can
// end it; it stands from that many pages short.
func TestFileAlertSaysOneBust(t *testing.T) {
	t.Parallel()
	s, w := crewRun(t)
	limit := s.Rules().Heat.EvidenceArrest(w)
	most := s.Sims().Heat.MostPages(w)
	if limit <= 2 || most < 2 {
		t.Fatalf("the file %d, the most one bust files %d", limit, most)
	}
	w.Heat.Evidence = limit - most
	got := ofKind(s, engine.AlertFile)
	if len(got) != 1 || got[0].Have != most || !got[0].Danger() {
		t.Fatalf("the file %d/%d: %+v", w.Heat.Evidence, limit, got)
	}
}
