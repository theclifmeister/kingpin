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

// ProductUnlocked reports that the supplier now offers a new product.
type ProductUnlocked struct {
	Day     int
	Product string
	Name    string
	Price   float64
}

func (ProductUnlocked) Kind() string { return "ProductUnlocked" }

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
// sim their temper. Standing says the order was theirs, not the player's.
type PlayerSold struct {
	Day            int
	City           string
	Product        string
	Wanted         int
	Sold           int
	Dial           Dial
	AvgPrice       float64
	Revenue        int
	Lieutenant     int
	LieutenantName string
	Standing       bool
}

func (PlayerSold) Kind() string { return "PlayerSold" }

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
	Evidence  int  // what went in the DA's file; 0 when a sting or raid found nothing to build a case on
	Stash     bool // a raid that went straight to the stash: somebody told them where
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
	Day    int
	Corner string
	Name   string
	Rival  string
	From   string // none, player
	Handed string
}

func (CornerTaken) Kind() string { return "CornerTaken" }

// RivalPushed is a push on a player corner that was held off.
type RivalPushed struct {
	Day    int
	Corner string
	Name   string
	Rival  string
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

// ShipmentSent is report-only bookkeeping: a shipment that left yesterday,
// with how long it is expected to take.
type ShipmentSent struct {
	Day     int
	ID      int
	Route   string
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
}

func (LieutenantActed) Kind() string { return "LieutenantActed" }

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
