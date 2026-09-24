package content

import "fmt"

// TrophiesConfig mirrors trophies.toml (#392): what the cartel buys to
// be seen buying it. A trophy is clean cash spent on nothing a sim
// needs: it counts in net worth at cost while it is yours and it
// talks, a little fear, respect and notoriety every day it is owned
// (the reputation sim reads the rows through its own copy of this
// file), and the task force takes it the way it takes an asset.
type TrophiesConfig struct {
	Offers []TrophyConfig `toml:"trophy"`
}

// TrophyConfig is one trophy: its id and name, the line the ledger
// shows, the peak clean cash that puts it on offer, its price in clean
// cash, and what owning it adds to each reputation axis a day.
type TrophyConfig struct {
	ID         string  `toml:"id"`
	Name       string  `toml:"name"`
	Blurb      string  `toml:"blurb"`
	UnlockCash int     `toml:"unlock_cash"` // peak clean cash
	Cost       int     `toml:"cost"`        // clean cash
	Fear       float64 `toml:"fear"`        // a day, while owned
	Respect    float64 `toml:"respect"`     // a day, while owned
	Notoriety  float64 `toml:"notoriety"`   // a day, while owned
}

// Trophy returns the row with id, or nil.
func (c TrophiesConfig) Trophy(id string) *TrophyConfig {
	return find(c.Offers, func(e *TrophyConfig) bool { return e.ID == id })
}

// validate checks the rows: ids unique, a name and a line each, the
// price positive and the line not negative, and the axes a day small
// enough that one trophy cannot pin an axis on its own.
func (c TrophiesConfig) validate() error {
	seen := map[string]bool{}
	for _, t := range c.Offers {
		if t.ID == "" || t.Name == "" || t.Blurb == "" || seen[t.ID] {
			return fmt.Errorf("trophy %q needs a unique id, a name and a blurb", t.ID)
		}
		seen[t.ID] = true
		if t.Cost <= 0 || t.UnlockCash < 0 {
			return fmt.Errorf("trophy %q: bad cost or unlock_cash", t.ID)
		}
		for _, v := range []float64{t.Fear, t.Respect, t.Notoriety} {
			if v < 0 || v > 2 {
				return fmt.Errorf("trophy %q: an axis a day out of 0..2", t.ID)
			}
		}
	}
	return nil
}
