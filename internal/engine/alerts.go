package engine

import (
	"fmt"
	"math"
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
	AlertCrewLine    AlertKind = "crew_line"    // Member is Gap over the Cross line (Line), Days at tonight's drift
	AlertSkim        AlertKind = "skim"         // skimming suspected: money went missing on Day
	AlertUnposted    AlertKind = "unposted"     // Member (a runner or an enforcer) has no post; Corner in City is one to put them on, or ""
	AlertIdleCorner  AlertKind = "idle_corner"  // nobody works Corner in City: back to the street in Days
	AlertStashFull   AlertKind = "stash_full"   // the stash in City holds Count of its Amount, at or over houses.toml's full_share
	AlertGate        AlertKind = "gate"         // Gate within reach
	AlertHouseKnown  AlertKind = "house_known"  // the police know about House
	AlertDARace      AlertKind = "da_race"      // the DA race is Days off and taking money
	AlertRetire      AlertKind = "retire"       // Ready, or Days quiet and Amount short
	AlertFavour      AlertKind = "favour"       // the chief owes you one and Level comes tonight
	AlertReign       AlertKind = "reign"        // day Days of the reign, Count crews paying Amount
)

// AlertKinds is every kind, loudest first: the order Alerts returns them
// in.
func AlertKinds() []AlertKind {
	return []AlertKind{AlertTalking, AlertContractDue, AlertDebtDue, AlertHeat, AlertTaskForce, AlertFloat, AlertWages,
		AlertCrewLine, AlertSkim, AlertUnposted, AlertIdleCorner, AlertStashFull, AlertGate, AlertHouseKnown,
		AlertDARace, AlertRetire, AlertFavour, AlertReign}
}

// Act is what answers an alert (#352): the screen that fixes it, the
// dialog it opens there, and which of the alert's own fields names what
// it opens on. The engine names it and each front end maps it onto its
// own screens and dialogs; one that lacks the screen shows the alert's
// words alone. An act opens nothing that spends: a dialog that takes
// money stays a key on the screen it lands on.
type Act struct {
	Screen  string `json:"screen"`            // a Screen* name
	Mode    string `json:"mode,omitempty"`    // a Mode* name, or "" for the screen alone
	Subject string `json:"subject,omitempty"` // a Subject* name: the alert's field that holds the id, or "" for none
}

// The screens an act lands on, by the TUI's names for its tabs.
const (
	ScreenDashboard = "dashboard"
	ScreenMarket    = "market"
	ScreenCrew      = "crew"
	ScreenMap       = "map"
	ScreenLedger    = "ledger"
)

// ModePost is the one dialog an act opens: the post picker on the
// alert's Corner.
const ModePost = "post"

// The subjects an act opens on, each the alert's field of that name.
const (
	SubjectMember   = "member"
	SubjectCorner   = "corner"
	SubjectContract = "contract"
	SubjectSupplier = "supplier"
	SubjectHouse    = "house"
	SubjectCity     = "city"
)

// The acts more than one kind carries.
var (
	actDashboard = Act{Screen: ScreenDashboard}
	actMarket    = Act{Screen: ScreenMarket}
	actCrew      = Act{Screen: ScreenCrew}
	actLedger    = Act{Screen: ScreenLedger}
	actMember    = Act{Screen: ScreenCrew, Subject: SubjectMember}
	actPost      = Act{Screen: ScreenMap, Mode: ModePost, Subject: SubjectMember}
	actCorner    = Act{Screen: ScreenMap, Mode: ModePost, Subject: SubjectCorner}
)

// alertActs is every act an alert of each kind can carry, the usual one
// first: a gate's is its door's screen (the market for a product or a
// connect, the ledger for a front or an asset), an unposted member's the
// post picker while there is a corner to put them on and else their
// row, and retirement's the ledger's account until the walk away is
// open on the dashboard.
var alertActs = map[AlertKind][]Act{
	AlertTalking:     {actCrew},
	AlertContractDue: {{Screen: ScreenMarket, Subject: SubjectContract}},
	AlertDebtDue:     {{Screen: ScreenMarket, Subject: SubjectSupplier}},
	AlertHeat:        {actDashboard},
	AlertTaskForce:   {actDashboard},
	AlertFloat:       {actLedger},
	AlertWages:       {actCrew},
	AlertCrewLine:    {actMember},
	AlertSkim:        {actCrew},
	AlertUnposted:    {actPost, actMember},
	AlertIdleCorner:  {actCorner},
	AlertStashFull:   {{Screen: ScreenLedger, Subject: SubjectCity}},
	AlertGate:        {actMarket, actLedger},
	AlertHouseKnown:  {{Screen: ScreenLedger, Subject: SubjectHouse}},
	AlertDARace:      {actLedger},
	AlertRetire:      {actLedger, actDashboard},
	AlertFavour:      {actLedger},
	AlertReign:       {actDashboard},
}

// ActsOf is every act an alert of the kind can carry, the usual one
// first.
func ActsOf(kind AlertKind) []Act { return alertActs[kind] }

// Alert is one thing that needs you this morning. Key is its identity
// morning to morning: a fast-forward stops on an alert whose key the
// morning before did not have, so a contract's key is the contract and
// the day (it stops once when due tomorrow and once when due today)
// and the rest carry no number that moves. The other fields are the
// kind's, as the constants say; the rest are zero.
type Alert struct {
	Kind AlertKind `json:"kind"`
	Key  string    `json:"key"`

	City     string  `json:"city,omitempty"`     // heat, da_race, idle_corner, unposted, stash_full: the city's id (heat: where you are; unposted: the corner's)
	Contract int     `json:"contract,omitempty"` // contract_due: the contract's id
	Supplier string  `json:"supplier,omitempty"` // debt_due: the connect's id
	House    string  `json:"house,omitempty"`    // house_known: the house's id
	Due      int     `json:"due,omitempty"`      // contract_due, debt_due: the day it is due
	Amount   int     `json:"amount,omitempty"`   // debt_due: the debt; float: the float; wages: the wages; retire: the cash short; reign: the homage a night; stash_full: the capacity
	Have     int     `json:"have,omitempty"`     // debt_due: the cash in hand; float, wages: the dirty cash
	Heat     float64 `json:"heat,omitempty"`     // heat: the city's heat
	Line     float64 `json:"line,omitempty"`     // heat: the patrol line; crew_line: the loyalty line
	Days     int     `json:"days,omitempty"`     // da_race: days to the election; retire: quiet days short; reign: the reign's day; crew_line: days to the line at tonight's drift (0: not falling); idle_corner: days before it drifts
	Count    int     `json:"count,omitempty"`    // reign: the crews paying homage; stash_full: the units held
	Ready    bool    `json:"ready,omitempty"`    // retire: retiring is open now
	Level    string  `json:"level,omitempty"`    // favour: the response due tonight
	Member   int     `json:"member,omitempty"`   // crew_line, unposted: the member's id
	Cross    string  `json:"cross,omitempty"`    // crew_line: the line ahead: skim, flip (a lieutenant's) or walk
	Gap      float64 `json:"gap,omitempty"`      // crew_line: the loyalty over the line
	Corner   string  `json:"corner,omitempty"`   // idle_corner: the corner's id; unposted: a corner to post them on, or ""
	Day      int     `json:"day,omitempty"`      // skim: the day money last went missing
	Gate     *Gate   `json:"gate,omitempty"`     // gate: the door

	Act Act `json:"act"` // what answers it (#352), one of ActsOf(Kind)
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
	out = append(out, s.crewLines()...)
	if tun := s.set.Crew.Tuning(); w.Crew.LastSkim > 0 && w.Day-w.Crew.LastSkim < tun.SuspectDays {
		out = append(out, Alert{Kind: AlertSkim, Key: "skimming suspected", Day: w.Crew.LastSkim})
	}
	out = append(out, s.unposted()...)
	out = append(out, s.idleCorners()...)
	out = append(out, s.stashesFull()...)
	for _, g := range s.NextGates() {
		if g.Near(w) {
			act := actMarket // a product or a connect
			if g.Kind == "front" || g.Kind == "asset" {
				act = actLedger
			}
			out = append(out, Alert{Kind: AlertGate, Key: "unlock:" + g.Kind + ":" + g.ID, Gate: &g, Act: act})
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
		a := Alert{Kind: AlertRetire, Key: "retirement", Ready: s.set.Laundering.CanRetire(w), Days: max(0, off.RetireDays-w.QuietDays), Amount: max(0, off.RetireCash-w.Offshore)}
		if a.Ready {
			a.Act = actDashboard // the walk away
		}
		out = append(out, a)
	}
	if due := s.set.Heat.Due(w); w.CanCallFavour(due != "") {
		out = append(out, Alert{Kind: AlertFavour, Key: "the favour on the " + due, Level: due})
	}
	if w.Reign > 0 {
		crews, homage := w.HomageDeals()
		out = append(out, Alert{Kind: AlertReign, Key: "the city is yours", Days: w.ReignDay(), Count: crews, Amount: homage})
	}
	for i := range out {
		if out[i].Act.Screen == "" {
			out[i].Act = alertActs[out[i].Kind][0]
		}
	}
	return out
}

// unposted are the runners and enforcers on the payroll with no post
// this morning (#352), in roster order: fit (not in a cell, laid up or
// under cover), on no corner and, an enforcer, in no house. Each names
// a corner to put them on, so the act is the post picker there: a
// runner the first corner you hold that nobody works (where you stand
// first), else the first free one where you stand; an enforcer the
// first corner of yours with nobody guarding it, a worked one first.
// With none, the act is their row on the crew screen. Keyed by the
// member, so a fast-forward stops once as they come off a post.
func (s *Session) unposted() []Alert {
	w := s.w
	var out []Alert
	for _, m := range w.Crew.Members {
		if (m.Role != game.RoleRunner && m.Role != game.RoleEnforcer) || !m.Fit(w.Day) || w.PostOf(m.ID) != nil || w.GuardOf(m.ID) != nil {
			continue
		}
		a := Alert{Kind: AlertUnposted, Key: fmt.Sprintf("unposted %d", m.ID), Member: m.ID, Act: actMember}
		if c := s.postFor(m.Role); c != nil {
			a.Corner, a.City, a.Act = c.ID, c.City, actPost
		}
		out = append(out, a)
	}
	return out
}

// postFor is the corner an unposted member of the role would go to, or
// nil.
func (s *Session) postFor(role string) *game.Corner {
	w := s.w
	here := w.Here().ID
	// The corners where you stand, then the rest, in city order.
	var cs []game.Corner
	cs = append(cs, w.Cities[here].Corners...)
	for _, c := range w.Corners() {
		if c.City != here {
			cs = append(cs, c)
		}
	}
	pick := func(ok func(game.Corner) bool) *game.Corner {
		for i := range cs {
			if ok(cs[i]) {
				return &cs[i]
			}
		}
		return nil
	}
	if role == game.RoleEnforcer {
		if c := pick(func(c game.Corner) bool { return c.Worked() && c.Enforcer == 0 }); c != nil {
			return c
		}
		return pick(func(c game.Corner) bool { return c.Held() && c.Enforcer == 0 })
	}
	if c := pick(func(c game.Corner) bool { return c.Held() && !c.Worked() }); c != nil {
		return c
	}
	return pick(func(c game.Corner) bool { return c.City == here && c.Owner == game.OwnerNone })
}

// stashesFull are the cities whose stash holds at least houses.toml's
// full_share of what the operation can hold there (#352,
// World.Capacity: the street and the houses), in city order: the buy
// that fills it is refused and the next lot lands on a full street.
// Keyed by the city, so a fast-forward stops once as it fills.
func (s *Session) stashesFull() []Alert {
	w := s.w
	share := s.cfg.Houses.Houses.FullShare
	if share <= 0 {
		return nil
	}
	var out []Alert
	for _, cid := range w.CityOrder {
		held, room := w.StockIn(cid), w.Capacity(cid)
		if room <= 0 || held <= 0 || float64(held) < share*float64(room) {
			continue
		}
		out = append(out, Alert{Kind: AlertStashFull, Key: "stash full in " + cid, City: cid, Count: held, Amount: room})
	}
	return out
}

// crewLines are the members near their next loyalty line (#345), in
// roster order: within alert_margin of it, or near enough that
// tonight's drift takes them over. The line ahead is the skim line (a
// lieutenant's flip) while they are over it, then the walk. A member
// has one line ahead, so one alert, keyed by the member and the line:
// a fast-forward stops once as they near the skim line and once more
// as they near the walk. Informants stay silent (docs/snitching.md):
// the informant line is nobody's alert.
func (s *Session) crewLines() []Alert {
	w := s.w
	tun := s.set.Crew.Tuning()
	if tun.AlertMargin <= 0 {
		return nil
	}
	var out []Alert
	for _, m := range w.Crew.Members {
		cross, line := "skim", tun.SkimThreshold
		if m.Lieutenant() {
			cross, line = "flip", s.set.Crew.FlipLine()
		}
		if m.Loyalty < line {
			cross, line = "walk", tun.QuitThreshold
		}
		gap := m.Loyalty - line
		days := 0
		if d := s.set.Crew.Drift(w, m); d < 0 {
			days = max(1, int(math.Ceil(gap/-d)))
		}
		if gap > tun.AlertMargin && days != 1 {
			continue
		}
		out = append(out, Alert{Kind: AlertCrewLine, Key: fmt.Sprintf("crew %d near the %s line", m.ID, cross),
			Member: m.ID, Cross: cross, Line: line, Gap: gap, Days: days})
	}
	return out
}

// idleCorners are the corners you hold that nobody works this morning
// (#345), in city order, with the days before they drift back to the
// street (territory's drift_days, the tree's bonus in). Keyed by the
// corner, so a fast-forward stops once as it goes idle; the night it
// drifts stops on the CornerLost.
func (s *Session) idleCorners() []Alert {
	w := s.w
	drift := s.set.Territory.DriftDays(w)
	if drift <= 0 {
		return nil
	}
	var out []Alert
	for _, c := range w.Corners() {
		if !c.Held() || c.Worked() {
			continue
		}
		out = append(out, Alert{Kind: AlertIdleCorner, Key: "idle corner " + c.ID, Corner: c.ID, City: c.City, Days: max(1, drift-c.Idle)})
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
	Kind  string `json:"kind"` // product | front | connect | asset
	ID    string `json:"id"`
	Name  string `json:"name"`
	Line  int    `json:"line"`            // the peak it opens at: clean cash for an asset, else cash
	Vouch string `json:"vouch,omitempty"` // a connect's second condition, `Cass at 60`, or ""
	Clean bool   `json:"clean,omitempty"` // the line is on peak clean cash (an asset's)
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
