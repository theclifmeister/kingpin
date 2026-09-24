package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// presetRoutine gives the model a routine the quiet preset moves: a
// standing order at normal on every product the stash holds where you
// stand, the wash normal and a supply contract.
func presetRoutine(t *testing.T, m *Model) {
	t.Helper()
	w := m.w
	city := w.Player.Location
	placed := 0
	for _, id := range w.Products {
		if n := w.Stock(city, id); n > 0 {
			if err := m.sess.PlaceStanding(city, id, n, events.DialNormal); err != nil {
				t.Fatal(err)
			}
			placed++
		}
	}
	if placed == 0 {
		w.AddStock(city, w.Products[0], 50, 0)
		if err := m.sess.PlaceStanding(city, w.Products[0], 50, events.DialNormal); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.sess.SetLaunderDial(events.LaunderNormal); err != nil {
		t.Fatal(err)
	}
	if err := m.sess.SetSupply(city, w.Products[0], 80); err != nil {
		t.Fatal(err)
	}
}

// P on the market (#357) opens the presets: the review names every
// setting the preset moves with its estimate, nothing moves before
// enter, enter applies it and never ends the day; s saves the routine
// in the profile (on disk, across runs), a saved preset reviews and
// applies like a built-in, and x deletes it.
func TestPresetsInTheGrammar(t *testing.T) {
	m := richModel(t, 120, 40)
	presetRoutine(t, m)
	w := m.w
	day := w.Day
	m.Update(key("2"))
	m.Update(key("P"))
	if m.mode != modePresets || len(m.pre.list) < 4 {
		t.Fatalf("P on the market: mode %v, %d presets, status %q", m.mode, len(m.pre.list), m.status)
	}
	m.Update(key("1"))
	view := stripANSI(m.View())
	for _, want := range []string{"PRESET · QUIET TRADING", "launder dial", "normal", "careful", "standing ", "~take ", "other setting"} {
		if !strings.Contains(view, want) {
			t.Errorf("the review has no %q:\n%s", want, view)
		}
	}
	if w.Laundering.Dial != events.LaunderNormal {
		t.Fatal("the review turned the dial")
	}
	m.Update(key("enter"))
	if m.mode != modePlay || w.Day != day || w.Laundering.Dial != events.LaunderCareful {
		t.Fatalf("enter on the review: mode %v, day %d, dial %v, status %q", m.mode, w.Day, w.Laundering.Dial, m.status)
	}
	for k, o := range w.Standing {
		if o.Dial != events.DialQuiet {
			t.Errorf("%s stands at %v after quiet trading", k, o.Dial)
		}
	}
	if !strings.Contains(m.status, "Quiet trading: ") {
		t.Errorf("status %q", m.status)
	}

	m.Update(key("P"))
	m.Update(key("s"))
	name := m.profile.Presets[0].Name
	if len(m.profile.Presets) != 1 || !m.presetSelected().Saved {
		t.Fatalf("s: %+v, cursor on %+v", m.profile.Presets, m.presetSelected())
	}
	disk, err := game.LoadProfile(time.Now())
	if err != nil || disk.Preset(name) == nil {
		t.Fatalf("the saved preset is not on disk: %v", err)
	}
	m.Update(key("esc"))
	if err := m.sess.SetLaunderDial(events.LaunderGreedy); err != nil {
		t.Fatal(err)
	}
	m.Update(key("P"))
	for range m.pre.list {
		m.Update(key("down"))
	}
	m.Update(key("enter"))
	if view := stripANSI(m.View()); !strings.Contains(view, "greedy") || !strings.Contains(view, "PRESET · "+strings.ToUpper(name)) {
		t.Fatalf("the saved preset's review:\n%s", view)
	}
	m.Update(key("enter"))
	if w.Laundering.Dial != events.LaunderCareful {
		t.Fatalf("the saved preset applied: dial %v, status %q", w.Laundering.Dial, m.status)
	}

	m.Update(key("P"))
	m.Update(key("x")) // on a built-in: refused
	if len(m.profile.Presets) != 1 || m.pre.err == "" {
		t.Fatalf("x on a built-in: %+v %q", m.profile.Presets, m.pre.err)
	}
	for range m.pre.list {
		m.Update(key("down"))
	}
	m.Update(key("x"))
	if len(m.profile.Presets) != 0 || len(m.pre.list) != 4 {
		t.Fatalf("x on the saved preset: %+v, list %d", m.profile.Presets, len(m.pre.list))
	}
}

// A preset that would change nothing says so and applies nothing.
func TestPresetWithNothingToChange(t *testing.T) {
	m := richModel(t, 120, 40)
	for _, o := range m.w.Standing {
		m.sess.CancelStanding(o.City, o.Product)
	}
	if err := m.sess.SetLaunderDial(events.LaunderGreedy); err != nil {
		t.Fatal(err)
	}
	m.Update(key("2"))
	m.Update(key("P"))
	m.Update(key("3")) // push: no standing order, and the wash greedy already
	if m.pre.step != 1 || len(m.pre.review.Changes) != 0 {
		t.Fatalf("push over nothing it moves: step %d, %+v", m.pre.step, m.pre.review.Changes)
	}
	if view := stripANSI(m.View()); !strings.Contains(view, "Nothing would change") {
		t.Errorf("the review:\n%s", view)
	}
	m.Update(key("enter"))
	if m.mode != modePresets || !strings.Contains(m.pre.err, "Nothing to change") {
		t.Fatalf("enter with nothing to change: mode %v, %q", m.mode, m.pre.err)
	}
}
