package game

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
)

// cardWorld is testWorld with somebody on the payroll, a rival in town and
// stock in the bag, so every effect has something to land on.
func cardWorld() *World {
	w := testWorld()
	w.Player.DirtyCash, w.Player.CleanCash = 1000, 500
	w.SetStock("test", "a", 40)
	w.Home().Heat = 50
	w.Crew.Members = []CrewMember{{ID: 1, Name: "Dre", Role: "runner", Loyalty: 50}, {ID: 2, Name: "Tank", Role: "enforcer", Loyalty: 50}}
	*w.Rival() = RivalState{Leader: "Ghost", War: 50, Grudge: 1, Muscle: 3, Cash: 5000, Arrived: 1}
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
	w.Dilemmas.Owes = 2
	w.Dilemmas.Pending = &Card{ID: "t", Day: 1, Title: "T", Amount: 100, Member: 1, Corner: "home",
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
		"heat":                        w.Home().Heat == 49,
		"loyalty":                     w.Crew.Members[0].Loyalty == 48, // -1 named, -1 crew
		"crew_loyalty":                w.Crew.Members[1].Loyalty == 49,
		"war":                         w.Rival().War == 49,
		"grudge":                      w.Rival().Grudge == 0,
		"rival_muscle":                w.Rival().Muscle == 2,
		"rival_cash":                  w.Rival().Cash == 4999,
		"stock_share":                 w.Stock("test", "a") == 20,
		"fear":                        w.Player.Reputation.Fear == 9,
		"respect":                     w.Player.Reputation.Respect == 19,
		"notoriety":                   w.Player.Reputation.Notoriety == 29,
		"corner":                      w.Corner("home").Owner == OwnerNone,
		"owes":                        w.Dilemmas.Owes == 1,
	}
	for what, ok := range checks {
		if !ok {
			t.Errorf("%s did not apply: %+v %+v %+v", what, w.Player, w.Home().Heat, w.Rival())
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
// list of legal keys and the table that applies them must agree.
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

// A choice with a key the world does not know changes nothing (#274):
// every key is checked before any applies, so the known keys that sort
// before the bad one ("clean_cash", "dirty_cash", "heat") and the one
// after it ("war") land nowhere, and the card is still pending.
func TestUnknownEffectChangesNothing(t *testing.T) {
	w := cardWorld()
	card := &Card{ID: "t", Amount: 100, Member: 1, Choices: []Choice{
		{Label: "x", Outcome: "x", Effects: map[string]float64{"clean_cash": 50, "dirty_cash": -50, "heat": 10, "hex": 1, "war": 10}},
		{Label: "y", Outcome: "y"},
	}}
	w.Dilemmas.Pending = card
	before, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Choose(0); err == nil || !strings.Contains(err.Error(), `unknown effect "hex"`) {
		t.Fatalf("err = %v", err)
	}
	after, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("a refused choice changed the world:\nbefore %s\nafter  %s", before, after)
	}
	if w.Dilemmas.Pending != card || w.Dilemmas.Answered != nil {
		t.Fatalf("pending %v answered %v", w.Dilemmas.Pending, w.Dilemmas.Answered)
	}
}

// EffectKeys is the table's keys, sorted: the order Choose applies them.
func TestEffectKeysAreSorted(t *testing.T) {
	if !sort.StringsAreSorted(EffectKeys) || len(EffectKeys) != len(effects) {
		t.Fatalf("EffectKeys %v", EffectKeys)
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
	if w.Player.DirtyCash != 0 || w.Player.CleanCash != 0 || w.Home().Heat != 100 || w.Crew.Members[1].Loyalty != 100 || w.Rival().War != 0 {
		t.Fatalf("clamps: %+v heat %v loyalty %v war %v", w.Player, w.Home().Heat, w.Crew.Members[1].Loyalty, w.Rival().War)
	}
	if w.Stock("test", "a") != 50 {
		t.Fatalf("found stock past capacity: %d", w.Stock("test", "a"))
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
	if err := Save(1, w); err != nil {
		t.Fatal(err)
	}
	w2, err := Load(1)
	if err != nil {
		t.Fatal(err)
	}
	c := w2.Dilemmas.Pending
	if c == nil || c.ID != "stash_hit" || c.Text != "Tank wants to hit the stash." || len(c.Choices) != 2 || c.Choices[0].Effects["heat"] != 6 || w2.Dilemmas.LastCard != 7 || w2.Dilemmas.Drawn["stash_hit"] != 1 {
		t.Fatalf("card did not survive the save: %+v", w2.Dilemmas)
	}
	if _, err := w2.Choose(0); err != nil || w2.Home().Heat != 56 || w2.Player.DirtyCash != 1800 {
		t.Fatalf("answering after load: %v heat %v cash %d", err, w2.Home().Heat, w2.Player.DirtyCash)
	}
}

// The two keys of #342: corner gives the card's corner up to the street,
// as Abandon does, and only a corner you still hold; owes runs the
// favours owed up and down, never under zero, and a trigger's owes_min
// reads it.
func TestCornerAndOwesEffects(t *testing.T) {
	w := cardWorld()
	if w.Corner("home").Owner != OwnerPlayer {
		t.Fatalf("the fixture works home: %+v", w.Corner("home"))
	}
	w.Dilemmas.Pending = &Card{ID: "t", Corner: "home", Choices: []Choice{
		{Label: "x", Outcome: "x", Effects: map[string]float64{"corner": -1, "owes": 1}}, {Label: "y", Outcome: "y"},
	}}
	if _, err := w.Choose(0); err != nil {
		t.Fatal(err)
	}
	if c := w.Corner("home"); c.Owner != OwnerNone || len(w.Today.Abandoned) != 1 {
		t.Fatalf("corner not given up: %+v, abandoned %v", c, w.Today.Abandoned)
	}
	if w.Dilemmas.Owes != 1 {
		t.Fatalf("owes = %d", w.Dilemmas.Owes)
	}
	collect := content.CardConfig{Trigger: content.CardTrigger{OwesMin: 1}}
	if _, ok := Eligible(w, collect); !ok {
		t.Fatal("owes_min 1 does not hold on a favour owed")
	}
	// A corner already gone is nothing to give up, and owes stops at zero.
	w.Dilemmas.Pending = &Card{ID: "t", Corner: "home", Choices: []Choice{
		{Label: "x", Outcome: "x", Effects: map[string]float64{"corner": -1, "owes": -3}}, {Label: "y", Outcome: "y"},
	}}
	if _, err := w.Choose(0); err != nil {
		t.Fatal(err)
	}
	if w.Dilemmas.Owes != 0 || len(w.Today.Abandoned) != 1 {
		t.Fatalf("owes %d abandoned %v", w.Dilemmas.Owes, w.Today.Abandoned)
	}
	if _, ok := Eligible(w, collect); ok {
		t.Fatal("owes_min 1 holds with nothing owed")
	}
}

// A card's sum (#342): its share of the bag, at least its amount, never
// over its amount_max, and a card with no cap grows with the bag.
func TestCardSumIsCapped(t *testing.T) {
	furnace := content.CardConfig{Amount: 250, AmountShare: 0.04, AmountMax: 5000}
	for _, r := range []struct{ dirty, want int }{{0, 250}, {10_000, 400}, {100_000, 4000}, {100_000_000, 5000}} {
		if got := CardSum(furnace, r.dirty); got != r.want {
			t.Errorf("furnace at $%d: $%d, want $%d", r.dirty, got, r.want)
		}
	}
	if got := CardSum(content.CardConfig{Amount: 800, AmountShare: 0.25}, 100_000_000); got != 25_000_000 {
		t.Errorf("an uncapped business card at $100M: $%d", got)
	}
}

// A card paid in full out of one pile names no more than the pile holds,
// rounded down to two figures, and is not drawn below its floor (#384):
// a permit sized on $82,000 dirty and paid in $2,550 clean said $4,100
// and charged $2,550.
func TestCardSumIsWhatThePileCanPay(t *testing.T) {
	permit := content.CardConfig{Amount: 400, AmountShare: 0.05, AmountMax: 50000,
		Choices: []content.ChoiceConfig{{Effects: map[string]float64{"clean_amount": -1}}, {Effects: map[string]float64{"heat": 5}}}}
	w := cardWorld()
	for _, r := range []struct {
		dirty, clean, want int
		ok                 bool
	}{{82_000, 2_550, 2_500, true}, {82_000, 100_000, 4_100, true}, {82_000, 450, 450, true}, {82_000, 399, 0, false}} {
		w.Player.DirtyCash, w.Player.CleanCash = r.dirty, r.clean
		s, ok := Eligible(w, permit)
		if ok != r.ok || (ok && s.Sum != r.want) {
			t.Errorf("$%d dirty, $%d clean: $%d %v, want $%d %v", r.dirty, r.clean, s.Sum, ok, r.want, r.ok)
		}
	}
	// Paid out of the dirty pile, it is the dirty pile that caps it.
	wash := content.CardConfig{Amount: 1000, AmountShare: 0.2,
		Choices: []content.ChoiceConfig{{Effects: map[string]float64{"dirty_amount": -1, "clean_amount": 0.7}}}}
	w.Player.DirtyCash, w.Player.CleanCash = 900, 50_000
	if _, ok := Eligible(w, wash); ok {
		t.Error("a $1,000 wash drawn on $900 dirty")
	}
	// A card that pays you is sized on the bag as before.
	gift := content.CardConfig{Amount: 1000, AmountShare: 0.1,
		Choices: []content.ChoiceConfig{{Effects: map[string]float64{"clean_amount": 1}}}}
	w.Player.DirtyCash, w.Player.CleanCash = 50_000, 0
	if s, ok := Eligible(w, gift); !ok || s.Sum != 5000 {
		t.Errorf("a gift: $%d %v", s.Sum, ok)
	}
}
