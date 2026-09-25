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

// A card is about a faction still at the table (#385): with the rival at
// home fragmented, a rival card names the faction that holds the ground,
// its effects and its preview land on that faction, a contested card
// borders only a standing faction's corner, and with nobody standing on
// a corner no rival card is drawn. A seed-41 replay dealt "Uncle Roy is
// in hospital" thirteen days after he was killed.
func TestCardIsAboutAStandingFaction(t *testing.T) {
	w := testWorld()
	w.Home().Corners = append(w.Home().Corners, Corner{ID: "far", City: "test", Name: "Far", Demand: 1, Heat: 1, Risk: 1, Owner: OwnerNone})
	cs := w.Home().Corners
	for i := range cs {
		cs[i].X, cs[i].Y = i, 0
	}
	roy := &RivalState{ID: FactionRival, Leader: "Uncle Roy", Arrived: 1, Fragmented: 1, Muscle: 3}
	lena := &RivalState{ID: "f2", Leader: "Lena", Arrived: 1, Muscle: 3, Personality: "defensive"}
	w.Rivals = []*RivalState{roy, lena}
	cs[1].Owner, cs[1].Faction = OwnerRival, FactionRival // Roy's, drifting to the street
	cs[2].Owner, cs[2].Faction = OwnerRival, "f2"

	push := []content.ChoiceConfig{{Effects: map[string]float64{"war": 12, "rival_muscle": -2}}, {}}
	laidUp := content.CardConfig{ID: "rival_laid_up", Trigger: content.CardTrigger{Rival: true, Contested: true}, Choices: push}
	sitDown := content.CardConfig{ID: "sit_down", Trigger: content.CardTrigger{Rival: true, Personality: "defensive"}, Choices: push}

	if _, ok := Eligible(w, laidUp); ok {
		t.Fatal("a contested card drawn on the corner of a fragmented faction")
	}
	s, ok := Eligible(w, sitDown)
	if !ok || s.Rival != "Lena" || s.Faction != "f2" {
		t.Fatalf("a rival card with Lena on the map: %+v %v", s, ok)
	}

	cs[1].Faction = "f2" // Lena takes the corner beside yours
	s, ok = Eligible(w, laidUp)
	if !ok || s.Rival != "Lena" || s.Theirs != "Docks" || s.Faction != "f2" {
		t.Fatalf("the contested card: %+v %v", s, ok)
	}
	c := &Card{ID: "rival_laid_up", Faction: s.Faction, Choices: []Choice{{Label: "Push", Effects: push[0].Effects}, {Label: "Wait"}}}
	pre, err := c.Preview(w, 0)
	if err != nil {
		t.Fatal(err)
	}
	w.Dilemmas.Pending = c
	before := Reading(w, c)
	if _, err := w.Choose(0); err != nil {
		t.Fatal(err)
	}
	if lena.War != 12 || lena.Muscle != 1 || roy.War != 0 || roy.Muscle != 3 {
		t.Fatalf("the push landed on Lena %+v and Roy %+v", *lena, *roy)
	}
	if got := Moved(before, w, c); len(pre) != 2 || len(got) != 2 || pre[0] != got[0] || pre[1] != got[1] {
		t.Fatalf("preview %+v, moved %+v", pre, got)
	}

	cs[1].Owner, cs[1].Faction = OwnerNone, ""
	cs[2].Owner, cs[2].Faction = OwnerNone, ""
	if _, ok := Eligible(w, sitDown); ok {
		t.Fatal("a rival card drawn with no standing faction on a corner")
	}
}

// A card about a crew member names the corner that member works, not
// the busiest one (#421): "Pep took twenties on The Projects" was dealt
// with Pep on The Docks and somebody else on The Projects.
func TestCardNamesTheMembersCorner(t *testing.T) {
	w := testWorld()
	w.Crew.Members = []CrewMember{{ID: 1, Name: "Dre", Role: "runner", Loyalty: 50}}
	if err := w.Post("docks", You); err != nil { // the busier corner is yours
		t.Fatal(err)
	}
	if err := w.Post("home", 1); err != nil {
		t.Fatal(err)
	}
	counterfeit := content.CardConfig{ID: "counterfeit", Trigger: content.CardTrigger{Role: "runner", Corners: 1},
		Choices: []content.ChoiceConfig{{}, {}}}
	s, ok := Eligible(w, counterfeit)
	if !ok || s.Name != "Dre" || s.Corner != "Home" {
		t.Fatalf("the card put %s on %q: %+v %v", s.Name, s.Corner, s, ok)
	}
	// A card with no member still names the busiest corner.
	busiest := content.CardConfig{ID: "b", Trigger: content.CardTrigger{Corners: 1}, Choices: []content.ChoiceConfig{{}, {}}}
	if s, ok := Eligible(w, busiest); !ok || s.Corner != "Docks" {
		t.Fatalf("a card about no one: %+v %v", s, ok)
	}
}

// A card that pays out in the morning raises the peaks at once (#439):
// the CASH panel's peak never reads below the pile, and the gates that
// read PeakCash see the money the day it lands, not the next night.
func TestChooseStampsThePeaks(t *testing.T) {
	w := cardWorld()
	w.Stats.PeakCash, w.Stats.PeakClean = w.Cash(), w.Player.CleanCash
	w.Dilemmas.Pending = &Card{ID: "t", Day: 1, Title: "T",
		Choices: []Choice{{Label: "take", Outcome: "paid", Effects: map[string]float64{"dirty_cash": 3100, "clean_cash": 200}}}}
	if _, err := w.Choose(0); err != nil {
		t.Fatal(err)
	}
	if w.Stats.PeakCash != w.Cash() || w.Stats.PeakClean != w.Player.CleanCash {
		t.Fatalf("peak %d clean %d, want %d and %d", w.Stats.PeakCash, w.Stats.PeakClean, w.Cash(), w.Player.CleanCash)
	}
}

// TestPeakCountsTheAccount (#477): the peak the unlock gates read is
// the cash in hand and the offshore account, since money sent there
// never comes back; a playtest's retiree held $118K of peak with its
// savings in the account and never saw the restaurant. PeakClean, the
// assets' line, stays the clean pile alone. A save from before, its
// peak under what it holds, catches up on its first night: the peak
// only rises, so no migration.
func TestPeakCountsTheAccount(t *testing.T) {
	w := cardWorld()
	w.Stats.PeakCash, w.Stats.PeakClean = w.Cash(), w.Player.CleanCash
	w.Offshore = 900_000
	NewClock(nil, &counter{}).EndDay(w)
	if want := w.Cash() + w.Offshore; w.Stats.PeakCash != want || w.Holdings() != want {
		t.Fatalf("peak %d, holdings %d, want %d with the account", w.Stats.PeakCash, w.Holdings(), want)
	}
	if w.Stats.PeakClean != w.Player.CleanCash {
		t.Fatalf("peak clean %d counts the account, want %d", w.Stats.PeakClean, w.Player.CleanCash)
	}
	// Money moved into the account is not a new high: the peak holds.
	w.Player.CleanCash -= 400
	w.Offshore += 400
	peak := w.Stats.PeakCash
	NewClock(nil, &counter{}).EndDay(w)
	if w.Stats.PeakCash != peak {
		t.Fatalf("moving clean offshore moved the peak %d -> %d", peak, w.Stats.PeakCash)
	}
}
