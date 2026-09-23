package content

import "fmt"

// HeadlinesConfig mirrors headlines.toml.
type HeadlinesConfig struct {
	FlavourChance float64             `toml:"flavour_chance"`
	Templates     map[string][]string `toml:"templates"`
	Flavour       []string            `toml:"flavour"`
	Swagger       []string            `toml:"swagger"` // the boss's headlines (#233): flavour that names you, while the city is yours
	Flow          FlowConfig          `toml:"flow"`    // the report's cash flow (#351)
}

// FlowConfig is the report's cash flow (#351): how many nights of it
// World.Flows keeps, the ledger's FLOW section's window, and the share of
// the night's opening cash past which a category's line is picked out.
type FlowConfig struct {
	Days     int     `toml:"days"`
	Shown    int     `toml:"shown"`
	BigShare float64 `toml:"big_share"`
}

func (c HeadlinesConfig) validate() error {
	f := c.Flow
	if f.Days < 1 || f.Shown < 1 || f.Shown > f.Days {
		return fmt.Errorf("flow: days %d and shown %d: want 1 <= shown <= days", f.Days, f.Shown)
	}
	if f.BigShare <= 0 || f.BigShare > 1 {
		return fmt.Errorf("flow: big_share %g: want 0 < big_share <= 1", f.BigShare)
	}
	return nil
}
