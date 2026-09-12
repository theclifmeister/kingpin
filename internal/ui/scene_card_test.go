package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/anim"
)

// The card's scene (#154): the morning a card is pending it is dealt,
// the title decrypting in the title bar and the prose wiping in under
// it; any key skips to the finished card; ticks to Done land on the
// same view a skip does; with animation off the card is exactly the
// one before the scene existed.

// cardLength is the scene's whole: the title's and the prose's.
const cardLength = anim.CardTitleLength + anim.CardProseLength

// dealAnimated puts a fresh run with animation on, on a seed of its
// own, on the morning a card is dealt: mode card, the scene up, the
// tick chain started.
func dealAnimated(t *testing.T, w, h int, anim bool) *Model {
	t.Helper()
	m := newModelWith(t, w, h, Options{Anim: anim})
	m.startRun(7)
	m.w.Dilemmas.Pending = testCard(m.w.Day + 1)
	day := m.w.Day
	_, cmd := m.Update(key("n"))
	if m.mode != modeCard || m.w.Day != day+1 || m.w.Dilemmas.Pending == nil {
		t.Fatalf("after n: mode %v day %d pending %v", m.mode, m.w.Day, m.w.Dilemmas.Pending)
	}
	if (cmd != nil) != anim || (m.scene != nil) != anim {
		t.Fatalf("animation %v: cmd %v scene %v", anim, cmd, m.scene)
	}
	return m
}

// TestCardSceneIsSkippable: a key during the scene lands on the
// finished card, consumed, and the following 1 answers it; the day
// had stepped and saved before the first frame; the finished card is
// byte-for-byte the card with animation off.
func TestCardSceneIsSkippable(t *testing.T) {
	m := dealAnimated(t, 80, 24, true)
	off := dealAnimated(t, 80, 24, false)
	now := time.Unix(1_700_000_000, 0)
	if cmd := tickAt(m, now); cmd == nil {
		t.Fatal("the first frame issued no tick")
	}
	mid := m.View()
	assertFits(t, mid, 80, 24, "card mid-scene")
	if p := stripANSI(mid); strings.Contains(p, "1 Pay") {
		t.Fatalf("the choices are up before Done:\n%s", p)
	}
	if m.status != fmt.Sprintf("Day %d saved.", m.w.Day) {
		t.Fatalf("the day was not saved before the first frame: %q", m.status)
	}
	cash := m.w.Player.DirtyCash
	if _, cmd := m.Update(key("1")); cmd != nil || m.scene != nil || m.mode != modeCard || m.cardDone || m.w.Player.DirtyCash != cash {
		t.Fatalf("1 during the scene: cmd %v scene %v mode %v done %v cash %d → %d", cmd, m.scene, m.mode, m.cardDone, cash, m.w.Player.DirtyCash)
	}
	if got, want := m.View(), off.View(); got != want {
		t.Fatalf("the finished card is not the card with animation off:\n%s\n---\n%s", stripANSI(got), stripANSI(want))
	}
	if _, cmd := m.Update(key("1")); cmd != nil || !m.cardDone || m.w.Player.DirtyCash != cash-100 || m.scene != nil {
		t.Fatalf("1 after the skip: cmd %v done %v cash %d → %d scene %v", cmd, m.cardDone, cash, m.w.Player.DirtyCash, m.scene)
	}
	// The outcome is not animated, and a straggling tick starts nothing.
	if cmd := tickAt(m, now.Add(time.Second)); cmd != nil || m.scene != nil {
		t.Fatal("a tick after the scene ended issued another")
	}
}

// TestCardSceneEndsItself: ticks to Done land on the same view a skip
// does, the tick chain ends there, and every frame on the way fits;
// the title is the card's from the title's length on.
func TestCardSceneEndsItself(t *testing.T) {
	m := dealAnimated(t, 80, 24, true)
	skipped := dealAnimated(t, 80, 24, true)
	skipped.Update(key("esc"))
	now := time.Unix(1_700_000_000, 0)
	titled := false
	for at := time.Duration(0); at < cardLength; at += anim.Frame {
		if cmd := tickAt(m, now.Add(at)); cmd == nil || m.scene == nil {
			t.Fatalf("%v: the scene ended early: cmd %v scene %v", at, cmd, m.scene)
		}
		view := m.View()
		assertFits(t, view, 80, 24, "card at "+at.String())
		_, _, box := modalBox(t, view)
		title := strings.TrimSpace(strings.Trim(strings.TrimSpace(stripANSI(box[1])), "║"))
		if at >= anim.CardTitleLength && title != "A TEST" {
			t.Errorf("%v: the title is %q", at, title)
		}
		titled = titled || title == "A TEST"
	}
	if !titled {
		t.Error("the title never settled")
	}
	if cmd := tickAt(m, now.Add(cardLength)); cmd != nil || m.scene != nil {
		t.Fatalf("Done: cmd %v scene %v", cmd, m.scene)
	}
	if got, want := m.View(), skipped.View(); got != want {
		t.Fatalf("Done is not the skip:\n%s\n---\n%s", stripANSI(got), stripANSI(want))
	}
	if _, cmd := m.Update(key("2")); cmd != nil || !m.cardDone {
		t.Fatalf("2 after Done: cmd %v done %v", cmd, m.cardDone)
	}
}

// TestCardSceneOnContinue: a save continued on a card plays the scene
// (the save restores the card; the scene is UI), from the menu and
// from -slot before the terminal has a size, where the scene waits
// for the first resize's tick; under 80x24 it plays nothing.
func TestCardSceneOnContinue(t *testing.T) {
	m := dealAnimated(t, 80, 24, true)
	m.Update(key("esc"))
	if err := m.continueRun(m.slot); err != nil {
		t.Fatal(err)
	}
	if m.mode != modeCard || m.scene == nil || m.scene.Idle {
		t.Fatalf("continued on a card: mode %v scene %v", m.mode, m.scene)
	}
	m.Update(key("esc"))
	// -slot N: NewSlot continues before the first WindowSizeMsg.
	m.width, m.height = 0, 0
	if err := m.continueRun(m.slot); err != nil {
		t.Fatal(err)
	}
	if m.scene == nil {
		t.Fatal("continued before the size was known: no scene")
	}
	if _, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24}); cmd == nil {
		t.Fatal("the first resize issued no tick")
	}
	tickAt(m, time.Unix(1_700_000_000, 0))
	assertFits(t, m.View(), 80, 24, "card after -slot")
	m.Update(key("esc"))
	// Under the floor: the card, no scene.
	m.Update(tea.WindowSizeMsg{Width: 79, Height: 24})
	if err := m.continueRun(m.slot); err != nil {
		t.Fatal(err)
	}
	if m.scene != nil {
		t.Fatal("a scene under 80x24")
	}
}

// TestCardSceneOnFastForward: the stopping morning's card plays its
// scene and no scene plays mid-loop (one scene over the whole run, the
// stop's).
func TestCardSceneOnFastForward(t *testing.T) {
	m := richModelSeeded(t, 80, 24, 4)
	m.opts.Anim = true
	gen := m.sceneGen
	for m.w.Day < 40 && m.mode != modeCard {
		fast(t, m, 30)
		if m.mode != modeCard {
			closeMorning(t, m)
		}
	}
	if m.mode != modeCard {
		t.Fatalf("seed 4 dealt no card by day 40")
	}
	if m.scene == nil || m.scene.Idle || m.sceneGen != gen+1 {
		t.Fatalf("on the stop: scene %v, %d scenes played", m.scene, m.sceneGen-gen)
	}
	assertFits(t, m.View(), 80, 24, "card after F")
	m.Update(key("esc"))
	if !strings.HasSuffix(m.fastStop, ": a card to answer.") {
		t.Fatalf("stop line %q", m.fastStop)
	}
}

// TestModalsFitCard: modeCard mid-scene at 80x24 and 120x40 is the one
// modal, as TestModalsFit holds every mode to: on row 2, the width, the
// title in caps, the blanks round the body, no key hint in it, and the
// box its finished size from the first frame.
func TestModalsFitCard(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		want := min(sz[0]-4, 76)
		m := richModel(t, sz[0], sz[1])
		m.opts.Anim = true
		m.w.Dilemmas.Pending = testCard(m.w.Day)
		m.showCard()
		if m.mode != modeCard || m.scene == nil {
			t.Fatalf("%dx%d: mode %v scene %v", sz[0], sz[1], m.mode, m.scene)
		}
		now := time.Unix(1_700_000_000, 0)
		var height int
		for _, at := range []time.Duration{0, anim.CardTitleLength / 2, anim.CardTitleLength, cardLength - anim.Frame, cardLength} {
			tickAt(m, now.Add(at))
			view := m.View()
			what := fmt.Sprintf("%dx%d card at %v", sz[0], sz[1], at)
			assertFits(t, view, sz[0], sz[1], what)
			top, width, box := modalBox(t, view)
			if top != 2 || width != want {
				t.Errorf("%s: the modal is on row %d, %d wide, not row 2 and %d:\n%s", what, top, width, want, stripANSI(view))
			}
			for i, l := range box {
				if lw := lipgloss.Width(strings.TrimSpace(stripANSI(l))); lw != want {
					t.Errorf("%s: box line %d is %d wide, not %d: %q", what, i, lw, want, stripANSI(l))
				}
			}
			inner := func(i int) string {
				return strings.TrimSpace(strings.Trim(strings.TrimSpace(stripANSI(box[i])), "║"))
			}
			if title := inner(1); title != strings.ToUpper(title) {
				t.Errorf("%s: the title is %q", what, title)
			}
			if inner(2) != "" || inner(len(box)-3) != "" {
				t.Errorf("%s: no blank around the body:\n%s", what, stripANSI(strings.Join(box, "\n")))
			}
			for _, l := range box[3 : len(box)-3] {
				for _, hint := range []string{"enter ", "esc ", "any other key", "any key"} {
					if strings.Contains(stripANSI(l), hint) {
						t.Errorf("%s: a key hint in the body: %q", what, strings.TrimSpace(stripANSI(l)))
					}
				}
			}
			if height == 0 {
				height = len(box)
			}
			if len(box) != height {
				t.Errorf("%s: the box is %d lines, was %d", what, len(box), height)
			}
		}
		if m.scene != nil {
			t.Errorf("%dx%d: the scene did not end", sz[0], sz[1])
		}
	}
}
