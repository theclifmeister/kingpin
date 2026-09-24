package content

import (
	"fmt"
	"slices"
)

// HeadlinesConfig mirrors headlines.toml.
type HeadlinesConfig struct {
	FlavourChance float64             `toml:"flavour_chance"`
	Templates     map[string][]string `toml:"templates"`
	Flavour       []string            `toml:"flavour"`
	Swagger       []string            `toml:"swagger"` // the boss's headlines (#233): flavour that names you, while the city is yours
	Flow          FlowConfig          `toml:"flow"`    // the report's cash flow (#351)
	Digest        DigestConfig        `toml:"digest"`  // the morning's lead (#354)
}

// DigestConfig is the morning's lead (#354): how many lines it holds,
// the week the cash flow's swing is read against, the smallest swing
// that is a line, and the weight of each kind of change. A line's score
// is its weight times its count (corners, crew, pages, idle runners;
// the swing's share of the week, capped at swing_cap), the biggest
// first. A report: no sim reads it and it rolls no dice.
type DigestConfig struct {
	Lines    int                `toml:"lines"`
	Week     int                `toml:"week"`
	MinSwing float64            `toml:"min_swing"`
	SwingCap float64            `toml:"swing_cap"`
	Weights  map[string]float64 `toml:"weights"`
}

// DigestKinds are the lead's kinds of change, in the order a tie is
// broken: every one has a weight in [digest.weights] and no other key
// does.
var DigestKinds = []string{"corner_lost", "crew_lost", "seizure", "pages", "flow", "faction", "corner_won", "idle_corner", "idle_runner"}

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
	d := c.Digest
	if d.Lines < 1 || d.Week < 1 || d.Week > f.Days {
		return fmt.Errorf("digest: lines %d and week %d: want lines >= 1 and 1 <= week <= flow days (%d)", d.Lines, d.Week, f.Days)
	}
	if d.MinSwing <= 0 || d.SwingCap < d.MinSwing {
		return fmt.Errorf("digest: min_swing %g and swing_cap %g: want 0 < min_swing <= swing_cap", d.MinSwing, d.SwingCap)
	}
	for _, k := range DigestKinds {
		if w, ok := d.Weights[k]; !ok || w <= 0 {
			return fmt.Errorf("digest: weights.%s: want a weight over 0", k)
		}
	}
	if len(d.Weights) != len(DigestKinds) {
		for k := range d.Weights {
			if !slices.Contains(DigestKinds, k) {
				return fmt.Errorf("digest: weights.%s: no such kind of change", k)
			}
		}
	}
	return nil
}
