package game

import (
	"reflect"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
)

// A saved preset (#357) lives in the profile: it survives the file,
// a second save under the same name (any case) replaces the first, and
// it deletes by name. A profile from before the presets reads with
// none, at the same schema.
func TestPresetsLiveInTheProfile(t *testing.T) {
	t.Setenv("KINGPIN_HOME", t.TempDir())
	p := NewProfile()
	a := Preset{Name: "Day 4", Day: 4, Standing: []SellOrder{{City: "eastside", Product: "weed", Qty: 40, Dial: events.DialQuiet}},
		Supply: []SupplyContract{{City: "eastside", Product: "weed", Units: 80, Since: 2}}, Launder: events.LaunderCareful, Pay: events.PayFair,
		Routes: []RoutePreset{{ID: "coast", Dial: events.RouteSlow, Target: map[string]int{"weed": 40}}}}
	p.SavePreset(a)
	p.SavePreset(Preset{Name: "Day 9", Day: 9})
	a.Pay = events.PayGenerous
	p.SavePreset(Preset{Name: "day 4"})
	p.SavePreset(a)
	if len(p.Presets) != 2 || p.Presets[0].Pay != events.PayGenerous || p.Preset("DAY 9") == nil {
		t.Fatalf("saved: %+v", p.Presets)
	}
	if err := SaveProfile(p); err != nil {
		t.Fatal(err)
	}
	again, err := LoadProfile(day)
	if err != nil || !reflect.DeepEqual(p.Presets, again.Presets) {
		t.Fatalf("round trip: %v\n%+v\n%+v", err, p.Presets, again.Presets)
	}
	if !again.DeletePreset("Day 9") || again.DeletePreset("Day 9") || len(again.Presets) != 1 {
		t.Fatalf("delete: %+v", again.Presets)
	}
	if !again.DeletePreset("day 4") || again.Presets != nil {
		t.Fatalf("the last deleted: %+v", again.Presets)
	}
	if (*Profile)(nil).Preset("x") != nil {
		t.Fatal("a nil profile has a preset")
	}
}
