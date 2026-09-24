package content

import (
	"errors"
	"fmt"
)

// AmbitionsConfig mirrors ambitions.toml (#347): the ambitions' names
// and copy, and the two-city milestone's numbers. It is the panel's
// file: game.Ambitions reads the world against the owners' thresholds
// (laundering.toml, rivals.toml, upgrades.toml) and this file names what
// it reads, so nothing here moves a sim.
type AmbitionsConfig struct {
	Ambitions []AmbitionConfig `toml:"ambition"`
	TwoCities TwoCitiesConfig  `toml:"two_cities"`
}

// AmbitionConfig is one ambition: its id (one of AmbitionIDs), the
// panel's name and blurb, and a label per step, keyed by the step's id.
type AmbitionConfig struct {
	ID    string            `toml:"id"`
	Name  string            `toml:"name"`
	Blurb string            `toml:"blurb"`
	Steps map[string]string `toml:"steps"`
}

// TwoCitiesConfig is the two-city milestone: a city counts once Share of
// its corners are held, a lieutenant runs it and those corners have been
// held Days days; Cities of them make it.
type TwoCitiesConfig struct {
	Share  float64 `toml:"share"`
	Days   int     `toml:"days"`
	Cities int     `toml:"cities"`
}

// The ambitions (#347), in the panel's order: four endings and the
// milestone.
const (
	AmbitionRetire    = "retire"
	AmbitionLegit     = "legit"
	AmbitionCity      = "city"
	AmbitionVanish    = "vanish"
	AmbitionTwoCities = "two_cities"
)

// AmbitionIDs is every ambition, in the panel's order.
var AmbitionIDs = []string{AmbitionRetire, AmbitionLegit, AmbitionCity, AmbitionVanish, AmbitionTwoCities}

// AmbitionSteps is each ambition's steps in order: what game.Ambitions
// reads, and what the file must label, every one and no other. The
// vanish steps are the upgrade nodes of the same id.
var AmbitionSteps = map[string][]string{
	AmbitionRetire:    {"offshore", "quiet"},
	AmbitionLegit:     {"income", "goodwill", "streak"},
	AmbitionCity:      {"share", "factions", "streak"},
	AmbitionVanish:    {"retainer", "identity"},
	AmbitionTwoCities: {"ground", "lieutenants", "held"},
}

// AmbitionEnding is the cause each ambition is the plan for; the
// milestone has none.
var AmbitionEnding = map[string]string{
	AmbitionRetire: CauseRetired,
	AmbitionLegit:  CauseBusinessman,
	AmbitionCity:   CauseKingpin,
	AmbitionVanish: CauseVanished,
}

// Ambition returns the row for the id, or nil.
func (a AmbitionsConfig) Ambition(id string) *AmbitionConfig {
	return find(a.Ambitions, func(x *AmbitionConfig) bool { return x.ID == id })
}

// validate checks one row an ambition, in the game's order, each with a
// name, a blurb and a label for every step and no other; the vanish
// steps name nodes of the tree; the two-city numbers are sane.
func (a AmbitionsConfig) validate(tree UpgradesConfig) error {
	if len(a.Ambitions) != len(AmbitionIDs) {
		return fmt.Errorf("%d ambitions, want %d (%v)", len(a.Ambitions), len(AmbitionIDs), AmbitionIDs)
	}
	for i, row := range a.Ambitions {
		if row.ID != AmbitionIDs[i] {
			return fmt.Errorf("ambition %d is %q, want %q", i+1, row.ID, AmbitionIDs[i])
		}
		if row.Name == "" || row.Blurb == "" {
			return fmt.Errorf("ambition %s: a name and a blurb", row.ID)
		}
		steps := AmbitionSteps[row.ID]
		if len(row.Steps) != len(steps) {
			return fmt.Errorf("ambition %s: %d steps labelled, want %v", row.ID, len(row.Steps), steps)
		}
		for _, s := range steps {
			if row.Steps[s] == "" {
				return fmt.Errorf("ambition %s: no label for step %s", row.ID, s)
			}
		}
	}
	for _, id := range AmbitionSteps[AmbitionVanish] {
		if tree.Upgrade(id) == nil {
			return fmt.Errorf("ambition vanish: step %s is no node of the tree", id)
		}
	}
	t := a.TwoCities
	if t.Share <= 0 || t.Share > 1 || t.Days < 0 || t.Cities < 2 {
		return errors.New("[two_cities]: share in (0, 1], days at least 0, cities at least 2")
	}
	return nil
}
