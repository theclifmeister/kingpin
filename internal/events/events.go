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
