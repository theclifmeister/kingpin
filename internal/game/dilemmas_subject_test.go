package game

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
)

// A once_per card never names the same subject twice (#466: "Yaya's
// brother was shot" on day 18 and again on day 94, the permit asked of
// a Laundromat that had paid it): the trigger passes over a subject it
// has named for the next in line, and the card is not drawn when none
// is left. A card with no once_per is as it was.
func TestOncePerCardPassesOverWhoItNamed(t *testing.T) {
	w := testWorld()
	w.Crew.Members = []CrewMember{{ID: 1, Name: "Yaya", Role: "runner", Loyalty: 80}, {ID: 2, Name: "Tee", Role: "runner", Loyalty: 60}}
	funeral := content.CardConfig{ID: "funeral", OncePer: content.OncePerMember, Trigger: content.CardTrigger{CrewMin: 2, LoyaltyAbove: 1}, Choices: []content.ChoiceConfig{{}, {}}}
	s, ok := Eligible(w, funeral)
	if !ok || s.Name != "Yaya" || s.Subject(content.OncePerMember) != "1" {
		t.Fatalf("the first funeral: %+v %v", s, ok)
	}
	w.Dilemmas.Drawn = map[string]int{"funeral": 1, CardSubject("funeral", content.OncePerMember, "1"): 1}
	if s, ok := Eligible(w, funeral); !ok || s.Name != "Tee" {
		t.Fatalf("the second funeral: %+v %v", s, ok)
	}
	w.Dilemmas.Drawn[CardSubject("funeral", content.OncePerMember, "2")] = 1
	if s, ok := Eligible(w, funeral); ok {
		t.Fatalf("a third funeral with every brother buried: %+v", s)
	}
	again := funeral
	again.OncePer = ""
	if s, ok := Eligible(w, again); !ok || s.Name != "Yaya" {
		t.Fatalf("a card with no once_per: %+v %v", s, ok)
	}

	w.Fronts = []Front{{ID: "laundromat", Name: "Suds"}, {ID: "carwash", Name: "Shine"}}
	permit := content.CardConfig{ID: "permit", OncePer: content.OncePerFront, Trigger: content.CardTrigger{Fronts: true}, Choices: []content.ChoiceConfig{{}, {}}}
	w.Dilemmas.Drawn[CardSubject("permit", content.OncePerFront, "laundromat")] = 1
	if s, ok := Eligible(w, permit); !ok || s.Front != "Shine" {
		t.Fatalf("the permit after the laundromat's: %+v %v", s, ok)
	}
	w.Dilemmas.Drawn[CardSubject("permit", content.OncePerFront, "carwash")] = 1
	if s, ok := Eligible(w, permit); ok {
		t.Fatalf("a permit with every front's paid: %+v", s)
	}
}

// A card's place is its corner's city (#466: "a kid hanging around
// Shipyard ... every outfit in Eastside" with Shipyard in Bayport), and
// a card that names a member and a corner names the member on the
// corner (#466: "FUNNY MONEY: Yaya took $3,000 on The Wharf" with Yaya
// idle and Tee on The Wharf).
func TestCardIsWhereItsCornerIs(t *testing.T) {
	w := twoCityWorld()
	w.Recall(You)
	w.Crew.Members = []CrewMember{{ID: 1, Name: "Yaya", Role: "runner", Loyalty: 80}, {ID: 2, Name: "Tee", Role: "runner", Loyalty: 40}}
	wharf := &w.Cities["port"].Corners[0]
	wharf.Owner, wharf.Runner = OwnerPlayer, 2
	kid := content.CardConfig{ID: "kid", Trigger: content.CardTrigger{Corners: 1, CrewMin: 1}, Choices: []content.ChoiceConfig{{}, {}}}
	if s, ok := Eligible(w, kid); !ok || s.Corner != "Wharf" || s.City != "Port" {
		t.Fatalf("the kid: %+v %v", s, ok)
	}
	counterfeit := content.CardConfig{ID: "counterfeit", Trigger: content.CardTrigger{Role: "runner", Corners: 1}, Choices: []content.ChoiceConfig{{}, {}}}
	if s, ok := Eligible(w, counterfeit); !ok || s.Name != "Tee" || s.Corner != "Wharf" {
		t.Fatalf("funny money named %s on %s: %+v %v", s.Name, s.Corner, s, ok)
	}
	// With nobody on a corner of their own the most loyal is named, as
	// before.
	anyone := content.CardConfig{ID: "raise", Trigger: content.CardTrigger{Role: "runner"}, Choices: []content.ChoiceConfig{{}, {}}}
	if s, ok := Eligible(w, anyone); !ok || s.Name != "Yaya" {
		t.Fatalf("a card about no corner: %+v %v", s, ok)
	}
}

// A share of the card's sum reads as the *_amount keys pay it (#501), so
// the card game's label names the stake and the net its chip shows.
func TestShareIsWhatTheAmountKeyPays(t *testing.T) {
	s := CardSlots{Sum: 10_001}
	w := &World{}
	if err := w.applyEffect(&Card{Amount: s.Sum}, "dirty_amount", 0.5); err != nil {
		t.Fatal(err)
	}
	if got, want := s.Share(0.5), "$5,001"; got != want || w.Player.DirtyCash != 5_001 {
		t.Errorf("half of $10,001 reads %s and pays %d, want %s", got, w.Player.DirtyCash, want)
	}
}
