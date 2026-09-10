package game

import (
	"errors"
	"strings"
	"testing"
)

// cardWorld is testWorld with somebody on the payroll, a rival in town and
// stock in the bag, so every effect has something to land on.
func cardWorld() *World {
	w := testWorld()
	w.Player.DirtyCash, w.Player.CleanCash = 1000, 500
	w.Player.Stock["a"] = 40
	w.Heat.Value = 50
	w.Crew.Members = []CrewMember{{ID: 1, Name: "Dre", Role: "runner", Loyalty: 50}, {ID: 2, Name: "Tank", Role: "enforcer", Loyalty: 50}}
	w.Rival = RivalState{Leader: "Ghost", War: 50, Grudge: 1, Muscle: 3, Cash: 5000, Arrived: 1}
	w.Player.Reputation = Reputation{Fear: 10, Respect: 20, Notoriety: 30}
	return w
}

// Every legal effect key does something observable, in one Choose, and
// the card is gone afterwards with its outcome in the journal.
func TestChooseAppliesEveryEffectKey(t *testing.T) {
	w := cardWorld()
	all := map[string]float64{}
	for _, k := range EffectKeys {
		all[k] = -1
	}
	all["stock_share"] = -0.5
	all["clean_amount"] = 2
	w.Dilemmas.Pending = &Card{ID: "t", Day: 1, Title: "T", Amount: 100, Member: 1,
		Choices: []Choice{{Label: "all", Outcome: "everything happened", Effects: all}, {Label: "none", Outcome: "nothing"}}}
	a, err := w.Choose(0)
	if err != nil {
		t.Fatal(err)
	}
	if a.Choice != "all" || w.Dilemmas.Pending != nil || w.Dilemmas.Answered == nil {
		t.Fatalf("answer %+v pending %v answered %v", a, w.Dilemmas.Pending, w.Dilemmas.Answered)
	}
	checks := map[string]bool{
		"dirty_cash and dirty_amount": w.Player.DirtyCash == 1000-1-100,
		"clean_cash and clean_amount": w.Player.CleanCash == 500-1+200,
		"heat":                        w.Heat.Value == 49,
		"loyalty":                     w.Crew.Members[0].Loyalty == 48, // -1 named, -1 crew
		"crew_loyalty":                w.Crew.Members[1].Loyalty == 49,
		"war":                         w.Rival.War == 49,
		"grudge":                      w.Rival.Grudge == 0,
		"rival_muscle":                w.Rival.Muscle == 2,
		"rival_cash":                  w.Rival.Cash == 4999,
		"stock_share":                 w.Player.Stock["a"] == 20,
		"fear":                        w.Player.Reputation.Fear == 9,
		"respect":                     w.Player.Reputation.Respect == 19,
		"notoriety":                   w.Player.Reputation.Notoriety == 29,
	}
	for what, ok := range checks {
		if !ok {
			t.Errorf("%s did not apply: %+v %+v %+v", what, w.Player, w.Heat.Value, w.Rival)
		}
	}
	last := w.Journal[len(w.Journal)-1]
	if last.Source != "dilemma" || last.Text != "everything happened" {
		t.Fatalf("journal: %+v", last)
	}
	if _, err := w.Choose(0); !errors.Is(err, ErrNoCard) {
		t.Fatalf("second answer: %v", err)
	}
}

// A key the world does not know is an error, never a silent no-op; the
// list of legal keys and the switch that applies them must agree.
func TestChooseRefusesUnknownEffect(t *testing.T) {
	w := cardWorld()
	w.Dilemmas.Pending = &Card{ID: "t", Choices: []Choice{{Label: "x", Outcome: "x", Effects: map[string]float64{"evidence": 1}}, {Label: "y", Outcome: "y"}}}
	_, err := w.Choose(0)
	if err == nil || !strings.Contains(err.Error(), `unknown effect "evidence"`) {
		t.Fatalf("err = %v", err)
	}
	for _, k := range EffectKeys {
		if !KnownEffect(k) {
			t.Errorf("%s is listed but not known", k)
		}
		if err := w.applyEffect(&Card{Amount: 1}, k, 0); err != nil {
			t.Errorf("%s is listed but applyEffect refuses it: %v", k, err)
		}
	}
	if _, err := w.Choose(5); !errors.Is(err, ErrBadChoice) {
		t.Fatalf("choice 5 of 2: %v", err)
	}
}

// Cash never goes negative, heat, loyalty and war stay in 0..100, and
// found stock never exceeds what the operation can hold.
func TestChooseClamps(t *testing.T) {
	w := cardWorld()
	w.Player.CarryLimit = 50
	w.Dilemmas.Pending = &Card{ID: "t", Amount: 10_000, Member: 2, Choices: []Choice{
		{Label: "x", Outcome: "x", Effects: map[string]float64{"dirty_amount": -1, "clean_cash": -9000, "heat": 80, "loyalty": 90, "war": -90, "stock_share": 2, "fear": 95, "respect": -50}},
		{Label: "y", Outcome: "y"},
	}}
	if _, err := w.Choose(0); err != nil {
		t.Fatal(err)
	}
	if w.Player.DirtyCash != 0 || w.Player.CleanCash != 0 || w.Heat.Value != 100 || w.Crew.Members[1].Loyalty != 100 || w.Rival.War != 0 {
		t.Fatalf("clamps: %+v heat %v loyalty %v war %v", w.Player, w.Heat.Value, w.Crew.Members[1].Loyalty, w.Rival.War)
	}
	if w.Player.Stock["a"] != 50 {
		t.Fatalf("found stock past capacity: %d", w.Player.Stock["a"])
	}
	if r := w.Player.Reputation; r.Fear != 100 || r.Respect != 0 {
		t.Fatalf("reputation not clamped: %+v", r)
	}
}

// A save on a card restores the card: quitting on one and continuing
// shows the same question with the same choices.
func TestSaveKeepsCard(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	w := cardWorld()
	w.Dilemmas.LastCard = 7
	w.Dilemmas.Drawn = map[string]int{"stash_hit": 1}
	w.Dilemmas.Pending = &Card{ID: "stash_hit", Day: 7, Title: "A night's work", Text: "Tank wants to hit the stash.", Member: 2, Amount: 800,
		Choices: []Choice{{Label: "Approve it", Outcome: "done", Headline: "Stash hit", Effects: map[string]float64{"heat": 6, "dirty_amount": 1}}, {Label: "Veto it", Outcome: "no"}}}
	if err := Save(w); err != nil {
		t.Fatal(err)
	}
	w2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	c := w2.Dilemmas.Pending
	if c == nil || c.ID != "stash_hit" || c.Text != "Tank wants to hit the stash." || len(c.Choices) != 2 || c.Choices[0].Effects["heat"] != 6 || w2.Dilemmas.LastCard != 7 || w2.Dilemmas.Drawn["stash_hit"] != 1 {
		t.Fatalf("card did not survive the save: %+v", w2.Dilemmas)
	}
	if _, err := w2.Choose(0); err != nil || w2.Heat.Value != 56 || w2.Player.DirtyCash != 1800 {
		t.Fatalf("answering after load: %v heat %v cash %d", err, w2.Heat.Value, w2.Player.DirtyCash)
	}
}
