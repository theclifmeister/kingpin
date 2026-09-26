package ui

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// TestContractRoomIsSaid (#524): a contract set where you stand whose
// room is your own carry warns when it is set, and the trip away names
// it; one the city holds without you says nothing.
func TestContractRoomIsSaid(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	home, weed := w.Player.Location, w.Products[0]
	if w.CapacityAway(home) != 0 {
		t.Fatalf("fixture: %d held without you", w.CapacityAway(home))
	}
	if err := w.SetSupply(home, weed, 100); err != nil {
		t.Fatal(err)
	}
	if got := m.contractRoom(home, weed, 100); !strings.Contains(got, "holds 0 of it without you") {
		t.Errorf("the contract's warning reads %q", got)
	}
	left := m.contractsLeft(home)
	if len(left) != 1 || !strings.Contains(left[0], "has room for 0 once you go") {
		t.Fatalf("leaving with the contract says %q", left)
	}
	if len(w.CityOrder) > 1 {
		m.askTravel()
		if view := squash(stripANSI(m.View())); !strings.Contains(view, "has room for 0 once you go") {
			t.Errorf("the trip does not name the contract:\n%s", view)
		}
		m.Update(key("esc"))
	}
	w.Houses = append(w.Houses, game.House{ID: "stash", City: home, Capacity: 500})
	if got := m.contractRoom(home, weed, 100); got != "" || len(m.contractsLeft(home)) != 0 {
		t.Errorf("with a house the contract still warns: %q %q", got, m.contractsLeft(home))
	}
}

// TestSellRefusalNamesTheRoom (#524): a contract that brings none
// because the contracts before it took the room says the room, not the
// cash, with the cash to spare.
func TestSellRefusalNamesTheRoom(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	home := w.Player.Location
	w.Player.DirtyCash = 10_000_000
	first, second := "", ""
	for _, id := range w.Products {
		if sup := w.BestSupplier(home, id); sup == nil || !w.Available(sup, id) {
			continue
		}
		if first == "" {
			first = id
		} else if second == "" {
			second = id
		}
	}
	if second == "" {
		t.Skip("fixture: fewer than two products sold at home")
	}
	// Each at the whole room: whichever the plan puts first (by margin,
	// short of room, #470) takes it all, and the other brings none.
	for _, id := range []string{first, second} {
		if err := w.SetSupply(home, id, w.Free(home)); err != nil {
			t.Fatal(err)
		}
	}
	starved := second
	if m.rules.Market.Due(w, home, first) == 0 {
		starved = first
	}
	if due := m.rules.Market.Due(w, home, starved); due != 0 {
		t.Fatalf("fixture: both contracts bring some (%d)", due)
	}
	got := m.contractBringsNone(home, starved)
	if !strings.Contains(got, "room") || strings.Contains(got, "does not cover") {
		t.Errorf("the refusal reads %q, want the room", got)
	}
}

// TestLedgerSaysTheRoadAndTheShut (#524): with a route idle on the till
// the ledger's till lines show the choice, what the route waits on over
// the float and the till that would save for it; and a front the night
// is expected to shut on its upkeep is warned of before it shuts.
func TestLedgerSaysTheRoadAndTheShut(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	r := m.cfg.Routes.Routes[0]
	coke := "coke"
	if w.Product(r.From, coke) == nil || w.Product(r.To, coke) == nil {
		t.Skip("fixture: no coke on the first route")
	}
	sup := w.WholesaleSupplier(r.From)
	if sup == nil {
		t.Skip("fixture: no wholesaler at the first route's start")
	}
	w.Stats.PeakCash = max(w.Stats.PeakCash, sup.UnlockCash)
	w.SetStock(r.From, coke, 0)
	if err := w.SetRoute(r.ID, events.RouteNormal); err != nil {
		t.Fatal(err)
	}
	if err := w.SetRouteTarget(r.ID, coke, 60); err != nil {
		t.Fatal(err)
	}
	w.Player.DirtyCash = m.floatLine() + 100
	need := m.rules.Logistics.Waits(w, r)
	if need <= 100 {
		t.Fatalf("fixture: the route waits on %d (idle %q)", need, m.rules.Logistics.Idle(w, r))
	}
	m.Update(key("7"))
	view := squash(stripANSI(m.View()))
	for _, want := range []string{r.Name + " waits on " + money(need) + " over the float", "the till to " + money(w.SupplyOutlay()+m.floatLine()+need) + " to save for it"} {
		if !strings.Contains(view, want) {
			t.Errorf("the ledger lacks %q:\n%s", want, view)
		}
	}
	fc := m.cfg.Laundering.Fronts[0]
	w.Fronts = append(w.Fronts, game.Front{ID: fc.ID, Name: fc.Name})
	w.Player.CleanCash = 0
	w.Player.DirtyCash = 0
	if view := squash(stripANSI(m.View())); !strings.Contains(view, fc.Name+" expected to shut tonight") {
		t.Errorf("the ledger does not warn of the shut:\n%s", view)
	}
}

// TestLieutenantKeepsAreShown (#524): the crew screen's lieutenant rows
// carry their own stock levels in their city.
func TestLieutenantKeepsAreShown(t *testing.T) {
	m := newTestModel(t, 120, 40)
	w := m.w
	city, id := w.CityOrder[len(w.CityOrder)-1], w.Products[0]
	lt := game.CrewMember{ID: 77, Name: "Hutch", Role: game.RoleLieutenant, City: city, Skill: 50, Loyalty: 80}
	w.Crew.Members = append(w.Crew.Members, lt)
	if w.DelegatedSupply == nil {
		w.DelegatedSupply = map[string]game.SupplyContract{}
	}
	w.DelegatedSupply[game.SupplyKey(city, id)] = game.SupplyContract{City: city, Product: id, Units: 678}
	lines := stripANSI(strings.Join(m.lieutenantLines(lt), "\n"))
	if want := "678 " + w.ProductName(id); !strings.Contains(lines, "keeps") || !strings.Contains(lines, want) {
		t.Errorf("the lieutenant's rows lack %q:\n%s", want, lines)
	}
}
