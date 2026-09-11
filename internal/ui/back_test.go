package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Back is one key and close is one key (#110). From the sell dialog's
// dial step, shift+tab twice is the product step with the product
// still selected and the quantity cleared, a third is a no-op, tab goes
// forward once the step is complete and is silent otherwise, and esc
// closes the dialog whole from any step; the same on the buy, target,
// cart and propose dialogs.
func TestBackIsOneKey(t *testing.T) {
	type dialog struct {
		name  string
		mode  mode
		open  func(m *Model) // opens the dialog on its first step
		step  func(m *Model) int
		pick  func(m *Model) int // what the first step chose: it survives going back
		last  int                // the last step, where tab is silent
		enter []string           // enter and what to type to reach the last step
	}
	dialogs := []dialog{
		{"sell", modeSell, func(m *Model) { m.Update(key("s")) }, func(m *Model) int { return m.dlg.step }, func(m *Model) int { return m.cursor }, 2, []string{"enter", "5", "enter"}},
		{"buy", modeBuy, func(m *Model) { m.Update(key("b")) }, func(m *Model) int { return m.dlg.step }, func(m *Model) int { return m.cursor }, 1, []string{"enter"}},
		{"target", modeTarget, func(m *Model) {
			m.Update(key("5"))
			m.Update(key("]"))
			m.onRoutes = true
			m.Update(key("R"))
		}, func(m *Model) int { return m.tgt.step }, func(m *Model) int { return m.cursor }, 2, []string{"enter", "enter"}},
		{"cart", modeCart, func(m *Model) { fillCart(t, m); m.Update(key("c")); m.Update(key("down")) }, func(m *Model) int { return m.crt.step }, func(m *Model) int { return m.crt.cursor }, 1, []string{"enter"}},
		{"propose", modePropose, func(m *Model) { m.Update(key("8")); m.Update(key("d")); m.Update(key("down")) }, func(m *Model) int { return m.proposeStep }, func(m *Model) int { return m.proposeCursor }, 1, []string{"enter"}},
	}
	for _, d := range dialogs {
		m := richModel(t, 100, 30)
		d.open(m)
		if m.mode != d.mode || d.step(m) != 0 {
			t.Fatalf("%s: mode %v step %d, status %q", d.name, m.mode, d.step(m), m.status)
		}
		screen := m.screen
		picked := d.pick(m)
		// shift+tab on the first step is a no-op, and neither key leaves
		// the dialog for another screen.
		m.Update(key("shift+tab"))
		if m.mode != d.mode || d.step(m) != 0 || d.pick(m) != picked || m.screen != screen {
			t.Fatalf("%s: shift+tab on the first step: mode %v step %d picked %d", d.name, m.mode, d.step(m), d.pick(m))
		}
		for _, k := range d.enter {
			m.Update(key(k))
		}
		if m.mode != d.mode || d.step(m) != d.last {
			t.Fatalf("%s: after %v: mode %v step %d, status %q", d.name, d.enter, m.mode, d.step(m), m.status)
		}
		// tab on the last step is silent: enter is what commits.
		m.Update(key("tab"))
		if m.mode != d.mode || d.step(m) != d.last || m.screen != screen {
			t.Fatalf("%s: tab on the last step: mode %v step %d screen %v", d.name, m.mode, d.step(m), m.screen)
		}
		if d.name == "sell" {
			// Back from the dial keeps the quantity; back from the quantity
			// clears it, as leaving the step does.
			m.Update(key("shift+tab"))
			if d.step(m) != 1 || m.dlg.qty.Value() != "5" {
				t.Fatalf("sell: shift+tab from the dial: step %d quantity %q", d.step(m), m.dlg.qty.Value())
			}
			if !strings.Contains(stripANSI(m.View()), "⇧tab back") {
				t.Fatalf("sell: the quantity step's footer lacks ⇧tab back:\n%s", stripANSI(m.View()))
			}
		}
		for d.step(m) > 0 {
			before := d.step(m)
			m.Update(key("shift+tab"))
			if m.mode != d.mode || d.step(m) != before-1 {
				t.Fatalf("%s: shift+tab from step %d: mode %v step %d", d.name, before, m.mode, d.step(m))
			}
		}
		if d.pick(m) != picked {
			t.Fatalf("%s: the first step's choice moved from %d to %d", d.name, picked, d.pick(m))
		}
		if d.name == "sell" && m.dlg.qty.Value() != "" {
			t.Fatalf("sell: the quantity is %q back on the product step", m.dlg.qty.Value())
		}
		m.Update(key("shift+tab"))
		if m.mode != d.mode || d.step(m) != 0 {
			t.Fatalf("%s: a third shift+tab: mode %v step %d", d.name, m.mode, d.step(m))
		}
		// tab walks forward through the complete steps to the last.
		for i := 0; i < d.last; i++ {
			m.Update(key("tab"))
			if m.mode != d.mode || d.step(m) != i+1 {
				t.Fatalf("%s: tab from step %d: mode %v step %d, status %q", d.name, i, m.mode, d.step(m), m.status)
			}
		}
		if !strings.Contains(stripANSI(m.View()), "⇧tab back") || !strings.Contains(stripANSI(m.View()), "esc close") {
			t.Fatalf("%s: the last step's footer lacks ⇧tab back or esc close:\n%s", d.name, stripANSI(m.View()))
		}
		// esc closes whole from the last step.
		m.Update(key("esc"))
		if m.mode != modePlay || m.screen != screen {
			t.Fatalf("%s: esc from the last step: mode %v screen %v", d.name, m.mode, m.screen)
		}
		// A quantity that does not read is not a complete step: the field
		// takes digits only (#112), so 0 is the one a player can type.
		if d.name == "sell" || d.name == "buy" {
			d.open(m)
			m.Update(key("enter"))
			for _, k := range []string{"backspace", "0"} {
				m.Update(key(k))
			}
			m.Update(key("tab"))
			if m.mode != d.mode || d.step(m) != 1 {
				t.Fatalf("%s: tab on a quantity that does not read: mode %v step %d", d.name, m.mode, d.step(m))
			}
		}
	}
}

// tab and shift+tab are one binding in the key table, and Bubble Tea's
// name for ESC [ Z, what Terminal.app, iTerm2 and tmux send for a
// shifted tab, is the one the binding's keys list.
func TestTabBindingTakesShiftTab(t *testing.T) {
	var tab *binding
	for i := range bindings {
		if bindings[i].key == "tab" {
			tab = &bindings[i]
		}
	}
	if tab == nil {
		t.Fatal("no tab binding")
	}
	for _, k := range []string{"tab", "shift+tab"} {
		if !tab.accepts(k) {
			t.Errorf("the tab binding does not accept %q", k)
		}
	}
	if got := (tea.KeyMsg{Type: tea.KeyShiftTab}).String(); got != "shift+tab" {
		t.Errorf("Bubble Tea names a shifted tab %q; add it to the tab binding's keys", got)
	}
	m := newTestModel(t, 80, 24)
	m.Update(key("tab"))
	m.Update(key("tab"))
	m.Update(key("shift+tab"))
	if m.screen != screenMarket {
		t.Errorf("tab, tab, shift+tab: screen %v, want the market", m.screen)
	}
}

// A modal with no pages leaves tab and shift+tab alone: a confirmation,
// a picker, the card's outcome, the report, help and the overlay stay
// open on either, and esc closes each as it did (the card's question
// has no close: it is answered).
func TestTabIsSilentWhereThereIsNoPage(t *testing.T) {
	cases := []struct {
		name string
		mode mode
		open func(m *Model)
	}{
		{"confirm end", modeConfirmEnd, func(m *Model) { m.Update(key("enter")) }},
		{"confirm new", modeConfirmNew, func(m *Model) { m.Update(key("N")) }},
		{"confirm fire", modeConfirmFire, func(m *Model) { m.Update(key("4")); m.Update(key("f")) }},
		{"confirm travel", modeConfirmTravel, func(m *Model) { m.Update(key("g")) }},
		{"post", modePost, func(m *Model) { m.Update(key("5")); m.mapCursor = 1; m.Update(key("c")) }},
		{"front", modeFront, func(m *Model) { m.Update(key("7")); m.Update(key("b")) }},
		{"fund", modeFund, func(m *Model) { m.Update(key("7")); m.Update(key("f")) }},
		{"report", modeReport, func(m *Model) { m.mode = modeReport }},
		{"help", modeHelp, func(m *Model) { m.Update(key("?")) }},
		{"card outcome", modeCard, func(m *Model) { m.w.Dilemmas.Pending = testCard(m.w.Day); m.showCard(); m.Update(key("enter")) }},
		{"details", modeDetails, func(m *Model) { m.Update(key("5")); m.mode = modeDetails }},
	}
	for _, c := range cases {
		m := richModel(t, 80, 24)
		day := m.w.Day
		c.open(m)
		if m.mode != c.mode {
			t.Fatalf("%s: mode %v, status %q", c.name, m.mode, m.status)
		}
		for _, k := range []string{"tab", "shift+tab"} {
			m.Update(key(k))
			if m.mode != c.mode || m.w.Day != day {
				t.Fatalf("%s: %s: mode %v day %d", c.name, k, m.mode, m.w.Day)
			}
		}
		if !strings.Contains(stripANSI(m.View()), "esc close") || strings.Contains(stripANSI(m.View()), "esc back") {
			t.Fatalf("%s: the footer does not say esc close:\n%s", c.name, stripANSI(m.View()))
		}
		m.Update(key("esc"))
		if m.mode == c.mode {
			t.Fatalf("%s: esc did not close it", c.name)
		}
	}
	// The card's question has no close, and tab and shift+tab do not
	// answer it either.
	m := richModel(t, 80, 24)
	m.w.Dilemmas.Pending = testCard(m.w.Day)
	m.showCard()
	for _, k := range []string{"tab", "shift+tab", "esc"} {
		m.Update(key(k))
		if m.mode != modeCard || m.cardDone {
			t.Fatalf("card: %s: mode %v answered %v", k, m.mode, m.cardDone)
		}
	}
	// Nothing in the footers says back but ⇧tab, and every mode that
	// closes on esc says close.
	for _, b := range modeBindings {
		if b.label == "back" && b.key != "⇧tab" {
			t.Errorf("%s %s: back is ⇧tab alone", b.key, b.label)
		}
		if strings.Contains(b.key, "esc") && b.label != "close" {
			t.Errorf("%s %s: esc closes", b.key, b.label)
		}
	}
}
