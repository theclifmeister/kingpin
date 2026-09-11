package game

import "testing"

func contractWorld(t *testing.T) (*World, Contract) {
	t.Helper()
	w := NewWorld(1, []StartingCity{
		{ID: "a", Name: "A", Products: []StartingProduct{{ID: "weed", Name: "Weed", Price: 20, Demand: 60}}},
		{ID: "b", Name: "B", Products: []StartingProduct{{ID: "weed", Name: "Weed", Price: 26, Demand: 60}}},
	}, 500, 100)
	w.Day = 10
	c := w.OfferContract(Contract{Buyer: "x", Name: "a tester", City: "a", Product: "weed", Units: 30, Premium: 1.5, HeatMul: 0.4, Since: 10, Expires: 12, Due: 14})
	return w, c
}

// An offer is taken or declined only while it stands; a handoff is queued
// only where you are, only against a contract you hold, only up to what
// it wants and what the stash there holds, and only until its due day.
func TestContractActions(t *testing.T) {
	w, c := contractWorld(t)
	if c.ID != 1 || w.Buyers.NextID != 1 || w.Buyers.LastOffer != 10 || w.Buyers.Drawn["x"] != 1 {
		t.Fatalf("the offer was not recorded: %+v %+v", c, w.Buyers)
	}
	if err := w.Deliver(c.ID, 10); err != ErrContractNotYours {
		t.Fatalf("delivered against an offer: %v", err)
	}
	w.Day = 13
	if err := w.AcceptContract(c.ID); err != ErrContractNotOpen {
		t.Fatalf("took a lapsed offer: %v", err)
	}
	if err := w.DeclineContract(c.ID); err != ErrContractNotOpen {
		t.Fatalf("declined a lapsed offer: %v", err)
	}
	w.Day = 11
	if err := w.AcceptContract(c.ID); err != nil {
		t.Fatal(err)
	}
	if got := w.Contract(c.ID); got.Status != ContractAccepted || got.Accepted != 11 {
		t.Fatalf("not taken: %+v", got)
	}
	if err := w.AcceptContract(c.ID); err != ErrContractNotOpen {
		t.Fatalf("took it twice: %v", err)
	}
	if err := w.Deliver(c.ID, 10); err == nil {
		t.Fatal("delivered from an empty stash")
	}
	w.Stash("a")["weed"] = 40
	if err := w.Deliver(c.ID, 31); err == nil {
		t.Fatal("delivered more than the contract wants")
	}
	if err := w.Deliver(c.ID, 0); err != ErrBadQuantity {
		t.Fatalf("delivered nothing: %v", err)
	}
	w.Player.Location = "b"
	if err := w.Deliver(c.ID, 10); err != ErrElsewhere {
		t.Fatalf("delivered from the other city: %v", err)
	}
	w.Player.Location = "a"
	if err := w.Deliver(c.ID, 10); err != nil {
		t.Fatal(err)
	}
	if err := w.Deliver(c.ID, 25); err != nil {
		t.Fatal(err)
	}
	if q := w.QueuedDelivery(c.ID); q != 25 {
		t.Fatalf("queued %d, want the later 25", q)
	}
	if n := w.Deliverable(*w.Contract(c.ID)); n != 30 {
		t.Fatalf("deliverable %d, want 30", n)
	}
	w.SetLieLow(true)
	if q := w.QueuedDelivery(c.ID); q != 0 {
		t.Fatalf("lying low kept %d queued", q)
	}
	if err := w.Deliver(c.ID, 10); err != ErrLyingLow {
		t.Fatalf("delivered while lying low: %v", err)
	}
	w.SetLieLow(false)
	w.Day = 15
	if err := w.Deliver(c.ID, 10); err != ErrContractDue {
		t.Fatalf("delivered after the due day: %v", err)
	}
	if err := w.Deliver(99, 10); err != ErrNoContract {
		t.Fatalf("delivered against nothing: %v", err)
	}
	w.Over = &Ending{Day: 15, Cause: "test"}
	if err := w.AcceptContract(c.ID); err != ErrGameOver {
		t.Fatalf("acted after the end: %v", err)
	}
}

// The bookkeeping the UI reads: contracts in a city, the ones due, what a
// contract still wants and how long it has.
func TestContractBookkeeping(t *testing.T) {
	w, c := contractWorld(t)
	d := w.OfferContract(Contract{Buyer: "y", Name: "another", City: "b", Product: "weed", Units: 10, Premium: 1.2, Since: 10, Expires: 11, Due: 11})
	if got := w.ContractsIn("a"); len(got) != 1 || got[0].ID != c.ID {
		t.Fatalf("contracts in a: %+v", got)
	}
	if !w.OpenOffer() {
		t.Fatal("no open offer")
	}
	if err := w.AcceptContract(c.ID); err != nil {
		t.Fatal(err)
	}
	if err := w.AcceptContract(d.ID); err != nil {
		t.Fatal(err)
	}
	w.Contract(c.ID).Delivered = 12
	if got := w.Contract(c.ID).Owed(); got != 18 {
		t.Fatalf("owed %d", got)
	}
	if got := w.Contract(c.ID).DaysLeft(10); got != 5 {
		t.Fatalf("days left %d", got)
	}
	if live, today, tomorrow := w.ContractsDue(); live != 2 || today != 0 || tomorrow != 1 {
		t.Fatalf("due: %d %d %d", live, today, tomorrow)
	}
	w.Day = 11
	if live, today, tomorrow := w.ContractsDue(); live != 2 || today != 1 || tomorrow != 0 {
		t.Fatalf("due: %d %d %d", live, today, tomorrow)
	}
	w.Contract(d.ID).Status = ContractFailed
	if got := w.ContractsIn("b"); len(got) != 0 {
		t.Fatalf("a failed contract is still listed: %+v", got)
	}
	if took := w.TakeCash(600); took != 500 || w.Player.DirtyCash != 0 {
		t.Fatalf("took %d, dirty %d", took, w.Player.DirtyCash)
	}
	w.Player.CleanCash = 50
	if took := w.TakeCash(20); took != 20 || w.Player.CleanCash != 30 {
		t.Fatalf("took %d, clean %d", took, w.Player.CleanCash)
	}
}
