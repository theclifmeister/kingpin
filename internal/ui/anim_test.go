package ui

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/anim"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The animation guards (#152). The one rule that changed is that the UI
// redraws on a tick as well as a key or a resize, and only while a
// scene is on screen: never in play mode, never with animation off, and
// the chain ends by itself when the scene does or a key ends it.

// newAnimModel is a fresh install with animation on: the one fixture
// that holds a scene, for the guards.
func newAnimModel(t *testing.T, w, h int) *Model {
	t.Helper()
	return newModelWith(t, w, h, Options{Anim: true})
}

// onMenu puts a fresh model on the start menu with the loop running:
// the resize is what starts it, inside Update, so Init has nothing to
// start.
func onMenu(t *testing.T, m *Model) tea.Cmd {
	t.Helper()
	m.mode = modeStart
	_, cmd := m.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
	return cmd
}

// tickAt lands a frame's tick at the time given, as the tea.Tick
// callback would, and returns Update's command.
func tickAt(m *Model, at time.Time) tea.Cmd {
	_, cmd := m.Update(frameMsg{at: at, gen: m.sceneGen})
	return cmd
}

// stub is a scene of one length for the interstitial mechanism, which
// no scene of the game's uses yet (#154 brings the first).
type stub struct{ over time.Duration }

func (s stub) Frame(t time.Duration, w, h int) []string { return []string{"scene"} }
func (s stub) Done(t time.Duration) bool                { return t >= s.over }

// TestNoTickInPlayMode: Update returns no command in play mode, on
// every screen for every key the table has, with animation on; and
// with animation off in every mode, so the fixtures and the README's
// captures never hold a scene.
func TestNoTickInPlayMode(t *testing.T) {
	m := newAnimModel(t, 120, 40)
	if m.mode != modePlay {
		t.Fatalf("a fresh install starts in mode %v", m.mode)
	}
	for s := 0; s < int(screenCount); s++ {
		m.mode, m.screen = modePlay, screen(s)
		for _, k := range handledKeys {
			if k == "q" || k == "ctrl+c" {
				continue // quitting is a command of its own
			}
			m.mode = modePlay
			if _, cmd := m.Update(key(k)); cmd != nil {
				t.Errorf("screen %d key %q: Update returned a command in play mode", s, k)
			}
			if m.scene != nil {
				t.Errorf("screen %d key %q: a scene is up in play mode", s, k)
			}
		}
		m.mode = modePlay
		if _, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40}); cmd != nil {
			t.Errorf("screen %d: a resize returned a command in play mode", s)
		}
	}
	off := richModel(t, 80, 24)
	for md := mode(0); md < modeCount; md++ {
		for _, k := range []string{"down", "esc"} {
			off.mode = md
			if _, cmd := off.Update(key(k)); cmd != nil {
				t.Errorf("mode %v key %q: Update returned a command with animation off", md, k)
			}
		}
		off.mode = md
		if _, cmd := off.Update(tea.WindowSizeMsg{Width: 80, Height: 24}); cmd != nil {
			t.Errorf("mode %v: a resize returned a command with animation off", md)
		}
		if off.scene != nil {
			t.Errorf("mode %v: a scene is up with animation off", md)
		}
	}
	if cmd := off.Init(); cmd != nil {
		t.Error("Init returned a command")
	}
}

// TestSceneStopsTicking: with animation on the start menu's Update
// returns a tick, from the resize that starts the loop and from every
// frame; a run started, it returns nil from then on; an interstitial
// ends on any key, which is consumed, and on Done, and the command
// after either is nil.
func TestSceneStopsTicking(t *testing.T) {
	m := newAnimModel(t, 80, 24)
	if cmd := m.Init(); cmd != nil {
		t.Fatal("Init returned a command")
	}
	if cmd := onMenu(t, m); cmd == nil || m.scene == nil || !m.scene.Idle {
		t.Fatalf("the start menu's resize started no loop: cmd %v scene %v", cmd, m.scene)
	}
	now := time.Unix(1_700_000_000, 0)
	for i := 0; i < 5; i++ {
		if cmd := tickAt(m, now.Add(time.Duration(i)*anim.Frame)); cmd == nil {
			t.Fatalf("frame %d: no tick followed", i)
		}
	}
	// A key on the menu acts on the menu and the loop plays on.
	if _, cmd := m.Update(key("down")); cmd != nil || m.startChoice != 1 || m.scene == nil {
		t.Fatalf("down on the menu: cmd %v row %d scene %v", cmd, m.startChoice, m.scene)
	}
	// A tick outstanding is already on its way: the key issues no second one.
	if _, cmd := m.Update(key("up")); cmd != nil {
		t.Fatal("a key issued a second tick while one was on its way")
	}
	// Enter on the empty slot starts a run: the loop stops and nothing
	// ticks from then on, a straggling frame included.
	if _, cmd := m.Update(key("enter")); cmd != nil || m.mode != modePlay || m.scene != nil {
		t.Fatalf("starting a run: cmd %v mode %v scene %v", cmd, m.mode, m.scene)
	}
	if _, cmd := m.Update(frameMsg{at: now.Add(time.Second), gen: m.sceneGen - 1}); cmd != nil {
		t.Fatal("a straggling tick issued another")
	}
	for _, k := range []string{"2", "n", "enter", "esc"} {
		if _, cmd := m.Update(key(k)); cmd != nil {
			t.Fatalf("key %q after the run started returned a command", k)
		}
	}
	// An interstitial: any key ends it and is consumed.
	m.mode = modePlay
	day := m.w.Day
	m.play(&anim.Player{Scene: stub{time.Second}, Accent: theme.Heat})
	if _, cmd := m.Update(key("n")); cmd != nil || m.scene != nil || m.w.Day != day {
		t.Fatalf("n on an interstitial: cmd %v scene %v day %d → %d", cmd, m.scene, day, m.w.Day)
	}
	if _, cmd := m.Update(key("n")); cmd != nil || m.w.Day != day+1 {
		t.Fatalf("the key after the scene ended: cmd %v day %d → %d", cmd, day, m.w.Day)
	}
	// And on Done: the tick chain runs it out and ends.
	m.mode = modePlay
	m.play(&anim.Player{Scene: stub{100 * time.Millisecond}, Accent: theme.Heat})
	if _, cmd := m.Update(key("esc")); cmd != nil || m.scene != nil {
		t.Fatal("esc did not end the interstitial")
	}
	m.play(&anim.Player{Scene: stub{100 * time.Millisecond}, Accent: theme.Heat})
	cmd := m.tick()
	if cmd == nil {
		t.Fatal("a scene played and no tick was issued")
	}
	if cmd := tickAt(m, now); cmd == nil || m.scene == nil {
		t.Fatal("the first frame ended the scene")
	}
	if cmd := tickAt(m, now.Add(50*time.Millisecond)); cmd == nil || m.scene == nil {
		t.Fatal("the scene ended before it was done")
	}
	if cmd := tickAt(m, now.Add(100*time.Millisecond)); cmd != nil || m.scene != nil {
		t.Fatalf("Done: cmd %v scene %v", cmd, m.scene)
	}
	if _, cmd := m.Update(key("esc")); cmd != nil {
		t.Fatal("a key after Done returned a command")
	}
}

// TestSceneNeverTouchesTheWorld: a run played through the UI with
// animation on, the title's loop run through a pass and its rest before
// the run starts, and one played with it off are gob-equal at day 10
// (compared decoded, as TestFastForwardIsTheSameDays does: gob walks a
// map in whatever order it finds it).
func TestSceneNeverTouchesTheWorld(t *testing.T) {
	play := func(opts Options) *game.World {
		m := newModelWith(t, 80, 24, opts)
		onMenu(t, m)
		now := time.Unix(1_700_000_000, 0)
		for i := 0; i < 200; i++ { // a pass, its rest and into the next
			tickAt(m, now.Add(time.Duration(i)*anim.Frame))
		}
		if opts.Anim && (m.scene == nil || m.scene.Pass() != 1) {
			t.Fatalf("the loop did not restart: %+v", m.scene)
		}
		m.startRun(7)
		for m.w.Day < 10 {
			m.w.SetStock(m.w.Player.Location, m.w.Products[0], 20)
			m.Update(key("s"))
			m.Update(key("enter"))
			m.Update(key("enter"))
			m.Update(key("enter"))
			m.Update(key("enter"))
			m.Update(key("esc"))
			endDay(t, m)
			m.Update(key("enter"))
		}
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(m.w); err != nil {
			t.Fatal(err)
		}
		var out game.World
		if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
			t.Fatal(err)
		}
		return &out
	}
	on, off := play(Options{Anim: true}), play(Options{Anim: false})
	if !reflect.DeepEqual(on, off) {
		t.Fatal("the run with animation on differs from the run with it off at day 10")
	}
}

// TestScenesFit: every registered scene's frames at t = 0, half and
// the end fit 80x24, 100x30 and 120x40 through assertFits, on their own
// and, for the title, drawn over the start menu.
func TestScenesFit(t *testing.T) {
	for _, sc := range anim.Scenes() {
		s := sc.New(1)
		end := time.Duration(0)
		for !s.Done(end) && end < 30*time.Second {
			end += anim.Frame
		}
		if end == 0 || end >= 30*time.Second {
			t.Fatalf("%s: done at %v", sc.Name, end)
		}
		for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
			for _, at := range []time.Duration{0, end / 2, end} {
				what := sc.Name + " at " + at.String()
				frame := s.Frame(at, sz[0], sz[1])
				if len(frame) != sz[1] {
					t.Errorf("%s: %d lines at %dx%d", what, len(frame), sz[0], sz[1])
				}
				assertFits(t, strings.Join(frame, "\n"), sz[0], sz[1], what)
			}
		}
	}
	// The title over the menu: the art's six rows, the box under them,
	// nothing cut at any size, and the settled art in gold.
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0) // termenv.TrueColor, so the gold is a colour
	defer lipgloss.SetColorProfile(profile)
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m := newAnimModel(t, sz[0], sz[1])
		onMenu(t, m)
		now := time.Unix(1_700_000_000, 0)
		for _, at := range []time.Duration{0, anim.TitleLength / 2, anim.TitleLength} {
			tickAt(m, now.Add(at))
			view := m.View()
			assertFits(t, view, sz[0], sz[1], "title at "+at.String())
			top, _, _ := modalBox(t, view)
			if top != 1+anim.NewText(anim.Kingpin).Height()+1 {
				t.Errorf("%dx%d at %v: the box is on row %d, not under the art", sz[0], sz[1], at, top)
			}
		}
		view := m.View()
		if !strings.Contains(view, theme.Fg(theme.Money).Render("██")) {
			t.Errorf("%dx%d: the settled art is not in gold", sz[0], sz[1])
		}
		if plain := stripANSI(view); !strings.Contains(plain, strings.Split(strings.Trim(anim.Kingpin, "\n"), "\n")[0]) {
			t.Errorf("%dx%d: the art did not settle:\n%s", sz[0], sz[1], plain)
		}
	}
}

// TestSceneIsDeterministic: the same seed renders the same frames
// twice, and another seed renders others.
func TestSceneIsDeterministic(t *testing.T) {
	for _, sc := range anim.Scenes() {
		a, b, c := sc.New(3), sc.New(3), sc.New(4)
		same, differ := true, false
		for at := time.Duration(0); at < 2*time.Second; at += anim.Frame {
			fa, fb, fc := a.Frame(at, 80, 24), b.Frame(at, 80, 24), c.Frame(at, 80, 24)
			same = same && strings.Join(fa, "\n") == strings.Join(fb, "\n")
			differ = differ || strings.Join(fa, "\n") != strings.Join(fc, "\n")
		}
		if !same {
			t.Errorf("%s: the same seed rendered different frames", sc.Name)
		}
		if !differ {
			t.Errorf("%s: two seeds rendered the same frames", sc.Name)
		}
	}
}

// TestTitleLoopIsTheMenuOtherwise: under 80x24, or with animation off,
// the start menu draws exactly as it did; the loop stops on a resize
// under the floor and starts again over it; and it stops when a slot
// is picked.
func TestTitleLoopIsTheMenuOtherwise(t *testing.T) {
	off := newTestModel(t, 80, 24)
	if cmd := onMenu(t, off); cmd != nil || off.scene != nil {
		t.Fatal("animation off: the menu started a loop")
	}
	on := newAnimModel(t, 80, 24)
	onMenu(t, on)
	// The models' status lines differ by seed; blank both.
	on.status, off.status = "", ""
	if _, cmd := on.Update(tea.WindowSizeMsg{Width: 79, Height: 24}); cmd != nil || on.scene != nil {
		t.Fatal("under 80 columns the loop plays on")
	}
	off.Update(tea.WindowSizeMsg{Width: 79, Height: 24})
	if on.View() != off.View() {
		t.Errorf("under 80x24 the menu is not the menu:\n%s\n%s", on.View(), off.View())
	}
	if _, cmd := on.Update(tea.WindowSizeMsg{Width: 80, Height: 23}); cmd != nil || on.scene != nil {
		t.Fatal("under 24 rows the loop plays on")
	}
	if _, cmd := on.Update(tea.WindowSizeMsg{Width: 80, Height: 24}); cmd == nil || on.scene == nil {
		t.Fatal("back at 80x24 the loop did not start")
	}
	off.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if on.View() == off.View() {
		t.Error("at 80x24 with animation on the menu shows no art")
	}
	// Continuing a save stops the loop as starting a run does.
	if err := game.Save(2, off.w); err != nil {
		t.Fatal(err)
	}
	on.startChoice = 1
	if _, cmd := on.Update(key("enter")); cmd != nil || on.scene != nil || on.mode != modePlay || on.slot != 2 {
		t.Fatalf("continuing slot 2: cmd %v scene %v mode %v slot %d", cmd, on.scene, on.mode, on.slot)
	}
}
