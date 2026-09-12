package content

import "fmt"

// ProgressionConfig mirrors progression.toml (#147): the tiers of §6.1
// in order. A tier is data the world is read against, never a gate a
// sim reads: its Enter trigger describes the gates that open the stage.
type ProgressionConfig struct {
	Tiers []TierConfig `toml:"tier"`
}

// TierConfig is one stage: its name and blurb, the day the harness reads
// the money curve at, what the stage opens, what the next one takes, and
// the trigger that must hold to enter it (none for the first: day 0).
type TierConfig struct {
	ID         string      `toml:"id"`
	Name       string      `toml:"name"`
	Blurb      string      `toml:"blurb"`
	Checkpoint int         `toml:"checkpoint"`
	Opens      []string    `toml:"opens"`
	Next       string      `toml:"next"`
	Enter      CardTrigger `toml:"enter"`
}

// Checkpoints is the day each tier is read at, in order: the harness's
// TierDays.
func (p ProgressionConfig) Checkpoints() []int {
	days := make([]int, 0, len(p.Tiers))
	for _, t := range p.Tiers {
		days = append(days, t.Checkpoint)
	}
	return days
}

// Tier returns the tier numbered n (1 is the first), or nil.
func (p ProgressionConfig) Tier(n int) *TierConfig {
	if n < 1 || n > len(p.Tiers) {
		return nil
	}
	return &p.Tiers[n-1]
}

// validate checks the file reads as a ladder: at least one tier, ids
// unique, every tier named, the checkpoints rising, and a trigger on
// every tier past the first (the first is day 0 and has none).
func (p ProgressionConfig) validate() error {
	if len(p.Tiers) == 0 {
		return fmt.Errorf("no tiers defined")
	}
	seen := map[string]bool{}
	for i, t := range p.Tiers {
		if t.ID == "" || t.Name == "" || t.Blurb == "" {
			return fmt.Errorf("tier %d needs an id, a name and a blurb", i+1)
		}
		if seen[t.ID] {
			return fmt.Errorf("tier %q is defined twice", t.ID)
		}
		seen[t.ID] = true
		if t.Checkpoint <= 0 || (i > 0 && t.Checkpoint <= p.Tiers[i-1].Checkpoint) {
			return fmt.Errorf("tier %q: checkpoint %d must be past the tier before it", t.ID, t.Checkpoint)
		}
		if i == 0 && t.Enter.Set() {
			return fmt.Errorf("tier %q is the first and is entered on day 0: it takes no [tier.enter]", t.ID)
		}
		if i > 0 && !t.Enter.Set() {
			return fmt.Errorf("tier %q needs a [tier.enter] trigger", t.ID)
		}
	}
	return nil
}
