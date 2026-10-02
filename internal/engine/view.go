package engine

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// ViewVersion is the shape of View (#299, docs/engine.md). It moves
// when a field is added, renamed, retyped or dropped, never with the
// save's game.SchemaVersion: the world is free to change shape, the view
// is the contract a front end in another process is written against.
// TestViewShapeIsPinned fails on a shape change that keeps the number.
// 12 added exports and trophies (#405): the lanes and the trophies the
// wire could order and buy since #391 and #392 but not show. 13 added
// the front_shut alert's front (#458), 14 the port alert's share (#476).
// 15 (#550) exposes the state the TUI reads and the view did not carry:
// a member's wounds, jail, bail, a named informant, a lieutenant's day
// in the city, a driver's route and the chemist at the lab; the law's
// favours, campaign, bought days, leads and patrol cap; a front's
// days and debts, the till, the sweep, the assets; the orders, the
// supply contracts, the routes' targets and checkpoints; the war, the
// deals' terms, the proposal, a faction's police and tribute nights; the
// score, the bodies, the summary's stats, the walk away's pending pages
// and the stage waiting to be seen. Nothing in it is new state.
// 16 every alert's danger and notice, the reign alert's slip and the
// scouts alert's faction (#549). 17 (#557) the cut and the cook
// dialogs' two numbers: the quality of each lot you hold and the free
// room in each city's stash. 18 (#554) what Street Edition's endings
// and rivals' table read: the reign's day and slip, each faction's
// city, stance, fall, betrayal and split lines, the lifetime table
// counters, the fallen, and the ending's title, epilogue and story.
// 19 (#581) a house's price, rent, day bought and nights the rent has
// gone unpaid, which Street Edition's owned houses read. 20 (#582)
// what Street Edition's connects, market and route panes and its cart
// read: the day's buys and the room a city keeps without you; a
// connect's door, terms and record; a product's glut and the demand your
// corners serve, and a shock's multiplier; the corners worked; the
// street's quality; and the day
// the feds stop watching the skies.
const ViewVersion = 20

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
	Exports   []LaneView     `json:"exports"`  // every export lane in the file (#391, #405), open or not
	Trophies  []TrophyView   `json:"trophies"` // the trophies you own (#392, #405)
	Factions  []FactionView  `json:"factions"`
	Law       LawView        `json:"law"`
	Card      *CardView      `json:"card,omitempty"`
	Report    ReportView     `json:"report"`
	Alerts    []Alert        `json:"alerts"`
	Ambitions []AmbitionView `json:"ambitions"` // the endings as plans with their progress (#347)

	// View 15 (#550): what the screens read that the view did not carry.
	Assets   []AssetView          `json:"assets"`             // the assets you own (#48), in the order bought
	Cooks    []CookView           `json:"cooks"`              // the chemist's lots on their way (#47)
	Orders   []OrderView          `json:"orders"`             // today's sell orders, resolved tonight
	Standing []OrderView          `json:"standing"`           // your standing sell orders (#114, #503), every night at a cut
	Supply   []SupplyContractView `json:"supply"`             // the supply contracts (#113), yours and the lieutenants' (#174)
	Stats    StatsView            `json:"stats"`              // the run summary's counters (#49)
	Stage    *StageView           `json:"stage,omitempty"`    // a stage entered and not yet seen (#149): see_stage marks it
	Proposal *ProposalView        `json:"proposal,omitempty"` // the deal put to a faction today; it answers in the morning

	// View 18 (#554).
	Fallen []FallenView `json:"fallen"` // the crew shot dead on your corners (#46), oldest first

	// View 20 (#582).
	Buys []BuyView `json:"buys"` // today's buys and what the supply contracts bought this morning (World.Today.Buys), in the order made
}

// BuyView is a buy made today (game.Purchase): by hand, on a connect's
// book (Credit), by a lieutenant for you (#174), or by a supply contract
// this morning (Contract, #113). The cart lists them and returns them.
type BuyView struct {
	City       string  `json:"city"`
	Product    string  `json:"product"`
	Qty        int     `json:"qty"`
	Unit       float64 `json:"unit"` // the price paid a unit
	Cost       int     `json:"cost"` // what it cost, or went on the book
	Supplier   string  `json:"supplier"`
	Contract   bool    `json:"contract,omitempty"`
	Credit     bool    `json:"credit,omitempty"`
	SmallLot   bool    `json:"small_lot,omitempty"`
	Lieutenant string  `json:"lieutenant,omitempty"` // the lieutenant who bought it for you
}

// FallenView is a member of the crew who fell on a corner (#46).
type FallenView struct {
	Name string `json:"name"`
	Role string `json:"role"`
	Day  int    `json:"day"`
}

// StoryView is one headline of the run summary's story (#465,
// Session.Story).
type StoryView struct {
	Day  int    `json:"day"`
	Text string `json:"text"`
}

// AssetView is an asset you own (#48): what it is, where, what it cost
// and costs a day, and the day it stands idle until, its upkeep unpaid.
type AssetView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Effect      string `json:"effect"`
	City        string `json:"city"`
	Cost        int    `json:"cost"`
	Upkeep      int    `json:"upkeep"`
	Bought      int    `json:"bought"`
	FrozenUntil int    `json:"frozen_until,omitempty"` // idle on every day before it; 0 or past is standing
}

// CookView is a chemist's lot on its way (#47): landing in city's stash
// on ready at quality.
type CookView struct {
	ID      int     `json:"id"`
	City    string  `json:"city"`
	Product string  `json:"product"`
	Units   int     `json:"units"`
	Quality float64 `json:"quality"`
	Ordered int     `json:"ordered"`
	Ready   int     `json:"ready"`
	Chemist string  `json:"chemist"`
}

// OrderView is a sell order: today's, or a standing one (All: the whole
// stash every night, #503, Qty the stash it was set on).
type OrderView struct {
	City    string `json:"city"`
	Product string `json:"product"`
	Qty     int    `json:"qty"`
	Dial    string `json:"dial"`
	All     bool   `json:"all,omitempty"`
}

// SupplyContractView is a supply contract (#113): the units a city's stash of
// a product is kept to every morning, since the day it was set; a
// lieutenant's (#174) names them, and fills only where you set none.
type SupplyContractView struct {
	City       string `json:"city"`
	Product    string `json:"product"`
	Units      int    `json:"units"`
	Since      int    `json:"since"`
	Lieutenant int    `json:"lieutenant,omitempty"` // crew id of the lieutenant whose contract it is; 0 yours
}

// StatsView is the run summary's counters (#49, the TUI's summary):
// the money, the people and the ground, lifetime.
type StatsView struct {
	Revenue     int     `json:"revenue"`
	UnitsSold   int     `json:"units_sold"`
	Laundered   int     `json:"laundered"`
	Seized      int     `json:"seized"` // clean cash lost to audits
	Wages       int     `json:"wages"`
	Skimmed     int     `json:"skimmed"`
	Robbed      int     `json:"robbed"`
	Cuts        int     `json:"cuts"`
	Invested    int     `json:"invested"`
	Earned      int     `json:"earned"`
	Taxed       int     `json:"taxed"`
	Reserved    int     `json:"reserved"` // moved offshore, after the fee
	Fees        int     `json:"fees"`
	Fallen      int     `json:"fallen"` // of the bodies, yours
	CornersWon  int     `json:"corners_won"`
	CornersLost int     `json:"corners_lost"`
	Stings      int     `json:"stings"`
	Raids       int     `json:"raids"`
	Betrayals   int     `json:"betrayals"`   // deals you broke
	BetrayedBy  int     `json:"betrayed_by"` // deals a faction broke
	Defections  int     `json:"defections"`
	Walked      int     `json:"walked"` // lieutenants who walked with their city
	PeakHeat    float64 `json:"peak_heat"`

	// View 18 (#554): the table's lifetime and the rest of the betrayals.
	Deals        int `json:"deals"`         // deals struck, either way
	DealsRefused int `json:"deals_refused"` // proposals a faction turned down
	Tribute      int `json:"tribute"`       // dirty cash paid in tribute
	Homage       int `json:"homage"`        // dirty cash the factions paid you in homage (#43)
	Informants   int `json:"informants"`    // crew who turned on you
	CrewPoached  int `json:"crew_poached"`  // crew a faction poached (#43, #465)
}

// StageView is the stage entered and not yet seen (#149): the tier,
// the file's words for it, what it opened and what the next takes
// (StageNext). see_stage with Pending marks it seen.
type StageView struct {
	Pending int      `json:"pending"`
	Name    string   `json:"name"`
	Blurb   string   `json:"blurb"`
	Text    []string `json:"text"`
	Opened  []string `json:"opened"`
	Next    string   `json:"next"`
}

// ProposalView is the deal put to a faction today.
type ProposalView struct {
	Faction string    `json:"faction"`
	Kind    string    `json:"kind"`
	Terms   TermsView `json:"terms"`
}

// DealView is a live deal with a faction (#32): its kind and terms, the
// day it was struck, the first day it no longer holds (0 none) and the
// days it has left, and whether the faction put it on the table.
type DealView struct {
	Kind   string    `json:"kind"`
	Terms  TermsView `json:"terms"`
	Since  int       `json:"since"`
	Until  int       `json:"until,omitempty"`
	Left   int       `json:"left,omitempty"`
	Theirs bool      `json:"theirs,omitempty"`
}

// CampaignView is the money behind a DA ticket in a city (#193).
type CampaignView struct {
	Ticket string `json:"ticket"`
	Cash   int    `json:"cash"`
	Hedged bool   `json:"hedged,omitempty"`
}

// EndingView is how the run ended.
type EndingView struct {
	Day   int    `json:"day"`
	Cause string `json:"cause"` // one of content.Causes
	Who   string `json:"who,omitempty"`

	// View 18 (#554): the summary's words, as the TUI's summary reads them.
	Title    string      `json:"title"`    // the ending's title (endings.toml)
	Won      bool        `json:"won"`      // an ending you chose, not one that befell you
	Epilogue string      `json:"epilogue"` // Session.Epilogue
	Story    []StoryView `json:"story"`    // Session.Story, in the order it happened
	Reached  int         `json:"reached"`  // the day the tier you ended at was entered (World.ReachedOn); 0 the first
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
	Stock     map[string]map[string]int `json:"stock"` // city id -> product id -> units stashed, the street and the houses (World.Stock)
	Upgrades  []string                  `json:"upgrades"`
	QuietDays int                       `json:"quiet_days"`
	Character string                    `json:"character,omitempty"`
	HardDA    bool                      `json:"hard_da,omitempty"`
	Ambition  string                    `json:"ambition,omitempty"` // the plan pinned (#347), an ambitions[] id

	// View 15 (#550).
	Till         int    `json:"till"`                     // the dirty cash the wash leaves in hand as you set it (#496); 0 is the float
	SweepOn      bool   `json:"sweep_on,omitempty"`       // the nightly sweep offshore is on (#478) ...
	SweepKeep    int    `json:"sweep_keep,omitempty"`     // ... leaving this much clean in hand
	War          string `json:"war,omitempty"`            // the faction the war order stands against (#229)
	Score        int    `json:"score"`                    // the account over one plus the bodies (#49)
	Bodies       int    `json:"bodies"`                   // the dead on your corners, both sides (#46)
	PagesDue     int    `json:"pages_due,omitempty"`      // pages last night's lump offshore files tonight (#494): the walk away waits
	PagesPending int    `json:"pages_pending,omitempty"`  // ... and today's reserve's, filed tomorrow night (#525)
	Reserved     int    `json:"reserved_today,omitempty"` // clean cash on its way offshore tonight (#195)

	// View 17 (#557).
	Quality map[string]map[string]float64 `json:"quality"` // city id -> product id -> the quality of the lot held there (#47, World.Quality), street and houses; a product with no units is absent
	Room    map[string]int                `json:"room"`    // city id -> the units the stash there has free (World.Free): what a cut adds and a cook lands into

	// View 18 (#554).
	Reign     int `json:"reign,omitempty"`      // the day the reign began (#227); 0 no reign
	ReignSlip int `json:"reign_slip,omitempty"` // the mornings the reign has been under the share (#399)

	// View 20 (#582).
	Away          map[string]int `json:"away"`           // city id -> the units it holds with you elsewhere (World.CapacityAway, #524): where you stand, its capacity without your carry
	StreetQuality float64        `json:"street_quality"` // the quality a lot is sold at by default (World.StreetQuality, #47)
}

// CityView is a city: its heat, its law and its market and corners.
type CityView struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Heat     float64       `json:"heat"`
	Pressure float64       `json:"pressure"`
	Goodwill float64       `json:"goodwill"`
	Response string        `json:"response,omitempty"`      // the police's next rung, as the file knows it
	Due      int           `json:"response_day,omitempty"`  // ... the first day it can fire
	Sure     float64       `json:"response_sure,omitempty"` // ... and how sure the word is today (#355): an estimate
	Ladder   []RungView    `json:"ladder"`                  // the police's lines here and what each takes (#355, heat.Sim.Rungs)
	Products []ProductView `json:"products"`
	Corners  []CornerView  `json:"corners"`
	Campaign *CampaignView `json:"campaign,omitempty"` // the money behind a DA ticket here (#193, #550)

	// View 20 (#582).
	Worked int `json:"worked"` // the corners worked here (World.WorkedIn), the demand's "on N corners"
}

// RungView is one rung of a city's police ladder (#355): the heat it
// fires at today and what it takes, as heat.Sim.Rungs folds it.
type RungView struct {
	Level     string  `json:"level"`
	Line      float64 `json:"line"`
	StockLoss float64 `json:"stock_loss,omitempty"` // share of the stock a bust takes
	CashLoss  float64 `json:"cash_loss,omitempty"`  // share of the dirty cash
	Pages     int     `json:"pages,omitempty"`      // pages it files on a day you sold
	Cap       float64 `json:"cap,omitempty"`        // a patrol: the share of demand it lets through, before the chief
	CapDays   int     `json:"cap_days,omitempty"`   // ... for this many days
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

	// View 20 (#582).
	Glut   float64 `json:"glut"`            // oversupply from recent selling, which pushes the price down (ProductMarket.Glut)
	Served float64 `json:"served"`          // the demand your corners here serve a day (World.Demand)
	Shock  float64 `json:"shock,omitempty"` // the shock's or the slump's multiplier on the price while ShockDays run
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
	Carry       int     `json:"carry"`             // the sell capacity they add
	Fee         int     `json:"fee,omitempty"`     // in the pool: what hiring them costs, dirty cash
	Trait       string  `json:"trait,omitempty"`   // what a veteran showed at the traits' days of service (#346)
	Captain     string  `json:"captain,omitempty"` // the city they are captain of (#346)
	Budget      int     `json:"budget,omitempty"`  // ... and their pay-off budget a night there

	// View 15 (#550).
	Wounded     int    `json:"wounded,omitempty"`      // days still laid up (#46)
	JailedUntil int    `json:"jailed_until,omitempty"` // the day they are out of the cell (#46)
	Bailed      bool   `json:"bailed,omitempty"`       // ... on bail you put down
	Exposed     bool   `json:"exposed,omitempty"`      // an investigation named them the informant; the unnamed are never marked
	Assigned    int    `json:"assigned,omitempty"`     // the day a lieutenant took their city
	Route       string `json:"route,omitempty"`        // the route a driver rides
	Lab         bool   `json:"lab,omitempty"`          // the chemist who cooks and cuts, the best on the payroll (#47); another waits
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

	// View 15 (#550).
	Target          map[string]int `json:"target"`                     // product id -> units the destination is kept stocked to
	DaysTarget      map[string]int `json:"days_target"`                // product id -> days of the destination's demand it is kept to
	CheckpointUntil int            `json:"checkpoint_until,omitempty"` // the day a live checkpoint or customs deal runs out (#42)
	ClosedUntil     int            `json:"closed_until,omitempty"`     // the day it opens again while Closed (#44)
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

	// View 20 (#582): the door, the terms and the record, as the TUI's
	// SUPPLIERS block and the connect's pane read them.
	DayCap      int                `json:"day_cap"`                // units they get you a day; Cap is what is left of it today
	Products    []string           `json:"products"`               // what they deal in; empty is everything sold in the city
	Quality     map[string]float64 `json:"quality"`                // product id -> the quality they sell it at, where it is not the street's
	SmallLot    float64            `json:"small_lot,omitempty"`    // the price multiplier on a buy under the lot; 1 none
	CreditRatio float64            `json:"credit_ratio,omitempty"` // the price multiplier on a unit taken on credit
	Locked      bool               `json:"locked,omitempty"`       // the door is not open to you yet ...
	UnlockCash  int                `json:"unlock_cash,omitempty"`  // ... until you have moved this much
	UnlockRel   float64            `json:"unlock_rel,omitempty"`   // ... and the street connect there is at this rel
	FrozenUntil int                `json:"frozen_until,omitempty"` // the day they take calls again, while they will not
	Warned      bool               `json:"warned,omitempty"`       // they tipped you off today
	Late        int                `json:"late,omitempty"`         // payments you have missed, lifetime
	Extended    bool               `json:"extended,omitempty"`     // a patient connect has let this debt ride once already
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
	Known    bool           `json:"known,omitempty"`  // the police have it in the file
	Price    int            `json:"price"`            // what it cost, dirty cash, once (#581)
	Rent     int            `json:"rent"`             // clean cash a day
	Bought   int            `json:"bought"`           // the day it was bought
	Unpaid   int            `json:"unpaid,omitempty"` // days the rent has gone unpaid in a row
}

// FrontView is a front you own.
type FrontView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Level  int    `json:"level"`
	Frozen bool   `json:"frozen,omitempty"`
	Washed int    `json:"washed"`

	// View 15 (#550): why it is shut, and since when it is yours.
	City        string `json:"city"`
	Bought      int    `json:"bought"`
	FrozenUntil int    `json:"frozen_until,omitempty"` // the day it opens again while Frozen
	Unpaid      int    `json:"unpaid,omitempty"`       // the upkeep the clean pile was short the night it shut (#458); 0 an audit's shut
	Audited     int    `json:"audited,omitempty"`      // the day of the last audit
}

// LaneView is an export lane (#391) as the ledger shows it (#405): open
// or what opens it (Needs, the asset's name), the standing order, the
// rate a unit abroad tonight on the ordered product, what it carries
// tonight, and the loads out with the next landing and what it pays.
type LaneView struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	City     string   `json:"city"`
	Mode     string   `json:"mode"`
	Days     int      `json:"days"`
	Products []string `json:"products"`
	Open     bool     `json:"open"`
	Needs    string   `json:"needs,omitempty"`
	Capacity int      `json:"capacity"` // units a night tonight
	Product  string   `json:"product,omitempty"`
	Units    int      `json:"units,omitempty"`
	Price    float64  `json:"price,omitempty"` // a unit abroad tonight on Product, glut and all
	Out      int      `json:"out"`             // loads at sea or in the air
	Lands    int      `json:"lands,omitempty"` // the day the next one lands
	Pays     int      `json:"pays,omitempty"`  // what the next one pays on landing
}

// TrophyView is a trophy you own (#392).
type TrophyView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Cost   int    `json:"cost"`
	Bought int    `json:"bought"`
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

	// View 15 (#550).
	DealTerms     []DealView `json:"deal_terms"`               // the deals live with it, whole, in the order of Deals
	Police        float64    `json:"police"`                   // the police's attention on it, 0..100: your tips (#70)
	LastRaid      int        `json:"last_raid,omitempty"`      // the day the police last took a corner off it on your tip
	TributeNights int        `json:"tribute_nights,omitempty"` // the nights of your takings a tribute is priced off (#532); 0 prices off the potential

	// View 18 (#554).
	City       string     `json:"city"`                  // the city it contests (World.CityOf)
	Stance     string     `json:"stance"`                // where it stands with you (World.Stance): quiet, war, truce, tribute, split, homage, not yet, fragmented, absorbed, scattered
	Absorbed   int        `json:"absorbed,omitempty"`    // the day it was absorbed or scattered (#43)
	AbsorbedBy string     `json:"absorbed_by,omitempty"` // ... by this faction; "" scattered
	Fragmented int        `json:"fragmented,omitempty"`  // the day it lost its leader
	Betrayed   int        `json:"betrayed,omitempty"`    // the day you last broke a deal with it (rules.rivals.distrusted reads the clock)
	SplitLines [][]string `json:"split_lines"`           // the three lines a split could run (World.SplitLinesWith): what you hold, plus the free corners on your side, plus every free corner
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
	ArrestLine   int    `json:"arrest_line"`   // the pages an indictment needs (#355, heat.Sim.EvidenceArrest); 0 with no file
	ExposureLine int    `json:"exposure_line"` // the dirty cash past which the pile draws heat (heat.Sim.ExposureLine)
	Cover        int    `json:"cover"`         // ... of which the fronts cover this much (heat.Sim.Cover)

	// View 15 (#550).
	Favours       int     `json:"favours,omitempty"`         // what the bought chief owes you (#228)
	CampaignOpen  bool    `json:"campaign_open,omitempty"`   // the tickets take money (#193)
	ChiefTermEnds int     `json:"chief_term_ends,omitempty"` // 0 a chief for life
	ChiefBought   int     `json:"chief_bought,omitempty"`    // the day a live bribe on the chief runs out (#42)
	DABought      int     `json:"da_bought,omitempty"`       // ... and on the DA
	Leads         int     `json:"leads,omitempty"`           // what the DA's office has heard about your envelopes
	SellCap       float64 `json:"sell_cap,omitempty"`        // a patrol's cap: the share of demand any sale moves, wherever ...
	SellCapDays   int     `json:"sell_cap_days,omitempty"`   // ... for this many days more ...
	SellCapCity   string  `json:"sell_cap_city,omitempty"`   // ... set by the patrol in this city

	// View 20 (#582).
	WatchUntil int `json:"watch_until,omitempty"` // the feds watch the skies until this day (#48): the plane route's risk is the file's before it
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

// ReportView is the morning report: the night's lead (#354), then its
// sections in the one order every front end draws them in
// (ReportSections), each a list of lines in words.
type ReportView struct {
	Day        int                 `json:"day"`
	Lead       []LeadView          `json:"lead"`     // the night's biggest changes, biggest first (#354): the report opens with them under TODAY
	Sections   []ReportSectionView `json:"sections"` // every section in ReportSections' order, empty ones too
	CashBefore int                 `json:"cash_before"`
	CashAfter  int                 `json:"cash_after"`
	Flow       FlowView            `json:"flow"` // the night's cash flow (#351): drawn in place of the money lines
}

// LeadView is one line of the lead (#354): the headlines.toml [digest]
// kind that scored it, the words, and what answers it, the act and the
// ids its subject names, as an alert's are.
type LeadView struct {
	Kind   string `json:"kind"`
	Text   string `json:"text"`
	Act    Act    `json:"act"`
	Member int    `json:"member,omitempty"`
	Corner string `json:"corner,omitempty"`
	City   string `json:"city,omitempty"`
	House  string `json:"house,omitempty"`
}

// ReportSectionView is one section of the report: its id
// (ReportSection's), its heading and its lines.
type ReportSectionView struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Lines []string `json:"lines"`
}

// ReportSection is one section of the morning report.
type ReportSection struct {
	ID    string // incident, tier, plan, unlocked, prices, sales, shipments, heat, law, intel, crew, territory, money, upgrades, news
	Title string // the heading: INCIDENT, TIER, ...
	Lines []string
}

// ReportSections is the report's sections in the one order every front
// end draws them in (#354): the world's incident, the tier, the plan
// pinned (#347) and the doors that opened first, the day is about them; then the market, the
// road, the police and the law, what was learnt, the crew and the
// ground; then the money, the upgrades and the paper. Every section is
// there, empty or not: a front end skips an empty one, or adds lines of
// its own (the TUI's crew trouble, the cash flow's waterfall).
func ReportSections(r *game.DayReport) []ReportSection {
	return []ReportSection{
		{"incident", "INCIDENT", r.Incident}, // the world's incident this morning (#44)
		{"tier", "TIER", r.Tier},             // the tier entered this morning (#147)
		{"plan", "PLAN", nil},                // the plan pinned (#347): the front end's, off the view's ambitions; the report holds none
		{"unlocked", "UNLOCKED", r.Unlocked}, // a gate crossed (#148)
		{"prices", "PRICES", r.Prices},
		{"sales", "SALES", r.Sales},
		{"shipments", "SHIPMENTS", r.Shipments},
		{"heat", "HEAT", r.Heat},
		{"law", "LAW", r.Law},
		{"intel", "INTEL", r.Intel}, // what was learnt tonight (#45)
		{"crew", "CREW", r.Crew},
		{"territory", "TERRITORY", r.Territory},
		{"money", "MONEY", r.Money},
		{"upgrades", "UPGRADES", r.Upgrades},
		{"news", "NEWS", r.News},
	}
}

// LeadAlert is a lead line as the alert its act opens (#354): the act
// and the ids its subject names, so a front end jumps from the lead the
// way it jumps from an alert.
func LeadAlert(l game.Line) Alert {
	return Alert{Act: l.Act, Member: l.Member, Corner: l.Corner, City: l.City, House: l.House}
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
		v.Over = &EndingView{Day: w.Over.Day, Cause: w.Over.Cause, Who: w.Over.Who,
			Title: s.cfg.Endings.Title(w.Over.Cause), Won: s.cfg.Endings.Won(w.Over.Cause), Epilogue: s.Epilogue(), Reached: w.ReachedOn(w.Tier())}
		if w.Over.Cause == content.CauseKingpin && w.Reign > 0 {
			v.Over.Reached = w.Reign // the reign's first morning (#227), as the summary's line reads it
		}
		for _, h := range s.Story() {
			v.Over.Story = append(v.Over.Story, StoryView{Day: h.Day, Text: h.Text})
		}
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
		Ambition:  w.Ambition,

		Till:         w.Laundering.Till,
		SweepOn:      w.Laundering.Sweep.On,
		SweepKeep:    w.Laundering.Sweep.Keep,
		War:          w.War,
		Score:        w.Score(),
		Bodies:       w.Stats.Bodies,
		PagesDue:     s.PagesDue(),
		PagesPending: s.PagesPending(),
		Reserved:     w.ReservedToday(),

		Reign:     w.Reign,
		ReignSlip: w.ReignSlip,
	}
	for _, id := range sortedKeys(w.Upgrades) {
		if w.Upgrades[id] {
			v.You.Upgrades = append(v.You.Upgrades, id)
		}
	}
	street := map[string]map[string]int{} // city -> product -> units, built here so the view shares no map with the world
	v.You.Quality, v.You.Room, v.You.Away = map[string]map[string]float64{}, map[string]int{}, map[string]int{}
	v.You.StreetQuality = w.StreetQuality()
	for _, cid := range w.CityOrder {
		c := w.Cities[cid]
		stock, quality := map[string]int{}, map[string]float64{}
		for _, pid := range w.Products {
			if n := w.Stock(cid, pid); n > 0 {
				stock[pid] = n
				quality[pid] = w.Quality(cid, pid)
			}
		}
		street[cid] = stock
		v.You.Quality[cid], v.You.Room[cid], v.You.Away[cid] = quality, w.Free(cid), w.CapacityAway(cid)
		cv := CityView{ID: c.ID, Name: c.Name, Heat: c.Heat, Pressure: c.Pressure, Goodwill: c.Goodwill, Worked: w.WorkedIn(cid)}
		// Today's backing included (World.Campaigning, #552), as the
		// TUI's race reads it: money put behind a ticket shows at once.
		if cp := w.Campaigning(cid); cp.Ticket != "" || cp.Cash > 0 {
			cv.Campaign = &CampaignView{Ticket: cp.Ticket, Cash: cp.Cash, Hedged: cp.Hedged}
		}
		if level, day, ok := known.Response(cid); ok {
			cv.Response, cv.Due = level, day
			if f, ok := known.Fact(cid, game.FactResponse); ok {
				cv.Sure = f.Now(w.Day)
			}
		}
		for _, r := range s.set.Heat.Rungs(w, c) {
			cv.Ladder = append(cv.Ladder, RungView{Level: r.Level, Line: r.Threshold, StockLoss: r.StockLoss, CashLoss: r.CashLoss, Pages: r.Evidence, Cap: r.Cap, CapDays: r.CapDays})
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
				Glut: p.Glut, Served: w.Demand(cid, pid),
			})
			if p.ShockDays > 0 {
				cv.Products[len(cv.Products)-1].Shock = p.ShockFactor
			}
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
		v.Offers = append(v.Offers, OfferView{ID: o.ID, Faction: o.With(), Kind: o.Deal.Kind, Expires: o.Expires, Terms: termsView(t)})
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
		mv := MemberView{ID: m.ID, Name: m.Name, Role: m.Role, Age: m.Age, Skill: m.Skill, Loyalty: m.Loyalty, Wage: m.Wage, Hired: m.Hired, City: m.City, Jailed: m.Jailed(w.Day), Carry: m.Units, Trait: m.Trait, Captain: m.Captain, Budget: m.Budget}
		if c := w.PostOf(m.ID); c != nil {
			mv.Post = c.ID
		}
		if m.Wounded(w.Day) {
			mv.Wounded = m.WoundedUntil - w.Day
		}
		if m.Jailed(w.Day) {
			mv.JailedUntil, mv.Bailed = m.JailedUntil, m.Bailed
		}
		// Named, never flagged (#550): the mark is the investigation's
		// answer, Crew.Exposed, so the unnamed informant reads as anyone.
		mv.Exposed = w.Crew.Exposed != 0 && w.Crew.Exposed == m.ID
		if m.Runs() {
			mv.Assigned = m.Assigned
		}
		if m.Role == game.RoleDriver {
			mv.Route = w.DrivenRoute(m.ID)
		}
		if m.Role == game.RoleChemist {
			best := w.Crew.Chemist()
			mv.Lab = best != nil && best.ID == m.ID
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
			rv := RouteView{ID: r.ID, Name: r.Name, Mode: r.Mode, From: r.From, To: r.To, Dial: rs.Dial.String(), Closed: rs.Closed(w.Day), Driver: rs.Driver,
				Target: copyInts(rs.Target), DaysTarget: copyInts(rs.Days)}
			if until, live := w.Checkpoint(r.ID); live {
				rv.CheckpointUntil = until
			}
			if rv.Closed {
				rv.ClosedUntil = rs.ClosedUntil
			}
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
		cv := ConnectView{ID: sup.ID, Name: sup.Name, City: sup.City, Wholesale: sup.Wholesale, Temper: sup.Temper, Open: sup.Open(w), Owned: sup.Owned, Prices: map[string]float64{}, Cap: sup.Left(), Lot: sup.Lot, CreditDays: sup.CreditDays, Limit: sup.Limit, Rel: sup.Rel, Debt: sup.Debt, DebtDue: sup.DebtDue,
			DayCap: sup.Cap, Products: append([]string{}, sup.Products...), Quality: map[string]float64{}, CreditRatio: sup.CreditRatio, Locked: sup.Locked(w), UnlockCash: sup.UnlockCash, UnlockRel: sup.UnlockRel,
			Warned: sup.Warned == w.Day && w.Day > 0, Late: sup.Late, Extended: sup.Extended}
		if sup.SmallLot > 1 {
			cv.SmallLot = sup.SmallLot
		}
		if sup.Frozen(w.Day) {
			cv.FrozenUntil = sup.FrozenUntil
		}
		for _, pid := range w.Products {
			if w.Available(sup, pid) {
				cv.Prices[pid] = sup.Price[pid]
			}
			if q := sup.QualityOf(w, pid); sup.Sells(pid) && q != w.StreetQuality() {
				cv.Quality[pid] = q
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
		v.Houses = append(v.Houses, HouseView{ID: h.ID, Name: h.Name, City: h.City, Corner: h.Corner, Capacity: h.Capacity, Stock: stock, Guard: h.Guard, Known: h.Known,
			Price: h.Price, Rent: h.Rent, Bought: h.Bought, Unpaid: h.Unpaid})
	}
	for _, f := range w.Fronts {
		fv := FrontView{ID: f.ID, Name: f.Name, Level: f.Level, Frozen: f.Frozen(w.Day), Washed: f.Washed,
			City: w.FrontCity(f), Bought: f.Bought, Unpaid: f.Unpaid, Audited: f.Audited}
		if fv.Frozen {
			fv.FrozenUntil = f.FrozenUntil
		}
		v.Fronts = append(v.Fronts, fv)
	}
	for _, a := range w.Assets {
		av := AssetView{ID: a.ID, Name: a.Name, Effect: a.Effect, City: a.City, Cost: a.Cost, Upkeep: a.Upkeep, Bought: a.Bought}
		if a.Frozen(w.Day) {
			av.FrozenUntil = a.FrozenUntil
		}
		v.Assets = append(v.Assets, av)
	}
	for _, k := range w.Crew.Cooks {
		v.Cooks = append(v.Cooks, CookView{ID: k.ID, City: k.City, Product: k.Product, Units: k.Units, Quality: k.Quality, Ordered: k.Ordered, Ready: k.Ready, Chemist: k.Chemist})
	}
	v.Orders = orderViews(w.Today.Orders)
	v.Standing = orderViews(w.Standing)
	v.Supply = s.supplyViews()
	v.Buys = []BuyView{}
	for _, b := range w.Today.Buys {
		v.Buys = append(v.Buys, BuyView{City: b.City, Product: b.Product, Qty: b.Qty, Unit: b.UnitPrice, Cost: b.Cost, Supplier: b.Supplier,
			Contract: b.Contract, Credit: b.Credit, SmallLot: b.SmallLot, Lieutenant: b.Lieutenant})
	}
	v.Exports = s.laneViews()
	for _, t := range w.Trophies {
		v.Trophies = append(v.Trophies, TrophyView{ID: t.ID, Name: t.Name, Cost: t.Cost, Bought: t.Bought})
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
			fv.DealTerms = append(fv.DealTerms, DealView{Kind: d.Kind, Terms: termsView(d.Terms), Since: d.Since, Until: d.Until, Left: d.Left(w.Day), Theirs: d.Offered})
		}
		fv.Police, fv.LastRaid = r.Heat, r.LastRaid
		if c := w.CityOf(r); c != nil && s.cfg.Rivals.Diplomacy.TributeDays > 0 {
			if _, n, ok := w.Taking(c.ID); ok {
				fv.TributeNights = n
			}
		}
		if c := w.CityOf(r); c != nil {
			fv.City = c.ID
		}
		fv.Stance = w.Stance(r, s.set.Rivals.Tuning().WarThreshold)
		fv.Absorbed, fv.AbsorbedBy, fv.Fragmented, fv.Betrayed = r.Absorbed, r.AbsorbedBy, r.Fragmented, r.Betrayed
		fv.SplitLines = w.SplitLinesWith(id)
		v.Factions = append(v.Factions, fv)
	}
	v.Law = LawView{Chief: w.Law.Chief.Name, ChiefTemper: known.Chief(), DA: w.Law.DA.Name, DAStance: w.Law.DA.Stance, NextElection: s.set.Law.NextElection(w),
		ArrestLine: s.set.Heat.EvidenceArrest(w), ExposureLine: s.set.Heat.ExposureLine(w), Cover: s.set.Heat.Cover(w),
		Favours: w.Law.Favours, CampaignOpen: w.Law.CampaignOpen, ChiefTermEnds: s.set.Law.ChiefTermEnds(w), Leads: w.Law.Leads}
	if w.Law.ChiefBoughtOn(w.Day) {
		v.Law.ChiefBought = w.Law.ChiefBought
	}
	if w.Law.DABoughtOn(w.Day) {
		v.Law.DABought = w.Law.DABought
	}
	if w.Heat.WatchUntil > w.Day {
		v.Law.WatchUntil = w.Heat.WatchUntil
	}
	if h := w.Heat; h.SellCapDays > 0 && h.SellCap > 0 {
		v.Law.SellCap, v.Law.SellCapDays, v.Law.SellCapCity = h.SellCap, h.SellCapDays, h.SellCapCity
	}
	if p := w.Today.Proposal; p != nil {
		v.Proposal = &ProposalView{Faction: p.With(), Kind: p.Kind, Terms: termsView(p.Terms)}
	}
	v.Stats = statsView(w)
	for _, f := range w.Crew.Fallen {
		v.Fallen = append(v.Fallen, FallenView{Name: f.Name, Role: f.Role, Day: f.Day})
	}
	v.Stage = s.stageView()
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
	v.Ambitions = s.ambitionViews()
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

// flowView is a cash flow as the view carries it, the big lines picked
// out at bigShare of the opening: the report's (#351) and the day's
// preview's (#353).
func flowView(f game.CashFlow, bigShare float64) FlowView {
	fv := FlowView{
		Opening: PoolsView{Dirty: f.Opening.Dirty, Clean: f.Opening.Clean},
		Closing: PoolsView{Dirty: f.Closing.Dirty, Clean: f.Closing.Clean},
		Net:     f.Net(),
	}
	for _, l := range f.Lines {
		fv.Lines = append(fv.Lines, FlowLineView{Cat: l.Cat, Label: game.FlowLabel(l.Cat), Dirty: l.Dirty, Clean: l.Clean, Big: f.Big(l, bigShare)})
	}
	return fv
}

// reportView is the morning report as the view carries it, its flow's
// big lines picked out at bigShare of the opening.
func reportView(r *game.DayReport, bigShare float64) ReportView {
	v := ReportView{Day: r.Day, Flow: flowView(r.Flow, bigShare), CashBefore: r.CashBefore, CashAfter: r.CashAfter}
	for _, l := range r.Lead {
		v.Lead = append(v.Lead, LeadView{Kind: l.Kind, Text: l.Text, Act: l.Act, Member: l.Member, Corner: l.Corner, City: l.City, House: l.House})
	}
	for _, sec := range ReportSections(r) {
		v.Sections = append(v.Sections, ReportSectionView{ID: sec.ID, Title: sec.Title, Lines: lines(sec.Lines)})
	}
	return v
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

// laneViews is every export lane as the view carries it (#405): the
// TUI's EXPORTS block, read the same way.
func (s *Session) laneViews() []LaneView {
	w, lg := s.w, s.set.Logistics
	var out []LaneView
	for _, l := range lg.Lanes() {
		lv := LaneView{ID: l.ID, Name: l.Name, City: l.City, Mode: l.Mode, Days: l.Days, Products: append([]string{}, l.Products...), Open: lg.LaneOpen(w, l)}
		if lv.Open {
			lv.Capacity = lg.LaneCapacity(w, l)
		} else {
			lv.Needs = s.laneNeeds(l)
		}
		if o := w.ExportOrder(l.ID); o.On() {
			lv.Product, lv.Units, lv.Price = o.Product, o.Units, lg.ExportPrice(w, l, o.Product)
		}
		if loads := w.ExportsOut(l.ID); len(loads) > 0 {
			lv.Out, lv.Lands, lv.Pays = len(loads), loads[0].Lands, loads[0].Revenue()
		}
		out = append(out, lv)
	}
	return out
}

// laneNeeds is the asset a shut lane waits on: the book when that is
// what is missing, else the lane's own.
func (s *Session) laneNeeds(l content.LaneConfig) string {
	w := s.w
	if b := s.cfg.Assets.ByEffect(content.AssetSupplier); b != nil && !w.AssetLive(b.ID) {
		return b.Name
	}
	if a := s.cfg.Assets.Asset(l.Asset); a != nil && !w.AssetLive(a.ID) {
		return a.Name
	}
	return ""
}

// termsView is a deal's terms as the view carries them.
func termsView(t game.Terms) TermsView {
	return TermsView{Days: t.Days, PerDay: t.PerDay, Corners: append([]string(nil), t.Corners...), Route: t.Route, Units: t.Units}
}

// copyInts copies a map, so the view shares none with the world.
func copyInts(m map[string]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, n := range m {
		out[k] = n
	}
	return out
}

// orderViews is a map of sell orders as the view carries them, in key
// order (city, then product).
func orderViews(orders map[string]game.SellOrder) []OrderView {
	var out []OrderView
	for _, k := range sortedKeys(orders) {
		o := orders[k]
		out = append(out, OrderView{City: o.City, Product: o.Product, Qty: o.Qty, Dial: o.Dial.String(), All: o.All})
	}
	return out
}

// supplyViews is the supply contracts (#113), yours then the
// lieutenants' (#174), each in key order; a lieutenant's names the one
// running its city.
func (s *Session) supplyViews() []SupplyContractView {
	w := s.w
	var out []SupplyContractView
	for _, k := range sortedKeys(w.Supply) {
		c := w.Supply[k]
		out = append(out, SupplyContractView{City: c.City, Product: c.Product, Units: c.Units, Since: c.Since})
	}
	for _, k := range sortedKeys(w.DelegatedSupply) {
		c := w.DelegatedSupply[k]
		sv := SupplyContractView{City: c.City, Product: c.Product, Units: c.Units, Since: c.Since}
		for _, m := range w.Crew.Members {
			if m.Runs() && m.City == c.City {
				sv.Lieutenant = m.ID
				break
			}
		}
		out = append(out, sv)
	}
	return out
}

// statsView is the summary's counters.
func statsView(w *game.World) StatsView {
	st := w.Stats
	return StatsView{
		Revenue: st.TotalRevenue, UnitsSold: st.UnitsSold, Laundered: st.Laundered, Seized: st.Seized,
		Wages: st.Wages, Skimmed: st.Skimmed, Robbed: st.Robbed, Cuts: st.Cuts, Invested: st.Invested,
		Earned: st.Earned, Taxed: st.Taxed, Reserved: st.Reserved, Fees: st.Fees, Fallen: st.Fallen,
		CornersWon: st.CornersWon, CornersLost: st.CornersLost, Stings: st.Stings, Raids: st.Raids,
		Betrayals: st.Betrayals, BetrayedBy: st.BetrayedBy, Defections: st.Defections, Walked: st.Walked,
		PeakHeat: w.Heat.Peak,
		Deals:    st.Deals, DealsRefused: st.DealsRefused, Tribute: st.Tribute, Homage: st.Homage,
		Informants: st.Informants, CrewPoached: st.CrewPoached,
	}
}

// stageView is the stage waiting to be seen (#149), or nil.
func (s *Session) stageView() *StageView {
	n := s.w.StagePending()
	if n <= 0 {
		return nil
	}
	sv := &StageView{Pending: n, Next: s.StageNext(n)}
	if tier := s.cfg.Progression.Tier(n); tier != nil {
		sv.Name, sv.Blurb = tier.Name, tier.Blurb
		sv.Text = append([]string(nil), tier.Text...)
		sv.Opened = append([]string(nil), tier.Opens...)
	}
	return sv
}

// StageNext is the NEXT line of tier n's stage (#149): the file's line
// on what the next stage takes, the closing line at the top of the
// ladder, or, where the next stage's line is crossed already, that it
// opens in the morning (#537: STAGE 2 shown on day 31 said `Move $25K:
// the laundromat opens` of a laundromat open since day 26). The news
// sim enters one stage a morning, the first whose trigger holds
// (game.Eligible over its enter), so that is the check. The TUI's stage
// modal and the view's stage read it.
func (s *Session) StageNext(n int) string {
	prog := s.cfg.Progression
	tier := prog.Tier(n)
	if tier == nil {
		return ""
	}
	if n == len(prog.Tiers) && tier.Closing != "" {
		return tier.Closing
	}
	next := prog.Tier(n + 1)
	if next == nil {
		return tier.Next
	}
	if _, ok := game.Eligible(s.w, content.CardConfig{Trigger: next.Enter}); ok {
		return fmt.Sprintf("You have crossed the line to %s already: it opens in the morning.", next.Name)
	}
	return tier.Next
}
