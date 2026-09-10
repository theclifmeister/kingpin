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
// whose corners moved it.
type PlayerSold struct {
	Day      int
	City     string
	Product  string
	Wanted   int
	Sold     int
	Dial     Dial
	AvgPrice float64
	Revenue  int
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
