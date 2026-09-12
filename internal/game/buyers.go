package game

import (
	"errors"
	"fmt"
)

var (
	ErrNoContract       = errors.New("no such contract")
	ErrContractNotOpen  = errors.New("that offer is not on the table")
	ErrContractNotYours = errors.New("you have not taken that contract")
	ErrContractDue      = errors.New("that contract's day has passed")
	ErrContractDone     = errors.New("that contract is delivered")
	ErrLyingLow         = errors.New("you are lying low today")
)

// ContractStatus is where a contract stands.
type ContractStatus int

const (
	ContractOffered   ContractStatus = iota // on the table, until Expires
	ContractAccepted                        // yours to deliver by Due
	ContractDelivered                       // delivered in full
	ContractFailed                          // short at the due day
	ContractExpired                         // the offer lapsed unanswered
	ContractDeclined                        // you turned it down
)

func (s ContractStatus) String() string {
	switch s {
	case ContractOffered:
		return "offered"
	case ContractAccepted:
		return "accepted"
	case ContractDelivered:
		return "delivered"
	case ContractFailed:
		return "failed"
	case ContractExpired:
		return "expired"
	default:
		return "declined"
	}
}

// Contract is a buyer's order (#71): so many units of a product handed
// over in a city by a day, paid at Premium times that city's street price
// on the day of each handoff. It is rendered when offered (the name, the
// pitch and the terms are all here) so a save needs no deck to show it,
// and the market sim resolves it.
type Contract struct {
	ID          int
	Buyer       string // deck id
	Name        string // the buyer as the offer names them
	Pitch       string // what they said, rendered
	City        string
	Product     string
	Units       int
	Delivered   int
	Premium     float64 // multiplier on the street price at delivery
	Penalty     float64 // respect lost if short at the due day
	PenaltyCash float64 // of the undelivered units' street value, taken in cash
	HeatMul     float64 // heat per unit handed over, relative to a street unit
	Street      float64 // the street price the day it was offered: what the bet was against
	Status      ContractStatus
	Since       int // day offered
	Expires     int // last day the offer can be taken
	Accepted    int // day taken; 0 never
	Due         int // last day a delivery can be handed over
	Paid        int // dirty cash it has paid
	Resolved    int // day it was delivered, failed, lapsed or declined; 0 not yet
}

// Owed is what is still to deliver.
func (c Contract) Owed() int { return max(0, c.Units-c.Delivered) }

// Open reports whether the offer can still be taken on day.
func (c Contract) Open(day int) bool { return c.Status == ContractOffered && day <= c.Expires }

// Live reports whether the contract is yours and can still be delivered
// on day.
func (c Contract) Live(day int) bool { return c.Status == ContractAccepted && day <= c.Due }

// DaysLeft is how many days remain to deliver (or to answer, for an
// offer) counting day itself: 1 means today is the last.
func (c Contract) DaysLeft(day int) int {
	if c.Status == ContractOffered {
		return c.Expires - day + 1
	}
	return c.Due - day + 1
}

// Done reports whether the contract is off the table for good.
func (c Contract) Done() bool { return c.Status >= ContractDelivered }

// BuyersState is the deck as it stands in this run: the pacing and who is
// not talking to you.
type BuyersState struct {
	NextID    int
	LastOffer int            // day the last offer came; paces the next
	Drawn     map[string]int // buyer id -> offers made this run
	Blacklist map[string]int // buyer id -> first day they will deal again
}

// Contract returns the contract with id, or nil.
func (w *World) Contract(id int) *Contract {
	for i := range w.Contracts {
		if w.Contracts[i].ID == id {
			return &w.Contracts[i]
		}
	}
	return nil
}

// ContractsIn lists the contracts on the books in a city that are still
// somebody's business: open offers and accepted ones, oldest first.
func (w *World) ContractsIn(city string) []Contract {
	var out []Contract
	for _, c := range w.Contracts {
		if c.City == city && !c.Done() {
			out = append(out, c)
		}
	}
	return out
}

// OfferContract puts a contract on the table: the market sim's, when
// the deck deals one. The world gives it an id and records the day.
// (Offer is the rival's, diplomacy.go.)
func (w *World) OfferContract(c Contract) Contract {
	w.Buyers.NextID++
	c.ID = w.Buyers.NextID
	c.Status = ContractOffered
	if c.Since == 0 {
		c.Since = w.Day
	}
	w.Contracts = append(w.Contracts, c)
	w.Buyers.LastOffer = c.Since
	if w.Buyers.Drawn == nil {
		w.Buyers.Drawn = map[string]int{}
	}
	w.Buyers.Drawn[c.Buyer]++
	return c
}

// OpenOffer reports whether an offer is on the table today.
func (w *World) OpenOffer() bool {
	for _, c := range w.Contracts {
		if c.Open(w.Day) {
			return true
		}
	}
	return false
}

// AcceptContract takes an offer: it is yours to deliver by its due day,
// from today. The market sim reports it in the morning. Named so because
// Accept is the rival's table (diplomacy.go).
func (w *World) AcceptContract(id int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	c := w.Contract(id)
	if c == nil {
		return ErrNoContract
	}
	if !c.Open(w.Day) {
		return ErrContractNotOpen
	}
	c.Status = ContractAccepted
	c.Accepted = w.Day
	return nil
}

// DeclineContract turns an offer down. The buyer does not hold it
// against you.
func (w *World) DeclineContract(id int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	c := w.Contract(id)
	if c == nil {
		return ErrNoContract
	}
	if !c.Open(w.Day) {
		return ErrContractNotOpen
	}
	c.Status = ContractDeclined
	c.Resolved = w.Day
	return nil
}

// Deliver queues a handoff of units against a contract tonight, out of
// the stash in its city. A handoff is what only you can do somewhere:
// you have to be standing in the buyer's city (ErrElsewhere), the stock
// has to be there already (by road, or bought there) and the contract
// still live. Queuing again replaces the earlier figure. The market sim
// hands it over at that city's street price times the premium.
func (w *World) Deliver(id, units int) error {
	if w.Over != nil {
		return ErrGameOver
	}
	c := w.Contract(id)
	if c == nil {
		return ErrNoContract
	}
	switch {
	case c.Status == ContractDelivered:
		return ErrContractDone
	case c.Status != ContractAccepted:
		return ErrContractNotYours
	case w.Day > c.Due:
		return ErrContractDue
	}
	if w.Player.Location != c.City {
		return ErrElsewhere
	}
	if w.Today.LieLow {
		return ErrLyingLow
	}
	if units <= 0 {
		return ErrBadQuantity
	}
	if units > c.Owed() {
		return fmt.Errorf("%s only wants %d more %s", c.Name, c.Owed(), w.ProductName(c.Product))
	}
	if have := w.Stock(c.City, c.Product); units > have {
		return fmt.Errorf("only %d %s in %s", have, w.ProductName(c.Product), w.CityName(c.City))
	}
	if w.Today.Deliveries == nil {
		w.Today.Deliveries = map[int]int{}
	}
	w.Today.Deliveries[id] = units
	return nil
}

// QueuedDelivery is what is queued against a contract tonight.
func (w *World) QueuedDelivery(id int) int { return w.Today.Deliveries[id] }

// Deliverable is how many units you could hand over against a contract
// right now: what it still wants, capped by the stash in its city.
func (w *World) Deliverable(c Contract) int {
	return min(c.Owed(), w.Stock(c.City, c.Product))
}

// ContractsDue counts the contracts you have taken, how many of them
// are due today (their last day) and how many tomorrow, for the
// dashboard's line and the alert.
func (w *World) ContractsDue() (live, today, tomorrow int) {
	for _, c := range w.Contracts {
		if c.Status != ContractAccepted {
			continue
		}
		live++
		switch c.Due {
		case w.Day:
			today++
		case w.Day + 1:
			tomorrow++
		}
	}
	return live, today, tomorrow
}

// TakeCash takes what it can of cost, dirty cash first and clean for the
// rest, and reports what it took: a buyer let down collects what is
// there. The market sim's, for a failed contract's penalty.
func (w *World) TakeCash(cost int) int {
	if cost <= 0 {
		return 0
	}
	took := min(cost, w.Cash())
	dirty := min(took, w.Player.DirtyCash)
	w.Player.DirtyCash -= dirty
	w.Player.CleanCash -= took - dirty
	return took
}
