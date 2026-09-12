package game

import (
	"errors"
	"math"
	"testing"
)

// A buy names its connect (#72): it is refused from another city, from
// a connect who does not deal in the product, past their day, while
// they are frozen or locked, and on credit past what they run you; a
// cash lot costs exactly the lot times the price, a credit lot exactly
// that times the credit ratio, on the book and not out of the till,
// due credit_days on; under the lot the small-lot premium is on it;
// cash and stock are conserved across a credit buy and its return, and
// the pressure and the day's book move as a cash buy moves them.
func TestBuyFromAConnect(t *testing.T) {
	w := twoCityWorld()
	priceAt(w, "test", "a", 10)
	street := w.Supplier("street")
	street.Lot, street.SmallLot, street.Cap, street.Limit = 20, 1.5, 100, 500
	w.Player.DirtyCash = 10_000
	if _, err := w.Buy("nobody", "a", 1, false, 0); !errors.Is(err, ErrNoSupplier) {
		t.Fatalf("a connect that does not exist: %v", err)
	}
	if _, err := w.Buy("portstreet", "a", 1, false, 0); !errors.Is(err, ErrElsewhere) {
		t.Fatalf("a connect in another city: %v", err)
	}
	street.Products = []string{"b"}
	if _, err := w.Buy("street", "a", 1, false, 0); !errors.Is(err, ErrNotSupplied) {
		t.Fatalf("a product they do not deal in: %v", err)
	}
	street.Products = nil
	if _, err := w.Buy("street", "a", 101, false, 0); !errors.Is(err, ErrSupplierCapacity) {
		t.Fatalf("past their day: %v", err)
	}
	street.FrozenUntil = w.Day + 2
	if _, err := w.Buy("street", "a", 1, false, 0); !errors.Is(err, ErrSupplierFrozen) {
		t.Fatalf("frozen: %v", err)
	}
	street.FrozenUntil = 0
	street.UnlockCash = 1_000_000
	if _, err := w.Buy("street", "a", 1, false, 0); !errors.Is(err, ErrSupplierLocked) {
		t.Fatalf("locked: %v", err)
	}
	street.UnlockCash = 0
	if _, err := w.Buy("street", "a", 60, true, 0); !errors.Is(err, ErrCreditLimit) {
		t.Fatalf("past the book: %v", err)
	}
	// A cash lot: lot x price, out of the till.
	p, err := w.Buy("street", "a", 20, false, 0)
	if err != nil || p.Cost != 200 || p.UnitPrice != 10 || p.Credit || p.SmallLot || w.Player.DirtyCash != 9_800 || w.Stock("test", "a") != 20 {
		t.Fatalf("a cash lot: %v %+v, cash %d, stock %d", err, p, w.Player.DirtyCash, w.Stock("test", "a"))
	}
	// Under the lot: the premium.
	p, err = w.Buy("street", "a", 10, false, 0)
	if err != nil || p.Cost != 150 || !p.SmallLot || w.Player.DirtyCash != 9_650 {
		t.Fatalf("under the lot: %v %+v, cash %d", err, p, w.Player.DirtyCash)
	}
	// A credit lot: lot x price x credit_ratio on the book, the till
	// untouched, due in credit_days, and the day's book counting it.
	cash, stock := w.Player.DirtyCash, w.Stock("test", "a")
	p, err = w.Buy("street", "a", 20, true, 0)
	want := int(math.Ceil(20 * 10 * 1.1))
	if err != nil || p.Cost != want || !p.Credit || w.Player.DirtyCash != cash || w.Stock("test", "a") != stock+20 || street.Debt != want || street.DebtDue != w.Day+7 || w.Stats.Credit != want {
		t.Fatalf("a credit lot: %v %+v, cash %d, stock %d, debt %d due %d", err, p, w.Player.DirtyCash, w.Stock("test", "a"), street.Debt, street.DebtDue)
	}
	if street.BoughtToday != 50 || street.Left() != 50 || street.Bought != 50 || math.Abs(street.Lots-2.5) > 1e-9 {
		t.Fatalf("the day's book: %+v", street)
	}
	if w.Bought("test", "a") != 30 || w.Booked("test", "a") != 20 {
		t.Fatalf("receipts: cash %d credit %d", w.Bought("test", "a"), w.Booked("test", "a"))
	}
	// More on the book keeps the first due day.
	if _, err := w.Buy("street", "a", 10, true, 0); err != nil || street.DebtDue != w.Day+7 || street.Debt != want+int(math.Ceil(10*10*1.1*1.5)) {
		t.Fatalf("more on the book: %v, debt %d due %d", err, street.Debt, street.DebtDue)
	}
	// Returning the credit lot takes it off the book and nothing comes
	// into the till; cash and stock are where they were.
	refund, err := w.ReturnCredit("test", "a", 30)
	if err != nil || refund != 0 || street.Debt != 0 || street.DebtDue != 0 || w.Player.DirtyCash != cash || w.Stock("test", "a") != stock || w.Booked("test", "a") != 0 || w.Stats.Credit != 0 {
		t.Fatalf("returning the credit lot: %v refund %d, debt %d due %d, cash %d, stock %d", err, refund, street.Debt, street.DebtDue, w.Player.DirtyCash, w.Stock("test", "a"))
	}
	if street.BoughtToday != 30 || street.Bought != 30 {
		t.Fatalf("the day's book after the return: %+v", street)
	}
	// The pressure moves the connect's price and the market's supplier
	// price with it, and a return walks it back.
	before := street.Price["a"]
	if _, err := w.Buy("street", "a", 20, false, 0.5); err != nil {
		t.Fatal(err)
	}
	if street.Price["a"] <= before || w.Home().Market["a"].SupplierPrice != street.Price["a"] {
		t.Fatalf("pressure: %.4f -> %.4f, market %.4f", before, street.Price["a"], w.Home().Market["a"].SupplierPrice)
	}
	if _, err := w.Return("test", "a", 20); err != nil || math.Abs(street.Price["a"]-before) > 1e-9 {
		t.Fatalf("the return did not walk the price back: %v %.4f", err, street.Price["a"])
	}
}

// The best connect is the cheapest available one, and the market's
// supplier price is its price; with none available it is the cheapest
// that deals in the product at all, so stock is never valued at
// nothing.
func TestBestSupplier(t *testing.T) {
	w := twoCityWorld()
	w.Stats.PeakCash = 5_000
	priceAt(w, "port", "a", 4)
	whole := w.Supplier("wholesaler")
	whole.Price["a"] = 2
	if best := w.BestSupplier("port", "a"); best == nil || best.ID != "wholesaler" || w.SupplierPrice("port", "a") != 2 {
		t.Fatalf("best %+v, price %.2f", best, w.SupplierPrice("port", "a"))
	}
	whole.FrozenUntil = w.Day + 1
	if best := w.BestSupplier("port", "a"); best == nil || best.ID != "portstreet" || w.SupplierPrice("port", "a") != 4 {
		t.Fatalf("with the wholesaler frozen: best %+v, price %.2f", best, w.SupplierPrice("port", "a"))
	}
	w.Supplier("portstreet").FrozenUntil = w.Day + 1
	if best := w.BestSupplier("port", "a"); best != nil || w.SupplierPrice("port", "a") != 2 {
		t.Fatalf("with both frozen: best %+v, price %.2f (want the cheapest that deals in it)", best, w.SupplierPrice("port", "a"))
	}
	if w.DebtsDue(w.Day) != nil || w.Owed() != 0 {
		t.Fatal("debts from nowhere")
	}
	whole.Debt, whole.DebtDue = 100, w.Day
	if d := w.DebtsDue(w.Day); len(d) != 1 || d[0].ID != "wholesaler" || w.Owed() != 100 {
		t.Fatalf("debts due: %+v owed %d", d, w.Owed())
	}
}

// The connects survive a save (#72): the relationship, the debt and its
// day, the freeze, the prices and the day's book.
func TestSaveKeepsSuppliers(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	w := twoCityWorld()
	street := w.Supplier("street")
	street.Rel, street.Debt, street.DebtDue, street.FrozenUntil, street.BoughtToday, street.Lots, street.Extended, street.Opened = 71.5, 1_234, 9, 12, 30, 4.5, true, true
	if err := Save(2, w); err != nil {
		t.Fatal(err)
	}
	got, err := Load(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Suppliers) != len(w.Suppliers) {
		t.Fatalf("saved %d connects, loaded %d", len(w.Suppliers), len(got.Suppliers))
	}
	s := got.Supplier("street")
	if s == nil || s.Rel != 71.5 || s.Debt != 1_234 || s.DebtDue != 9 || s.FrozenUntil != 12 || s.BoughtToday != 30 || s.Lots != 4.5 || !s.Extended || !s.Opened || s.Price["a"] != street.Price["a"] {
		t.Fatalf("the street connect after a save: %+v", s)
	}
}
