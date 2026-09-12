package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/ui/anim"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The bust's scene (#155): a morning with an Enforcement past a patrol
// opens the report on it instead of the morning's scene, any key skips
// it, it ends by itself, either way on today's report byte for byte,
// an arrest that ends the run hands over to the ending's, and the
// fast-forward's stopping morning plays it.

// bustMorning is a model on seed 7 with the options given, a little
// stock on the street and the city's heat set, n pressed on day 1: the
// police answer overnight (45 a patrol, 70 a sting, 85 a raid on the
// seed), and with an informant on the payroll a raid goes straight to
// the stash (tier 2's stage marked seen, so the report is the morning's
// first modal).
func bustMorning(t *testing.T, w, h int, opts Options, heat float64, informant bool) *Model {
	t.Helper()
	m := newModelWith(t, w, h, opts)
	m.startRun(7)
	if informant {
		hireOne(m)
		m.w.Crew.Members[0].Informant = true
		m.w.Progression.Seen = map[int]bool{2: true}
	}
	m.w.SetStock(m.w.Player.Location, m.w.Products[0], 20)
	m.w.Here().Heat = heat
	day := m.w.Day
	m.Update(key("n"))
	if m.mode != modeReport || m.w.Day != day+1 {
		t.Fatalf("after n: mode %v day %d → %d", m.mode, day, m.w.Day)
	}
	return m
}

var animOn = Options{Anim: true, MorningAnim: true}

// TestBustSceneOnAnEnforcement: a sting starts the bust's scene, a
// raid too, a patrol the morning's and a quiet morning the morning's;
// the level, the loss and the stash tell are read off the event and
// the report's own line; the highest level wins a morning with two;
// animation off starts none; and a fast-forward's stopping morning
// plays it with the stop line on the report.
func TestBustSceneOnAnEnforcement(t *testing.T) {
	if anim.BustLength > 1200*time.Millisecond {
		t.Fatalf("the bust's scene is %v long, over 1.2 s", anim.BustLength)
	}
	for _, c := range []struct {
		heat float64
		want reportSceneKind
	}{{0, reportMorning}, {45, reportMorning}, {70, reportBust}, {85, reportBust}} {
		m := bustMorning(t, 80, 24, animOn, c.heat, false)
		if !m.onReportScene(c.want) || !m.ticking {
			t.Errorf("heat %v: scene %v, want kind %v; ticking %v", c.heat, m.reportScene, c.want, m.ticking)
		}
	}
	// The event and the line: a sting's level and its loss as the
	// report writes it.
	m := bustMorning(t, 80, 24, animOn, 70, false)
	ev, ok := m.bust()
	if !ok || ev.Level != "sting" || ev.Stash {
		t.Fatalf("the sting: %+v ok %v", ev, ok)
	}
	if loss := m.bustLoss("STING"); !strings.HasPrefix(loss, ": lost ") {
		t.Errorf("the sting's loss reads %q", loss)
	}
	if bustLevel("arrest") != "ARRESTED" || bustLevel("raid") != "RAID" {
		t.Error("the levels' words")
	}
	// Somebody talked: the raid went to the stash.
	m = bustMorning(t, 80, 24, animOn, 85, true)
	if ev, ok := m.bust(); !ok || ev.Level != "raid" || !ev.Stash || !m.onReportScene(reportBust) {
		t.Fatalf("the informant's raid: %+v ok %v scene %v", ev, ok, m.reportScene)
	}
	// Two in a morning: the highest level, then the first.
	m.flash = []events.Enforcement{{Level: "patrol"}, {Level: "sting", City: "a"}, {Level: "raid", City: "b"}, {Level: "raid", City: "c"}, {Level: "sting", City: "d"}}
	if ev, ok := m.bust(); !ok || ev.Level != "raid" || ev.City != "b" {
		t.Errorf("the pick of the morning: %+v ok %v", ev, ok)
	}
	m.flash = []events.Enforcement{{Level: "patrol"}}
	if _, ok := m.bust(); ok {
		t.Error("a patrol is a bust")
	}
	// Animation off: no scene, and the report at once.
	off := bustMorning(t, 80, 24, Options{Anim: false}, 70, false)
	if off.scene != nil || off.reportScene != reportNone {
		t.Fatalf("animation off: scene %v kind %v", off.scene, off.reportScene)
	}
	// Under the floor: none.
	small := bustMorning(t, 79, 24, animOn, 70, false)
	if small.scene != nil {
		t.Fatal("under 80 columns: a scene")
	}
	// A fast-forward stops on the sting and its morning plays the scene
	// over the report with the stop line.
	m = newAnimModel(t, 80, 24)
	m.startRun(7)
	m.w.SetStock(m.w.Player.Location, m.w.Products[0], 20)
	m.w.Here().Heat = 70
	day := m.w.Day
	fast(t, m, 5)
	if m.mode != modeReport || m.w.Day != day+1 || !m.onReportScene(reportBust) || m.fastStop == "" {
		t.Fatalf("after F: mode %v day %d → %d scene %v stop %q", m.mode, day, m.w.Day, m.reportScene, m.fastStop)
	}
	skipScene(m)
	if line := reportLine(t, m); !strings.HasPrefix(line, "Stopped after 1 day") {
		t.Errorf("the report's first line is %q", line)
	}
}

// TestBustSceneIsSkippable: a key mid-scene is consumed and lands on
// today's report, byte for byte the one animation off draws; enter
// then closes it and never ends the day.
func TestBustSceneIsSkippable(t *testing.T) {
	off := bustMorning(t, 80, 24, Options{Anim: false}, 70, false)
	on := bustMorning(t, 80, 24, animOn, 70, false)
	now := time.Unix(1_700_000_000, 0)
	tickAt(on, now.Add(anim.BustLength/2))
	mid := on.View()
	if mid == off.View() {
		t.Fatal("mid-scene the view is the finished report")
	}
	if !strings.Contains(stripANSI(mid), "STING") {
		t.Errorf("mid-scene the level is not up:\n%s", stripANSI(mid))
	}
	day := on.w.Day
	if _, cmd := on.Update(key("x")); cmd != nil || on.scene != nil || on.mode != modeReport {
		t.Fatalf("the skip: cmd %v scene %v mode %v", cmd, on.scene, on.mode)
	}
	if on.View() != off.View() {
		t.Fatalf("after the skip the report is not today's:\n%s\n%s", stripANSI(on.View()), stripANSI(off.View()))
	}
	if _, cmd := on.Update(key("enter")); cmd != nil || on.mode != modePlay || on.w.Day != day {
		t.Fatalf("enter after the skip: cmd %v mode %v day %d → %d", cmd, on.mode, day, on.w.Day)
	}
}

// TestBustSceneEndsItself: the ticks run the scene out at BustLength,
// every frame fitting, the chain ends there on today's report byte for
// byte, and a key after is the report's; the stash raid shows the
// rival's purple for exactly one frame, the plain raid never.
func TestBustSceneEndsItself(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0)
	defer lipgloss.SetColorProfile(profile)
	purple := strings.TrimSuffix(theme.Fg(theme.Rivals).Render("R"), "R\x1b[0m") // the colour's open sequence
	for _, informant := range []bool{false, true} {
		off := bustMorning(t, 80, 24, Options{Anim: false}, 85, informant)
		on := bustMorning(t, 80, 24, animOn, 85, informant)
		now := time.Unix(1_700_000_000, 0)
		frames, purples := map[string]bool{}, 0
		for at := time.Duration(0); at < anim.BustLength; at += anim.Frame {
			if cmd := tickAt(on, now.Add(at)); cmd == nil || on.scene == nil {
				t.Fatalf("informant %v at %v: cmd %v scene %v", informant, at, cmd, on.scene)
			}
			view := on.View()
			assertFits(t, view, 80, 24, fmt.Sprintf("bust scene (informant %v) at %v", informant, at))
			frames[view] = true
			// The bust's line is the body's first row (the report's own
			// TERRITORY lines are purple too, further down).
			if _, _, box := modalBox(t, view); strings.Contains(box[3], purple) {
				purples++
			}
		}
		if len(frames) < 20 {
			t.Errorf("informant %v: %d distinct frames over the scene", informant, len(frames))
		}
		if want := map[bool]int{false: 0, true: 1}[informant]; purples != want {
			t.Errorf("informant %v: %d frames in purple, want %d", informant, purples, want)
		}
		if cmd := tickAt(on, now.Add(anim.BustLength)); cmd != nil || on.scene != nil || on.mode != modeReport {
			t.Fatalf("Done: cmd %v scene %v mode %v", cmd, on.scene, on.mode)
		}
		if on.View() != off.View() {
			t.Fatalf("informant %v: after Done the report is not today's:\n%s\n%s", informant, stripANSI(on.View()), stripANSI(off.View()))
		}
		if _, cmd := on.Update(key("down")); cmd != nil || on.mode != modeReport {
			t.Fatalf("a key after Done: cmd %v mode %v", cmd, on.mode)
		}
	}
}

// TestBustSceneYieldsToTheEnding: an arrest that ends the run is the
// ending's morning: modeOver on the ending's scene, none of the
// bust's; a card morning with a sting plays the card's scene and the
// report after the outcome opens on none.
func TestBustSceneYieldsToTheEnding(t *testing.T) {
	m := newAnimModel(t, 80, 24)
	m.startRun(7)
	endIndicted(t, m)
	arrested := false
	for _, ev := range m.flash {
		arrested = arrested || ev.Level == "arrest"
	}
	if !arrested || m.scene == nil || m.reportScene != reportNone {
		t.Fatalf("the arrest: flash %+v scene %v kind %v", m.flash, m.scene, m.reportScene)
	}
	// The card outranks the bust.
	m = newAnimModel(t, 80, 24)
	m.startRun(7)
	m.w.SetStock(m.w.Player.Location, m.w.Products[0], 20)
	m.w.Here().Heat = 70
	m.w.Dilemmas.Pending = testCard(m.w.Day + 1)
	m.Update(key("n"))
	if _, ok := m.bust(); !ok || m.mode != modeCard || m.reportScene != reportNone {
		t.Fatalf("a card morning with a sting: ok %v mode %v kind %v", ok, m.mode, m.reportScene)
	}
	skipScene(m)
	m.Update(key("1"))
	if _, cmd := m.Update(key("enter")); cmd != nil || m.mode != modeReport || m.scene != nil {
		t.Fatalf("the report after the card: cmd %v mode %v scene %v", cmd, m.mode, m.scene)
	}
}

// TestModalsFitBust: modeReport mid-scene is the one modal at 80x24
// and 120x40: on body row 2, min(width-4, 76) wide, every box line
// whole, at most two rows taller than the report's for the bust's line
// and its blank, at the start, the quarters and the last frame.
func TestModalsFitBust(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		want := min(sz[0]-4, 76)
		m := richModel(t, sz[0], sz[1])
		m.opts.Anim = true
		m.mode, m.fastStop = modePlay, ""
		m.flash = []events.Enforcement{{Day: m.w.Day, City: m.w.Player.Location, Level: "raid", Stash: true}}
		m.w.Report.Heat = append([]string{"RAID: lost 40 Weed and $2,000"}, m.w.Report.Heat...)
		m.openReport()
		if !m.onReportScene(reportBust) {
			t.Fatalf("%dx%d: mode %v scene %v kind %v", sz[0], sz[1], m.mode, m.scene, m.reportScene)
		}
		_, _, offBox := modalBox(t, m.modal(m.reportTitle(), m.reportLines(), m.modalFooter()))
		q := anim.BustLength / 4
		for _, at := range []time.Duration{0, q, 2 * q, 3 * q, anim.BustLength - anim.Frame} {
			tickAt(m, now.Add(at))
			if m.scene == nil {
				t.Fatalf("%dx%d at %v: the scene ended early", sz[0], sz[1], at)
			}
			view := m.View()
			what := fmt.Sprintf("%dx%d bust scene at %v", sz[0], sz[1], at)
			assertFits(t, view, sz[0], sz[1], what)
			top, width, box := modalBox(t, view)
			if top != 2 || width != want {
				t.Errorf("%s: the modal is on row %d, %d wide; want row 2, %d wide", what, top, width, want)
			}
			for i, l := range box {
				if lw := lipgloss.Width(strings.TrimSpace(stripANSI(l))); lw != want {
					t.Errorf("%s: box line %d is %d wide, not %d: %q", what, i, lw, want, stripANSI(l))
				}
			}
			if len(box) < len(offBox) || len(box) > len(offBox)+2 {
				t.Errorf("%s: a box of %d lines, the report's is %d", what, len(box), len(offBox))
			}
		}
	}
}
