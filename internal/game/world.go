// Package game holds the world state, the game clock and save/load. It is
// the only package simulations depend on, and it never imports the UI.
package game

import (
	"math/rand/v2"
	"time"

	"github.com/theclifmeister/kingpin/internal/events"
)

// SchemaVersion is bumped whenever World changes shape incompatibly.
const SchemaVersion = 6

// World is the complete state of a run. Every field is a plain value so the
// whole struct can be serialised with encoding/gob.
type World struct {
	SchemaVersion int
	Seed          uint64
	Day           int
	City          string

	Player      Player
	Products    []string                  // ordered product ids
	Market      map[string]*ProductMarket // keyed by product id
	Heat        HeatState
	Crew        CrewState
	Territory   TerritoryState
	Rival       RivalState
	Upgrades    map[string]bool // upgrade ids owned; effects fold from these (FoldEffects)
	FallGuyUsed bool            // the fall guy has taken his one fall
	Fronts      []Front         // businesses the player owns, in the order bought
	Laundering  LaunderingState

	// Per-day scratch, cleared by the clock after every EndDay.
	Orders        map[string]SellOrder // pending sell orders keyed by product id
	Buys          []Purchase           // purchases made today
	LieLow        bool                 // player chose to lie low today
	Strike        *StrikeOrder         // enforcers sent against a rival corner tonight
	Investigation *InvestigationOrder  // somebody asking the crew questions tonight
	UpgradesToday []string             // upgrade ids bought today, for the report

	Journal []Headline // full headline history, oldest first
	Report  *DayReport // morning report for the current day
	Over    *Ending    // non-nil once the run has ended
	Stats   Stats
}

// Player is the human's cash and inventory.
type Player struct {
	DirtyCash  int
	CleanCash  int
	Stock      map[string]int
	CarryLimit int
}

// TotalStock is the number of units the player holds across all products.
func (p Player) TotalStock() int {
	n := 0
	for _, q := range p.Stock {
		n += q
	}
	return n
}

// ProductMarket is the live market state for one product in one city.
type ProductMarket struct {
	Name          string
	Price         float64   // street price per unit
	SupplierPrice float64   // what the player pays per unit today
	Demand        float64   // units one standard corner absorbs per day; see World.Demand
	Glut          float64   // oversupply from recent selling; pushes price down
	ShockFactor   float64   // multiplier on equilibrium while ShockDays > 0
	ShockDays     int       // remaining days of the current shock
	ShockSlump    bool      // true if the shock is a demand slump
	History       []float64 // closing prices, oldest first
	BoughtToday   int
}

// HeatState is the law-enforcement pressure on the player.
type HeatState struct {
	Value        float64        // 0..100
	SellCapDays  int            // days the patrol cap is still in force
	SellCap      float64        // fraction of demand you can sell while capped
	LastResponse map[string]int // level -> last day it fired
	Responses    map[string]int // level -> how many times it has fired this run
	Evidence     int            // what the DA has on you; enough of it is an indictment
	EvidenceDay  int            // day the file last grew; a retained lawyer lets old pages go cold
	LeakDay      int            // day an informant last fed the file (or turned); the next leak is due informant_days later
	Leaks        int            // pages an informant has fed the DA since one was last on the payroll; the tell shows at two
	Peak         float64
}

// CrewState is the player's crew: the roster, the hiring pool and the pay
// dial. HiredToday, FiredToday and PaidOffToday are per-day scratch the
// clock clears.
type CrewState struct {
	Members      []CrewMember
	Candidates   []CrewMember
	Pay          events.Pay
	NextID       int
	PoolDay      int // day the candidate pool last rotated
	LastSkim     int // day skimming was last reported; 0 means never
	Exposed      int // member an investigation named as the informant; 0 nobody (they may be gone)
	Investigated int // investigations that found nobody since the last that did; each makes the next more likely to
	HiredToday   []CrewMember
	FiredToday   []CrewMember
	PaidOffToday []Payoff
}

// CrewMember is one person on the payroll (or in the hiring pool). Stats are
// 0..100. Units, Wage and Fee are fixed when the candidate is generated so
// the world never needs crew tuning to price them. Informant is the hidden
// flag: the roster never shows it, the report does, in its own way.
type CrewMember struct {
	ID        int
	Name      string
	Role      string // runner, enforcer, accountant
	Skill     int
	Loyalty   float64
	Greed     int
	Nerve     int
	Units     int  // sell capacity this member adds
	Wage      int  // daily wage at fair pay
	Fee       int  // signing fee
	Hired     int  // day hired
	Informant bool // talking to the police; only firing them stops it
}

// Informants counts the members talking to the police.
func (c CrewState) Informants() int {
	n := 0
	for _, m := range c.Members {
		if m.Informant {
			n++
		}
	}
	return n
}

// Payoff is a member paid to stay loyal today, for the report.
type Payoff struct {
	ID   int
	Name string
	Cost int
}

// InvestigationOrder is the player asking questions of the crew tonight,
// paid up front and resolved by the crew sim at end of day.
type InvestigationOrder struct {
	Cost int
}

// Runners counts members in the runner role.
func (c CrewState) Runners() int { return c.Role("runner") }

// Role counts members in a role.
func (c CrewState) Role(role string) int {
	n := 0
	for _, m := range c.Members {
		if m.Role == role {
			n++
		}
	}
	return n
}

// Member returns the roster entry with id, or nil.
func (c *CrewState) Member(id int) *CrewMember {
	for i := range c.Members {
		if c.Members[i].ID == id {
			return &c.Members[i]
		}
	}
	return nil
}

// LaunderingState is the launder dial: a persistent setting, not scratch.
type LaunderingState struct {
	Dial events.Launder
}

// Front is a business the player owns that washes dirty cash. Its rate,
// upkeep and risk live in the laundering config; the world only records
// ownership and what has happened to it.
type Front struct {
	ID          string
	Name        string
	Cost        int            // what it was bought for
	Bought      int            // day bought
	FrozenUntil int            // day it reopens; 0 or past means open
	Washed      int            // lifetime dirty cash washed through it
	WashedToday int            // what it washed on the last day stepped
	Audited     int            // day of the last audit; 0 means never
	AuditDial   events.Launder // the dial it was run at when that audit hit
}

// Frozen reports whether the front is shut on day.
func (f Front) Frozen(day int) bool { return f.FrozenUntil > day }

// Front returns the owned front with id, or nil.
func (w *World) Front(id string) *Front {
	for i := range w.Fronts {
		if w.Fronts[i].ID == id {
			return &w.Fronts[i]
		}
	}
	return nil
}

// SellOrder is a queued street sale, resolved at end of day.
type SellOrder struct {
	Product string
	Qty     int
	Dial    events.Dial
}

// StrikeOrder is the player's enforcers sent against a rival corner at a
// force, resolved by the rival sim at end of day.
type StrikeOrder struct {
	Corner string
	Force  events.Force
}

// RivalState is the faction competing for the city's corners. Leader is
// empty until the sim seeds it; Arrived is 0 until it holds its first
// corner. War is how loud the fight has got, 0..100: past the crackdown
// line the police clear both sides.
type RivalState struct {
	Leader      string
	Personality string  // expansionist, defensive, opportunist, chaotic
	Supplier    float64 // its supplier price as a fraction of street; a better connect undercuts harder
	Cash        int
	Muscle      int // enforcers on its side, abstract
	Arrived     int // day it moved in; 0 = not yet
	Routed      int // day it last lost its last corner; 0 = never
	Observed    bool
	Grudge      int     // losses it has not yet paid back
	War         float64 // 0..100
	Claims      int     // lifetime counters for the run summary
	Flips       int     // corners it took from the player
	Tips        int
	Leads       []Lead // what defectors brought it, to act on next step
}

// Lead is a crew member who went over to the rival: their name and the
// corner they ran (empty if none), which they walk the rival onto.
type Lead struct {
	Name   string
	Corner string
}

// Purchase is a buy from the supplier, applied immediately.
type Purchase struct {
	Product   string
	Qty       int
	UnitPrice float64
	Cost      int
}

// Headline is a journal entry.
type Headline struct {
	Day    int
	Source string
	Text   string
}

// DayReport is what the player reads in the morning.
type DayReport struct {
	Day        int
	Prices     []string
	Sales      []string
	Heat       []string
	Crew       []string
	Territory  []string
	Money      []string
	Upgrades   []string
	News       []string
	CashBefore int
	CashAfter  int
}

// Ending records how a run finished.
type Ending struct {
	Day      int
	Cause    string
	PeakCash int
}

// Stats are lifetime counters for the run summary.
type Stats struct {
	PeakCash       int
	TotalRevenue   int
	UnitsSold      int
	Raids          int
	Stings         int
	Wages          int
	Skimmed        int
	Robbed         int
	Strikes        int // enforcers sent against a rival corner
	CornersWon     int // rival corners taken by force
	CornersLost    int // corners the rival took from you
	Laundered      int // dirty cash washed clean
	Seized         int // clean cash lost to audits
	Informants     int // crew who turned on you
	Defections     int // crew who went over to the rival
	Investigations int
}

// StartingProduct describes a product as it exists at the start of a run.
type StartingProduct struct {
	ID     string
	Name   string
	Price  float64
	Demand float64
}

// NewWorld creates a fresh run. Product state is seeded from starting values
// so the world does not depend on the content package.
func NewWorld(seed uint64, city string, products []StartingProduct, startCash, carryLimit int) *World {
	w := &World{
		SchemaVersion: SchemaVersion,
		Seed:          seed,
		Day:           0,
		City:          city,
		Player: Player{
			DirtyCash:  startCash,
			Stock:      map[string]int{},
			CarryLimit: carryLimit,
		},
		Market:   map[string]*ProductMarket{},
		Heat:     HeatState{LastResponse: map[string]int{}, Responses: map[string]int{}},
		Upgrades: map[string]bool{},
		Orders:   map[string]SellOrder{},
	}
	for _, p := range products {
		w.AddProduct(p)
	}
	w.Stats.PeakCash = startCash
	return w
}

// AddProduct puts a product on the market at its starting values. It is a
// no-op if the product is already listed.
func (w *World) AddProduct(p StartingProduct) {
	if _, ok := w.Market[p.ID]; ok {
		return
	}
	w.Products = append(w.Products, p.ID)
	w.Market[p.ID] = &ProductMarket{
		Name:          p.Name,
		Price:         p.Price,
		SupplierPrice: p.Price * 0.55,
		Demand:        p.Demand,
		ShockFactor:   1,
		History:       []float64{p.Price},
	}
	w.Player.Stock[p.ID] = 0
}

// NewSeed returns a seed derived from the wall clock.
func NewSeed() uint64 { return uint64(time.Now().UnixNano()) }

// RNGFor returns the deterministic random source for a given day of a run.
// Deriving it from (seed, day) means nothing about the RNG needs saving.
func RNGFor(seed uint64, day int) *rand.Rand {
	return rand.New(rand.NewPCG(seed, uint64(day)*0x9E3779B97F4A7C15+1))
}

// Cash is the player's total cash, dirty plus clean.
func (w *World) Cash() int { return w.Player.DirtyCash + w.Player.CleanCash }

// NetWorth is cash, dirty and clean, plus stock and fronts valued at what
// they cost to replace.
func (w *World) NetWorth() int {
	n := w.Cash()
	for id, q := range w.Player.Stock {
		if m := w.Market[id]; m != nil {
			n += int(float64(q) * m.SupplierPrice)
		}
	}
	for _, f := range w.Fronts {
		n += f.Cost
	}
	return n
}

// Capacity is how many units the operation can hold and move: the player's
// own carry limit plus what the crew adds.
func (w *World) Capacity() int {
	n := w.Player.CarryLimit
	for _, m := range w.Crew.Members {
		n += m.Units
	}
	return n
}

// Product returns the market state for id, or nil.
func (w *World) Product(id string) *ProductMarket { return w.Market[id] }

// ProductName returns a display name for id.
func (w *World) ProductName(id string) string {
	if m := w.Market[id]; m != nil {
		return m.Name
	}
	return id
}
