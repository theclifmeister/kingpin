package engine

import (
	"fmt"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The facts a front end shows but must not work out for itself (#298,
// docs/engine.md): what needs you this morning, the doors ahead, whether
// a day stops a fast-forward, what a price is doing. The engine decides
// them and hands them over as values; the words and the colours are the
// front end's (the TUI's are in docs/copy.md's voice).

// AlertKind is what an alert is about.
type AlertKind string

// The alerts, loudest first: the order Alerts returns them in.
const (
	AlertTalking     AlertKind = "talking"      // somebody on the payroll is talking
	AlertContractDue AlertKind = "contract_due" // Contract due Due (today or tomorrow)
	AlertDebtDue     AlertKind = "debt_due"     // Supplier owed Amount on Due, Have in hand
	AlertHeat        AlertKind = "heat"         // Heat in City at or over the patrol Line
	AlertTaskForce   AlertKind = "task_force"   // a task force formed this morning
	AlertFloat       AlertKind = "float"        // Have dirty under the float, Amount
	AlertWages       AlertKind = "wages"        // Amount in wages tonight, Have dirty
	AlertGate        AlertKind = "gate"         // Gate within reach
	AlertHouseKnown  AlertKind = "house_known"  // the police know about House
	AlertDARace      AlertKind = "da_race"      // the DA race is Days off and taking money
	AlertRetire      AlertKind = "retire"       // Ready, or Days quiet and Amount short
	AlertFavour      AlertKind = "favour"       // the chief owes you one and Level comes tonight
	AlertReign       AlertKind = "reign"        // day Days of the reign, Count crews paying Amount
)

// Alert is one thing that needs you this morning. Key is its identity
// morning to morning: a fast-forward stops on an alert whose key the
// morning before did not have, so a contract's key is the contract and
// the day (it stops once when due tomorrow and once when due today)
// and the rest carry no number that moves. The other fields are the
// kind's, as the constants say; the rest are zero.
type Alert struct {
	Kind AlertKind
	Key  string

	City     string  // heat, da_race: the city's id (heat: where you are)
	Contract int     // contract_due: the contract's id
	Supplier string  // debt_due: the connect's id
	House    string  // house_known: the house's id
	Due      int     // contract_due, debt_due: the day it is due
	Amount   int     // debt_due: the debt; float: the float; wages: the wages; retire: the cash short; reign: the homage a night
	Have     int     // debt_due: the cash in hand; float, wages: the dirty cash
	Heat     float64 // heat: the city's heat
	Line     float64 // heat: the patrol line
	Days     int     // da_race: days to the election; retire: quiet days short; reign: the reign's day
	Count    int     // reign: the crews paying homage
	Ready    bool    // retire: retiring is open now
	Level    string  // favour: the response due tonight
	Gate     *Gate   // gate: the door
}

// Alerts is what needs you this morning, loudest first, in the order
// the AlertKind constants list: the dashboard's and a fast-forward's one
// source. Nil before a run.
func (s *Session) Alerts() []Alert {
	w := s.w
	if w == nil {
		return nil
	}
	here := w.Here()
	var out []Alert
	if w.Heat.Leaks >= 2 {
		out = append(out, Alert{Kind: AlertTalking, Key: "somebody is talking"})
	}
	for _, c := range w.Contracts {
		if c.Status != game.ContractAccepted || c.Due > w.Day+1 {
			continue
		}
		when := "tomorrow"
		if c.Due <= w.Day {
			when = "today"
		}
		out = append(out, Alert{Kind: AlertContractDue, Key: fmt.Sprintf("contract %d due %s", c.ID, when), Contract: c.ID, Due: c.Due})
	}
	for _, sup := range w.Suppliers {
		if sup.Debt <= 0 || sup.DebtDue > w.Day+1 {
			continue
		}
		out = append(out, Alert{Kind: AlertDebtDue, Key: fmt.Sprintf("debt %s due %d", sup.ID, sup.DebtDue), Supplier: sup.ID, Due: sup.DebtDue, Amount: sup.Debt, Have: w.Cash()})
	}
	for _, r := range s.set.Heat.ThresholdsIn(w, here) {
		if r.Level == content.Patrol && here.Heat >= r.Threshold {
			out = append(out, Alert{Kind: AlertHeat, Key: "heat in " + here.Name + " over the patrol line", City: here.ID, Heat: here.Heat, Line: r.Threshold})
		}
	}
	if s.set.Heat.TaskForceForming(w) {
		out = append(out, Alert{Kind: AlertTaskForce, Key: "a task force formed"})
	}
	if fl := s.set.Laundering.Float(w); w.Player.DirtyCash < fl && s.FloatMatters() {
		out = append(out, Alert{Kind: AlertFloat, Key: "dirty cash under the float", Amount: fl, Have: w.Player.DirtyCash})
	}
	if wages := s.set.Crew.Wages(w, w.Crew.Pay); wages > w.Player.DirtyCash {
		out = append(out, Alert{Kind: AlertWages, Key: "wages short", Amount: wages, Have: w.Player.DirtyCash})
	}
	for _, g := range s.NextGates() {
		if g.Near(w) {
			out = append(out, Alert{Kind: AlertGate, Key: "unlock:" + g.Kind + ":" + g.ID, Gate: &g})
		}
	}
	for _, h := range w.Houses {
		if h.Known {
			out = append(out, Alert{Kind: AlertHouseKnown, Key: "known " + h.ID, House: h.ID})
		}
	}
	if next := s.set.Law.NextElection(w); w.Law.CampaignOpen && w.Player.CleanCash > 0 && w.Campaigning(here.ID).Cash == 0 && next > 0 {
		out = append(out, Alert{Kind: AlertDARace, Key: "the DA race is taking money", City: here.ID, Days: max(0, next-w.Day)})
	}
	if off := s.set.Laundering.Offshore(); w.Offshore > 0 && off.RetireCash > 0 {
		out = append(out, Alert{Kind: AlertRetire, Key: "retirement", Ready: s.set.Laundering.CanRetire(w), Days: max(0, off.RetireDays-w.QuietDays), Amount: max(0, off.RetireCash-w.Offshore)})
	}
	if due := s.set.Heat.Due(w); w.CanCallFavour(due != "") {
		out = append(out, Alert{Kind: AlertFavour, Key: "the favour on the " + due, Level: due})
	}
	if w.Reign > 0 {
		crews, homage := w.HomageDeals()
		out = append(out, Alert{Kind: AlertReign, Key: "the city is yours", Days: w.ReignDay(), Count: crews, Amount: homage})
	}
	return out
}

// FloatMatters is whether anything reads the laundering float: a front
// to wash with or a route with its dial on. A new run's $500 against a
// $50,000 float is nobody's business until then.
func (s *Session) FloatMatters() bool {
	if len(s.w.Fronts) > 0 {
		return true
	}
	for _, r := range s.w.Routes {
		if r.Dial != events.RouteOff {
			return true
		}
	}
	return false
}

// ---- the gates (#148)

// GateNear is how close a gate must be before it is an alert: the
// distance to go under this share of the line.
const GateNear = 0.5

// Gate is one door ahead, kept behind a line of peak cash: a product on
// the ladder, a front's offer, a connect, an asset (on peak clean cash,
// #48).
type Gate struct {
	Kind  string // product | front | connect | asset
	ID    string
	Name  string
	Line  int    // the peak it opens at: clean cash for an asset, else cash
	Vouch string // a connect's second condition, `Cass at 60`, or ""
	Clean bool   // the line is on peak clean cash (an asset's)
}

// Peak is the peak the gate reads against: clean for an asset, else
// cash.
func (g Gate) Peak(w *game.World) int {
	if g.Clean {
		return w.Stats.PeakClean
	}
	return w.Stats.PeakCash
}

// ToGo is the distance to the line.
func (g Gate) ToGo(w *game.World) int { return g.Line - g.Peak(w) }

// Near reports whether the gate is within reach: under GateNear of its
// line still to go.
func (g Gate) Near(w *game.World) bool {
	return float64(g.ToGo(w)) < float64(g.Line)*GateNear
}

// GatesAhead is every cash gate above the peak, nearest first (file
// order within a line): the products not yet listed where you stand,
// the fronts not owned and still locked, the connects whose door is
// shut, the assets not owned or found and still locked.
func (s *Session) GatesAhead() []Gate {
	w := s.w
	peak := w.Stats.PeakCash
	var out []Gate
	for _, p := range s.cfg.Market.Products {
		if p.UnlockCash > peak && w.Product(w.Here().ID, p.ID) == nil {
			out = append(out, Gate{Kind: "product", ID: p.ID, Name: p.Name, Line: p.UnlockCash})
		}
	}
	for _, o := range s.FrontOffers() {
		if o.UnlockCash > peak {
			out = append(out, Gate{Kind: "front", ID: o.ID, Name: o.Name, Line: o.UnlockCash})
		}
	}
	for i := range w.Suppliers {
		sup := &w.Suppliers[i]
		if sup.Opened || sup.UnlockCash <= peak {
			continue
		}
		g := Gate{Kind: "connect", ID: sup.ID, Name: sup.Name, Line: sup.UnlockCash}
		if st := w.StreetSupplier(sup.City); sup.UnlockRel > 0 && st != nil && st.ID != sup.ID {
			g.Vouch = fmt.Sprintf("%s at %.0f", st.Name, sup.UnlockRel)
		}
		out = append(out, g)
	}
	for _, o := range s.AssetOffers() {
		if o.UnlockCash > w.Stats.PeakClean {
			out = append(out, Gate{Kind: "asset", ID: o.ID, Name: o.Name, Line: o.UnlockCash, Clean: true})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out
}

// NextGates are the gates at the nearest line ahead.
func (s *Session) NextGates() []Gate {
	all := s.GatesAhead()
	var out []Gate
	for _, g := range all {
		if g.Line != all[0].Line {
			break
		}
		out = append(out, g)
	}
	return out
}

// FrontOffers is the fronts on offer you do not own, locked or not.
func (s *Session) FrontOffers() []game.FrontOffer {
	var out []game.FrontOffer
	for _, o := range s.set.Laundering.Offers() {
		if s.w.Front(o.ID) == nil {
			out = append(out, o)
		}
	}
	return out
}

// AssetOffers is the assets on offer you do not own and the task force
// has not found (#48), locked or not.
func (s *Session) AssetOffers() []game.AssetOffer {
	var out []game.AssetOffer
	for _, o := range s.set.Laundering.AssetOffers() {
		if s.w.HasAsset(o.ID) {
			continue
		}
		if lost := s.w.AssetLost(o.ID); lost != nil && lost.Why == "found" {
			continue
		}
		out = append(out, o)
	}
	return out
}
