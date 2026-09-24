package engine_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// routineSession is a run on seed 3 with a routine for the presets to
// move (#357): two standing orders of yours at normal, one of them
// for more than the stash holds (the quiet preset's re-dial is refused
// for it), a supply contract, a route running at normal with a
// target, and a sell order queued tonight.
func routineSession(t *testing.T) (*engine.Session, *game.World) {
	t.Helper()
	s, w := newSession(t)
	w.Player.DirtyCash = 200_000
	home := w.CityOrder[0]
	a, b := w.Products[0], w.Products[1]
	w.AddStock(home, a, 100, 0)
	w.AddStock(home, b, 100, 0)
	must(t, s.PlaceStanding(home, a, 60, events.DialNormal))
	must(t, s.PlaceStanding(home, b, 100, events.DialNormal))
	w.TakeStock(home, b, 90) // the order stands for 100 with 10 stashed
	must(t, s.SetSupply(home, a, 150))
	must(t, s.PlaceSell(home, a, 10, events.DialQuiet))
	routes := s.Rules().Logistics.RoutesOpen(w, home)
	if len(routes) == 0 {
		t.Fatal("no route open from home")
	}
	must(t, s.SetRoute(routes[0].ID, events.RouteNormal))
	must(t, s.SetRouteTarget(routes[0].ID, a, 40))
	return s, w
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func encode(t *testing.T, w *game.World) []byte {
	t.Helper()
	b, err := json.Marshal(w) // gob writes a map in any order; json sorts the keys
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// byHand issues one command through the session method of its name,
// written out here rather than through Session.Do, as a front end
// calling the commands itself would.
func byHand(s *engine.Session, c engine.Command) error {
	switch c.Op {
	case "place_standing":
		d, _ := events.ParseDial(c.Dial)
		return s.PlaceStanding(c.City, c.Product, c.N, d)
	case "cancel_standing":
		s.CancelStanding(c.City, c.Product)
	case "set_supply":
		return s.SetSupply(c.City, c.Product, c.N)
	case "clear_supply":
		s.ClearSupply(c.City, c.Product)
	case "set_launder_dial":
		d, _ := events.ParseLaunder(c.Dial)
		return s.SetLaunderDial(d)
	case "set_pay":
		p, _ := events.ParsePay(c.Dial)
		return s.SetPay(p)
	case "set_route":
		d, _ := events.ParseRouteDial(c.Dial)
		return s.SetRoute(c.Route, d)
	case "set_route_target":
		return s.SetRouteTarget(c.Route, c.Product, c.N)
	case "set_route_days":
		return s.SetRouteDays(c.Route, c.Product, c.N)
	case "set_lie_low":
		s.SetLieLow(c.On)
	}
	return nil
}

// savedPreset is a routine unlike routineSession's, snapshotted: every
// standing order quiet and smaller, the contract gone for another, the
// route slow on a days target, the wash greedy and the pay generous.
func savedPreset(t *testing.T) game.Preset {
	t.Helper()
	s, w := routineSession(t)
	home := w.CityOrder[0]
	a, c := w.Products[0], w.Products[2]
	s.CancelStanding(home, w.Products[1])
	must(t, s.PlaceStanding(home, a, 20, events.DialQuiet))
	s.ClearSupply(home, a)
	must(t, s.SetSupply(home, c, 30))
	r := s.Rules().Logistics.RoutesOpen(w, home)[0].ID
	must(t, s.SetRoute(r, events.RouteSlow))
	must(t, s.SetRouteDays(r, a, 3))
	must(t, s.SetLaunderDial(events.LaunderGreedy))
	must(t, s.SetPay(events.PayGenerous))
	return s.Snapshot("Mine")
}

// TestPresetIsItsCommands (#357): applying a preset is the same world,
// byte for byte, as issuing its commands by hand, for every built-in
// and a saved one; the quiet preset's commands are the ones the file's
// row names; and a saved preset applied sets the routine back to what
// was saved.
func TestPresetIsItsCommands(t *testing.T) {
	t.Parallel()
	saved := savedPreset(t)
	ids := []string{"Mine"}
	for _, p := range content.MustLoad().Presets.Presets {
		ids = append(ids, p.ID)
	}
	for _, id := range ids {
		a, wa := routineSession(t)
		b, wb := routineSession(t)
		a.UsePresets([]game.Preset{saved})
		b.UsePresets([]game.Preset{saved})
		if _, err := a.ApplyPreset(id); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		cmds, err := b.PresetCommands(id)
		if err != nil || len(cmds) == 0 {
			t.Fatalf("%s: %d commands, %v", id, len(cmds), err)
		}
		for _, c := range cmds {
			_ = byHand(b, c)
		}
		if !bytes.Equal(encode(t, wa), encode(t, wb)) {
			t.Errorf("%s: the preset applied is not its commands issued by hand", id)
		}
	}

	s, w := routineSession(t)
	home, route := w.CityOrder[0], s.Rules().Logistics.RoutesOpen(w, w.CityOrder[0])[0].ID
	cmds, _ := s.PresetCommands("quiet")
	want := []engine.Command{
		{Op: "place_standing", City: home, Product: w.Products[0], N: 60, Dial: "quiet"},
		{Op: "place_standing", City: home, Product: w.Products[1], N: 100, Dial: "quiet"},
		{Op: "set_launder_dial", Dial: "careful"},
		{Op: "set_route", Route: route, Dial: "slow"},
	}
	if !reflect.DeepEqual(cmds, want) {
		t.Errorf("quiet's commands:\n got %+v\nwant %+v", cmds, want)
	}
	if byName, _ := s.PresetCommands("quiet trading"); !reflect.DeepEqual(byName, cmds) {
		t.Errorf("by name: %+v", byName)
	}
	if _, err := s.PresetCommands("nothing"); err != engine.ErrNoPreset {
		t.Errorf("an unknown preset: %v", err)
	}

	s.UsePresets([]game.Preset{saved})
	if _, err := s.ApplyPreset("mine"); err != nil {
		t.Fatal(err)
	}
	got := s.Snapshot("Mine")
	got.Day = saved.Day
	if !reflect.DeepEqual(got, saved) {
		t.Errorf("the saved preset applied:\n got %+v\nwant %+v", got, saved)
	}
	if again, _ := s.PresetCommands("Mine"); len(again) != 0 {
		t.Errorf("a saved preset applied twice issues %+v", again)
	}
}

// settings is the routine in words by setting, read off the world
// without the engine's diff: the test's own reading of what moved.
func settings(s *engine.Session, w *game.World) map[string]string {
	out := map[string]string{"launder": w.Laundering.Dial.String(), "pay": w.Crew.Pay.String(), "lie_low": "off"}
	if w.Today.LieLow {
		out["lie_low"] = "on"
	}
	p := s.Snapshot("")
	for _, o := range p.Standing {
		out["standing "+o.City+" "+o.Product] = o.Dial.String() + "/" + strconv.Itoa(o.Qty)
	}
	for _, c := range p.Supply {
		out["supply "+c.City+" "+c.Product] = strconv.Itoa(c.Units)
	}
	for _, r := range p.Routes {
		out["route "+r.ID] = r.Dial.String()
		for pid, u := range r.Target {
			out["target "+r.ID+" "+pid] = strconv.Itoa(u) + " units"
		}
		for pid, d := range r.Days {
			out["target "+r.ID+" "+pid] = strconv.Itoa(d) + " days"
		}
	}
	return out
}

// changeKey is a Change's key in settings.
func changeKey(c engine.Change) string {
	switch c.Setting {
	case "standing", "supply":
		return c.Setting + " " + c.City + " " + c.Product
	case "route":
		return "route " + c.Route
	case "target":
		return "target " + c.Route + " " + c.Product
	}
	return c.Setting
}

// TestPresetDiffIsExact (#357): the review lists precisely the settings
// that move, for every preset: the keys whose words differ between the
// routine before and after the preset applied, read without the diff;
// the review before is the review after, and reading it leaves the run
// as it was. A command the rules refuse (the quiet re-dial of an order
// larger than the stash) is listed, and its setting is not a change.
func TestPresetDiffIsExact(t *testing.T) {
	t.Parallel()
	saved := savedPreset(t)
	ids := []string{"Mine"}
	for _, p := range content.MustLoad().Presets.Presets {
		ids = append(ids, p.ID)
	}
	for _, id := range ids {
		s, w := routineSession(t)
		s.UsePresets([]game.Preset{saved})
		before := settings(s, w)
		raw := encode(t, w)
		review, err := s.PresetDiff(id)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(raw, encode(t, w)) {
			t.Fatalf("%s: the diff touched the run", id)
		}
		applied, err := s.ApplyPreset(id)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(review, applied) {
			t.Errorf("%s: the review\n %+v\nis not what applying did\n %+v", id, review, applied)
		}
		after := settings(s, w)
		var moved []string
		for k := range before {
			if before[k] != after[k] {
				moved = append(moved, k)
			}
		}
		for k := range after {
			if _, ok := before[k]; !ok {
				moved = append(moved, k)
			}
		}
		var listed []string
		for _, c := range review.Changes {
			listed = append(listed, changeKey(c))
			if c.From == c.To {
				t.Errorf("%s: a change that moves nothing: %+v", id, c)
			}
		}
		slices.Sort(moved)
		slices.Sort(listed)
		if !slices.Equal(moved, listed) {
			t.Errorf("%s: moved %v, the review lists %v", id, moved, listed)
		}
		if review.Same < 0 {
			t.Errorf("%s: %d settings the same", id, review.Same)
		}
		if id == "quiet" {
			if len(review.Refused) != 1 || review.Refused[0].Command.Product != w.Products[1] {
				t.Errorf("quiet: refused %+v, want the order larger than the stash", review.Refused)
			}
		}
		if id == "dark" {
			last := review.Changes[len(review.Changes)-1]
			if last.Setting != "lie_low" || last.Dropped != 1 {
				t.Errorf("dark: lying low is %+v, want it last with tonight's order dropped", last)
			}
		}
		if id == "preserve" {
			for _, c := range review.Changes {
				if c.Setting == "supply" && (c.To != "none" || c.CostTo != 0) {
					t.Errorf("preserve: %+v", c)
				}
			}
		}
	}
}

// TestNoPresetIsTheOldRun (#357): the presets are commands and reads,
// so a run that lists every preset and reads every diff each morning,
// and never applies one, is the run that never looked, byte for byte.
func TestNoPresetIsTheOldRun(t *testing.T) {
	t.Parallel()
	a, wa := newSession(t)
	b, wb := newSession(t)
	for d := 0; d < 30 && wa.Over == nil; d++ {
		for _, p := range b.Presets() {
			if _, err := b.PresetCommands(p.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := b.PresetDiff(p.ID); err != nil {
				t.Fatal(err)
			}
		}
		a.EndDay()
		b.EndDay()
		if !bytes.Equal(encode(t, wa), encode(t, wb)) {
			t.Fatalf("day %d: reading the presets moved the run", wa.Day)
		}
	}
}

// TestPresetsList (#357): the built-ins in the file's order, then the
// saved ones; a saved preset under a built-in's name is shadowed.
func TestPresetsList(t *testing.T) {
	t.Parallel()
	s, _ := newSession(t)
	s.UsePresets([]game.Preset{{Name: "Mine", Day: 4}, {Name: "Go dark"}})
	var got []string
	for _, p := range s.Presets() {
		got = append(got, p.ID)
		if p.Blurb == "" {
			t.Errorf("%s has no blurb", p.ID)
		}
	}
	if want := []string{"quiet", "preserve", "push", "dark", "Mine"}; !slices.Equal(got, want) {
		t.Errorf("presets %v, want %v", got, want)
	}
	if !slices.Equal(engine.Ops(), []string{"cancel_standing", "clear_supply", "set_supply", "place_standing", "set_launder_dial", "set_pay", "set_route", "set_route_target", "set_route_days", "set_lie_low"}) {
		t.Errorf("ops %v", engine.Ops())
	}
}
