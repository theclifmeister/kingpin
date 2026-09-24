package content

import (
	"fmt"

	"github.com/theclifmeister/kingpin/internal/events"
)

// PresetsConfig mirrors presets.toml (#357, docs/presets.md): the
// built-in operation presets, each a named bundle of the routine's
// dials. A preset is its commands and no mechanic of its own: the
// engine turns a row into the session commands that set what it names
// (engine.Session.PresetCommands), and no sim reads this file.
type PresetsConfig struct {
	Presets []PresetConfig `toml:"preset"`
}

// PresetConfig is one built-in preset: its id, the name and the line of
// copy the list prints, and what it sets. An empty field leaves that
// setting alone.
type PresetConfig struct {
	ID    string `toml:"id"`
	Name  string `toml:"name"`
	Blurb string `toml:"blurb"`

	Standing string `toml:"standing"` // a sell dial for every standing order of yours, or PresetCancel
	Supply   string `toml:"supply"`   // PresetClear ends every supply contract of yours
	Launder  string `toml:"launder"`  // the launder dial by name
	Pay      string `toml:"pay"`      // the pay dial by name
	Routes   string `toml:"routes"`   // a route dial by name for every route running
	LieLow   bool   `toml:"lie_low"`  // lie low today
}

// The two words a preset uses beside the dials' names.
const (
	PresetCancel = "cancel" // standing: every standing order of yours cancelled
	PresetClear  = "clear"  // supply: every supply contract of yours cleared
)

func (c PresetsConfig) validate() error {
	ids, names := map[string]bool{}, map[string]bool{}
	for i, p := range c.Presets {
		if p.ID == "" || p.Name == "" || p.Blurb == "" {
			return fmt.Errorf("preset %d needs an id, a name and a blurb", i)
		}
		if ids[p.ID] || names[p.Name] {
			return fmt.Errorf("preset %q is defined twice", p.ID)
		}
		ids[p.ID], names[p.Name] = true, true
		if p.Standing != "" && p.Standing != PresetCancel {
			if _, ok := events.ParseDial(p.Standing); !ok {
				return fmt.Errorf("preset %q: standing %q is no sell dial and not %q", p.ID, p.Standing, PresetCancel)
			}
		}
		if p.Supply != "" && p.Supply != PresetClear {
			return fmt.Errorf("preset %q: supply %q is not %q", p.ID, p.Supply, PresetClear)
		}
		if _, ok := events.ParseLaunder(p.Launder); p.Launder != "" && !ok {
			return fmt.Errorf("preset %q: no launder dial %q", p.ID, p.Launder)
		}
		if _, ok := events.ParsePay(p.Pay); p.Pay != "" && !ok {
			return fmt.Errorf("preset %q: no pay dial %q", p.ID, p.Pay)
		}
		if _, ok := events.ParseRouteDial(p.Routes); p.Routes != "" && !ok {
			return fmt.Errorf("preset %q: no route dial %q", p.ID, p.Routes)
		}
		if p.Standing == "" && p.Supply == "" && p.Launder == "" && p.Pay == "" && p.Routes == "" && !p.LieLow {
			return fmt.Errorf("preset %q sets nothing", p.ID)
		}
	}
	return nil
}
