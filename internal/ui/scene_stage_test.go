package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/anim"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The stage's scene (#157): it plays where showStage opens the stage,
// any key skips it, it ends by itself, and either way the modal after
// is #149's, byte for byte.

// stageMorning is an anim model the morning tier 2 is entered: a hire
// on the payroll, n pressed, the stage up with its scene. It returns
// the model and the command the morning's Update issued.
func stageMorning(t *testing.T, w, h int) (*Model, tea.Cmd) {
	t.Helper()
	m := newAnimModel(t, w, h)
	hireOne(m)
	_, cmd := m.Update(key("n"))
	return m, cmd
}

// TestStageSceneOnAStage: a morning entering a tier starts the scene
// (and the tick chain), the day stepped and saved before its first
// frame; a plain morning starts none; animation off starts none; under
// 80x24 none.
func TestStageSceneOnAStage(t *testing.T) {
	m, cmd := stageMorning(t, 80, 24)
	if m.mode != modeStage || m.stage != 2 || m.scene == nil || m.scene.Idle || cmd == nil {
		t.Fatalf("the morning: mode %v stage %d scene %v cmd %v", m.mode, m.stage, m.scene, cmd)
	}
	if saved, err := game.Load(m.slot); err != nil || saved.Day != m.w.Day || saved.StagePending() != 2 {
		t.Fatalf("the save before the first frame: day %d pending %d err %v", saved.Day, saved.StagePending(), err)
	}
	// Closed, the next morning is a plain one: the report on the
	// morning's scene (#159), not the stage's.
	m.Update(key("x")) // the skip
	m.Update(key("enter"))
	closeMorning(t, m)
	if _, cmd := m.Update(key("n")); cmd == nil || m.mode != modeReport || !m.onReportScene(reportMorning) {
		t.Fatalf("a plain morning: cmd %v scene %v mode %v", cmd, m.scene, m.mode)
	}
	// Animation off: the modal alone.
	off := newTestModel(t, 80, 24)
	hireOne(off)
	if _, cmd := off.Update(key("n")); cmd != nil || off.scene != nil || off.mode != modeStage {
		t.Fatalf("animation off: cmd %v scene %v mode %v", cmd, off.scene, off.mode)
	}
	// Under the floor: the modal alone.
	small := newAnimModel(t, 79, 24)
	hireOne(small)
	if _, cmd := small.Update(key("n")); cmd != nil || small.scene != nil || small.mode != modeStage {
		t.Fatalf("under 80 columns: cmd %v scene %v mode %v", cmd, small.scene, small.mode)
	}
}

// TestStageSceneIsSkippable: a key during the scene ends it and is
// consumed (the stage is not yet seen), the view after is the modal
// animation off draws for the same save, byte for byte, and the next
// enter is the modal's: SeeStage fires on it and the report opens.
func TestStageSceneIsSkippable(t *testing.T) {
	m, _ := stageMorning(t, 80, 24)
	off := continued(t, m, Options{Anim: false})
	on := continued(t, m, Options{Anim: true})
	if on.scene == nil || on.mode != modeStage || off.scene != nil || off.mode != modeStage {
		t.Fatalf("continued: on scene %v mode %v, off scene %v mode %v", on.scene, on.mode, off.scene, off.mode)
	}
	if on.View() == off.View() {
		t.Fatal("the scene's first frame is the finished modal")
	}
	if _, cmd := on.Update(key("x")); cmd != nil || on.scene != nil || on.mode != modeStage || on.w.Progression.Seen[2] {
		t.Fatalf("the skip: cmd %v scene %v mode %v seen %v", cmd, on.scene, on.mode, on.w.Progression.Seen)
	}
	if on.View() != off.View() {
		t.Fatalf("after the skip the modal is not #149's:\n%s\n%s", on.View(), off.View())
	}
	on.Update(key("enter"))
	if on.mode != modeReport || !on.w.Progression.Seen[2] || on.w.StagePending() != 0 {
		t.Fatalf("enter after the skip: mode %v seen %v", on.mode, on.w.Progression.Seen)
	}
}

// TestStageSceneEndsItself: ticked to Done the scene comes off and the
// view is the same finished modal, byte for byte; a key after it is
// the modal's.
func TestStageSceneEndsItself(t *testing.T) {
	m, _ := stageMorning(t, 80, 24)
	off := continued(t, m, Options{Anim: false})
	on := continued(t, m, Options{Anim: true})
	now := time.Unix(1_700_000_000, 0)
	if cmd := on.tick(); cmd == nil {
		t.Fatal("no tick for the scene")
	}
	frames := map[string]bool{}
	for at := time.Duration(0); at < anim.StageLength; at += anim.Frame {
		if cmd := tickAt(on, now.Add(at)); cmd == nil || on.scene == nil {
			t.Fatalf("at %v: cmd %v scene %v", at, cmd, on.scene)
		}
		view := on.View()
		assertFits(t, view, 80, 24, "stage scene at "+at.String())
		frames[view] = true
	}
	if len(frames) < 20 {
		t.Errorf("%d distinct frames over the scene", len(frames))
	}
	if cmd := tickAt(on, now.Add(anim.StageLength)); cmd != nil || on.scene != nil || on.mode != modeStage {
		t.Fatalf("Done: cmd %v scene %v mode %v", cmd, on.scene, on.mode)
	}
	if on.View() != off.View() {
		t.Fatalf("after Done the modal is not #149's:\n%s\n%s", on.View(), off.View())
	}
	on.Update(key("enter"))
	if on.mode != modeReport || !on.w.Progression.Seen[2] {
		t.Fatalf("enter after Done: mode %v seen %v", on.mode, on.w.Progression.Seen)
	}
}

// continued is a fresh model with the options given, continuing m's
// slot the way a start from the menu does: the size known first.
func continued(t *testing.T, m *Model, opts Options) *Model {
	t.Helper()
	c, err := New(m.cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	c.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
	if err := c.continueRun(m.slot); err != nil {
		t.Fatal(err)
	}
	return c
}

// TestStageSceneReadsTheModal: the scene's title and body are the
// modal's own lines (stageTitle, stageLines less their styles), never
// a copy: its last frame reads line for line as the modal's body, the
// title in gold; and it plays on the fast-forward's stopping morning.
func TestStageSceneReadsTheModal(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0)
	defer lipgloss.SetColorProfile(profile)
	m := newAnimModel(t, 80, 24)
	hireOne(m)
	day := m.w.Day
	fast(t, m, 30)
	if m.mode != modeStage || m.w.Day != day+1 || m.scene == nil || !m.ticking {
		t.Fatalf("after F: mode %v day %d scene %v ticking %v", m.mode, m.w.Day, m.scene, m.ticking)
	}
	body := m.stageLines(m.stage)
	frame := m.scene.Scene.Frame(anim.StageLength, m.modalInner(), 1+len(body))
	if got, want := stripANSI(frame[0]), strings.ToUpper(m.stageTitle(m.stage)); got != want {
		t.Errorf("the title is %q, want %q", got, want)
	}
	if !strings.Contains(frame[0], theme.Fg(theme.Money).Render("STAGE")) {
		t.Errorf("the title is not in gold: %q", frame[0])
	}
	for i, l := range body {
		if got, want := frame[1+i], strings.TrimRight(ansi.Strip(l), " "); got != want {
			t.Errorf("body line %d is %q, want %q", i, got, want)
		}
	}
	// The frame draws in the same box: the title row and the body as
	// the modal lays them out.
	view := stripANSI(m.View())
	if !strings.Contains(view, "STAGE 2 · CREW") && !strings.Contains(view, "█") {
		t.Errorf("the first frame shows neither the title nor the head:\n%s", view)
	}
}

// TestModalsFitStage: modeStage mid-scene is the one modal at 80x24
// and 120x40: on body row 2, min(width-4, 76) wide, every box line
// whole, at the start, the quarters and the last frame.
func TestModalsFitStage(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		want := min(sz[0]-4, 76)
		m := richModel(t, sz[0], sz[1])
		m.opts.Anim = true
		delete(m.w.Progression.Seen, 4)
		m.showStage()
		if m.mode != modeStage || m.scene == nil {
			t.Fatalf("%dx%d: mode %v scene %v", sz[0], sz[1], m.mode, m.scene)
		}
		q := anim.StageLength / 4
		for _, at := range []time.Duration{0, q, 2 * q, 3 * q, anim.StageLength - anim.Frame} {
			tickAt(m, now.Add(at))
			if m.scene == nil {
				t.Fatalf("%dx%d at %v: the scene ended early", sz[0], sz[1], at)
			}
			view := m.View()
			what := fmt.Sprintf("%dx%d stage scene at %v", sz[0], sz[1], at)
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
			// The borders, the title, two blanks and the footer round the body.
			if len(box) != len(m.stageLines(m.stage))+6 {
				t.Errorf("%s: a box of %d lines for a body of %d", what, len(box), len(m.stageLines(m.stage)))
			}
		}
	}
}

// TestModalTitledIsTheModal: modal(title, ...) is modalTitled with the
// title in caps in theme.Title, cut to the width, as it rendered before
// the split (#157), a long title included.
func TestModalTitledIsTheModal(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		m := richModel(t, sz[0], sz[1])
		body := []string{"One line.", "", theme.Gold.Render("Two.")}
		for _, title := range []string{"Stage 2 · Crew", "NEW RUN?", strings.Repeat("A long title ", 8)} {
			want := m.modalTitled(theme.Title.Render(ansi.Truncate(strings.ToUpper(title), m.modalInner(), "…")), body, m.modalFooter())
			if got := m.modal(title, body, m.modalFooter()); got != want {
				t.Errorf("%dx%d %q: modal and modalTitled differ:\n%s\n%s", sz[0], sz[1], title, got, want)
			}
		}
	}
}
