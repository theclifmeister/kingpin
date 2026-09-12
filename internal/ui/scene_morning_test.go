package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/anim"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The morning's scene (#159): the report opening on a plain morning
// rolls the title bar's day over and wipes the report's title in, a
// quarter of a second; any key skips to the report; never after F; a
// card's, a stage's or an ending's scene replaces it; and with the
// morning's animation off the report is today's, byte for byte.

// plainMorning is a model on seed 7 with the options given, n pressed
// on day 1: the report is the first thing the morning shows.
func plainMorning(t *testing.T, w, h int, opts Options) *Model {
	t.Helper()
	m := newModelWith(t, w, h, opts)
	m.startRun(7)
	day := m.w.Day
	m.Update(key("n"))
	if m.mode != modeReport || m.w.Day != day+1 {
		t.Fatalf("after n: mode %v day %d → %d", m.mode, day, m.w.Day)
	}
	return m
}

// TestMorningSceneIsShort: n opens the report on the scene, the day
// saved before its first frame; the ticks run it out within
// anim.MorningLength (250 ms) and the chain ends there on the report
// animation off draws, byte for byte; a key mid-scene lands on the
// report, consumed, and enter then closes it without ending the day.
func TestMorningSceneIsShort(t *testing.T) {
	if anim.MorningLength > 250*time.Millisecond {
		t.Fatalf("the morning's scene is %v long, over 250 ms", anim.MorningLength)
	}
	off := plainMorning(t, 80, 24, Options{Anim: false})
	on := plainMorning(t, 80, 24, Options{Anim: true, MorningAnim: true})
	if !on.onReportScene(reportMorning) || on.status != fmt.Sprintf("Day %d saved.", on.w.Day) {
		t.Fatalf("after n: scene %v status %q", on.scene, on.status)
	}
	if !on.ticking {
		t.Fatal("no tick on its way for the scene")
	}
	now := time.Unix(1_700_000_000, 0)
	frames := map[string]bool{}
	for at := time.Duration(0); at < anim.MorningLength; at += anim.Frame {
		if cmd := tickAt(on, now.Add(at)); cmd == nil || on.scene == nil {
			t.Fatalf("at %v: cmd %v scene %v", at, cmd, on.scene)
		}
		view := on.View()
		assertFits(t, view, 80, 24, "morning scene at "+at.String())
		frames[view] = true
	}
	if len(frames) < 5 {
		t.Errorf("%d distinct frames over the scene", len(frames))
	}
	if cmd := tickAt(on, now.Add(anim.MorningLength)); cmd != nil || on.scene != nil || on.mode != modeReport {
		t.Fatalf("Done: cmd %v scene %v mode %v", cmd, on.scene, on.mode)
	}
	if on.View() != off.View() {
		t.Fatalf("after Done the report is not today's:\n%s\n%s", stripANSI(on.View()), stripANSI(off.View()))
	}
	// Skipped: a key mid-scene is consumed and lands on the same report.
	on = plainMorning(t, 80, 24, Options{Anim: true, MorningAnim: true})
	tickAt(on, now)
	if on.View() == off.View() {
		t.Fatal("the scene's first frame is the finished report")
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
	// r reopens the report with no scene: a reopen is no morning.
	if _, cmd := on.Update(key("r")); cmd != nil || on.mode != modeReport || on.scene != nil {
		t.Fatalf("r: cmd %v mode %v scene %v", cmd, on.mode, on.scene)
	}
}

// TestMorningSceneRollsTheDay: the title bar's day is yesterday's at
// the first frame and today's at the last, the report's title wiping
// in beside it from nothing to the whole, in the modal's gold; the bar
// holds its width throughout.
func TestMorningSceneRollsTheDay(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0)
	defer lipgloss.SetColorProfile(profile)
	m := plainMorning(t, 80, 24, Options{Anim: true, MorningAnim: true})
	now := time.Unix(1_700_000_000, 0)
	tickAt(m, now)
	bar := func() string { return strings.Split(stripANSI(m.View()), "\n")[0] }
	first := bar()
	if !strings.Contains(first, fmt.Sprintf("Day %d ", m.w.Day-1)) {
		t.Errorf("the first frame's bar does not carry yesterday's day:\n%s", first)
	}
	if _, _, box := modalBox(t, m.View()); strings.Contains(stripANSI(box[1]), "MORNING REPORT") {
		t.Errorf("the title is up at the first frame: %q", stripANSI(box[1]))
	}
	tickAt(m, now.Add(anim.MorningLength-anim.Frame))
	last := bar()
	if !strings.Contains(last, fmt.Sprintf("Day %d ", m.w.Day)) {
		t.Errorf("the last frame's bar does not carry today's day:\n%s", last)
	}
	if lipgloss.Width(first) != lipgloss.Width(last) {
		t.Errorf("the bar moved: %d then %d cells", lipgloss.Width(first), lipgloss.Width(last))
	}
	frame := m.scene.Scene.Frame(anim.MorningLength, m.modalInner(), 2)
	if got, want := stripANSI(frame[1]), m.reportTitle(); got != want {
		t.Errorf("the settled title is %q, want %q", got, want)
	}
	if !strings.Contains(frame[1], theme.Fg(theme.Money).Render("MORNING")) {
		t.Errorf("the settled title is not in gold: %q", frame[1])
	}
}

// TestMorningSceneNotAfterFast: F for five days opens the report with
// its stop line and no scene, whatever the morning; n after it plays
// the scene again.
func TestMorningSceneNotAfterFast(t *testing.T) {
	m := newAnimModel(t, 80, 24)
	m.startRun(7)
	fast(t, m, 5)
	if m.mode == modeStage || m.mode == modeCard {
		skipScene(m)
		closeMorning(t, m)
		m.Update(key("r"))
	}
	if m.mode != modeReport || m.scene != nil || m.fastStop == "" {
		t.Fatalf("after F: mode %v scene %v stop %q", m.mode, m.scene, m.fastStop)
	}
	if line := reportLine(t, m); !strings.HasPrefix(line, "Stopped after") {
		t.Errorf("the report's first line is %q", line)
	}
	m.Update(key("enter"))
	if _, cmd := m.Update(key("n")); cmd == nil || !m.onReportScene(reportMorning) {
		t.Fatalf("n after F: cmd %v scene %v mode %v", cmd, m.scene, m.mode)
	}
}

// TestMorningSceneYieldsToTheCard: a card morning plays the card's
// scene, not the morning's, and the report after the outcome opens on
// none; a stage morning the same.
func TestMorningSceneYieldsToTheCard(t *testing.T) {
	m := newAnimModel(t, 80, 24)
	m.startRun(7)
	m.w.Dilemmas.Pending = testCard(m.w.Day + 1)
	if _, cmd := m.Update(key("n")); cmd == nil || m.mode != modeCard || m.scene == nil || m.reportScene != reportNone {
		t.Fatalf("a card morning: cmd %v mode %v scene %v report scene %v", cmd, m.mode, m.scene, m.reportScene)
	}
	skipScene(m)
	m.Update(key("1"))
	if _, cmd := m.Update(key("enter")); cmd != nil || m.mode != modeReport || m.scene != nil {
		t.Fatalf("the report after the card: cmd %v mode %v scene %v", cmd, m.mode, m.scene)
	}
	// A stage morning: the stage's scene, then the report on none.
	m, _ = stageMorning(t, 80, 24)
	if m.mode != modeStage || m.reportScene != reportNone {
		t.Fatalf("a stage morning: mode %v report scene %v", m.mode, m.reportScene)
	}
	skipScene(m)
	if _, cmd := m.Update(key("enter")); cmd != nil || m.mode != modeReport || m.scene != nil {
		t.Fatalf("the report after the stage: cmd %v mode %v scene %v", cmd, m.mode, m.scene)
	}
}

// TestMorningSceneOff: with the morning's animation off alone
// (KINGPIN_NO_MORNING_ANIM), with animation off, or under 80x24, n
// opens today's report at once, byte for byte, and no tick leaves
// Update.
func TestMorningSceneOff(t *testing.T) {
	want := plainMorning(t, 80, 24, Options{Anim: false}).View()
	for name, opts := range map[string]Options{
		"morning off": {Anim: true, MorningAnim: false},
		"all off":     {Anim: false, MorningAnim: true},
	} {
		m := newModelWith(t, 80, 24, opts)
		m.startRun(7)
		if _, cmd := m.Update(key("n")); cmd != nil || m.scene != nil || m.mode != modeReport {
			t.Fatalf("%s: cmd %v scene %v mode %v", name, cmd, m.scene, m.mode)
		}
		if m.View() != want {
			t.Errorf("%s: the report is not today's:\n%s", name, stripANSI(m.View()))
		}
	}
	small := newModelWith(t, 79, 24, Options{Anim: true, MorningAnim: true})
	small.startRun(7)
	if _, cmd := small.Update(key("n")); cmd != nil || small.scene != nil || small.mode != modeReport {
		t.Fatalf("under 80 columns: cmd %v scene %v mode %v", cmd, small.scene, small.mode)
	}
}

// TestModalsFitMorning: modeReport mid-scene is the one modal at 80x24
// and 120x40: on body row 2, min(width-4, 76) wide, every box line
// whole and the body's rows the report's, at the start, the quarters
// and the last frame.
func TestModalsFitMorning(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		want := min(sz[0]-4, 76)
		m := richModel(t, sz[0], sz[1])
		m.opts.Anim, m.opts.MorningAnim = true, true
		m.mode, m.fastStop = modePlay, ""
		m.openReport()
		if !m.onReportScene(reportMorning) {
			t.Fatalf("%dx%d: mode %v scene %v", sz[0], sz[1], m.mode, m.scene)
		}
		off := m.modal(m.reportTitle(), m.reportLines(), m.modalFooter())
		_, _, offBox := modalBox(t, off)
		q := anim.MorningLength / 4
		for _, at := range []time.Duration{0, q, 2 * q, 3 * q, anim.MorningLength - anim.Frame} {
			tickAt(m, now.Add(at))
			if m.scene == nil {
				t.Fatalf("%dx%d at %v: the scene ended early", sz[0], sz[1], at)
			}
			view := m.View()
			what := fmt.Sprintf("%dx%d morning scene at %v", sz[0], sz[1], at)
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
			if len(box) != len(offBox) {
				t.Errorf("%s: a box of %d lines, the report's is %d", what, len(box), len(offBox))
			}
			// The body is the report's as it is: every row but the
			// title's is the finished modal's.
			for i := 2; i < len(box) && i < len(offBox); i++ {
				if box[i] != offBox[i] {
					t.Errorf("%s: box line %d is not the report's:\n%s\n%s", what, i, stripANSI(box[i]), stripANSI(offBox[i]))
				}
			}
		}
	}
}
