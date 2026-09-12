package game

import (
	"errors"
	"math"
	"testing"
)

// delegatePort puts a lieutenant on the payroll running the port, with a
// runner of theirs on the wharf so the stash there has room, the player
// standing at home.
func delegatePort(w *World) *CrewMember {
	w.Crew.Members = append(w.Crew.Members,
		CrewMember{ID: 1, Name: "Marta", Role: RoleLieutenant, City: "port", Units: 0},
		CrewMember{ID: 2, Name: "Dre", Role: "runner", Units: 100})
	if err := w.Post("wharf", 2); err != nil {
		panic(err)
	}
	return w.Crew.Member(1)
}

// A buy through the lieutenant (#174): from home, a buy from a connect
// in the city they run lands in the stash there at the contract markup,
// the receipt naming them; the connect's book, the day's count and the
// price pressure move exactly as for the same buy by hand standing
// there; without a lieutenant it is ErrElsewhere as before; a credit
// buy goes on the connect's book as yours; and the cart's Return walks
// it back exactly, the markup included. A buy where you stand pays no
// markup, lieutenant or not.
func TestBuyThroughTheLieutenant(t *testing.T) {
	// The same buy by hand, standing in the port.
	hand := twoCityWorld()
	hand.Crew.Members = append(hand.Crew.Members, CrewMember{ID: 2, Name: "Dre", Role: "runner", Units: 100})
	if err := hand.Post("wharf", 2); err != nil {
		t.Fatal(err)
	}
	hand.Player.Location = "port"
	priceAt(hand, "port", "a", 4)
	hand.Player.DirtyCash = 10_000
	hp, err := hand.Buy("portstreet", "a", 20, false, 0.5)
	if err != nil {
		t.Fatal(err)
	}

	w := twoCityWorld()
	priceAt(w, "port", "a", 4)
	w.Player.DirtyCash = 10_000
	w.Markup = 1.05
	if _, err := w.Buy("portstreet", "a", 20, false, 0.5); !errors.Is(err, ErrElsewhere) {
		t.Fatalf("with nobody running the port: %v", err)
	}
	lt := delegatePort(w)
	if !w.CanBuyIn("port") || w.BuyMarkup("port") != 1.05 || w.BuyMarkup("test") != 1 {
		t.Fatalf("the port is run: can buy %v, markup %v, at home %v", w.CanBuyIn("port"), w.BuyMarkup("port"), w.BuyMarkup("test"))
	}
	if q := w.Quote(w.Supplier("portstreet"), "a", 20, false); q != int(math.Ceil(20*4*1.05)) {
		t.Fatalf("the quote through the lieutenant: %d", q)
	}
	p, err := w.Buy("portstreet", "a", 20, false, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if p.Lieutenant != lt.Name || p.City != "port" || p.Contract || p.Qty != 20 || p.UnitPrice != 4*1.05 || p.Cost != int(math.Ceil(20*4*1.05)) {
		t.Fatalf("the receipt: %+v", p)
	}
	if w.Stock("port", "a") != 20 || w.Stock("test", "a") != 0 || w.Player.DirtyCash != 10_000-p.Cost {
		t.Fatalf("the stash: port %d home %d, cash %d", w.Stock("port", "a"), w.Stock("test", "a"), w.Player.DirtyCash)
	}
	// The connect and the market moved as for the buy by hand: the
	// book, the day's count, the pressure on the price.
	hs, ws := hand.Supplier("portstreet"), w.Supplier("portstreet")
	if ws.BoughtToday != hs.BoughtToday || ws.Bought != hs.Bought || ws.Lots != hs.Lots || ws.Price["a"] != hs.Price["a"] {
		t.Fatalf("the connect through the lieutenant %+v, by hand %+v", *ws, *hs)
	}
	if a, b := w.Product("port", "a"), hand.Product("port", "a"); a.BoughtToday != b.BoughtToday || a.SupplierPrice != b.SupplierPrice {
		t.Fatalf("the market through the lieutenant %+v, by hand %+v", *a, *b)
	}
	if hp.Lieutenant != "" || hp.UnitPrice != 4 || p.Cost != int(math.Ceil(float64(hp.Cost)*1.05)) {
		t.Fatalf("by hand %+v, through the lieutenant %+v", hp, p)
	}
	// The cart returns it: cash, stash, the book and the price exactly
	// as they were.
	before := tally(w, "port", "a")
	refund, err := w.Return("port", "a", 20)
	if err != nil || refund != p.Cost {
		t.Fatalf("returned: %v, %d back, want %d", err, refund, p.Cost)
	}
	if got := tally(w, "port", "a"); got.cash != 10_000 || got.stock != 0 || got.bought != 0 || got.price != 4 || len(w.Buys) != 0 {
		t.Fatalf("after the return: %+v (before %+v)", got, before)
	}
	// On credit: on the connect's book as yours, at the markup under
	// the credit premium.
	ws.CreditDays, ws.CreditRatio, ws.Limit = 7, 1.1, 1_000
	cash := w.Player.DirtyCash
	c, err := w.Buy("portstreet", "a", 10, true, 0)
	want := int(math.Ceil(10 * 4 * 1.05 * 1.1))
	if err != nil || !c.Credit || c.Lieutenant != lt.Name || c.Cost != want || ws.Debt != want || ws.DebtDue != w.Day+7 || w.Player.DirtyCash != cash || w.Booked("port", "a") != 10 {
		t.Fatalf("on credit: %v %+v, debt %d due %d, cash %d", err, c, ws.Debt, ws.DebtDue, w.Player.DirtyCash)
	}
	if _, err := w.ReturnCredit("port", "a", 10); err != nil || ws.Debt != 0 {
		t.Fatalf("off the book: %v, debt %d", err, ws.Debt)
	}
	// Where you stand the markup is not paid, lieutenant or not.
	priceAt(w, "test", "a", 10)
	h, err := w.Buy("street", "a", 10, false, 0)
	if err != nil || h.Lieutenant != "" || h.Cost != 100 {
		t.Fatalf("at home: %v %+v", err, h)
	}
	// Unassigned, the door shuts again.
	if err := w.Unassign(lt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Buy("portstreet", "a", 1, false, 0); !errors.Is(err, ErrElsewhere) {
		t.Fatalf("unassigned: %v", err)
	}
}

// The lieutenant's supply contract (#174) mirrors their standing order:
// DelegatedSupplied reads it only while they run the city, the
// player's own contract wins (StandingSupply, SupplyDue), a contract
// receipt names them where the contract is theirs, and Unassign, Fire
// and a move drop it.
func TestDelegatedSupplyMirrorsTheOrder(t *testing.T) {
	w := twoCityWorld()
	priceAt(w, "port", "a", 4)
	w.Player.DirtyCash = 10_000
	lt := delegatePort(w)
	w.DelegateSupply("port", "a", 30)
	if c, ok := w.DelegatedSupplied("port", "a"); !ok || c.Units != 30 || c.City != "port" || c.Product != "a" {
		t.Fatalf("the lieutenant's contract: %+v %v", c, ok)
	}
	if c, ok := w.StandingSupply("port", "a"); !ok || c.Units != 30 {
		t.Fatalf("standing: %+v %v", c, ok)
	}
	if due := w.SupplyDue("port", "a"); due != 30 {
		t.Fatalf("due %d", due)
	}
	// A contract filled where the player set none is the lieutenant's.
	p, err := w.FillSupply("portstreet", "a", 30, 1.05, 0)
	if err != nil || p.Lieutenant != lt.Name || !p.Contract {
		t.Fatalf("the lieutenant's contract's receipt: %v %+v", err, p)
	}
	// The player's own contract wins.
	if err := w.SetSupply("port", "a", 50); err != nil {
		t.Fatal(err)
	}
	if c, _ := w.StandingSupply("port", "a"); c.Units != 50 || w.SupplyDue("port", "a") != 20 {
		t.Fatalf("yours first: %+v, due %d", c, w.SupplyDue("port", "a"))
	}
	if p, err := w.FillSupply("portstreet", "a", 10, 1.05, 0); err != nil || p.Lieutenant != "" {
		t.Fatalf("your contract's receipt: %v %+v", err, p)
	}
	w.ClearSupply("port", "a")
	// Unassigned: gone, and not read even if it were there.
	if err := w.Unassign(lt.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := w.DelegatedSupplied("port", "a"); ok || w.DelegatedSupply != nil {
		t.Fatalf("after unassign: %+v", w.DelegatedSupply)
	}
	if err := w.Assign(lt.ID, "port"); err != nil {
		t.Fatal(err)
	}
	w.DelegateSupply("port", "a", 30)
	if err := w.Assign(lt.ID, "test"); err != nil {
		t.Fatal(err)
	}
	if _, ok := w.DelegatedSupplied("port", "a"); ok || w.DelegatedSupply != nil {
		t.Fatalf("after a move: %+v", w.DelegatedSupply)
	}
	if err := w.Assign(lt.ID, "port"); err != nil {
		t.Fatal(err)
	}
	w.DelegateSupply("port", "a", 30)
	if _, err := w.Fire(lt.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := w.DelegatedSupplied("port", "a"); ok || w.DelegatedSupply != nil {
		t.Fatalf("after firing: %+v", w.DelegatedSupply)
	}
}
