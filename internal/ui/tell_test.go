package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// eyeFree points the tell at the first free corner of the home city and
// returns it.
func eyeFree(t *testing.T, m *Model) *game.Corner {
	t.Helper()
	home := m.w.Home()
	for i := range home.Corners {
		if c := &home.Corners[i]; c.Owner == game.OwnerNone {
			m.w.Rival.Eyeing, m.w.Rival.EyeingDay = c.ID, m.w.Day
			return c
		}
	}
	t.Fatal("fixture: no free corner at home")
	return nil
}

// The tell (#69) is drawn where the player looks: the map marks the
// eyed corner ? in the rival's colour and its cell reads `theirs
// tomorrow`, the inspector says they set up there tomorrow and c is the
// answer, the dashboard's RIVALS panel (LAW's last line at 80 columns)
// and the rivals screen's leader line read `eyeing <corner>`; and once
// somebody is posted on it every trace goes.
func TestTellIsDrawn(t *testing.T) {
	for _, sz := range [][2]int{{120, 40}, {80, 24}} {
		m := richModel(t, sz[0], sz[1])
		m.w.Rival.Deals = nil // the fixture's split gives the free corners to the rival; a post there is refused
		eyed := eyeFree(t, m)
		word := "eyeing " + eyed.Name
		m.Update(key("5"))
		for i := range m.w.Home().Corners {
			if m.w.Home().Corners[i].ID == eyed.ID {
				m.mapCursor = i
			}
		}
		view := stripANSI(m.View())
		assertFits(t, m.View(), sz[0], sz[1], "map with the tell")
		if !strings.Contains(view, "? "+strings.ToUpper(eyed.Name)) || !strings.Contains(view, "theirs tomorrow") {
			t.Errorf("%dx%d: the map does not mark %s:\n%s", sz[0], sz[1], eyed.Name, view)
		}
		if text := paneText(m); !strings.Contains(text, "they set up here tomorrow") || !strings.Contains(text, "post a runner to keep them off") {
			t.Errorf("%dx%d: the inspector does not carry the tell:\n%s", sz[0], sz[1], text)
		}
		m.Update(key("1"))
		if view := stripANSI(m.View()); !strings.Contains(view, word) {
			t.Errorf("%dx%d: the dashboard does not read %q:\n%s", sz[0], sz[1], word, view)
		}
		assertFits(t, m.View(), sz[0], sz[1], "dashboard with the tell")
		m.Update(key("8"))
		if line := strings.Split(stripANSI(m.View()), "\n")[2]; !strings.Contains(line, word) {
			t.Errorf("%dx%d: the rivals screen's leader line does not read %q: %q", sz[0], sz[1], word, line)
		}
		assertFits(t, m.View(), sz[0], sz[1], "rivals with the tell")
		// Posted on, the corner is yours and the tell is nothing to say.
		if err := m.w.Post(eyed.ID, game.You); err != nil {
			t.Fatal(err)
		}
		for _, s := range []string{"1", "8", "5"} {
			m.Update(key(s))
			if view := stripANSI(m.View()); strings.Contains(view, word) || strings.Contains(view, "theirs tomorrow") || strings.Contains(view, "? "+strings.ToUpper(eyed.Name)) {
				t.Errorf("%dx%d: screen %s still carries the tell after the post:\n%s", sz[0], sz[1], s, view)
			}
		}
	}
}

// A fast-forward stops on the morning of the tell (#69): the rival that
// claims every day it can gives one the first night, and F stops there
// with the report naming it, the day the player is given to answer.
func TestFastForwardStopsOnTheTell(t *testing.T) {
	cfg := content.MustLoad()
	for k, p := range cfg.Rivals.Personality {
		p.ClaimChance = 1
		cfg.Rivals.Personality[k] = p
	}
	cfg.Rivals.Pace.ClaimScaleMin, cfg.Rivals.Pace.ClaimScaleMax = 1, 1
	t.Setenv("KINGPIN_HOME", t.TempDir())
	m, err := New(cfg, Options{Anim: false})
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.startRun(7)
	w := m.w
	home := w.Home()
	home.Corners[0].Owner, home.Corners[0].Since = game.OwnerRival, 1
	w.Rival.Arrived, w.Rival.Cash = 1, 50_000
	day := w.Day
	fast(t, m, 30)
	if w.Day != day+1 || w.Rival.Eyeing == "" {
		t.Fatalf("day %d -> %d, eyeing %q", day, w.Day, w.Rival.Eyeing)
	}
	if m.mode == modeCard {
		m.Update(key("enter"))
		m.Update(key("enter"))
	}
	want := "Stopped after 1 day: " + w.Rival.Leader + " is eyeing " + w.Corner(w.Rival.Eyeing).Name + "."
	if got := reportLine(t, m); got != want {
		t.Fatalf("the report opens with %q, want %q", got, want)
	}
}
