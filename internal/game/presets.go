package game

import (
	"strings"

	"github.com/theclifmeister/kingpin/internal/events"
)

// Preset is a player's saved operation preset (#357, docs/presets.md):
// the routine as it stood the day it was saved, under a name. It lives
// in the profile, not the run, so it carries across runs; applying one
// is the session commands that set the routine back to it
// (engine.Session.PresetCommands), skipping what the run has not got (a
// city not reached, a route not open). Lying low is the day's, never
// saved.
type Preset struct {
	Name     string
	Day      int              // the day of the run it was saved on, for the list
	Standing []SellOrder      // every standing order of yours, in city then ladder order
	Supply   []SupplyContract // every supply contract of yours, in the same order
	Launder  events.Launder
	Pay      events.Pay
	Routes   []RoutePreset // every route running or keeping a target, by id
}

// RoutePreset is one route in a saved preset: its dial and its targets,
// in units or in days of demand.
type RoutePreset struct {
	ID     string
	Dial   events.RouteDial
	Target map[string]int `json:",omitempty"`
	Days   map[string]int `json:",omitempty"`
}

// Preset is the saved preset named name (any case), or nil.
func (p *Profile) Preset(name string) *Preset {
	if p == nil {
		return nil
	}
	for i := range p.Presets {
		if strings.EqualFold(p.Presets[i].Name, name) {
			return &p.Presets[i]
		}
	}
	return nil
}

// SavePreset keeps a preset in the profile, in the place of one saved
// under the same name (any case), else after the rest.
func (p *Profile) SavePreset(pr Preset) {
	if old := p.Preset(pr.Name); old != nil {
		*old = pr
		return
	}
	p.Presets = append(p.Presets, pr)
}

// DeletePreset drops the saved preset named name, reporting whether one
// was there.
func (p *Profile) DeletePreset(name string) bool {
	for i := range p.Presets {
		if strings.EqualFold(p.Presets[i].Name, name) {
			p.Presets = append(p.Presets[:i], p.Presets[i+1:]...)
			if len(p.Presets) == 0 {
				p.Presets = nil
			}
			return true
		}
	}
	return false
}
