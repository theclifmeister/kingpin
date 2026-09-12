package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/anim"
)

// The ending's scene (#156): modeOver opens on a scene for the cause,
// any key or its end lands on the summary, which is today's byte for
// byte, and with animation off there is no scene at all.

// endIndicted ends the run on the next day: the DA's file is deeper
// than the arrest line, so the morning indicts. The day steps and
// saves through the same path as a played n.
func endIndicted(t *testing.T, m *Model) {
	t.Helper()
	m.w.Heat.Evidence = 10
	m.Update(key("n"))
	if m.w.Over == nil || m.w.Over.Cause != "indicted" || m.mode != modeOver {
		t.Fatalf("the run did not end: over %+v mode %v", m.w.Over, m.mode)
	}
}

// overCauses is every cause today, and one nobody has, which plays the
// default.
var overCauses = []string{"indicted", "arrested", "broke", "retired"}

// TestEndingSceneByCause: each cause starts its scene, an unknown one
// the default (the arrested scene), the day has stepped and saved
// before the first frame, and with animation off no scene starts and
// the summary is what is up.
func TestEndingSceneByCause(t *testing.T) {
	for _, cause := range overCauses {
		m := newAnimModel(t, 80, 24)
		m.w.Over = &game.Ending{Day: m.w.Day, Cause: cause, PeakCash: m.w.Stats.PeakCash}
		m.morning()
		if m.mode != modeOver || m.scene == nil || m.scene.Idle {
			t.Fatalf("%s: mode %v scene %v", cause, m.mode, m.scene)
		}
		var want anim.Scene
		switch cause {
		case "indicted":
			want = anim.Indicted(m.w.Heat.Evidence, m.overHeadline(), anim.Seed(m.w.Seed, m.w.Day, "over"))
		case "broke":
			want = anim.Broke(m.overFigures(), anim.Seed(m.w.Seed, m.w.Day, "over"))
		default:
			want = anim.Arrested(anim.Seed(m.w.Seed, m.w.Day, "over"))
		}
		for _, at := range []time.Duration{0, anim.OverLength / 2, anim.OverLength} {
			if got, w := strings.Join(m.scene.Scene.Frame(at, 72, 15), "\n"), strings.Join(want.Frame(at, 72, 15), "\n"); got != w {
				t.Errorf("%s at %v: the scene up is not the cause's", cause, at)
			}
		}
		if m.scene.Scene.Done(anim.OverLength-anim.Frame) || !m.scene.Scene.Done(anim.OverLength) {
			t.Errorf("%s: the scene is not %v long", cause, anim.OverLength)
		}
	}
	// The real path: n ends the day, the step indicts, the save holds
	// the ending and the scene is up with the first frame still to come.
	m := newAnimModel(t, 80, 24)
	m.startRun(11)
	day := m.w.Day
	endIndicted(t, m)
	if m.w.Day != day+1 || m.scene == nil {
		t.Fatalf("after n: day %d → %d, scene %v", day, m.w.Day, m.scene)
	}
	if saved, err := game.Load(m.slot, m.set.Migrations()...); err != nil || saved.Over == nil || saved.Over.Cause != "indicted" {
		t.Fatalf("the ending was not saved before the scene: %v %+v", err, saved.Over)
	}
	if !strings.Contains(m.View(), "GAME OVER") || strings.Contains(m.View(), "days survived") {
		t.Errorf("mid-scene the modal is not the scene's:\n%s", stripANSI(m.View()))
	}
	// Animation off: no scene, the summary at once.
	off := newTestModel(t, 80, 24)
	off.startRun(11)
	endIndicted(t, off)
	if off.scene != nil {
		t.Fatal("animation off: a scene started")
	}
	if v := stripANSI(off.View()); !strings.Contains(v, "INDICTED on day") || !strings.Contains(v, "days survived") {
		t.Errorf("animation off: the summary is not up:\n%s", v)
	}
	// A finished run continued from its save shows the summary alone.
	on := newAnimModel(t, 80, 24)
	if err := game.Save(2, off.w); err != nil {
		t.Fatal(err)
	}
	if err := on.continueRun(2); err != nil {
		t.Fatal(err)
	}
	if on.mode != modeOver || on.scene != nil {
		t.Fatalf("continuing a finished run: mode %v scene %v", on.mode, on.scene)
	}
	if !strings.Contains(stripANSI(on.View()), "days survived") {
		t.Error("continuing a finished run: the summary is not up")
	}
}

// summaryOf is the run's summary with animation off: the same seed
// ended the same way on a model that never holds a scene.
func summaryOf(t *testing.T, seed uint64) string {
	t.Helper()
	off := newTestModel(t, 80, 24)
	off.startRun(seed)
	endIndicted(t, off)
	return off.View()
}

// TestEndingSceneIsSkippable: any key ends the scene, is consumed (the
// run is still over, no new run started) and lands on the summary,
// byte for byte the one animation off draws; the key after acts on the
// summary.
func TestEndingSceneIsSkippable(t *testing.T) {
	want := summaryOf(t, 11)
	m := newAnimModel(t, 80, 24)
	m.startRun(11)
	endIndicted(t, m)
	if m.View() == want {
		t.Fatal("mid-scene the view is already the summary")
	}
	// n would start a new run on the summary; on the scene it is
	// consumed.
	if _, cmd := m.Update(key("n")); cmd != nil || m.scene != nil || m.mode != modeOver || m.w.Over == nil {
		t.Fatalf("n on the scene: cmd %v scene %v mode %v over %v", cmd, m.scene, m.mode, m.w.Over)
	}
	if got := m.View(); got != want {
		t.Errorf("the summary after the skip is not today's:\n%s\n%s", stripANSI(got), stripANSI(want))
	}
	if _, cmd := m.Update(key("n")); cmd != nil || m.w.Over != nil || m.mode != modePlay {
		t.Fatalf("n on the summary: cmd %v over %v mode %v", cmd, m.w.Over, m.mode)
	}
}

// TestEndingSceneEndsItself: the tick chain runs the scene out at
// OverLength, the command after is nil, and the summary is today's.
func TestEndingSceneEndsItself(t *testing.T) {
	want := summaryOf(t, 11)
	m := newAnimModel(t, 80, 24)
	m.startRun(11)
	m.w.Heat.Evidence = 10
	if _, cmd := m.Update(key("n")); cmd == nil || m.scene == nil {
		t.Fatalf("ending the day started no tick chain: cmd %v scene %v", cmd, m.scene)
	}
	now := time.Unix(1_700_000_000, 0)
	if cmd := tickAt(m, now); cmd == nil || m.scene == nil {
		t.Fatal("the first frame ended the scene")
	}
	if cmd := tickAt(m, now.Add(anim.OverLength/2)); cmd == nil || m.scene == nil {
		t.Fatal("the scene ended before its length")
	}
	if cmd := tickAt(m, now.Add(anim.OverLength)); cmd != nil || m.scene != nil {
		t.Fatalf("at the length: cmd %v scene %v", cmd, m.scene)
	}
	if got := m.View(); got != want {
		t.Errorf("the summary after the scene is not today's:\n%s\n%s", stripANSI(got), stripANSI(want))
	}
	if _, cmd := m.Update(key("down")); cmd != nil {
		t.Error("a key after the scene returned a command")
	}
}

// TestModalsFitOver: the GAME OVER modal mid-scene, for every cause, at
// 80x24 and 120x40: the one modal (on row 2, min(width-4, 76) wide,
// every box line that width) with the scene's frame filling its body,
// through assertFits.
func TestModalsFitOver(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		want := min(sz[0]-4, 76)
		for _, cause := range overCauses {
			m := richModel(t, sz[0], sz[1])
			m.opts.Anim = true
			m.w.Over = &game.Ending{Day: m.w.Day, Cause: cause, PeakCash: m.w.Stats.PeakCash}
			m.morning()
			if m.mode != modeOver || m.scene == nil {
				t.Fatalf("%s: mode %v scene %v", cause, m.mode, m.scene)
			}
			now := time.Unix(1_700_000_000, 0)
			tickAt(m, now)
			for _, at := range []time.Duration{0, anim.OverLength / 4, anim.OverLength / 2, anim.OverLength * 3 / 4, anim.OverLength - anim.Frame} {
				tickAt(m, now.Add(at))
				if m.scene == nil {
					t.Fatalf("%s: the scene ended at %v", cause, at)
				}
				view := m.View()
				what := fmt.Sprintf("%dx%d %s at %v", sz[0], sz[1], cause, at)
				assertFits(t, view, sz[0], sz[1], what)
				top, width, box := modalBox(t, view)
				if top != 2 || width != want {
					t.Errorf("%s: the modal is on row %d, %d wide, not row 2 and %d", what, top, width, want)
				}
				for i, l := range box {
					if lw := lipgloss.Width(strings.TrimSpace(stripANSI(l))); lw != want {
						t.Errorf("%s: box line %d is %d wide, not %d: %q", what, i, lw, want, stripANSI(l))
					}
				}
				if len(box) != m.modalRoom()+6 {
					t.Errorf("%s: the box is %d lines, the frame does not fill the room (%d)", what, len(box), m.modalRoom()+6)
				}
			}
		}
	}
}
