// Package game holds the world state, the game clock and save/load. It is
// the only package simulations depend on, and it never imports the UI.
package game

import (
	"math/rand/v2"
	"time"

	"github.com/theclifmeister/kingpin/internal/events"
)

// SchemaVersion is bumped whenever World changes shape incompatibly.
const SchemaVersion = 12

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
	Rival       RivalState
	Upgrades    map[string]bool // upgrade ids owned; effects fold from these (FoldEffects)
	FallsTaken  int             // fall guys who have taken their fall (#117: fall_guys is a count; each takes one)
	Fronts      []Front         // businesses the player owns, in the order bought
	Laundering  LaunderingState
	Dilemmas    DilemmaState              // the card waiting for an answer, and the deck's pacing
	Shipments   []Shipment                // product on the road, in the order sent
	Logistics   LogisticsState            // the shipment counter, the seizure record and the routes' books
	Routes      map[string]RouteSetting   // the route dials, keyed by route id; a route not here is off
	Offers      []Offer                   // deals the rival has put on the table, oldest first
	Delegated   map[string]SellOrder      // the lieutenants' standing sell orders, keyed like Orders; the crew step refreshes them
	Law         LawState                  // the chief and the DA (#41); pressure and goodwill are per city
	Contracts   []Contract                // the buyers' orders (#71), oldest first; the market sim deals and resolves them
	Buyers      BuyersState               // the buyer deck's pacing and blacklist
	Supply      map[string]SupplyContract // the supply contracts (#113), keyed like Orders; the market sim fills them every morning
	Standing    map[string]SellOrder      // your standing sell orders (#114), keyed like Orders; the market sim resolves them every night at a cut
	Suppliers   []Supplier                // the connects (#72), in the order seeded: the street one in every city, the wholesaler, one more by seed
	Progression Progression               // the tiers reached (#147), stamped by the news sim; nothing gates on it
	Houses      []House                   // the stash houses (#73), in the order bought: where the stock sits beyond the street, and which one the raid finds

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

	// Today is the player's per-day scratch (#144): what the actions
	// queued since the morning, for the sims to resolve tonight. The
	// clock zeroes it as a unit after every EndDay (ClearToday), bar the
	// morning's supply-contract receipts, which it keeps in Buys through
	// the day for the cart (#113).
	Today Today

	Journal []Headline // full headline history, oldest first
	Report  *DayReport // morning report for the current day
	Over    *Ending    // non-nil once the run has ended
	Stats   Stats

	legacy *v6  // what a pre-7 save carried for its one city; Load sets it, MigrateCities consumes it
	fell   bool // what a pre-10 save carried as FallGuyUsed; Load sets it, MigrateFallGuys consumes it
}

// Today is the player's per-day scratch on the World (#144): what the
// actions (Buy, PlaceSell, SetLieLow, SendEnforcers, Boost, Investigate,
// BuyUpgrade, Propose, Accept, Abandon, Fund, Back, Deliver, Undercut,
// MoveStock, BuyHouse, Scout, Tip, BuyOff) queue by day and the sims
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
	Deliveries    map[int]int            // contract id -> units handed over tonight; the market sim resolves them
	Undercuts     map[string]events.Dial // rival corner id -> the dial tonight's orders undercut it at (#68); the market sim resolves them
	Moved         []Move                 // stock moved between places today (#73); the heat sim counts the units as exposure
	HousesBought  []string               // house ids bought today; the territory sim reports them
	Scouting      *ScoutOrder            // somebody reading the rival's books tonight (#70); the rivals sim resolves it
	Tipoff        *TipOrder              // the rival corner you tipped the police on tonight (#70); the rivals sim resolves it
	Poach         *PoachOrder            // the rival's muscle you are paying to go home tonight (#70); the rivals sim resolves it
	Invested      []Investment           // levels bought at the fronts today (#192), applied at once; the laundering sim reports them
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
	Stash      map[string]map[string]int // city id -> product id -> units on the street there (#73: the houses are World.Houses)
	Location   string                    // city id the player is in
	CarryLimit int
	Reputation Reputation
}

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
	Dial    events.Ship
	Sent    int // day it left
	Arrives int // day it lands
	Cost    int // what sending it cost, dirty cash
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
	Busts        []Bust         // stings and raids that took stock, kept a while: the market sim reads yesterday's for the connect there (#72)
}

// Bust is a sting or raid that took stock in a city: what the connect
// there holds against you the next morning.
type Bust struct {
	Day   int
	City  string
	Level string
	Units int
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
	Offered      map[string]bool // roles announced as looking for work (#148): accountant, lieutenant; nil is none
	Leads        []Lead          // who went over to the rival last night, for the rivals sim to act on next step (#144); the crew sim writes it fresh every step and nothing else writes it
}

// CrewMember is one person on the payroll (or in the hiring pool). Stats are
// 0..100. Units, Wage and Fee are fixed when the candidate is generated so
// the world never needs crew tuning to price them. Informant is the hidden
// flag: the roster never shows it, the report does, in its own way. A
// lieutenant has a Personality (violent, greedy, careful, steady), fixed
// when they are generated and hidden until Observed, and runs the City
// they are assigned to.
type CrewMember struct {
	ID          int
	Name        string
	Role        string // runner, enforcer, accountant, lieutenant
	Skill       int
	Loyalty     float64
	Greed       int
	Nerve       int
	Units       int    // sell capacity this member adds
	Wage        int    // daily wage at fair pay
	Fee         int    // signing fee
	Hired       int    // day hired
	Informant   bool   // talking to the police; only firing them stops it
	Personality string // a lieutenant's: violent, greedy, careful, steady
	City        string // the city a lieutenant runs; empty when unassigned
	Assigned    int    // day the lieutenant was last given a city
	Observed    bool   // the lieutenant has been on the job long enough for the report to name their personality
}

// Lieutenant reports whether the member is a lieutenant.
func (m CrewMember) Lieutenant() bool { return m.Role == RoleLieutenant }

// Runs reports whether the member is a lieutenant running a city.
func (m CrewMember) Runs() bool { return m.Lieutenant() && m.City != "" }

// RoleLieutenant is the role of a crew member who runs a city for the
// player.
const RoleLieutenant = "lieutenant"

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

// Lieutenant returns the member running a city, or nil.
func (c *CrewState) Lieutenant(city string) *CrewMember {
	for i := range c.Members {
		if m := &c.Members[i]; m.Runs() && m.City == city {
			return m
		}
	}
	return nil
}

// Lieutenants counts the members running a city.
func (c CrewState) Lieutenants() int {
	n := 0
	for _, m := range c.Members {
		if m.Runs() {
			n++
		}
	}
	return n
}

// LaunderingState is the launder dial: a persistent setting, not scratch.
type LaunderingState struct {
	Dial    events.Launder
	Offered map[string]bool // fronts whose offer has opened and been announced (#148); nil is none
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

// StrikeOrder is the player's enforcers sent against a rival corner at a
// force, resolved by the rival sim at end of day. Boost sends them for
// the corner's takings rather than the ground (#70): one order a night
// either way, at the force's odds.
type StrikeOrder struct {
	Corner string
	Force  events.Force
	Boost  bool
}

// ScoutOrder is the player paying for a look at the rival's books
// tonight (#70), paid up front and resolved by the rivals sim.
type ScoutOrder struct {
	Cost int
}

// TipOrder is the player tipping the police on a rival corner tonight
// (#70): free in cash, resolved by the rivals sim.
type TipOrder struct {
	Corner string
}

// PoachOrder is the player paying Units heads of the rival's muscle to
// go home tonight (#70), Cost paid up front, resolved by the rivals sim.
type PoachOrder struct {
	Units int
	Cost  int
}

// Known is the rival's books as last read by a scout (#70): a snapshot,
// never a live feed. Day is the day it was read, 0 for never; the
// numbers are the rival's cash, its income and its wage bill that day
// and its muscle that night. Nothing but a successful scout writes it.
type Known struct {
	Day    int
	Cash   int
	Income int
	Muscle int
	Wages  int
}

// Read reports whether the books have ever been read.
func (k Known) Read() bool { return k.Day > 0 }

// Age is how many days old the snapshot is on day.
func (k Known) Age(day int) int { return day - k.Day }

// RivalState is the faction competing for the city's corners. Leader is
// empty until the sim seeds it; Arrived is 0 until it holds its first
// corner. War is how loud the fight has got, 0..100: past the crackdown
// line the police clear both sides.
type RivalState struct {
	ID          string // faction id (#144): FactionRival today, seeded with the rival; read it through Faction, which resolves the zero id of an older save
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
	LastClaim   int    // day it last chose a free corner to set up on (the tell, #69); 0 never (#60: the pace's cooldown)
	LastStruck  int    // day the player's enforcers last went in; 0 never (#60: under attack it grows as fast as it can)
	Eyeing      string // corner id it sets up on next step, the tell (#69); "" none. Post somebody on it first and the claim fails.
	EyeingDay   int    // day the tell was given

	// Diplomacy (#32): what it thinks of you and what you have agreed.
	Trust     float64 // 0..100; seeded by personality, earned by kept deals, spent by force
	Deals     []Deal  // live deals, oldest first
	Betrayed  int     // day the player last broke a deal; 0 never. It takes nothing for a while after.
	LastFlip  int     // day it last took a corner from the player; 0 never
	NextOffer int     // id of the next offer it makes

	// The books (#139): wages the day's take did not cover, carried
	// forward; a surplus day pays them down and at a full wage a head
	// walks. Zero is a payroll the take covers, the pre-#139 state.
	Arrears float64

	// The player's moves against it (#70). Heat is the police's
	// attention on it, 0..100: your tips, and its own pushes while it is
	// over zero; past the notice line they take a corner off it. Known
	// is its books as last scouted; Scouted counts the scouts that read
	// nothing since the last that did. Zero values are the pre-#70 state.
	Heat     float64
	Known    Known
	Scouted  int
	LastRaid int // day the police last took a corner off it on your tip; 0 never
	Away     int // heads bought off or arrested and not yet back: what it wants less, for a while
	AwayDay  int // day the last of them came back, or was sent away; the next returns away_days later
}

// Faction is the rival's faction id (#144): ID, or FactionRival for a
// save from before ids carried one (the zero value), so no migration.
func (r RivalState) Faction() string {
	if r.ID == "" {
		return FactionRival
	}
	return r.ID
}

// Lead is a crew member who went over to the rival: their name and the
// corner they ran (empty if none), which they walk the rival onto. The
// crew sim queues them on CrewState.Leads; the rivals sim acts on them
// next step.
type Lead struct {
	Name   string
	Corner string
}

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

// Headline is a journal entry.
type Headline struct {
	Day    int
	Source string
	Text   string
}

// DayReport is what the player reads in the morning.
type DayReport struct {
	Day        int
	Unlocked   []string // gates crossed this morning (#148): first in the report
	Prices     []string
	Sales      []string
	Heat       []string
	Crew       []string
	Territory  []string
	Shipments  []string
	Law        []string // elections, a new chief, pressure bands crossed, what you gave a city
	Money      []string
	Upgrades   []string
	Tier       []string // the tier entered this morning (#147), first in the report
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
	Boosts         int // enforcers sent for a rival corner's takings (#70)
	Boosted        int // dirty cash they took off it
	Scouts         int // looks bought at the rival's books
	Tips           int // tips you gave the police on a rival corner
	RivalRaids     int // rival corners the police took on your tips
	Poached        int // heads of the rival's muscle you paid to go home
	CornersWon     int // rival corners taken by force
	CornersLost    int // corners the rival took from you
	Laundered      int // dirty cash washed clean
	Seized         int // clean cash lost to audits
	Informants     int // crew who turned on you
	Defections     int // crew who went over to the rival
	Investigations int
	Shipments      int // shipments sent
	Shipped        int // units sent over a route
	Seizures       int // shipments the police took on the road
	SeizedOnRoad   int // units lost to them
	Deals          int // deals struck with the rival, either way
	DealsRefused   int // proposals it turned down
	Betrayals      int // deals you broke
	BetrayedBy     int // deals it broke
	Tribute        int // dirty cash paid the rival in tribute
	Cuts           int // dirty cash the lieutenants kept as their cut, and the crew's cut on your standing orders (#114)
	Walked         int // lieutenants who walked with their city
	Funded         int // clean cash given to the cities (#41)
	Backed         int // clean cash put behind DA campaigns (#193)
	Campaigns      int // campaigns backed, one a city an election
	CampaignsWon   int // of those, the ticket that won
	Elections      int // DA elections held
	Chiefs         int // police chiefs replaced
	Contracts      int // buyers' contracts delivered in full (#71)
	ContractUnits  int // units handed over to buyers
	ContractCash   int // dirty cash the buyers paid
	ContractsShort int // contracts short at the due day
	Credit         int // dirty cash's worth of product taken on credit from the connects (#72)
	Repaid         int // what has been paid back
	LatePayments   int // debts that were short on their day
	DebtDays       int // days ended owing a connect something
	Collected      int // units a connect took from the stash for a debt
	Rent           int // clean cash paid the landlords (#73)
	HousesLost     int // houses the landlord threw you out of
	HouseUnits     int // units lost out of the houses to raids and robberies
	Earned         int // clean cash the levelled fronts earned on their own (#192)
	Invested       int // clean cash put into the fronts' levels
}

// StartingProduct describes a product as it exists at the start of a run,
// priced for one city.
type StartingProduct struct {
	ID       string
	Name     string
	Price    float64
	Demand   float64
	NoSupply bool // the supplier in this city does not sell it
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
		SupplierPrice: p.Price * 0.55,
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

// stash is the player's stock on a city's street, created empty on
// first use. It is the one handle on the map: every unit that enters or
// leaves a stash goes through AddStock, TakeStock or SetStock below
// (#144), so #73 and #47 can change what the map means or holds by
// changing these and nothing else. TestStashHasNoWriters holds the rest
// of the code to it. Since #73 the map is the street and the houses
// (World.Houses) are the rest: the accessors below read and write both,
// and a house's Stock map is written only here and in houses.go.
func (w *World) stash(city string) map[string]int {
	if w.Player.Stash == nil {
		w.Player.Stash = map[string]map[string]int{}
	}
	s := w.Player.Stash[city]
	if s == nil {
		s = map[string]int{}
		w.Player.Stash[city] = s
	}
	return s
}

// StashOf is a copy of the player's stock in a city, product by product,
// the street and the houses there together, for a reader that walks it
// (the UI, the harness, the cart). Writing to the copy changes nothing;
// the writers are AddStock and TakeStock.
func (w *World) StashOf(city string) map[string]int {
	out := w.StreetOf(city)
	for _, h := range w.Houses {
		if h.City != city {
			continue
		}
		for id, q := range h.Stock {
			out[id] += q
		}
	}
	return out
}

// StreetOf is a copy of what is on a city's street, product by product:
// what you carry there and what the runners posted there hold.
func (w *World) StreetOf(city string) map[string]int {
	s := w.Player.Stash[city]
	out := make(map[string]int, len(s))
	for id, q := range s {
		out[id] = q
	}
	return out
}

// Stock is how many units of a product the player holds in a city: the
// street and the houses there.
func (w *World) Stock(city, product string) int {
	n := w.Player.Stash[city][product]
	for _, h := range w.Houses {
		if h.City == city {
			n += h.Stock[product]
		}
	}
	return n
}

// Street is how many units of a product are on a city's street.
func (w *World) Street(city, product string) int { return w.Player.Stash[city][product] }

// AddStock puts units of a product into a city: a buy, a contract's
// morning lot, a shipment landing, the road's lots, a card. It is put
// away (#73): into the emptiest house there with room, the next once
// that is full, and onto the street when the houses are full or there
// are none (the road's rule stands: a lot that fits nowhere sits on the
// street until the shipment takes it). A negative count is a TakeStock.
func (w *World) AddStock(city, product string, units int) {
	if units < 0 {
		w.TakeStock(city, product, -units)
		return
	}
	for units > 0 {
		h := w.emptiest(city)
		if h == nil {
			break
		}
		n := min(units, h.Room())
		w.putInHouse(h, product, n)
		units -= n
	}
	if units > 0 {
		w.stash(city)[product] += units
	}
}

// emptiest is the house in a city with the most room, the earlier
// bought between two with the same, or nil when none has any.
func (w *World) emptiest(city string) *House {
	var best *House
	for i := range w.Houses {
		h := &w.Houses[i]
		if h.City != city || h.Room() <= 0 {
			continue
		}
		if best == nil || h.Room() > best.Room() {
			best = h
		}
	}
	return best
}

// putInHouse is the one write into a house's stock going up.
func (w *World) putInHouse(h *House, product string, units int) {
	if units <= 0 {
		return
	}
	if h.Stock == nil {
		h.Stock = map[string]int{}
	}
	h.Stock[product] += units
}

// takeFromHouse is the one write into a house's stock going down: up to
// units, clamped at what is there, and what it took.
func (w *World) takeFromHouse(h *House, product string, units int) int {
	taken := max(0, min(units, h.Stock[product]))
	if taken > 0 {
		h.Stock[product] -= taken
		if h.Stock[product] == 0 {
			delete(h.Stock, product)
		}
	}
	return taken
}

// TakeStock takes up to units of a product out of a city (a sale, a
// shipment leaving, a handoff, a collection, a card) and returns what it
// took, so the stash never goes under zero and a caller that asked for
// more than was there learns what it got. The street goes first, then
// the houses in the order bought (#73), so a sale or a shipment never
// depends on iteration order.
func (w *World) TakeStock(city, product string, units int) int {
	taken := w.TakeStreet(city, product, units)
	for i := range w.Houses {
		if taken >= units {
			break
		}
		if h := &w.Houses[i]; h.City == city {
			taken += w.takeFromHouse(h, product, units-taken)
		}
	}
	return taken
}

// TakeStreet takes up to units of a product off a city's street alone
// (a corner robbery: what the runner was carrying) and returns what it
// took.
func (w *World) TakeStreet(city, product string, units int) int {
	s := w.stash(city)
	taken := max(0, min(units, s[product]))
	s[product] -= taken
	return taken
}

// TakeFromHouse takes up to units of a product out of one house (a
// raid, a robbery there) and returns what it took; 0 for a house that
// is not there.
func (w *World) TakeFromHouse(house, product string, units int) int {
	h := w.House(house)
	if h == nil {
		return 0
	}
	return w.takeFromHouse(h, product, units)
}

// SetStock puts a city's street stock of a product at exactly units,
// whatever it was. It is the tests' and the migration's (a save from
// before the cities carried its stock on the player); nothing in play
// sets a stash to a figure, it adds to it or takes from it.
func (w *World) SetStock(city, product string, units int) {
	w.stash(city)[product] = max(0, units)
}

// StockIn is the number of units the player holds in a city, all
// products: the street and the houses there.
func (w *World) StockIn(city string) int {
	n := w.Player.StockIn(city)
	for _, h := range w.Houses {
		if h.City == city {
			n += h.Units()
		}
	}
	return n
}

// Stashed is every unit in every city, street and houses, the road left
// out.
func (w *World) Stashed() int {
	n := w.Player.TotalStock()
	for _, h := range w.Houses {
		n += h.Units()
	}
	return n
}

// InTransit is how many units of a product are on the road, bound
// anywhere.
func (w *World) InTransit(product string) int {
	n := 0
	for _, s := range w.Shipments {
		if s.Product == product {
			n += s.Units
		}
	}
	return n
}

// Bound is how many units of a product are on the road to a city.
func (w *World) Bound(to, product string) int {
	n := 0
	for _, s := range w.Shipments {
		if s.To == to && s.Product == product {
			n += s.Units
		}
	}
	return n
}

// TotalStock is every unit the operation holds: every street, every
// house and everything on the road.
func (w *World) TotalStock() int {
	n := w.Stashed()
	for _, s := range w.Shipments {
		n += s.Units
	}
	return n
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
// they cost to replace: every stash at its city's supplier price, what is
// on the road at its destination's.
func (w *World) NetWorth() int {
	n := w.Cash()
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

// ProductName returns a display name for id.
func (w *World) ProductName(id string) string {
	if h := w.Home(); h != nil {
		if m := h.Market[id]; m != nil {
			return m.Name
		}
	}
	return id
}
