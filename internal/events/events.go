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
	Source string // sim that caused it: market, heat, news
	Text   string
}

func (Headline) Kind() string { return "Headline" }

// PriceShock is a supply shock (Factor > 1) or demand slump (Slump) on a product.
type PriceShock struct {
	Day     int
	Product string
	Factor  float64
	Days    int
	Slump   bool
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

// PriceMove reports a product's price at the start and end of the day.
type PriceMove struct {
	Day      int
	Product  string
	From, To float64
}

func (PriceMove) Kind() string { return "PriceMove" }

// PlayerSold is the resolution of a sell order at end of day.
type PlayerSold struct {
	Day      int
	Product  string
	Wanted   int
	Sold     int
	Dial     Dial
	AvgPrice float64
	Revenue  int
}

func (PlayerSold) Kind() string { return "PlayerSold" }

// HeatChanged reports the day's heat delta and why.
type HeatChanged struct {
	Day      int
	From, To float64
	Reasons  []string
}

func (HeatChanged) Kind() string { return "HeatChanged" }

// Enforcement is a police response triggered by a heat threshold.
type Enforcement struct {
	Day       int
	Level     string // patrol, sting, raid, arrest
	StockLost map[string]int
	CashLost  int
	Evidence  int // what went in the DA's file; 0 when a sting or raid found nothing to build a case on
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

// CrewFired records a member the player let go during the day.
type CrewFired struct {
	Day  int
	Name string
	Role string
}

func (CrewFired) Kind() string { return "CrewFired" }

// CrewQuit records a member who walked because loyalty bottomed out.
type CrewQuit struct {
	Day  int
	Name string
	Role string
}

func (CrewQuit) Kind() string { return "CrewQuit" }

// CrewSkimmed reports takings that went missing. It never names names.
type CrewSkimmed struct {
	Day      int
	Amount   int
	Skimmers int
}

func (CrewSkimmed) Kind() string { return "CrewSkimmed" }

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
// flipped from the player (From player), whose people walked back.
type CornerTaken struct {
	Day    int
	Corner string
	Name   string
	Rival  string
	From   string // none, player
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
