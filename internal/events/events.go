// Package events defines the typed events that simulations emit and the bus
// that delivers them to subscribers such as the UI.
package events

// Event is anything a simulation can emit. Kind returns a short stable name
// used for headline templates and logging.
type Event interface {
	Kind() string
}

// Dial is the risk dial attached to a sell order: every action trades money
// against heat.
type Dial int

const (
	DialQuiet Dial = iota
	DialNormal
	DialAggressive
)

func (d Dial) String() string {
	switch d {
	case DialQuiet:
		return "quiet"
	case DialAggressive:
		return "aggressive"
	default:
		return "normal"
	}
}

// Pay is the crew pay dial: every day the whole crew is paid stingy, fair or
// generous, trading cash against loyalty.
type Pay int

const (
	PayStingy Pay = iota
	PayFair
	PayGenerous
)

func (p Pay) String() string {
	switch p {
	case PayStingy:
		return "stingy"
	case PayGenerous:
		return "generous"
	default:
		return "fair"
	}
}

// DayEnded closes a day. It is always the last event of a tick.
type DayEnded struct{ Day int }

func (DayEnded) Kind() string { return "DayEnded" }

// Headline is a line of news produced by the news simulation.
type Headline struct {
	Day    int
	Source string // sim that caused it: market, heat, crew, territory, laundering, news
	Text   string
}

func (Headline) Kind() string { return "Headline" }

// PriceShock is a supply shock (Factor > 1) or demand slump (Slump) on a
// product in a city. Seized says a seizure on the road caused it.
type PriceShock struct {
	Day     int
	City    string
	Product string
	Factor  float64
	Days    int
	Slump   bool
	Seized  bool
}

func (PriceShock) Kind() string { return "PriceShock" }

// Unlocked is a gate crossed (#148): something the game kept behind a
// line is open to you from this morning. Every gate is announced through
// it, by the sim that owns the gate: the market for a product listed
// (Gate "product", in every city at once; Price is its base price) and a
// connect who will deal with you (Gate "connect", in their City); the
// laundering sim for a front whose offer opens (Gate "front"); the crew
// sim for a role that joins the hiring pool (Gate "role": the accountant
// once a front is owned, the lieutenant once corners are held in two
// cities). Why is the line crossed, in words the report prints (`peak
// cash $25K`, `a front owned`, `corners in two cities`, `Cass at 60`).
// The news sim's UNLOCKED section, the headline (`Unlocked` + the gate,
// capitalised: `UnlockedFront`), the fast-forward stop and nothing else
// read it: the law and reputation ignore it. (The gate is `Gate`, not
// `Kind`: `Kind()` is the event's.)
type Unlocked struct {
	Day   int
	Gate  string // product | front | connect | role
	ID    string
	Name  string
	City  string // the city it opens in, or "" for everywhere
	Why   string
	Price float64 // a product's base price
	Cost  int     // a front's price
}

func (Unlocked) Kind() string { return "Unlocked" }

// PriceMove reports a product's price in a city at the start and end of
// the day.
type PriceMove struct {
	Day      int
	City     string
	Product  string
	From, To float64
}

func (PriceMove) Kind() string { return "PriceMove" }

// PlayerSold is the resolution of a sell order at end of day, in the city
// whose corners moved it. Lieutenant names the crew member running that
// city, if one does (0 otherwise): the crew sim takes their cut, the heat
// sim their temper. Standing says the order stood rather than being
// placed that day, Delegated that it was theirs and not the player's.
type PlayerSold struct {
	Day             int
	City            string
	Product         string
	Wanted          int
	Sold            int
	Dial            Dial
	AvgPrice        float64
	Revenue         int
	Lieutenant      int
	LieutenantName  string
	Standing        bool // a standing order, yours (#114) or the lieutenant's, not one placed today
	Delegated       bool // the standing order was the lieutenant's (World.Delegated), not one you set
	Cut             int  // dirty cash the crew kept off a standing order of yours ([standing] cut); zero otherwise
	Undercut        int  // of Sold, the units served off the rival's corners by a price war (#68); PlayerUndercut has them per corner
	UndercutRevenue int  // of Revenue, what those units made at price_cut off
}

func (PlayerSold) Kind() string { return "PlayerSold" }

// StandingShort is report-only bookkeeping (#114): a standing order of
// yours found less in the stash than it is for tonight, and sold what
// was there (nothing, with nothing stashed). It is the cue to restock,
// and the stop a fast-forward reads (#116).
type StandingShort struct {
	Day     int
	City    string
	Product string
	Units   int // the order
	Stock   int // what the stash held
}

func (StandingShort) Kind() string { return "StandingShort" }

// HeatChanged reports the day's heat delta in a city and why.
type HeatChanged struct {
	Day      int
	City     string
	From, To float64
	Reasons  []string
}

func (HeatChanged) Kind() string { return "HeatChanged" }

// Enforcement is a police response triggered by a heat threshold, in the
// city whose heat crossed it; what it took came out of the stash there.
type Enforcement struct {
	Day       int
	City      string
	Level     string // patrol, sting, raid, arrest
	StockLost map[string]int
	CashLost  int
	Evidence  int    // what went in the DA's file; 0 when a sting or raid found nothing to build a case on
	Stash     bool   // a raid that went straight to the stash: somebody told them where
	House     string // the house the stock came out of (#73), "" for the street
	HouseName string
}

func (Enforcement) Kind() string { return "Enforcement" }

// LaidLow records that the player skipped trading to let heat decay.
type LaidLow struct{ Day int }

func (LaidLow) Kind() string { return "LaidLow" }

// GameOver ends the run.
type GameOver struct {
	Day   int
	Cause string
}

func (GameOver) Kind() string { return "GameOver" }

// CrewHired records a signing made during the day.
type CrewHired struct {
	Day  int
	Name string
	Role string
	Fee  int
}

func (CrewHired) Kind() string { return "CrewHired" }

// CrewFired records a member the player let go during the day. Informant
// says they had been talking to the police: the file stops growing, and
// the rest of the crew do not hold it against you.
type CrewFired struct {
	Day       int
	Name      string
	Role      string
	Informant bool
}

func (CrewFired) Kind() string { return "CrewFired" }

// CrewQuit records a member who walked because loyalty bottomed out.
type CrewQuit struct {
	Day  int
	Name string
	Role string
}

func (CrewQuit) Kind() string { return "CrewQuit" }

// CrewTurnedInformant is a member starting to talk to the police. It is
// bookkeeping for the heat sim, which starts the leak clock on it; it is
// never published as a headline, the roster never shows it, and the
// report only ever shows its effects.
type CrewTurnedInformant struct {
	Day  int
	ID   int
	Name string
}

func (CrewTurnedInformant) Kind() string { return "CrewTurnedInformant" }

// CrewDefected is a member whose loyalty bottomed out going over to the
// rival, taking what they knew about the corner they ran (Corner is empty
// if they ran none).
type CrewDefected struct {
	Day        int
	Name       string
	Role       string
	Rival      string
	Corner     string
	CornerName string
}

func (CrewDefected) Kind() string { return "CrewDefected" }

// InvestigationRun is the result of the player's questions: Found names
// the informant; otherwise nobody was named, whether or not there was
// one, and the crew resent being asked.
type InvestigationRun struct {
	Day   int
	Cost  int
	Found bool
	Name  string
}

func (InvestigationRun) Kind() string { return "InvestigationRun" }

// CrewSkimmed reports takings that went missing. It never names names.
// FromWash is the part of Amount an accountant took out of the wash, in
// clean cash.
type CrewSkimmed struct {
	Day      int
	Amount   int
	Skimmers int
	FromWash int
}

func (CrewSkimmed) Kind() string { return "CrewSkimmed" }

// CrewPaidOff is report-only bookkeeping: a member the player paid for
// loyalty during the day.
type CrewPaidOff struct {
	Day  int
	Name string
	Cost int
}

func (CrewPaidOff) Kind() string { return "CrewPaidOff" }

// CrewPaid is the day's wage bill. Short is what could not be covered.
type CrewPaid struct {
	Day   int
	Pay   Pay
	Wages int
	Short int
}

func (CrewPaid) Kind() string { return "CrewPaid" }

// CornerClaimed records a corner the player took during the day.
type CornerClaimed struct {
	Day    int
	Corner string // corner id
	Name   string
	Worker string // who is on it: "you" or a crew member's name
}

func (CornerClaimed) Kind() string { return "CornerClaimed" }

// CornerLost records a corner going back to the street: nobody worked it
// (idle) or the police cleared it in a crackdown, from either side.
type CornerLost struct {
	Day    int
	Corner string
	Name   string
	Reason string // idle, crackdown
	Owner  string // who lost it: player, rival
}

func (CornerLost) Kind() string { return "CornerLost" }

// CornerRobbed is a stick-up on a worked corner: cash and product gone.
type CornerRobbed struct {
	Day       int
	Corner    string
	Name      string
	Cash      int
	StockLost map[string]int
}

func (CornerRobbed) Kind() string { return "CornerRobbed" }

// Force is the dial on a strike against a rival corner: how hard the
// enforcers go in. Harder flips corners faster and draws more heat.
type Force int

const (
	ForceWarn Force = iota
	ForcePush
	ForceHit
)

func (f Force) String() string {
	switch f {
	case ForceWarn:
		return "warn"
	case ForceHit:
		return "hit"
	default:
		return "push"
	}
}

// RivalMovedIn is the rival faction's arrival: its first corner.
type RivalMovedIn struct {
	Day    int
	Rival  string // leader's name
	Corner string
	Name   string
}

func (RivalMovedIn) Kind() string { return "RivalMovedIn" }

// CornerTaken is a corner the rival gained: claimed free (From none) or
// flipped from the player (From player), whose people walked back. Handed
// names the defector who walked the rival onto it, if that is how.
type CornerTaken struct {
	Day      int
	Corner   string
	Name     string
	Rival    string
	From     string // none, player
	Handed   string
	Pricewar bool // the push was its answer to a price war on a corner next door (#68)
}

func (CornerTaken) Kind() string { return "CornerTaken" }

// RivalEyeing is the tell (#69): the rival has picked the free corner it
// sets up on tomorrow. Post somebody on it tonight and the claim fails.
type RivalEyeing struct {
	Day    int
	Rival  string
	Corner string
	Name   string
}

func (RivalEyeing) Kind() string { return "RivalEyeing" }

// RivalOutbid is a claim that failed: somebody was on the corner it
// eyed when it came. No cash spent; a grudge held.
type RivalOutbid struct {
	Day    int
	Rival  string
	Corner string
	Name   string
}

func (RivalOutbid) Kind() string { return "RivalOutbid" }

// RivalPushed is a push on a player corner that was held off.
type RivalPushed struct {
	Day      int
	Corner   string
	Name     string
	Rival    string
	Pricewar bool // the push was its answer to a price war on a corner next door (#68)
}

func (RivalPushed) Kind() string { return "RivalPushed" }

// CornerStruck is the resolution of the player's strike on a rival corner.
// Heat is what the strike drew; Toll is the loyalty an enforcer with no
// nerve loses over it (the crew sim scales it by nerve); Routed means it
// was the rival's last corner.
type CornerStruck struct {
	Day    int
	Corner string
	Name   string
	Rival  string
	Force  Force
	Taken  bool
	Routed bool
	Heat   float64
	Toll   float64
}

func (CornerStruck) Kind() string { return "CornerStruck" }

// RivalTippedPolice is the rival calling the cops on the player: heat.
type RivalTippedPolice struct {
	Day   int
	Rival string
	Heat  float64
}

func (RivalTippedPolice) Kind() string { return "RivalTippedPolice" }

// RivalUndercut is report-only bookkeeping: the corners the rival is
// undercutting the player on today and the share of their demand it steals.
type RivalUndercut struct {
	Day     int
	Rival   string
	Corners []string // corner names
	Share   float64
}

func (RivalUndercut) Kind() string { return "RivalUndercut" }

// WarEscalated marks the war getting loud (Stage open) or the police
// crushing both sides (Stage crackdown), when Lost names the corners
// cleared and Heat is what the player draws for it.
type WarEscalated struct {
	Day   int
	Stage string // open, crackdown
	War   float64
	Lost  []string // corner names cleared, both sides
	Heat  float64
}

func (WarEscalated) Kind() string { return "WarEscalated" }

// UpgradeBought records a node of the upgrade tree the player bought
// during the day. It is reported, not reacted to: the effect is already in
// force.
type UpgradeBought struct {
	Day    int
	ID     string
	Name   string
	Branch string
	Cost   int
	Clean  bool
}

func (UpgradeBought) Kind() string { return "UpgradeBought" }

// FallGuyBurned records the fall guy taking the indictment that would have
// ended the run: the case is closed, heat cools, half the cash goes with
// him, and he is gone.
type FallGuyBurned struct {
	Day      int
	CashLost int
}

func (FallGuyBurned) Kind() string { return "FallGuyBurned" }

// ReputationShifted is one of the player's reputation axes crossing a
// band line (a multiple of reputation.toml's band), either way. From and
// To are the axis before and after the day.
type ReputationShifted struct {
	Day      int
	Axis     string // fear, respect, notoriety
	From, To float64
}

func (ReputationShifted) Kind() string { return "ReputationShifted" }

// Up reports whether the axis crossed the line going up.
func (r ReputationShifted) Up() bool { return r.To > r.From }

// Launder is the laundering dial: how hard every front is pushed, trading
// throughput against audits.
type Launder int

const (
	LaunderCareful Launder = iota
	LaunderNormal
	LaunderGreedy
)

func (l Launder) String() string {
	switch l {
	case LaunderCareful:
		return "careful"
	case LaunderGreedy:
		return "greedy"
	default:
		return "normal"
	}
}

// FrontBought records a front the player bought during the day.
type FrontBought struct {
	Day   int
	Front string // front id
	Name  string
	Cost  int
}

func (FrontBought) Kind() string { return "FrontBought" }

// FrontAudited is the taxman looking at a front's books: it is frozen for
// Days and part of what it washed today is seized.
type FrontAudited struct {
	Day    int
	Front  string
	Name   string
	Dial   Launder // the dial the front was run at when the audit hit
	Seized int
	Days   int
}

func (FrontAudited) Kind() string { return "FrontAudited" }

// FrontFrozen is a front shut because its upkeep went unpaid.
type FrontFrozen struct {
	Day    int
	Front  string
	Name   string
	Upkeep int
	Days   int
}

func (FrontFrozen) Kind() string { return "FrontFrozen" }

// CashLaundered is the day's wash: dirty cash turned clean across every
// open front, and the upkeep paid for it. Report-only bookkeeping.
type CashLaundered struct {
	Day    int
	Amount int
	Upkeep int
	Fronts int // fronts that washed something
}

func (CashLaundered) Kind() string { return "CashLaundered" }

// DilemmaDrawn is a card put in front of the player overnight, to be
// answered before the morning report. Report-only bookkeeping: the card is
// the player's business, not the paper's.
type DilemmaDrawn struct {
	Day   int
	Card  string // card id
	Title string
}

func (DilemmaDrawn) Kind() string { return "DilemmaDrawn" }

// DilemmaAnswered is the choice the player made on yesterday's card. Its
// outcome is already in the journal and any follow-up headline comes from
// the card itself, so it has no template.
type DilemmaAnswered struct {
	Day    int
	Card   string
	Choice string
}

func (DilemmaAnswered) Kind() string { return "DilemmaAnswered" }

// Ship is the shipping dial: how fast a shipment is pushed over its route,
// trading days in transit against the chance of a seizure on each.
type Ship int

const (
	ShipSlow Ship = iota
	ShipNormal
	ShipFast
)

func (s Ship) String() string {
	switch s {
	case ShipSlow:
		return "slow"
	case ShipFast:
		return "fast"
	default:
		return "normal"
	}
}

// RouteDial is the route dial (#61): a persistent setting per route, off or
// the ship dial the logistics sim runs the route at every day. Off is the
// zero value, so a route nobody has touched runs nothing.
type RouteDial int

const (
	RouteOff RouteDial = iota
	RouteSlow
	RouteNormal
	RouteFast
)

func (r RouteDial) String() string {
	switch r {
	case RouteSlow:
		return "slow"
	case RouteNormal:
		return "normal"
	case RouteFast:
		return "fast"
	default:
		return "off"
	}
}

// On reports whether the route runs at all.
func (r RouteDial) On() bool { return r > RouteOff && r <= RouteFast }

// Ship is the ship dial a running route sends at; normal for one that is
// off, which never sends.
func (r RouteDial) Ship() Ship {
	switch r {
	case RouteSlow:
		return ShipSlow
	case RouteFast:
		return ShipFast
	default:
		return ShipNormal
	}
}

// WholesaleBought is report-only bookkeeping: lots the logistics sim
// bought at the source of a route to cover the far city's shortfall.
type WholesaleBought struct {
	Day      int
	City     string // city id the lots were bought in
	Route    string // route id they were bought for
	Name     string // the route's name, for the report
	Product  string
	Lots     int
	Units    int
	Cost     int    // dirty cash
	Supplier string // the wholesale connect they came from (#72)
}

func (WholesaleBought) Kind() string { return "WholesaleBought" }

// SupplyBought is report-only bookkeeping: what a supply contract (#113)
// bought from the supplier in its city this morning, at the contract
// markup, to bring the stash there back to its level. Lieutenant names
// the lieutenant whose contract it was (#174), empty for your own; the
// crew sim reads it for the report's CREW line.
type SupplyBought struct {
	Day        int
	City       string
	Product    string
	Units      int
	Level      int     // the contract's level
	Price      float64 // per unit paid, the markup included
	Cost       int     // dirty cash
	Supplier   string  // the connect it bought from (#72)
	Lieutenant string  // the lieutenant whose contract it was (#174); empty for yours
}

func (SupplyBought) Kind() string { return "SupplyBought" }

// SupplierBought is report-only bookkeeping (#72): a buy by hand from a
// connect, as the market sim reads it off the day's receipts the next
// morning, with whether it went on credit and whether it was under the
// lot.
type SupplierBought struct {
	Day      int
	City     string
	Supplier string // connect id
	Name     string
	Product  string
	Units    int
	Price    float64 // per unit paid
	Cost     int
	Credit   bool
	SmallLot bool
}

func (SupplierBought) Kind() string { return "SupplierBought" }

// CreditTaken is report-only bookkeeping (#72): what a connect put on
// your book yesterday, all of it, what you owe them now and when it is
// due.
type CreditTaken struct {
	Day      int
	City     string
	Supplier string
	Name     string
	Amount   int // taken yesterday
	Debt     int // owed now
	Due      int // the day
}

func (CreditTaken) Kind() string { return "CreditTaken" }

// DebtPaid is report-only bookkeeping (#72): a debt cleared on its day,
// dirty cash first, then clean.
type DebtPaid struct {
	Day      int
	City     string
	Supplier string
	Name     string
	Amount   int
}

func (DebtPaid) Kind() string { return "DebtPaid" }

// DebtLate is a debt short on its day (#72): what was owed, what could
// be paid, and what the temper did about it (What: "extended" for the
// patient one the first time, "frozen" for the sharp one and the
// patient one the second, "collected" for the connected one, who sent
// somebody), the fee a sharp one added, and what is left and when it
// is due again.
type DebtLate struct {
	Day      int
	City     string
	Supplier string
	Name     string
	Temper   string
	Owed     int
	Paid     int
	Fee      int
	Left     int
	Due      int
	What     string
}

func (DebtLate) Kind() string { return "DebtLate" }

// SupplierFrozen is a connect that has stopped taking your calls (#72),
// for Days: after a late payment (Why "late") or with the relationship
// at the floor ("floor"). Nothing sells from them, to you or to the
// road, until it lifts.
type SupplierFrozen struct {
	Day      int
	City     string
	Supplier string
	Name     string
	Days     int
	Why      string
}

func (SupplierFrozen) Kind() string { return "SupplierFrozen" }

// SupplierWarned is a connect at the top of the ladder tipping you off
// (#72): a shock, or a slump, on a product they sell in their city,
// tomorrow.
type SupplierWarned struct {
	Day      int
	City     string
	Supplier string
	Name     string
	Product  string
	Slump    bool
}

func (SupplierWarned) Kind() string { return "SupplierWarned" }

// SupplierCollected is the connected temper sending somebody for a
// short payment (#72): your best enforcer lost Loyalty and, Hurt, took
// a beating (skill lost, off their corner); with no enforcer on the
// payroll they took Units of Product from the stash in their city
// against what was Owed.
type SupplierCollected struct {
	Day        int
	City       string
	Supplier   string
	Name       string
	Owed       int
	Member     int
	MemberName string
	Loyalty    float64
	Hurt       bool
	Units      int
	Product    string
}

func (SupplierCollected) Kind() string { return "SupplierCollected" }

// SupplyShort is report-only bookkeeping: a supply contract that could
// not bring the stash to its level this morning, for want of cash over
// the float or of room in the stash, and what it bought instead.
type SupplyShort struct {
	Day        int
	City       string
	Product    string
	Units      int    // bought
	Short      int    // still under the level
	Why        string // "cash", "room", or "supplier" when no connect there sells it today (#72)
	Lieutenant string // the lieutenant whose contract it was (#174); empty for yours
}

func (SupplyShort) Kind() string { return "SupplyShort" }

// ShipmentSent is report-only bookkeeping: a shipment the route put on
// the road this morning, with how long it is expected to take.
type ShipmentSent struct {
	Day     int
	ID      int
	Route   string
	Name    string // the route's name, for the report
	Mode    string
	From    string // city ids
	To      string
	Product string
	Units   int
	Cost    int
	Dial    Ship
	Days    int
}

func (ShipmentSent) Kind() string { return "ShipmentSent" }

// ShipmentArrived is report-only bookkeeping: a shipment landing in the
// destination's stash.
type ShipmentArrived struct {
	Day     int
	ID      int
	Route   string
	Mode    string
	From    string
	To      string
	Product string
	Units   int
}

func (ShipmentArrived) Kind() string { return "ShipmentArrived" }

// ShipmentSeized is a shipment intercepted on the road: every unit is
// gone. Heat reacts in both cities (and the DA gets a page if it was sent
// fast); the market in the city it was bound for reacts the next morning
// with a supply shock. It is a market shock, not a bust: what was seized
// was never sold.
type ShipmentSeized struct {
	Day     int
	ID      int
	Route   string
	Mode    string
	From    string
	To      string
	Product string
	Units   int
	Dial    Ship
}

func (ShipmentSeized) Kind() string { return "ShipmentSeized" }

// DealOffered is the rival putting a deal on the table: it sits in
// World.Offers until the player answers or it expires. Terms is the deal
// in words.
type DealOffered struct {
	Day     int
	ID      int
	Rival   string
	Deal    string // truce, tribute, split
	Terms   string
	Expires int // last day it can be accepted
}

func (DealOffered) Kind() string { return "DealOffered" }

// DealAccepted is a deal struck: the rival took the player's proposal
// (Offered false) or the player took the rival's offer (Offered true).
type DealAccepted struct {
	Day     int
	Rival   string
	Deal    string
	Terms   string
	Until   int // last day it runs; 0 for a deal with no end
	Offered bool
}

func (DealAccepted) Kind() string { return "DealAccepted" }

// DealRefused is the rival turning the player's proposal down.
type DealRefused struct {
	Day   int
	Rival string
	Deal  string
	Terms string
}

func (DealRefused) Kind() string { return "DealRefused" }

// DealBroken is a live deal betrayed: by the player (By "you": a push or
// hit, a missed tribute, a split corner abandoned) or by the rival (By
// "rival"). Why says how.
type DealBroken struct {
	Day   int
	Rival string
	Deal  string
	By    string // you, rival
	Why   string
}

func (DealBroken) Kind() string { return "DealBroken" }

// DealEnded is report-only bookkeeping: a deal that ran its course.
type DealEnded struct {
	Day   int
	Rival string
	Deal  string
}

func (DealEnded) Kind() string { return "DealEnded" }

// TributePaid is report-only bookkeeping: the day's tribute handed over.
type TributePaid struct {
	Day    int
	Rival  string
	Amount int
}

func (TributePaid) Kind() string { return "TributePaid" }

// LieutenantFlipped is a lieutenant turning informant: their loyalty fell
// under the line and they know where everything is. Like
// CrewTurnedInformant it is bookkeeping for the heat sim, never a
// headline; the tells are the same, the pages come thicker.
type LieutenantFlipped struct {
	Day  int
	ID   int
	Name string
	City string
}

func (LieutenantFlipped) Kind() string { return "LieutenantFlipped" }

// LieutenantWalked is a lieutenant leaving with the city they ran: every
// corner they held there is gone (to the rival, named, if it holds
// ground there, else back to the street) and so is the stash.
type LieutenantWalked struct {
	Day      int
	ID       int
	Name     string
	City     string
	CityName string
	Corners  []string // names of the corners that went with them
	Units    int      // stock lost with the stash
	Rival    string   // who the corners went to, "" for the street
}

func (LieutenantWalked) Kind() string { return "LieutenantWalked" }

// LieutenantActed is report-only bookkeeping: what a lieutenant did for
// their city tonight, and what it cost. Personality is named once the
// player has seen enough of them (Revealed on the first such morning).
type LieutenantActed struct {
	Day         int
	ID          int
	Name        string
	City        string
	CityName    string
	Personality string
	Revealed    bool
	Dial        Dial
	Posted      []string // corner names runners were put on
	Guarded     []string // corner names enforcers were put on
	Dropped     []string // corner names given up, after a second robbery or for want of a runner
	Orders      int      // standing orders placed for tomorrow
	Revenue     int      // what their city took today
	Cut         int      // what they kept of it
	Skimmed     int      // what a greedy one took on top; the report never says so
	Bought      []Bought // what their supply contracts bought this morning (#174), in ladder order
	Contracts   int      // supply contracts kept for tomorrow (#174)
}

func (LieutenantActed) Kind() string { return "LieutenantActed" }

// Bought is one product a lieutenant's contract bought this morning:
// the units and what they cost, for the report's CREW line.
type Bought struct {
	Product string
	Units   int
	Cost    int
}

// DAElected is the district attorney's race decided (#41): who won and
// on what ticket (law_and_order, moderate, reform). Incumbent says the
// sitting DA kept the seat. Pressure is the mean public pressure the
// vote swung on.
type DAElected struct {
	Day       int
	Name      string
	Stance    string
	Incumbent bool
	Pressure  float64
}

func (DAElected) Kind() string { return "DAElected" }

// ChiefReplaced is a new police chief taking office (#41): on schedule
// (Why "term") or because a law-and-order DA wanted one (Why "da"). The
// personality stays hidden until the player has seen them work.
type ChiefReplaced struct {
	Day  int
	Name string
	Old  string
	Why  string // term, da
}

func (ChiefReplaced) Kind() string { return "ChiefReplaced" }

// PressureShifted is a city's public pressure crossing a band line (a
// multiple of law.toml's band), either way.
type PressureShifted struct {
	Day      int
	City     string
	From, To float64
}

func (PressureShifted) Kind() string { return "PressureShifted" }

// Up reports whether pressure crossed the line going up.
func (p PressureShifted) Up() bool { return p.To > p.From }

// CityFunded is report-only bookkeeping: clean cash the player gave a
// city today and the goodwill it bought.
type CityFunded struct {
	Day      int
	City     string
	Amount   int
	Goodwill float64
}

func (CityFunded) Kind() string { return "CityFunded" }

// ContractOffered is a buyer putting an order on the table (#71): so
// many units of a product in a city, by a day, at Premium times that
// city's street price on the day it is handed over. It sits in
// World.Contracts until the player answers it on the market screen or it
// lapses on Expires.
type ContractOffered struct {
	Day     int
	ID      int
	Buyer   string // deck id
	Name    string // the buyer as the offer names them
	Pitch   string // what they said
	City    string
	Product string
	Units   int
	Premium float64
	Due     int // last day to deliver
	Expires int // last day to answer
}

func (ContractOffered) Kind() string { return "ContractOffered" }

// ContractAccepted is report-only bookkeeping: a contract the player took
// during the day.
type ContractAccepted struct {
	Day     int
	ID      int
	Name    string
	City    string
	Product string
	Units   int
	Due     int
}

func (ContractAccepted) Kind() string { return "ContractAccepted" }

// ContractDelivered is a handoff against a contract: Units moved out of
// the city's stash at Price each (Street times the premium, on the
// day), for Revenue in dirty cash. Complete says the contract is now
// delivered in full, and Respect is what that earns (0 for a part). It
// is off-corner: it consumed no demand and took no dial, so the heat sim
// weights it by HeatMul alone, with no corner, and the law sim counts it
// as product sold in City. No lieutenant takes a cut: it is your
// handoff, not the city's takings.
type ContractDelivered struct {
	Day      int
	ID       int
	Buyer    string
	Name     string
	City     string
	Product  string
	Units    int     // handed over today
	Owed     int     // still to deliver after today
	Total    int     // the contract's size
	Price    float64 // per unit paid
	Street   float64 // the street price in City today
	Signed   float64 // the street price the day the offer came: what the bet was against
	Revenue  int
	Complete bool
	HeatMul  float64
	Respect  float64
}

func (ContractDelivered) Kind() string { return "ContractDelivered" }

// ContractFailed is a contract short at its due day: Delivered of Units
// went over, Cash was taken for the rest, Respect is lost and Notoriety
// gained, and the buyer stays away until Blacklisted.
type ContractFailed struct {
	Day         int
	ID          int
	Buyer       string
	Name        string
	City        string
	Product     string
	Units       int
	Delivered   int
	Cash        int
	Respect     float64 // positive: what respect loses
	Notoriety   float64 // what notoriety gains
	Blacklisted int     // first day the buyer will deal again
}

func (ContractFailed) Kind() string { return "ContractFailed" }

// ContractExpired is report-only bookkeeping: an offer that lapsed
// unanswered.
type ContractExpired struct {
	Day     int
	ID      int
	Name    string
	City    string
	Product string
	Units   int
}

func (ContractExpired) Kind() string { return "ContractExpired" }

// PlayerUndercut is report-only bookkeeping (#68): the units of a product
// tonight's order moved off one of the rival's corners at price_cut off,
// the share of that corner's demand for the product they were, and what
// they made. The units are part of the night's PlayerSold, which every
// sim counts them in; this is the breakdown per corner.
type PlayerUndercut struct {
	Day     int
	Corner  string // corner id
	Name    string
	Product string
	Units   int
	Share   float64 // of the corner's demand for the product
	Revenue int
	Dial    Dial // the undercut's dial
}

func (PlayerUndercut) Kind() string { return "PlayerUndercut" }

// RivalAbandoned is the rival giving a corner back to the street of its
// own accord: a price war (#68, Reason pricewar) made it not worth
// holding. The corner is free: post a runner before it drifts to
// somebody else.
type RivalAbandoned struct {
	Day    int
	Rival  string
	Corner string
	Name   string
	Reason string // pricewar
}

func (RivalAbandoned) Kind() string { return "RivalAbandoned" }

// The books (#70): the player's moves against the rival's machine.

// RivalScouted is report-only bookkeeping: the night's look at the
// rival's books. Read says whether it read them; the snapshot itself is
// Rival.Known.
type RivalScouted struct {
	Day  int
	Cost int
	Read bool
}

func (RivalScouted) Kind() string { return "RivalScouted" }

// RivalBoosted is the resolution of the player's enforcers going in on a
// rival corner for its takings rather than the ground. Taken says the
// boost landed and Cash what it took (into dirty cash, off the rival's
// chest); Heat is what it drew; Toll the loyalty an enforcer with no
// nerve loses over it (the crew sim scales it by nerve); Hurt the skill
// one enforcer loses on a failure against real muscle, 0 for none.
type RivalBoosted struct {
	Day    int
	Corner string
	Name   string
	Rival  string
	Force  Force
	Taken  bool
	Cash   int
	Heat   float64
	Toll   float64
	Hurt   int
}

func (RivalBoosted) Kind() string { return "RivalBoosted" }

// PoliceTipped is report-only bookkeeping: your tip on a rival corner
// landed. RivalHeat is where their heat stands after it; Betrayal says
// it broke a deal.
type PoliceTipped struct {
	Day       int
	Corner    string
	Name      string
	Rival     string
	RivalHeat float64
	Betrayal  bool
}

func (PoliceTipped) Kind() string { return "PoliceTipped" }

// RivalRaided is the police taking a rival corner on your tips: the
// corner goes back to the street and Muscle heads are gone. The mirror
// of RivalTippedPolice.
type RivalRaided struct {
	Day    int
	Corner string
	Name   string
	Rival  string
	Muscle int
}

func (RivalRaided) Kind() string { return "RivalRaided" }

// RivalMusclePoached is the resolution of the player buying off the
// rival's muscle: Wanted heads paid for, Got sent home (never more than
// it had; Refund is what came back for the rest), or none when the order
// failed (Got 0, the money gone, a grudge held). They never join you.
type RivalMusclePoached struct {
	Day    int
	Rival  string
	Wanted int
	Got    int
	Cost   int
	Refund int
	Failed bool
}

func (RivalMusclePoached) Kind() string { return "RivalMusclePoached" }

// TierReached is the run entering a progression tier (#147): the news
// sim stamps it the first morning the tier's trigger holds, one tier a
// morning, so a night that crosses two lines is two mornings. Tier is
// the tier's number (1 the first, never emitted: it is day 0), Name the
// file's name for it. Nothing gates on it: the tier describes the gates.
type TierReached struct {
	Day  int
	Tier int
	Name string
}

func (TierReached) Kind() string { return "TierReached" }

// The stash houses (#73).

// HouseBought is a lease taken on a stash house: dirty cash once, clean
// cash a day from tomorrow.
type HouseBought struct {
	Day   int
	House string // house id
	Name  string
	City  string
	Price int
	Rent  int
}

func (HouseBought) Kind() string { return "HouseBought" }

// HouseRobbed is a stash house stuck up: robbery_stock of what it held,
// and now the street knows where it is (Known).
type HouseRobbed struct {
	Day       int
	House     string
	Name      string
	City      string
	Corner    string // the block's name
	Guarded   bool
	StockLost map[string]int
}

func (HouseRobbed) Kind() string { return "HouseRobbed" }

// HouseRaided is the police hitting one stash house in a sting or a
// raid: what came out of it, and whether they took the lot (Whole: an
// informant told them where). The Enforcement for the same night names
// the house too; this is the house's own record and headline.
type HouseRaided struct {
	Day       int
	House     string
	Name      string
	City      string
	Level     string // sting or raid
	Whole     bool
	StockLost map[string]int
}

func (HouseRaided) Kind() string { return "HouseRaided" }

// HouseLost is the landlord throwing you out: the rent went unpaid
// rent_days running, and the stock went with the house.
type HouseLost struct {
	Day   int
	House string
	Name  string
	City  string
	Units int // what was in it
}

func (HouseLost) Kind() string { return "HouseLost" }

// HouseCompromised is a house the police now know about (Known): an
// informant on the payroll, a robbery (word gets out) or a bust there.
// It is the one the raid finds until it is dropped.
type HouseCompromised struct {
	Day   int
	House string
	Name  string
	City  string
	Why   string // informant, robbery, bust
}

func (HouseCompromised) Kind() string { return "HouseCompromised" }

// StockMoved is report-only bookkeeping (#73): units moved between two
// places in a city today, and the heat the drive drew.
type StockMoved struct {
	Day     int
	City    string
	From    string // house name, or "the street"
	To      string
	Product string
	Units   int
}

func (StockMoved) Kind() string { return "StockMoved" }

// RentPaid is report-only bookkeeping (#73): the day's rent on the
// houses, clean cash, and the houses it could not be paid for.
type RentPaid struct {
	Day    int
	Amount int
	Houses int
	Unpaid []string // names of the houses whose rent went unpaid today
}

func (RentPaid) Kind() string { return "RentPaid" }
