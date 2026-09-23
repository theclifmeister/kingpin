package engine

import (
	"reflect"
	"sort"

	"github.com/theclifmeister/kingpin/internal/game"
)

// ViewVersion is the shape of View (#299, docs/engine.md). It moves
// when a field is added, renamed, retyped or dropped, never with the
// save's game.SchemaVersion: the world is free to change shape, the view
// is the contract a front end in another process is written against.
// TestViewShapeIsPinned fails on a shape change that keeps the number.
const ViewVersion = 8

// View is a snapshot of what the player can see: what a front end draws
// (#299). It is built from the world the way the TUI reads it and holds
// no pointer into it, so it can be kept, compared and sent as JSON. What
// the player does not know is not in it: a faction's temper and heads,
// the chief's temper and a route's risk are the intel file's
// (game.Known), never the truth (TestViewReadsTheFile), and who on the
// payroll is talking is not in it at all.
type View struct {
	Version   int            `json:"version"`
	Seed      uint64         `json:"seed"`
	Day       int            `json:"day"`
	Over      *EndingView    `json:"over,omitempty"`
	You       YouView        `json:"you"`
	Cities    []CityView     `json:"cities"`
	Crew      []MemberView   `json:"crew"`
	Pool      []MemberView   `json:"pool"` // looking for work (#332): hire by id
	Contracts []ContractView `json:"contracts"`
	Offers    []OfferView    `json:"offers"`
	Upgrades  []UpgradeView  `json:"upgrades"`
	Routes    []RouteView    `json:"routes"`
	Shipments []ShipmentView `json:"shipments"`
	Connects  []ConnectView  `json:"connects"`
	Houses    []HouseView    `json:"houses"`
	Fronts    []FrontView    `json:"fronts"`
	Factions  []FactionView  `json:"factions"`
	Law       LawView        `json:"law"`
	Card      *CardView      `json:"card,omitempty"`
	Report    ReportView     `json:"report"`
	Alerts    []Alert        `json:"alerts"`
}

// EndingView is how the run ended.
type EndingView struct {
	Day   int    `json:"day"`
	Cause string `json:"cause"` // one of content.Causes
	Who   string `json:"who,omitempty"`
}

// YouView is the player.
type YouView struct {
	City      string                    `json:"city"`
	DirtyCash int                       `json:"dirty_cash"`
	CleanCash int                       `json:"clean_cash"`
	Offshore  int                       `json:"offshore"`
	NetWorth  int                       `json:"net_worth"`
	PeakCash  int                       `json:"peak_cash"`
	LieLow    bool                      `json:"lie_low"`
	Tier      int                       `json:"tier"`
	TierName  string                    `json:"tier_name"`
	Pay       string                    `json:"pay"`
	Launder   string                    `json:"launder"`
	Evidence  int                       `json:"evidence"`
	Fear      float64                   `json:"fear"`
	Respect   float64                   `json:"respect"`
	Notoriety float64                   `json:"notoriety"`
	Stock     map[string]map[string]int `json:"stock"` // city id -> product id -> units on the street
	Upgrades  []string                  `json:"upgrades"`
	QuietDays int                       `json:"quiet_days"`
	Character string                    `json:"character,omitempty"`
	HardDA    bool                      `json:"hard_da,omitempty"`
}

// CityView is a city: its heat, its law and its market and corners.
type CityView struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Heat     float64       `json:"heat"`
	Pressure float64       `json:"pressure"`
	Goodwill float64       `json:"goodwill"`
	Response string        `json:"response,omitempty"`     // the police's next rung, as the file knows it
	Due      int           `json:"response_day,omitempty"` // ... the first day it can fire
	Products []ProductView `json:"products"`
	Corners  []CornerView  `json:"corners"`
}

// ProductView is a product's market in a city.
type ProductView struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Price         float64    `json:"price"`
	SupplierPrice float64    `json:"supplier_price"`
	Demand        float64    `json:"demand"`
	NoSupply      bool       `json:"no_supply,omitempty"`
	ShockDays     int        `json:"shock_days,omitempty"`
	Slump         bool       `json:"slump,omitempty"`
	History       []float64  `json:"history"`
	Facts         PriceFacts `json:"facts"`
}

// CornerView is a corner on the map.
type CornerView struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	X        int     `json:"x"`
	Y        int     `json:"y"`
	Demand   float64 `json:"demand"`
	Owner    string  `json:"owner"`             // none, player, rival
	Faction  string  `json:"faction,omitempty"` // who holds it while Owner is rival
	Runner   int     `json:"runner,omitempty"`  // crew id, game.You for you, 0 nobody
	Enforcer int     `json:"enforcer,omitempty"`
	Since    int     `json:"since"`
	Deed     bool    `json:"deed,omitempty"`
}

// MemberView is someone on the payroll. A lieutenant's personality is
// here once the report has named it (Observed); an informant is not
// marked.
type MemberView struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Role        string  `json:"role"`
	Age         int     `json:"age"`
	Skill       int     `json:"skill"`
	Loyalty     float64 `json:"loyalty"`
	Wage        int     `json:"wage"`
	Hired       int     `json:"hired"`
	City        string  `json:"city,omitempty"` // the city a lieutenant runs
	Post        string  `json:"post,omitempty"` // the corner they work or guard
	Jailed      bool    `json:"jailed,omitempty"`
	Personality string  `json:"personality,omitempty"`
	Carry       int     `json:"carry"`         // the sell capacity they add
	Fee         int     `json:"fee,omitempty"` // in the pool: what hiring them costs, dirty cash
}

// ContractView is a buyer's contract still somebody's business (#71,
// #332): on offer, or taken and owed.
type ContractView struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"` // the buyer, as the offer names them
	Pitch       string  `json:"pitch"`
	City        string  `json:"city"`
	Product     string  `json:"product"`
	Units       int     `json:"units"`
	Delivered   int     `json:"delivered"`
	Premium     float64 `json:"premium"`      // on the street price at delivery
	Street      float64 `json:"street"`       // the street price the day it was offered
	Penalty     float64 `json:"penalty"`      // respect lost if short at the due day
	PenaltyCash float64 `json:"penalty_cash"` // of the short units' street value
	Status      string  `json:"status"`       // offered, accepted
	Expires     int     `json:"expires"`      // the last day the offer can be taken
	Due         int     `json:"due"`          // the last day a delivery can be handed over
}

// OfferView is a deal a faction has put on the table (#32, #332).
type OfferView struct {
	ID      int       `json:"id"`
	Faction string    `json:"faction"`
	Kind    string    `json:"kind"`
	Terms   TermsView `json:"terms"`
	Expires int       `json:"expires"`
}

// TermsView is a deal's terms, as the commands spell them.
type TermsView struct {
	Days    int      `json:"days,omitempty"`
	PerDay  int      `json:"per_day,omitempty"`
	Corners []string `json:"corners,omitempty"`
	Route   string   `json:"route,omitempty"`
	Units   int      `json:"units,omitempty"`
}

// UpgradeView is a node of the upgrade tree and where it stands for
// you (#332): owned, available (its prerequisites owned) or locked.
type UpgradeView struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Branch   string   `json:"branch"`
	Desc     string   `json:"desc"`
	Cost     int      `json:"cost"`
	Clean    bool     `json:"clean,omitempty"` // paid in clean cash, not dirty
	Requires []string `json:"requires"`
	State    string   `json:"state"`
}

// RouteView is a route open to you, with its dial and what the file
// says of its risk.
type RouteView struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Mode      string  `json:"mode"`
	From      string  `json:"from"`
	To        string  `json:"to"`
	Dial      string  `json:"dial"`
	Closed    bool    `json:"closed,omitempty"`
	Driver    int     `json:"driver,omitempty"`
	Risk      float64 `json:"risk,omitempty"` // a day in transit, as the file knows it
	RiskKnown bool    `json:"risk_known,omitempty"`
}

// ShipmentView is product on the road.
type ShipmentView struct {
	ID      int    `json:"id"`
	Route   string `json:"route"`
	From    string `json:"from"`
	To      string `json:"to"`
	Product string `json:"product"`
	Units   int    `json:"units"`
	Sent    int    `json:"sent"`
	Arrives int    `json:"arrives"`
}

// ConnectView is a supplier (#72): who sells what where, today's
// prices for what they will sell you now, and where you stand with them.
type ConnectView struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	City       string             `json:"city"`
	Wholesale  bool               `json:"wholesale,omitempty"`
	Temper     string             `json:"temper"`
	Open       bool               `json:"open"`            // the door is open to you
	Owned      bool               `json:"owned,omitempty"` // the connect is yours (#48)
	Prices     map[string]float64 `json:"prices"`          // product id -> per unit today, for what they sell you now
	Cap        int                `json:"cap"`             // units they can get you today
	Lot        int                `json:"lot"`
	CreditDays int                `json:"credit_days,omitempty"`
	Limit      int                `json:"limit,omitempty"` // credit they will run you to today
	Rel        float64            `json:"rel"`
	Debt       int                `json:"debt,omitempty"`
	DebtDue    int                `json:"debt_due,omitempty"`
}

// HouseView is a stash house.
type HouseView struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	City     string         `json:"city"`
	Corner   string         `json:"corner"`
	Capacity int            `json:"capacity"`
	Stock    map[string]int `json:"stock"`
	Guard    int            `json:"guard,omitempty"`
	Known    bool           `json:"known,omitempty"` // the police have it in the file
}

// FrontView is a front you own.
type FrontView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Level  int    `json:"level"`
	Frozen bool   `json:"frozen,omitempty"`
	Washed int    `json:"washed"`
}

// FactionView is a faction at the table as the player knows it: what
// anyone can see (who leads it, when it arrived, the corners it holds,
// the trust and the war between you) and what the file holds on the
// rest.
type FactionView struct {
	ID          string     `json:"id"`
	Leader      string     `json:"leader"`
	Alive       bool       `json:"alive"`
	Arrived     int        `json:"arrived"`
	Corners     int        `json:"corners"`
	Trust       float64    `json:"trust"`
	War         float64    `json:"war"`
	Personality string     `json:"personality"` // game.Unknown ("?") until the file has it
	MuscleLo    int        `json:"muscle_lo,omitempty"`
	MuscleHi    int        `json:"muscle_hi,omitempty"`
	MuscleKnown bool       `json:"muscle_known,omitempty"`
	Books       *BooksView `json:"books,omitempty"` // the last read of its books
	Move        string     `json:"move,omitempty"`  // the corner the file says it moves on next
	Deals       []string   `json:"deals,omitempty"` // the kinds of the deals live with it
}

// BooksView is a read of a faction's books (#70, #45).
type BooksView struct {
	Day    int `json:"day"`
	Cash   int `json:"cash"`
	Income int `json:"income"`
	Muscle int `json:"muscle"`
	Wages  int `json:"wages"`
}

// LawView is the chief and the DA.
type LawView struct {
	Chief        string `json:"chief"`
	ChiefTemper  string `json:"chief_temper"` // game.Unknown until the file has it
	DA           string `json:"da"`
	DAStance     string `json:"da_stance"`
	NextElection int    `json:"next_election,omitempty"`
}

// CardView is the dilemma card waiting for an answer.
type CardView struct {
	ID      string       `json:"id"`
	Title   string       `json:"title"`
	Text    string       `json:"text"`
	Choices []ChoiceView `json:"choices"`
}

// ChoiceView is one answer on the card: its label and what it does
// (#358, ChoiceChips), the chips the TUI draws under the label.
type ChoiceView struct {
	Label   string `json:"label"`
	Preview []Chip `json:"preview"`
}

// ReportView is the morning report, its sections in the order the TUI
// shows them, each a list of lines in words.
type ReportView struct {
	Day        int      `json:"day"`
	Incident   []string `json:"incident,omitempty"`
	Unlocked   []string `json:"unlocked,omitempty"`
	Tier       []string `json:"tier,omitempty"`
	Prices     []string `json:"prices,omitempty"`
	Sales      []string `json:"sales,omitempty"`
	Heat       []string `json:"heat,omitempty"`
	Crew       []string `json:"crew,omitempty"`
	Territory  []string `json:"territory,omitempty"`
	Shipments  []string `json:"shipments,omitempty"`
	Law        []string `json:"law,omitempty"`
	Intel      []string `json:"intel,omitempty"`
	Money      []string `json:"money,omitempty"`
	Upgrades   []string `json:"upgrades,omitempty"`
	News       []string `json:"news,omitempty"`
	CashBefore int      `json:"cash_before"`
	CashAfter  int      `json:"cash_after"`
	Flow       FlowView `json:"flow"` // the night's cash flow (#351): drawn in place of the money lines
}

// FlowView is the night's cash flow (#351): the piles the day opened
// on, one line a category in the order the money moves, and the piles
// it closed on. Opening plus the lines is the closing, dirty and clean
// each.
type FlowView struct {
	Opening PoolsView      `json:"opening"`
	Lines   []FlowLineView `json:"lines"`
	Closing PoolsView      `json:"closing"`
	Net     int            `json:"net"` // closing less opening, both piles
}

// PoolsView is cash in each pile.
type PoolsView struct {
	Dirty int `json:"dirty"`
	Clean int `json:"clean"`
}

// FlowLineView is one category of the flow: its id (game.FlowCats), the
// words for it, the signed amounts by pile, and whether it moved more
// than headlines.toml [flow] big_share of the opening (the line to look
// at first).
type FlowLineView struct {
	Cat   string `json:"cat"`
	Label string `json:"label"`
	Dirty int    `json:"dirty"`
	Clean int    `json:"clean"`
	Big   bool   `json:"big"`
}

// View is the run as the player sees it this morning. Before a run it
// is the zero View at the current version.
func (s *Session) View() View {
	w := s.w
	v := View{Version: ViewVersion}
	if w == nil {
		noNulls(reflect.ValueOf(&v).Elem())
		return v
	}
	known := game.Known(w)
	v.Seed, v.Day = w.Seed, w.Day
	if w.Over != nil {
		v.Over = &EndingView{Day: w.Over.Day, Cause: w.Over.Cause, Who: w.Over.Who}
	}
	v.You = YouView{
		City:      w.Player.Location,
		DirtyCash: w.Player.DirtyCash,
		CleanCash: w.Player.CleanCash,
		Offshore:  w.Offshore,
		NetWorth:  w.NetWorth(),
		PeakCash:  w.Stats.PeakCash,
		LieLow:    w.Today.LieLow,
		Tier:      w.Tier(),
		TierName:  w.TierName(s.cfg.Progression),
		Pay:       w.Crew.Pay.String(),
		Launder:   w.Laundering.Dial.String(),
		Evidence:  w.Heat.Evidence,
		Fear:      w.Player.Reputation.Fear,
		Respect:   w.Player.Reputation.Respect,
		Notoriety: w.Player.Reputation.Notoriety,
		QuietDays: w.QuietDays,
		Character: w.Start.Character,
		HardDA:    w.Start.HardDA,
	}
	for _, id := range sortedKeys(w.Upgrades) {
		if w.Upgrades[id] {
			v.You.Upgrades = append(v.You.Upgrades, id)
		}
	}
	street := map[string]map[string]int{} // city -> product -> units, built here so the view shares no map with the world
	for _, cid := range w.CityOrder {
		c := w.Cities[cid]
		stock := map[string]int{}
		for _, pid := range w.Products {
			if n := w.Stock(cid, pid); n > 0 {
				stock[pid] = n
			}
		}
		street[cid] = stock
		cv := CityView{ID: c.ID, Name: c.Name, Heat: c.Heat, Pressure: c.Pressure, Goodwill: c.Goodwill}
		if level, day, ok := known.Response(cid); ok {
			cv.Response, cv.Due = level, day
		}
		for _, pid := range w.Products {
			p := c.Market[pid]
			if p == nil {
				continue
			}
			cv.Products = append(cv.Products, ProductView{
				ID: pid, Name: p.Name, Price: p.Price, SupplierPrice: p.SupplierPrice, Demand: p.Demand,
				NoSupply: p.NoSupply, ShockDays: p.ShockDays, Slump: p.ShockDays > 0 && p.ShockSlump,
				History: append([]float64(nil), p.History...), Facts: Facts(p),
			})
		}
		for _, k := range c.Corners {
			cv.Corners = append(cv.Corners, CornerView{
				ID: k.ID, Name: k.Name, X: k.X, Y: k.Y, Demand: k.Demand, Owner: k.Owner, Faction: k.Faction,
				Runner: k.Runner, Enforcer: k.Enforcer, Since: k.Since, Deed: k.Deed != nil,
			})
		}
		v.Cities = append(v.Cities, cv)
	}
	v.You.Stock = street
	for _, m := range w.Crew.Candidates {
		v.Pool = append(v.Pool, MemberView{ID: m.ID, Name: m.Name, Role: m.Role, Age: m.Age, Skill: m.Skill, Loyalty: m.Loyalty, Wage: m.Wage, Carry: m.Units, Fee: m.Fee})
	}
	for _, c := range w.Contracts {
		if c.Done() {
			continue
		}
		v.Contracts = append(v.Contracts, ContractView{
			ID: c.ID, Name: c.Name, Pitch: c.Pitch, City: c.City, Product: c.Product, Units: c.Units, Delivered: c.Delivered,
			Premium: c.Premium, Street: c.Street, Penalty: c.Penalty, PenaltyCash: c.PenaltyCash, Status: c.Status.String(), Expires: c.Expires, Due: c.Due,
		})
	}
	for _, o := range w.Offers {
		t := o.Deal.Terms
		v.Offers = append(v.Offers, OfferView{ID: o.ID, Faction: o.With(), Kind: o.Deal.Kind, Expires: o.Expires,
			Terms: TermsView{Days: t.Days, PerDay: t.PerDay, Corners: append([]string(nil), t.Corners...), Route: t.Route, Units: t.Units}})
	}
	for _, u := range s.cfg.Upgrades.Nodes {
		state := "locked"
		switch {
		case w.Owns(u.ID):
			state = "owned"
		case len(w.Missing(u)) == 0:
			state = "available"
		}
		v.Upgrades = append(v.Upgrades, UpgradeView{ID: u.ID, Name: u.Name, Branch: u.Branch, Desc: u.Desc, Cost: u.Cost, Clean: u.Clean, Requires: append([]string(nil), u.Requires...), State: state})
	}
	for _, m := range w.Crew.Members {
		mv := MemberView{ID: m.ID, Name: m.Name, Role: m.Role, Age: m.Age, Skill: m.Skill, Loyalty: m.Loyalty, Wage: m.Wage, Hired: m.Hired, City: m.City, Jailed: m.Jailed(w.Day), Carry: m.Units}
		if c := w.PostOf(m.ID); c != nil {
			mv.Post = c.ID
		}
		if m.Observed {
			mv.Personality = m.Personality
		}
		v.Crew = append(v.Crew, mv)
	}
	seen := map[string]bool{}
	for _, cid := range w.CityOrder {
		for _, r := range s.set.Logistics.RoutesOpen(w, cid) {
			if seen[r.ID] {
				continue
			}
			seen[r.ID] = true
			rs := w.Route(r.ID)
			rv := RouteView{ID: r.ID, Name: r.Name, Mode: r.Mode, From: r.From, To: r.To, Dial: rs.Dial.String(), Closed: rs.Closed(w.Day), Driver: rs.Driver}
			if risk, ok := known.Risk(r.ID); ok {
				rv.Risk, rv.RiskKnown = risk, true
			}
			v.Routes = append(v.Routes, rv)
		}
	}
	for _, sh := range w.Shipments {
		v.Shipments = append(v.Shipments, ShipmentView{ID: sh.ID, Route: sh.Route, From: sh.From, To: sh.To, Product: sh.Product, Units: sh.Units, Sent: sh.Sent, Arrives: sh.Arrives})
	}
	for i := range w.Suppliers {
		sup := &w.Suppliers[i]
		cv := ConnectView{ID: sup.ID, Name: sup.Name, City: sup.City, Wholesale: sup.Wholesale, Temper: sup.Temper, Open: sup.Open(w), Owned: sup.Owned, Prices: map[string]float64{}, Cap: sup.Left(), Lot: sup.Lot, CreditDays: sup.CreditDays, Limit: sup.Limit, Rel: sup.Rel, Debt: sup.Debt, DebtDue: sup.DebtDue}
		for _, pid := range w.Products {
			if w.Available(sup, pid) {
				cv.Prices[pid] = sup.Price[pid]
			}
		}
		v.Connects = append(v.Connects, cv)
	}
	for _, h := range w.Houses {
		stock := map[string]int{}
		for pid, n := range h.Stock {
			if n > 0 {
				stock[pid] = n
			}
		}
		v.Houses = append(v.Houses, HouseView{ID: h.ID, Name: h.Name, City: h.City, Corner: h.Corner, Capacity: h.Capacity, Stock: stock, Guard: h.Guard, Known: h.Known})
	}
	for _, f := range w.Fronts {
		v.Fronts = append(v.Fronts, FrontView{ID: f.ID, Name: f.Name, Level: f.Level, Frozen: f.FrozenUntil > w.Day, Washed: f.Washed})
	}
	for _, r := range w.Rivals {
		id := r.Faction()
		fv := FactionView{ID: id, Leader: r.Leader, Alive: r.Alive(), Arrived: r.Arrived, Corners: w.RivalHeldBy(id), Trust: r.Trust, War: r.War, Personality: known.Personality(id)}
		if lo, hi, _, ok := known.Muscle(id); ok {
			fv.MuscleLo, fv.MuscleHi, fv.MuscleKnown = lo, hi, true
		}
		if b := known.Books(id); b.Read() {
			fv.Books = &BooksView{Day: b.Day, Cash: b.Cash, Income: b.Income, Muscle: b.Muscle, Wages: b.Wages}
		}
		if c, ok := known.Move(id); ok {
			fv.Move = c
		}
		for _, d := range r.Deals {
			fv.Deals = append(fv.Deals, d.Kind)
		}
		v.Factions = append(v.Factions, fv)
	}
	v.Law = LawView{Chief: w.Law.Chief.Name, ChiefTemper: known.Chief(), DA: w.Law.DA.Name, DAStance: w.Law.DA.Stance, NextElection: s.set.Law.NextElection(w)}
	if c := w.Dilemmas.Pending; c != nil {
		cv := &CardView{ID: c.ID, Title: c.Title, Text: c.Text}
		chips := ChoiceChips(s.cfg, s.Rules(), w, c)
		for i, ch := range c.Choices {
			cv.Choices = append(cv.Choices, ChoiceView{Label: ch.Label, Preview: chips[i]})
		}
		v.Card = cv
	}
	if r := w.Report; r != nil { // nil before the first morning
		v.Report = reportView(r, s.cfg.Headlines.Flow.BigShare)
	}
	v.Alerts = s.Alerts()
	noNulls(reflect.ValueOf(&v).Elem())
	return v
}

// noNulls makes every nil slice and map in v empty, all the way down
// (#333): encoding/json writes a nil one as null, so a list would come
// as [] one morning and null the next, and every client would have to
// guard it. A nil pointer (over, card, books) is a field that is absent
// and stays nil; each is omitempty, so none is ever null either. It
// walks the view's own types, so a list added later is covered.
func noNulls(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			noNulls(v.Elem())
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				noNulls(v.Field(i))
			}
		}
	case reflect.Slice:
		if v.IsNil() {
			v.Set(reflect.MakeSlice(v.Type(), 0, 0))
		}
		for i := 0; i < v.Len(); i++ {
			noNulls(v.Index(i))
		}
	case reflect.Map:
		if v.IsNil() {
			v.Set(reflect.MakeMap(v.Type()))
		}
		for _, k := range v.MapKeys() {
			e := reflect.New(v.Type().Elem()).Elem()
			e.Set(v.MapIndex(k))
			noNulls(e)
			v.SetMapIndex(k, e)
		}
	}
}

// reportView is the morning report as the view carries it, its flow's
// big lines picked out at bigShare of the opening.
func reportView(r *game.DayReport, bigShare float64) ReportView {
	f := r.Flow
	fv := FlowView{
		Opening: PoolsView{Dirty: f.Opening.Dirty, Clean: f.Opening.Clean},
		Closing: PoolsView{Dirty: f.Closing.Dirty, Clean: f.Closing.Clean},
		Net:     f.Net(),
	}
	for _, l := range f.Lines {
		fv.Lines = append(fv.Lines, FlowLineView{Cat: l.Cat, Label: game.FlowLabel(l.Cat), Dirty: l.Dirty, Clean: l.Clean, Big: f.Big(l, bigShare)})
	}
	return ReportView{
		Flow: fv,
		Day:  r.Day, Incident: lines(r.Incident), Unlocked: lines(r.Unlocked), Tier: lines(r.Tier), Prices: lines(r.Prices),
		Sales: lines(r.Sales), Heat: lines(r.Heat), Crew: lines(r.Crew), Territory: lines(r.Territory),
		Shipments: lines(r.Shipments), Law: lines(r.Law), Intel: lines(r.Intel), Money: lines(r.Money),
		Upgrades: lines(r.Upgrades), News: lines(r.News), CashBefore: r.CashBefore, CashAfter: r.CashAfter,
	}
}

// lines copies a report section, so the view holds nothing of the
// world's; nil stays nil.
func lines(xs []string) []string {
	if len(xs) == 0 {
		return nil
	}
	return append([]string(nil), xs...)
}

// sortedKeys is a map's keys in order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
