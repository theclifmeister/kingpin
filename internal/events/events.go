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
