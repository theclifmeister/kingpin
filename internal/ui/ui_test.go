package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/content"
)

func newTestModel(t *testing.T, w, h int) *Model {
	t.Helper()
	t.Setenv("KINGPIN_HOME", t.TempDir())
	m, err := New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func key(s string) tea.KeyMsg {
	if len(s) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func assertFits(t *testing.T, view string, w, h int, what string) {
	t.Helper()
	ls := strings.Split(view, "\n")
	if len(ls) > h {
		t.Errorf("%s: %d lines > height %d", what, len(ls), h)
	}
	for i, l := range ls {
		if lw := lipgloss.Width(l); lw > w {
			t.Errorf("%s: line %d is %d cells wide > %d: %q", what, i, lw, w, l)
		}
	}
}

func TestRendersAtCommonSizes(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}, {100, 30}} {
		m := newTestModel(t, sz[0], sz[1])
		// Play a few days with some trading so every panel has content.
		for i := 0; i < 5; i++ {
			m.w.Player.Stock[m.w.Products[0]] = 40
			m.Update(key("s"))
			m.Update(key("enter")) // product
			m.Update(key("enter")) // qty (blank = all)
			m.Update(key("3"))     // aggressive
			m.Update(key("enter")) // confirm
			m.Update(key("n"))     // end day -> report
			assertFits(t, m.View(), sz[0], sz[1], "report")
			m.Update(key("enter"))
		}
		for _, s := range []string{"1", "2", "3"} {
			m.Update(key(s))
			assertFits(t, m.View(), sz[0], sz[1], "screen "+s)
		}
		m.Update(key("1"))
		m.Update(key("b"))
		assertFits(t, m.View(), sz[0], sz[1], "buy dialog")
		m.Update(key("enter"))
		assertFits(t, m.View(), sz[0], sz[1], "buy qty")
		m.Update(key("esc"))
		m.Update(key("esc"))
		m.Update(key("?"))
		assertFits(t, m.View(), sz[0], sz[1], "help")
		m.Update(key("x"))
	}
}

func TestBuyThenSellFlow(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.Update(key("b"))
	m.Update(key("enter")) // pick first product
	for _, r := range "10" {
		m.Update(key(string(r)))
	}
	m.Update(key("enter"))
	if m.mode != modePlay {
		t.Fatalf("buy did not complete: mode=%v err=%q", m.mode, m.dlg.err)
	}
	id := m.w.Products[0]
	if m.w.Player.Stock[id] != 10 {
		t.Fatalf("stock after buy = %d", m.w.Player.Stock[id])
	}
	m.Update(key("s"))
	m.Update(key("enter"))
	m.Update(key("enter")) // blank = all
	m.Update(key("1"))     // quiet
	m.Update(key("enter"))
	if o, ok := m.w.Orders[id]; !ok || o.Qty != 10 {
		t.Fatalf("order not placed: %+v", m.w.Orders)
	}
	m.Update(key("n"))
	if m.mode != modeReport || m.w.Day != 1 {
		t.Fatalf("end day: mode=%v day=%d", m.mode, m.w.Day)
	}
}

func TestTickerNeverWiderThanTerminal(t *testing.T) {
	m := newTestModel(t, 60, 20)
	for i := 0; i < 30; i++ {
		m.Update(key("n"))
		m.Update(key("enter"))
	}
	for i := 0; i < 200; i++ {
		m.Update(tickMsg{})
		if w := lipgloss.Width(m.viewTicker()); w > 60 {
			t.Fatalf("ticker width %d at tick %d", w, i)
		}
	}
}

// Enter confirms dialogs and closes the report, so a stray extra press must
// never burn a day; only n ends the day.
func TestEnterDoesNotEndDay(t *testing.T) {
	m := newTestModel(t, 80, 24)
	m.Update(key("n"))
	m.Update(key("enter")) // close report
	day := m.w.Day
	m.Update(key("enter"))
	m.Update(key("enter"))
	if m.w.Day != day {
		t.Fatalf("enter advanced the day from %d to %d", day, m.w.Day)
	}
	m.Update(key("n"))
	if m.w.Day != day+1 {
		t.Fatalf("n did not advance the day: %d -> %d", day, m.w.Day)
	}
}
