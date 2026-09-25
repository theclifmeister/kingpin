// Package game holds the world state, the game clock and save/load. It is
// the only package simulations depend on, and it never imports the UI.
package game

import (
	"math/rand/v2"
	"slices"

	"github.com/theclifmeister/kingpin/internal/events"
)

// SchemaVersion is bumped whenever World changes shape incompatibly.
const SchemaVersion = 16

// World is the complete state of a run. Every field is a plain value so the
// whole struct can be serialised with encoding/gob.
type World struct {
	SchemaVersion int
	Seed          uint64
	Day           int

	Cities    map[string]*City // keyed by city id; CityCityOrder fixes their sequence
	CityOrder []string         // city ids in a fixed order, home first

	Player      Player
	Products    []string // ordered product ids, the same in every city
	Heat        HeatState
	Crew        CrewState
	Rivals      []*RivalState   // the factions (#43), in id order: Rivals[0] is the rival at home, Rival(); the rivals sim seeds the rest
	Upgrades    map[string]bool // upgrade ids owned; effects fold from these (FoldEffects)
	FallsTaken  int             // fall guys who have taken their fall (#117: fall_guys is a count; each takes one)
	Fronts      []Front         // businesses the player owns, in the order bought
	Laundering  LaunderingState
	Dilemmas    DilemmaState              // the card waiting for an answer, and the deck's pacing
	Shipments   []Shipment                // product on the road, in the order sent
	Logistics   LogisticsState            // the shipment counter, the seizure record and the routes' books
	Routes      map[string]RouteSetting   // the route dials, keyed by route id; a route not here is off
	Offers      []Offer                   // deals the factions have put on the table, oldest first (Offer.Faction says which)
	Delegated   map[string]SellOrder      // the lieutenants' standing sell orders, keyed like Orders; the crew step refreshes them
	Law         LawState                  // the chief and the DA (#41); pressure and goodwill are per city
	Contracts   []Contract                // the buyers' orders (#71), oldest first; the market sim deals and resolves them
	Buyers      BuyersState               // the buyer deck's pacing and blacklist
	Supply      map[string]SupplyContract // the supply contracts (#113), keyed like Orders; the market sim fills them every morning
	Standing    map[string]SellOrder      // your standing sell orders (#114), keyed like Orders; the market sim resolves them every night at a cut
	Suppliers   []Supplier                // the connects (#72), in the order seeded: the street one in every city, the wholesaler, one more by seed
	Progression Progression               // the tiers reached (#147), stamped by the news sim; nothing gates on it
	Houses      []House                   // the stash houses (#73), in the order bought: where the stock sits beyond the street, and which one the raid finds
	Incidents   IncidentState             // the world's incidents (#44): what has fired and the table's pacing; the world sim's, first in the order

	// The assets (#48, game/assets.go): what is owned, in the order
	// bought, and what the task force took or the police found, for the
	// record. Bought by BuyAsset with clean cash; the heat sim is the
	// one that takes one away. Zero is the run before: no bump.
	Assets     []Asset
	AssetsLost []Asset

	// The export lanes (#391, game/exports.go): the standing orders, the
	// loads out and the glut abroad. The logistics sim's; zero is the
	// run before the lanes.
	Exports ExportsState

	// The trophies (#392, game/trophies.go): bought with clean cash to
	// be seen buying them, and what the task force took. The laundering
	// sim's; zero is the run before them.
	Trophies     []Trophy
	TrophiesLost []Trophy

	// The lieutenants' supply contracts (#174), keyed like Supply: the
	// crew step refreshes them nightly by the temper's stock_days and
	// the market sim fills them where the player has set none of their
	// own. Markup is the supplier's price for a standing order or a buy
	// through a lieutenant as a multiple of the price by hand
	// (market.toml [supply] markup), stamped by the market sim at seed
	// and every morning as the connects' prices are; zero reads as one,
	// the pre-#174 state.
	DelegatedSupply map[string]SupplyContract
	Markup          float64

	// DelegatedHit is a lieutenant's answer to a faction moving on their
	// city (#379): the faction whose scouts they send the enforcers
	// after tomorrow night, the player's World.HitScouts made for them.
	// The crew step sets it nightly (a temper's hit_scouts) and the
	// rivals sim resolves it as it does Today.HitScouts; "" is nobody,
	// the run before it.
	DelegatedHit string

	// BaseQuality is the file's default quality (#47, market.toml
	// [quality] default): what the connects sell at, what a lot with no
	// figure reads, and where the price multiplier is 1. The market sim
	// stamps it at seed and every morning as it does Markup; zero reads
	// as StreetQuality, so a world built by hand prices as it did.
	BaseQuality float64

	// The offshore account (#195): clean cash reserved out of the pile
	// (Reserve), moved by the laundering sim at the end of its step
	// less the fee. It survives every ending, cannot be spent, and is
	// safe from the fall guy, an audit and a forfeiture: the score.
	// QuietDays is how many days in a row have been quiet enough to
	// retire on (the laundering sim counts them off the tick's events
	// and the cities' heat, and zeroes them on a loud one); Retire
	// needs enough of both. Zero is the run before either existed.
	Offshore  int
	QuietDays int

	// Intel (#45, intel.go): what the player knows, a file of facts
	// written by the sims that own the truth (Learn) and read by the
	// panels through Known. Nil is a run that has learnt nothing.
	Intel []Fact

	// Start (#50, profile.go) is what the run began as: the character,
	// the hard DA, the daily's date and whether it was practice. Stamped
	// at NewWorld, read by the summary and the profile, never by a sim;
	// zero is every run before the feature, so no bump.
	Start Start

	// LegitDays (#49) is how many days in a row the fronts' own income
	// has out-earned the street with home's goodwill over its pressure:
	// the laundering sim counts them at the end of its step and zeroes
	// them on a day that fails, and at laundering.toml [businessman]
	// legit_days the run ends a businessman. Zero is the run before.
	LegitDays int

	// Reign (#227) is the day the city became yours for good: the
	// rivals sim stamps it the morning Dominant() has held dominant_days
	// with more than kingpin_share of home's corners held, zeroes it the
	// morning that stops holding, and the crown (World.Crown) ends the
	// run a kingpin on the player's say-so while it stands. Zero is no
	// reign, the run before.
	Reign int
	// ReignSlip (#399) is how many mornings in a row the reign has held
	// under kingpin_share, riding [endings] reign_grace; the crown waits
	// while it is over zero. Reigns is how many reigns the run has begun,
	// so a reign begun again is not news the way the first was. Both the
	// rivals sim's; zero is the run before.
	ReignSlip int
	Reigns    int

	// War (#229) is the faction the war order stands against: its id,
	// "" for none. Declared from the rivals screen (DeclareWar), it is
	// the hand's strike sent every night the hand leaves empty, at
	// rivals.toml [war] dial on the faction's corner nearest your front
	// line (rivals.Sim.WarTarget); the rivals sim ends it the night the
	// faction is gone, pays homage or has no corner left in a city you
	// hold. One war at a time. Zero is the run before.
	War string

	// Takes (#341) is the rivals sim's window on the player's take in
	// each city away from home: by city id, the revenue of the last
	// window_days days, a day a slot (the day modulo the window). A city
	// gets a row the first day anything sells there, so a run that never
	// sells away from home keeps it nil, the run before.
	Takes map[string][]int

	// Ambition (#347, ambitions.go) is the plan the player pinned: one
	// of content.AmbitionIDs, "" for none. The dashboard shows it with
	// its next step and its milestones are alerts; no sim reads it, so a
	// run is the same run pinned or not. Zero is no plan, the run before.
	Ambition string

	// Today is the player's per-day scratch (#144): what the actions
	// queued since the morning, for the sims to resolve tonight. The
	// clock zeroes it as a unit after every EndDay (ClearToday), bar the
	// morning's supply-contract receipts, which it keeps in Buys through
	// the day for the cart (#113).
	Today Today

	Journal []Headline // full headline history, oldest first
	Report  *DayReport // morning report for the current day
	Flows   []CashFlow // the last nights' cash flows (#351), oldest first, headlines.toml [flow] days of them; the news sim's, and a report: no sim reads it
	Over    *Ending    // non-nil once the run has ended
	Stats   Stats

	legacy *v6  // what a pre-7 save carried for its one city; Load sets it, MigrateCities consumes it
	fell   bool // what a pre-10 save carried as FallGuyUsed; Load sets it, MigrateFallGuys consumes it
	old    *v14 // what a pre-15 save carried for its one rival; Load sets it, SeatRival consumes it (#43)
	books  *v15 // what a pre-16 save carried for the books read; Load sets it, MigrateBooks consumes it (#45)
}

// Today is the player's per-day scratch on the World (#144): what the
// actions (Buy, PlaceSell, SetLieLow, SendEnforcers, Boost, Investigate,
// BuyUpgrade, Propose, Accept, Abandon, Fund, Back, Bribe, BuyCheckpoint,
// Deliver, Undercut,
// MoveStock, BuyHouse, Scout, Tip, BuyOff, PayCop, PlantSpy) queue by day and the sims
// resolve at EndDay, then the clock zeroes as a unit (ClearToday). A
// field here is never read across a day; the one receipt kept into the
// morning is named on the clock. The fields keep the names they had on
// World before #144, so a save written since reads the same at each
// level; a save from before it loads with Today zero, which is what its
// clock would have left after the day it was saved on.
type Today struct {
	Orders        map[string]SellOrder   // pending sell orders keyed by product id
	Buys          []Purchase             // purchases made today, and what the supply contracts bought this morning (#113)
	LieLow        bool                   // player chose to lie low today
	Strike        *StrikeOrder           // enforcers sent against a rival corner tonight
	Investigation *InvestigationOrder    // somebody asking the crew questions tonight
	UpgradesToday []string               // upgrade ids bought today, for the report
	Proposal      *Deal                  // the deal put to the rival today; it answers in the morning
	Accepted      []Offer                // rival offers the player took today; the rival sim seals them
	Abandoned     []string               // corner ids given back to the street today
	Funded        []Funding              // clean cash given to a city today; the law sim turns it into goodwill
	Backed        []Backing              // clean cash put behind a DA ticket in a city today (#193); the law sim adds it to the city's campaign
	Bribes        []BribeOrder           // envelopes paid today (#42); the law sim resolves them
	Checkpoints   []CheckpointOrder      // checkpoints and customs deals paid today (#42); the logistics sim reports them
	Deliveries    map[int]int            // contract id -> units handed over tonight; the market sim resolves them
	Undercuts     map[string]events.Dial // rival corner id -> the dial tonight's orders undercut it at (#68); the market sim resolves them
	Moved         []Move                 // stock moved between places today (#73); the heat sim counts the units as exposure
	HousesBought  []string               // house ids bought today; the territory sim reports them
	Scouting      *ScoutOrder            // somebody reading the rival's books tonight (#70); the rivals sim resolves it
	Tipoff        *TipOrder              // the rival corner you tipped the police on tonight (#70); the rivals sim resolves it
	Poach         *PoachOrder            // the rival's muscle you are paying to go home tonight (#70); the rivals sim resolves it
	HitScouts     string                 // the faction whose scouts your enforcers hit tonight (#341); the rivals sim resolves it
	Invested      []Investment           // levels bought at the fronts today (#192), applied at once; the laundering sim reports them
	Cuts          []CutRecord            // the cuts made today (#47), applied at once; the market sim reports them
	Reserved      int                    // clean cash on its way offshore tonight (#195), out of the pile already; the laundering sim moves it and takes the fee
	CashedOut     CashOut                // clean cash drawn into the dirty pile today (#395), applied at once; the news sim books it
	Cop           *CopOrder              // a cop paid today (#45); the heat sim and the law sim each write what they know off it
	Spy           *SpyOrder              // a crew member going under tonight (#45); the crew sim sends them
	DeedsBought   []string               // corner ids whose block was bought today (#194), paid at once; the territory sim reports them
	AssetsBought  []string               // asset ids bought today (#48), applied at once; the laundering sim reports them
}

// Investment is clean cash put into a front's levels today (#192):
// which front, how many levels and what they cost between them.
type Investment struct {
	Front  string
	Levels int
	Cost   int
}

// City is one city of the run: its own street prices and demand, its own
// corners and its own police. Static tuning (HeatMul, Wholesale) is copied
// in from content so the world never needs the city config to step.
type City struct {
	ID        string
	Name      string
	HeatMul   float64                   // multiplier on the sale heat of every unit moved here
	Wholesale bool                      // the supplier here sells by the lot
	Market    map[string]*ProductMarket // keyed by product id
	Corners   []Corner
	Heat      float64  // city heat, 0..100: how hard the police here are looking
	Pressure  float64  // public pressure, 0..100: how loudly the city wants something done (#41)
	Goodwill  float64  // what the player has bought the city, 0..100: it takes pressure off a little every day
	Campaign  Campaign // the money you have put behind a DA ticket here before the next election (#193); the law sim owns it
}

// Player is the human's cash, where they are and what they keep where,
// and the face the city sees. Stock lives in a stash per city: it moves
// between them only by shipment. Since #73 the stash is the street of
// the city: what you carry if you stand there and what the runners
// posted there hold; the houses (World.Houses) are the rest, and the
// accessors on World read and write both.
type Player struct {
	DirtyCash  int
	CleanCash  int
	Stash      map[string]map[string]int     // city id -> product id -> units on the street there (#73: the houses are World.Houses)
	Quality    map[string]map[string]float64 // city id -> product id -> the quality of everything held there (#47), the street and the houses as one lot; missing or zero reads as the default
	Location   string                        // city id the player is in
	CarryLimit int
	Reputation Reputation
}

// StreetQuality is what a lot reads before the market sim has stamped
// the file's default onto the world (World.BaseQuality): the middle of
// the scale, where the price multiplier is 1.
const StreetQuality = 50

// Lot is what a stash holds of a product in a city (#47): the units,
// street and houses together, and their quality, 0..100, one number for
// the lot. Nothing holds a Lot; the accessors read and write both halves.
type Lot struct {
	Units   int
	Quality float64
}

// CutRecord is a cut made today (#47), for the report: what was there,
// what the cut added, the quality before and after, what it cost and
// whose hand was on it.
type CutRecord struct {
	City    string
	Product string
	Units   int
	Added   int
	From    float64
	To      float64
	Cost    int
	Chemist string
}

// Cook is a chemist's lot on its way (#47): ordered on Ordered, landing
// in City's stash on Ready at Quality, the chemist's the day it was
// ordered. The crew sim lands it; the precursors were paid for on the
// order.
type Cook struct {
	ID      int
	City    string
	Product string
	Units   int
	Quality float64
	Ordered int
	Ready   int
	Cost    int
	Chemist string
}

// DaysLeft is how many days the cook still has to go on day.
func (c Cook) DaysLeft(day int) int { return max(0, c.Ready-day) }

// Reputation is the player's public face on three axes, 0..100, that the
// reputation sim drifts from what happens and the other sims read. Its
// zero value is a nobody, which is what every run starts as.
type Reputation struct {
	Fear      float64 // rivals think twice; heat never quite cools
	Respect   float64 // the crew stay loyal, the supplier is generous
	Notoriety float64 // hiring is cheap; the DA knows your name
}

// Axes are the reputation axes in a fixed order, for the UI and the
// reputation sim.
var Axes = []string{"fear", "respect", "notoriety"}

// Axis returns a pointer to the named axis, or nil.
func (r *Reputation) Axis(name string) *float64 {
	switch name {
	case "fear":
		return &r.Fear
	case "respect":
		return &r.Respect
	case "notoriety":
		return &r.Notoriety
	}
	return nil
}

// TotalStock is the number of units on every street, all products. The
// houses and the road are not counted; World.Stashed and World.TotalStock
// are.
func (p Player) TotalStock() int {
	n := 0
	for _, s := range p.Stash {
		for _, q := range s {
			n += q
		}
	}
	return n
}

// StockIn is the number of units on one city's street, all products;
// World.StockIn counts the houses too.
func (p Player) StockIn(city string) int {
	n := 0
	for _, q := range p.Stash[city] {
		n += q
	}
	return n
}

// Shipment is product on the road between two cities. Every unit in it
// left the source stash when it was sent and lands in the destination's
// on Arrives, unless it is seized first.
type Shipment struct {
	ID      int
	Route   string // route id
	Mode    string
	From    string // city ids
	To      string
	Product string
	Units   int
	Quality float64 // the quality of the lot (#47): the source stash's when it left; zero reads as the default
	Dial    events.Ship
	Sent    int // day it left
	Arrives int // day it lands
	Cost    int // what sending it cost, dirty cash
	Driver  int // the crew member riding it (#46), 0 for nobody: jailed with it if it is seized
}

// DaysLeft is how many days the shipment still has to go on day.
func (s Shipment) DaysLeft(day int) int { return max(0, s.Arrives-day) }

// LogisticsState is the shipment counter, the seizure record the market
// reads the morning after (it steps before logistics), and the routes'
// books: what each day on each route cost, kept as long as the seizure
// record, and what each route has lost for good.
type LogisticsState struct {
	NextID   int
	Seizures []Seizure
	Days     []RouteDay     // what the routes bought and paid, one entry per route per day it moved something
	Lost     map[string]int // route id -> units seized on it, lifetime
}

// RouteDay is one day's spend on one route: the lots bought at its
// source and the fares paid, both dirty cash.
type RouteDay struct {
	Day       int
	Route     string
	Wholesale int
	Fares     int
}

// RouteSpend sums what a route cost over the last days days: the ledger's
// "this week".
func (l LogisticsState) RouteSpend(route string, day, days int) (wholesale, fares int) {
	for _, d := range l.Days {
		if d.Route == route && day-d.Day < days {
			wholesale += d.Wholesale
			fares += d.Fares
		}
	}
	return wholesale, fares
}

// Seizure is a shipment the police took on the road: what, how much, and
// where it was going.
type Seizure struct {
	Day     int
	Route   string
	From    string
	To      string
	Product string
	Units   int
}

// ProductMarket is the live market state for one product in one city.
type ProductMarket struct {
	Name          string
	Price         float64   // street price per unit
	SupplierPrice float64   // what the best available connect charges per unit today (World.SupplierPrice, #72)
	Demand        float64   // units one standard corner absorbs per day; see World.Demand
	Glut          float64   // oversupply from recent selling; pushes price down
	ShockFactor   float64   // multiplier on equilibrium while ShockDays > 0
	ShockDays     int       // remaining days of the current shock
	ShockSlump    bool      // true if the shock is a demand slump
	History       []float64 // closing prices, oldest first
	BoughtToday   int
	NoSupply      bool // the supplier here does not sell it: the road is the only way in
}

// HeatState is the law-enforcement pressure on the player: what the DA
// has and how the police have responded. City heat itself is per city
// (City.Heat); the case and the response ladder are personal.
type HeatState struct {
	SellCapDays  int            // days the patrol cap is still in force
	SellCap      float64        // fraction of demand you can sell while capped
	LastResponse map[string]int // level -> last day it fired
	Responses    map[string]int // level -> how many times it has fired this run
	Evidence     int            // what the DA has on you; enough of it is an indictment
	EvidenceDay  int            // day the file last grew; a retained lawyer lets old pages go cold
	LeakDay      int            // day an informant last fed the file (or turned); the next leak is due informant_days later
	Leaks        int            // pages an informant has fed the DA since one was last on the payroll; the tell shows at two
	Peak         float64        // the hottest any city has been
	FederalUntil int            // the feds are in town until this day (#44, an incident): the heat sim's decay is FederalDecay of itself on every tick before it
	FederalDecay float64        // ... by this much; 0 reads as no change
	TaskForceDay int            // the day a task force was announced (#48, TaskForceFormed); it fires the next tick, and 0 is none forming
	WarrantDay   int            // the night an arrest warrant was signed (#475, WarrantSigned): served heat.toml warrant_days nights later on a sale or heat still at the arrest line, else it lapses; 0 is none
	WatchUntil   int            // the feds watch the skies until this day (#48): the plane route's risk is the file's before it and zero after; stamped when a task force comes, fired or stopped by a favour (#281): the favour cancels the bust, not the watch
	LineUntil    int            // the task force's threshold is LineMul of itself on every tick before this day (#48, an incident: the extradition treaty)
	LineMul      float64        // ... by this much; 0 reads as no change
	Busts        []Bust         // stings and raids that took stock, kept a while: the market sim reads yesterday's for the connect there (#72)
	Sweep        Sweep          // the last sting or raid and who stood where when it came (#46): the crew sim reads yesterday's for the arrests
	// Trail is the heat by source the police can put a name to (#343,
	// heat.toml [investigation]): LeadKey to a tally that forgets
	// 1/window_days of itself a day. Nil with the feature off.
	Trail         map[string]float64
	Investigation Investigation // the one open investigation (#343); the zero value is none
}

// Sweep is a sting or raid as the crew remember it (#46): the city, the
// rung, the corners your crew stood on there and the crew who did when
// the police came, and whether it took stock (a raid that did is the
// lab's too). The heat sim stamps it; the crew sim, stepping before
// heat, reads yesterday's the next morning and rolls the arrests. The
// zero value is no sweep ever.
type Sweep struct {
	Day     int
	City    string
	Level   string
	Corners []string
	Crew    []int
	Units   int
}

// Bust is a sting or raid that took stock in a city: what the connect
// there holds against you the next morning.
type Bust struct {
	Day   int
	City  string
	Level string
	Units int
}

// LaunderingState is the launder dial: a persistent setting, not scratch.
type LaunderingState struct {
	Dial       events.Launder
	Offered    map[string]bool // fronts whose offer has opened and been announced (#148); nil is none
	Structured Structuring     // the last move offshore (#195): what the heat sim reads the morning after
	Sweep      OffshoreSweep   // the nightly sweep offshore (#478), the player's; zero is off, the run before
}

// OffshoreSweep is the player's standing order on the offshore account (#478):
// while On, the laundering sim moves the clean cash over Keep into the
// account every night, at the end of its step, up to what is left of
// the day's lot after anything reserved by hand (so it never files a
// page) and never into the night's upkeep (the fronts' and the assets'
// kept back). Set by World.SetSweep, ended by World.StopSweep; no sim
// writes it. Zero is off, the run before.
type OffshoreSweep struct {
	On   bool
	Keep int // the clean cash left in hand, the upkeep kept back over it where that is more
}

// Structuring is a day's move offshore as the laundering sim records
// it (#195): the day, what moved (before the fee) and how many lots
// over the line it was, the pages the heat sim files the morning after.
// Laundering steps after heat, so the record is how tonight's move
// reaches tomorrow's file, as a front's Audited stamp does an audit.
type Structuring struct {
	Day    int
	Amount int
	Lots   int
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
	Level       int            // the levels bought (#192); 0 is the front as bought, washing and costing what the file says
	Invested    int            // clean cash put into its levels, lifetime
	Grew        int            // the day its growth made the paper (#192); 0 means it has not
	City        string         // the city it stands in, where it was bought (#344); "" is home, a front from before
	Unpaid      int            // the clean cash its upkeep was short the night it last shut for it (#458); 0 once it pays again, and for an audit's shut
}

// FrontCity is the city a front stands in (#344): where it was bought,
// home for a front from before the fronts had roles.
func (w *World) FrontCity(f Front) string {
	if f.City != "" {
		return f.City
	}
	if h := w.Home(); h != nil {
		return h.ID
	}
	return ""
}

// Frozen reports whether the front is shut on day.
func (f Front) Frozen(day int) bool { return f.FrozenUntil > day }

// Front returns the owned front with id, or nil.
func (w *World) Front(id string) *Front {
	return find(w.Fronts, func(e *Front) bool { return e.ID == id })
}

// SellOrder is a queued street sale in a city, resolved at end of day by
// whoever works corners there.
type SellOrder struct {
	City    string
	Product string
	Qty     int
	Dial    events.Dial
}

// OrderKey is how Orders is keyed: one order per product per city.
func OrderKey(city, product string) string { return city + "/" + product }

// Purchase is a buy from a connect, applied immediately. Prior is the
// connect's price before the buy nudged it, so a Return the same day can
// put it back (#103). Contract marks a buy a supply contract made in the
// morning (#113) rather than one made by hand, Day the morning it was
// made: the clock keeps the morning's contract receipts through the day
// so the cart can show and return them, and drops them the next.
// Supplier names the connect (#72), Credit says it went on their book
// rather than out of the till, SmallLot that it was under their lot
// and paid their premium. Lieutenant names the lieutenant a buy went
// through (#174): a buy by hand into a city they run from elsewhere,
// or their own contract's; empty for a buy where you stand and the
// player's own contract.
type Purchase struct {
	City       string
	Product    string
	Qty        int
	UnitPrice  float64
	Cost       int
	Prior      float64
	Contract   bool
	Day        int
	Supplier   string
	Credit     bool
	SmallLot   bool
	Lieutenant string
}

// SupplyContract is a standing buy (#113): keep the stash in City at
// Units of Product, bought each morning from the supplier there. It is a
// persistent setting like the route target, not per-day scratch; the
// market sim fills it at the top of its step, with no dice.
type SupplyContract struct {
	City    string
	Product string
	Units   int
	Since   int // day it was set
}

// StartingProduct describes a product as it exists at the start of a run,
// priced for one city.
type StartingProduct struct {
	ID       string
	Name     string
	Price    float64
	Demand   float64
	NoSupply bool // the supplier in this city does not sell it
	// SupplierRatio is the flat supplier price as a fraction of street
	// (market.toml's supplier_ratio): what the market reads until its
	// first step prices the product.
	SupplierRatio float64
}

// StartingCity describes a city as a run starts: its identity, its static
// tuning and its market's starting values. Corners are laid out by the
// territory sim.
type StartingCity struct {
	ID        string
	Name      string
	HeatMul   float64
	Wholesale bool
	Products  []StartingProduct
}

// NewWorld creates a fresh run in the first city given, which is home.
// Market state is seeded from starting values so the world does not
// depend on the content package.
func NewWorld(seed uint64, cities []StartingCity, startCash, carryLimit int) *World {
	w := &World{
		SchemaVersion: SchemaVersion,
		Seed:          seed,
		Day:           0,
		Cities:        map[string]*City{},
		Player: Player{
			DirtyCash:  startCash,
			Stash:      map[string]map[string]int{},
			Quality:    map[string]map[string]float64{},
			CarryLimit: carryLimit,
		},
		Heat:     HeatState{LastResponse: map[string]int{}, Responses: map[string]int{}},
		Upgrades: map[string]bool{},
		Today:    Today{Orders: map[string]SellOrder{}},
	}
	for _, c := range cities {
		w.AddCity(c)
	}
	if len(w.CityOrder) > 0 {
		w.Player.Location = w.CityOrder[0]
	}
	w.Stats.PeakCash = startCash
	return w
}

// AddCity puts a city on the map with its market at starting values and an
// empty stash. It is a no-op if the city is already there.
func (w *World) AddCity(c StartingCity) *City {
	if have := w.Cities[c.ID]; have != nil {
		return have
	}
	city := &City{ID: c.ID, Name: c.Name, HeatMul: c.HeatMul, Wholesale: c.Wholesale, Market: map[string]*ProductMarket{}}
	if city.HeatMul <= 0 {
		city.HeatMul = 1
	}
	if w.Cities == nil {
		w.Cities = map[string]*City{}
	}
	w.Cities[c.ID] = city
	w.CityOrder = append(w.CityOrder, c.ID)
	for _, p := range c.Products {
		w.AddProduct(c.ID, p)
	}
	w.stash(c.ID)
	return city
}

// AddProduct puts a product on a city's market at its starting values and
// lists it if it is new. It is a no-op if the city already has it.
func (w *World) AddProduct(city string, p StartingProduct) {
	c := w.Cities[city]
	if c == nil {
		return
	}
	if _, ok := c.Market[p.ID]; ok {
		return
	}
	listed := false
	for _, id := range w.Products {
		listed = listed || id == p.ID
	}
	if !listed {
		w.Products = append(w.Products, p.ID)
	}
	c.Market[p.ID] = &ProductMarket{
		Name:          p.Name,
		Price:         p.Price,
		SupplierPrice: p.Price * p.SupplierRatio,
		Demand:        p.Demand,
		ShockFactor:   1,
		History:       []float64{p.Price},
		NoSupply:      p.NoSupply,
	}
	w.stash(city)[p.ID] += 0 // the key is there, so a walk over the stash sees every product
}

// City returns the city with id, or nil.
func (w *World) City(id string) *City { return w.Cities[id] }

// Home is the first city: where the run started and the rival lives.
func (w *World) Home() *City {
	if len(w.CityOrder) == 0 {
		return nil
	}
	return w.Cities[w.CityOrder[0]]
}

// Here is the city the player is in.
func (w *World) Here() *City {
	if c := w.Cities[w.Player.Location]; c != nil {
		return c
	}
	return w.Home()
}

// CityName is a display name for a city id.
func (w *World) CityName(id string) string {
	if c := w.Cities[id]; c != nil {
		return c.Name
	}
	return id
}

// MaxHeat is the heat of the hottest city: what the police ladder reads.
func (w *World) MaxHeat() float64 {
	h := 0.0
	for _, c := range w.Cities {
		if c.Heat > h {
			h = c.Heat
		}
	}
	return h
}

// HeatHere is the heat where the player is.
func (w *World) HeatHere() float64 {
	if c := w.Here(); c != nil {
		return c.Heat
	}
	return 0
}

// RNGFor returns the deterministic random source for a given day of a run.
// Deriving it from (seed, day) means nothing about the RNG needs saving.
func RNGFor(seed uint64, day int) *rand.Rand {
	return rand.New(rand.NewPCG(seed, uint64(day)*0x9E3779B97F4A7C15+1))
}

// Cash is the player's total cash, dirty plus clean.
func (w *World) Cash() int { return w.Player.DirtyCash + w.Player.CleanCash }

// Holdings is what the peak counts (#477): the cash in hand, dirty and
// clean, and the offshore account. Money sent offshore never comes
// back, so a peak that left it out held a player saving to retire under
// gates their money had passed (a playtest's retiree stuck at $118K).
// Stock and fronts are not in it: the peak is money. What is on its way
// offshore tonight is not either: it lands in the account the same
// night, before the clock stamps.
func (w *World) Holdings() int { return w.Cash() + w.Offshore }

// NetWorth is cash, dirty and clean, the offshore account and what is
// on its way there (#195), plus stock and fronts valued at what they
// cost to replace: every stash at its city's supplier price, what is on
// the road at its destination's, a front at its price and its levels.
func (w *World) NetWorth() int {
	n := w.Cash() + w.Offshore + w.Today.Reserved
	for _, cid := range w.CityOrder {
		for id, q := range w.StashOf(cid) {
			if m := w.Product(cid, id); m != nil {
				n += int(float64(q) * m.SupplierPrice)
			}
		}
	}
	for _, s := range w.Shipments {
		if m := w.Product(s.To, s.Product); m != nil {
			n += int(float64(s.Units) * m.SupplierPrice)
		}
	}
	for _, f := range w.Fronts {
		n += f.Cost + f.Invested
	}
	for _, h := range w.Houses {
		n += h.Price
	}
	for _, c := range w.Deeds() {
		n += c.Deed.Price
	}
	for _, a := range w.Assets {
		n += a.Cost // at cost (#48): what was paid, while it stands
	}
	for _, t := range w.Trophies {
		n += t.Cost // at cost (#392), as an asset is
	}
	for _, l := range w.Exports.Loads {
		n += l.Cost // a load out at what it cost (#391), as a shipment at the connect's price
	}
	return n
}

// Capacity is how many units the operation can hold in a city: the
// street (StreetCapacity) plus the houses there (#73). Shipments land
// regardless; capacity is what the supplier will sell to.
func (w *World) Capacity(city string) int {
	n := w.StreetCapacity(city)
	for _, h := range w.Houses {
		if h.City == city {
			n += h.Capacity
		}
	}
	return n
}

// StreetCapacity is what a city's street holds: the player's own carry
// limit if they are there, plus the runners posted on corners there,
// plus the ones on nobody's corner wherever the player is.
func (w *World) StreetCapacity(city string) int {
	n := 0
	here := city == w.Player.Location
	if here {
		n += w.Player.CarryLimit
	}
	for _, m := range w.Crew.Members {
		if m.Units == 0 {
			continue
		}
		switch c := w.PostOf(m.ID); {
		case c != nil && c.City == city:
			n += m.Units
		case c == nil && here:
			n += m.Units
		}
	}
	return n
}

// Product returns the market state for a product in a city, or nil.
func (w *World) Product(city, id string) *ProductMarket {
	if c := w.Cities[city]; c != nil {
		return c.Market[id]
	}
	return nil
}

// SortedProducts is a copy of the ladder sorted by id (#275): the fixed
// order a sim walks the products in when the walk draws dice or its
// order is reported, so the ladder's own order can change and no run
// moves.
func (w *World) SortedProducts() []string {
	ids := slices.Clone(w.Products)
	slices.Sort(ids)
	return ids
}

// ProductName returns a display name for id.
func (w *World) ProductName(id string) string {
	if h := w.Home(); h != nil {
		if m := h.Market[id]; m != nil {
			return m.Name
		}
	}
	return id
}

// find returns the first element of s that match accepts, or nil: the
// one find-by-id loop every lookup here shares (#275). The pointer is
// into s, so a caller writes through it as it wrote through &s[i].
func find[T any](s []T, match func(*T) bool) *T {
	for i := range s {
		if match(&s[i]) {
			return &s[i]
		}
	}
	return nil
}
