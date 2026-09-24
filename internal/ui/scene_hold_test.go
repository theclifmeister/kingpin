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

// The report's scenes hold (#203): resolved, the bust, the morning and
// the incident keep a quiet loop on the report until it closes, the
// report scrolling under it; the incident's scene is new, the weather
// printing in on the report's own row; and every held frame fits.

// incidentMorning is a model on seed 7 with the options given, played
// to day 3 (the first a row of the table can fire), the incident
// pacing exhausted so one is certain overnight, and n pressed: the
// report opens on an INCIDENT section.
func incidentMorning(t *testing.T, w, h int, opts Options, heat float64) *Model {
	t.Helper()
	m := newModelWith(t, w, h, opts)
	m.startRun(7)
	for m.w.Day < 3 {
		endDay(t, m)
		closeMorning(t, m)
	}
	m.w.Incidents.Last = m.w.Day - 40
	if heat > 0 {
		m.w.SetStock(m.w.Player.Location, m.w.Products[0], 20)
		m.w.Here().Heat = heat
	}
	day := m.w.Day
	m.Update(key("n"))
	if m.mode != modeReport || m.w.Day != day+1 || len(m.w.Report.Incident) == 0 {
		t.Fatalf("after n: mode %v day %d → %d incident %v", m.mode, day, m.w.Day, m.w.Report.Incident)
	}
	return m
}

// TestReportSceneHolds: on a bust morning, 3 s of ticks after Done
// still return a tick, the scene holding, and every frame still
// differs from the plain report; the title pulses and the level
// jitters once the rest is up; ↓ scrolls the report under the loop;
// enter closes it, the hold with it, and the next command is nil, a
// straggling tick's too. The morning's holds the same way, its wipe
// replaying.
func TestReportSceneHolds(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0)
	defer lipgloss.SetColorProfile(profile)
	off := bustMorning(t, 80, 24, Options{Anim: false}, 70, false)
	m := bustMorning(t, 80, 24, animOn, 70, false)
	now := time.Unix(1_700_000_000, 0)
	runOut(m, now, anim.BustLength)
	if m.scene == nil || !m.scene.Holding() {
		t.Fatalf("at the length: scene %v", m.scene)
	}
	red := strings.TrimSuffix(theme.Fg(theme.Heat).Render("R"), "R\x1b[0m")
	settled := m.View()
	reds, jitters := 0, 0
	for at := time.Duration(0); at <= 3*time.Second+anim.HoldRest/2; at += anim.Frame {
		if cmd := tickAt(m, now.Add(anim.BustLength+at)); cmd == nil || m.scene == nil || !m.scene.Holding() || m.mode != modeReport {
			t.Fatalf("%v into the hold: cmd %v scene %v mode %v", at, cmd, m.scene, m.mode)
		}
		view := m.View()
		if view == off.View() {
			t.Fatalf("%v into the hold the frame is the plain report", at)
		}
		_, _, box := modalBox(t, view)
		if strings.Contains(box[1], red+"MORNING") {
			reds++
		}
		if box[3] != strings.Split(settled, "\n")[5] {
			jitters++
		}
	}
	if reds == 0 || reds > 5*frames(300*time.Millisecond)+5 {
		t.Errorf("the title pulsed red on %d frames over the hold", reds)
	}
	if jitters == 0 {
		t.Error("the level never jittered past the rest")
	}
	// ↓ scrolls the report under the loop; the loop stays.
	if _, cmd := m.Update(key("down")); cmd != nil || m.modalScroll != 1 || m.scene == nil || !m.scene.Holding() {
		t.Fatalf("down on the hold: cmd %v scroll %d scene %v", cmd, m.modalScroll, m.scene)
	}
	if cmd := tickAt(m, now.Add(anim.BustLength+4*time.Second)); cmd == nil || !strings.Contains(stripANSI(m.View()), "↑ more") {
		t.Fatalf("scrolled under the loop: cmd %v\n%s", cmd, stripANSI(m.View()))
	}
	day := m.w.Day
	if _, cmd := m.Update(key("enter")); cmd != nil || m.scene != nil || m.mode != modePlay || m.w.Day != day {
		t.Fatalf("enter on the hold: cmd %v scene %v mode %v day %d → %d", cmd, m.scene, m.mode, day, m.w.Day)
	}
	if _, cmd := m.Update(frameMsg{at: now.Add(5 * time.Second), gen: m.sceneGen}); cmd != nil {
		t.Fatal("a straggling tick after the close issued another")
	}
	if _, cmd := m.Update(key("2")); cmd != nil || m.scene != nil {
		t.Fatalf("the key after the close: cmd %v scene %v", cmd, m.scene)
	}
	// The morning's: the same hold, the title's wipe replaying after
	// the rest.
	m = plainMorning(t, 80, 24, animOn)
	runOut(m, now, anim.MorningLength)
	whole := m.reportTitle()
	wipes := 0
	for at := time.Duration(0); at <= 3*time.Second+anim.HoldRest; at += anim.Frame {
		if cmd := tickAt(m, now.Add(anim.MorningLength+at)); cmd == nil || m.scene == nil || !m.scene.Holding() {
			t.Fatalf("%v into the morning's hold: cmd %v scene %v", at, cmd, m.scene)
		}
		_, _, box := modalBox(t, m.View())
		if title := strings.TrimSpace(strings.Trim(strings.TrimSpace(stripANSI(box[1])), "║")); title != whole {
			wipes++
			if at < anim.HoldRest || !strings.HasPrefix(whole, title) {
				t.Errorf("%v into the morning's hold the title is %q", at, title)
			}
		}
	}
	if wipes == 0 {
		t.Error("the morning's title never wiped again")
	}
	if _, cmd := m.Update(key("esc")); cmd != nil || m.scene != nil || m.mode != modePlay {
		t.Fatalf("esc on the morning's hold: cmd %v scene %v mode %v", cmd, m.scene, m.mode)
	}
}

// TestHeldFramesFit: every scene the registry says holds, up in its
// mode through DemoScene at 80x24 and 120x40, fits through assertFits
// on every frame of 5 s of its loop, the one modal on row 2 at its
// width with every box line whole, and the chain ticks throughout.
func TestHeldFramesFit(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		want := min(sz[0]-4, 76)
		d := demoModel(t, sz[0], sz[1])
		for _, sc := range anim.Scenes() {
			if !sc.Hold {
				continue
			}
			if cmd := d.DemoScene(sc.Name, ""); cmd == nil || d.scene == nil || !d.scene.Hold {
				t.Fatalf("%s: DemoScene started no hold: cmd %v scene %v", sc.Name, cmd, d.scene)
			}
			tickAt(d, now)
			distinct := map[string]bool{}
			for at := time.Duration(0); at <= 5*time.Second; at += anim.Frame {
				if cmd := tickAt(d, now.Add(sc.Length+at)); cmd == nil || d.scene == nil || !d.scene.Holding() {
					t.Fatalf("%s %v into the hold: cmd %v scene %v", sc.Name, at, cmd, d.scene)
				}
				view := d.View()
				what := fmt.Sprintf("%dx%d %s held at %v", sz[0], sz[1], sc.Name, at)
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
				distinct[view] = true
			}
			if len(distinct) < 3 {
				t.Errorf("%dx%d %s: %d distinct frames over 5 s of the hold", sz[0], sz[1], sc.Name, len(distinct))
			}
		}
	}
}

// TestIncidentSceneOnTheWeather: an incident morning opens the report
// on the incident's scene, printing on the INCIDENT section's own row
// and reading as the report at the end; a bust outranks it; a
// fast-forward's stopping morning plays it under the stop line; a key
// mid-scene resolves it, holding; enter closes on today's report byte
// for byte; animation off and under 80x24 start none.
func TestIncidentSceneOnTheWeather(t *testing.T) {
	if anim.IncidentLength > 1500*time.Millisecond {
		t.Fatalf("the incident's scene is %v long, over 1.5 s", anim.IncidentLength)
	}
	off := incidentMorning(t, 80, 24, Options{Anim: false}, 0)
	on := incidentMorning(t, 80, 24, animOn, 0)
	if !on.onReportScene(reportIncident) || !on.ticking {
		t.Fatalf("the incident morning: scene %v kind %v ticking %v", on.scene, on.reportScene, on.ticking)
	}
	now := time.Unix(1_700_000_000, 0)
	line := "  " + on.w.Report.Incident[0]
	row := incidentRow(off.reportLines(), off.w.Report)
	if row < 0 {
		t.Fatalf("the report has no row for %q", line)
	}
	tickAt(on, now)
	_, _, box := modalBox(t, on.View())
	if got := strings.TrimRight(strings.Trim(stripANSI(box[3+row]), "║ "), " "); strings.Contains(got, on.w.Report.Incident[0]) {
		t.Errorf("at the first frame the incident's row is up: %q", got)
	}
	frames := map[string]bool{}
	for at := time.Duration(0); at < anim.IncidentLength; at += anim.Frame {
		if cmd := tickAt(on, now.Add(at)); cmd == nil || on.scene == nil {
			t.Fatalf("at %v: cmd %v scene %v", at, cmd, on.scene)
		}
		assertFits(t, on.View(), 80, 24, "incident scene at "+at.String())
		frames[on.View()] = true
	}
	if len(frames) < 10 {
		t.Errorf("%d distinct frames over the scene", len(frames))
	}
	if cmd := tickAt(on, now.Add(anim.IncidentLength)); cmd == nil || on.scene == nil || !on.scene.Holding() {
		t.Fatalf("Done: cmd %v scene %v", cmd, on.scene)
	}
	if got, want := stripANSI(on.View()), stripANSI(off.View()); got != want {
		t.Fatalf("resolved, the report does not read as today's:\n%s\n%s", got, want)
	}
	if _, cmd := on.Update(key("enter")); cmd != nil || on.scene != nil || on.mode != modePlay {
		t.Fatalf("enter on the hold: cmd %v scene %v mode %v", cmd, on.scene, on.mode)
	}
	on.Update(key("r"))
	if on.View() != off.View() {
		t.Fatalf("reopened, the report is not today's:\n%s\n%s", stripANSI(on.View()), stripANSI(off.View()))
	}
	// A key mid-scene resolves it, holding.
	on = incidentMorning(t, 80, 24, animOn, 0)
	tickAt(on, now)
	if _, cmd := on.Update(key("x")); cmd != nil || on.scene == nil || !on.scene.Holding() || on.mode != modeReport {
		t.Fatalf("the skip: cmd %v scene %v mode %v", cmd, on.scene, on.mode)
	}
	// A bust outranks it.
	if m := incidentMorning(t, 80, 24, animOn, 70); !m.onReportScene(reportBust) {
		t.Errorf("a sting on an incident morning: kind %v", m.reportScene)
	}
	// Off, and under the floor: none, the report today's.
	if m := incidentMorning(t, 80, 24, Options{Anim: true, MorningAnim: false}, 0); !m.onReportScene(reportIncident) {
		t.Errorf("the morning's scene off alone: kind %v (the incident's is not the morning's)", m.reportScene)
	}
	if m := incidentMorning(t, 79, 24, animOn, 0); m.scene != nil {
		t.Error("under 80 columns: a scene")
	}
	if off.scene != nil || off.reportScene != reportNone {
		t.Errorf("animation off: scene %v kind %v", off.scene, off.reportScene)
	}
	// A fast-forward's stopping morning: the scene on the row under
	// the stop line.
	m := richModel(t, 80, 24)
	m.opts.Anim, m.opts.MorningAnim = true, true
	m.mode, m.fastStop = modePlay, "Stopped after 3 days: the strike on Fourth & Main."
	m.w.Report.Incident = []string{"The dockers are out: The Channel is shut for 6 days."}
	m.openReport()
	if !m.onReportScene(reportIncident) {
		t.Fatalf("after F: kind %v scene %v", m.reportScene, m.scene)
	}
	body := m.reportLines()
	want := 3 // the stop line, a blank, the heading
	if n := len(m.w.Report.Lead); n > 0 {
		want += n + 2 // TODAY, its lines and a blank (#354)
	}
	if i := incidentRow(body, m.w.Report); i != want || !strings.HasPrefix(stripANSI(body[0]), "Stopped after") {
		t.Errorf("the incident's row under the stop line is %d: %q", i, stripANSI(strings.Join(body[:4], "|")))
	}
	tickAt(m, now)
	tickAt(m, now.Add(anim.IncidentLength))
	_, _, held := modalBox(t, m.View())
	_, _, plain := modalBox(t, m.modal(m.reportTitle(), m.reportLines(), m.modalFooter()))
	if got, want := stripANSI(strings.Join(held, "\n")), stripANSI(strings.Join(plain, "\n")); got != want {
		t.Errorf("resolved after F, the report does not read as today's:\n%s\n%s", got, want)
	}
}

// TestModalsFitIncident: modeReport mid-scene is the one modal at
// 80x24 and 120x40, the box the report's own and every row but the
// incident's the report's, at the start, the quarters and the last
// frame.
func TestModalsFitIncident(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		want := min(sz[0]-4, 76)
		m := richModel(t, sz[0], sz[1])
		m.opts.Anim, m.opts.MorningAnim = true, true
		m.mode, m.fastStop = modePlay, ""
		m.w.Report.Incident = []string{"The dockers are out: The Channel is shut for 6 days."}
		m.openReport()
		if !m.onReportScene(reportIncident) {
			t.Fatalf("%dx%d: mode %v scene %v kind %v", sz[0], sz[1], m.mode, m.scene, m.reportScene)
		}
		_, _, offBox := modalBox(t, m.modal(m.reportTitle(), m.reportLines(), m.modalFooter()))
		row := 3 + incidentRow(m.reportLines(), m.w.Report)
		q := anim.IncidentLength / 4
		for _, at := range []time.Duration{0, q, 2 * q, 3 * q, anim.IncidentLength - anim.Frame} {
			tickAt(m, now.Add(at))
			view := m.View()
			what := fmt.Sprintf("%dx%d incident scene at %v", sz[0], sz[1], at)
			assertFits(t, view, sz[0], sz[1], what)
			top, width, box := modalBox(t, view)
			if top != 2 || width != want || len(box) != len(offBox) {
				t.Errorf("%s: the modal is on row %d, %d wide, %d lines; want row 2, %d wide, %d lines", what, top, width, len(box), want, len(offBox))
			}
			for i, l := range box {
				if lw := lipgloss.Width(strings.TrimSpace(stripANSI(l))); lw != want {
					t.Errorf("%s: box line %d is %d wide, not %d: %q", what, i, lw, want, stripANSI(l))
				}
				if i != row && i < len(offBox) && l != offBox[i] {
					t.Errorf("%s: box line %d is not the report's:\n%s\n%s", what, i, stripANSI(l), stripANSI(offBox[i]))
				}
			}
		}
	}
}

// frames is a duration in frames, the package's own.
func frames(d time.Duration) int { return int(d / anim.Frame) }
