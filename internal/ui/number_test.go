package ui

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// Every number field takes the same shortcuts (#112): from an empty
// field m is max and h half of it (rounded down), ↑ past max stays at
// max, pgdn past zero stays at zero, a is m, the field shows `/ N max`
// after the number and its footer lists the shortcuts; and a typed
// number over max is refused where it always was, by the game, the
// route target excepted, which is set as it always was (a target is
// stock to keep, and the wholesaler sells into the stash whatever its
// room). The five fields: the buy and sell quantities, the cart's (a buy
// and an order), the route target's units and the fund's amount.
func TestNumberField(t *testing.T) {
	type field struct {
		name    string
		open    func(m *Model) // opens the dialog on the number field
		field   func(m *Model) *numberField
		max     func(m *Model) int
		err     func(m *Model) string // the dialog's error after enter
		money   bool                  // the max reads as money
		refused bool                  // a typed number over max is refused
	}
	fields := []field{
		{"sell", func(m *Model) {
			m.w.Stash(m.w.Player.Location)[m.w.Products[0]] = 37
			m.Update(key("s"))
			m.Update(key("enter"))
		}, func(m *Model) *numberField { return &m.dlg.qty }, func(m *Model) int { return m.w.Stock(m.w.Player.Location, m.w.Products[0]) },
			func(m *Model) string { return m.dlg.err }, false, true},
		{"buy", func(m *Model) {
			m.Update(key("b"))
			m.Update(key("enter"))
		}, func(m *Model) *numberField { return &m.dlg.qty }, func(m *Model) int { return m.maxBuy(m.w.Products[0]) },
			func(m *Model) string { return m.dlg.err }, false, true},
		{"cart buy", func(m *Model) {
			fillCart(t, m)
			m.Update(key("c"))
			m.Update(key("down")) // past the contract's line (#113), which only goes back
			m.Update(key("enter"))
		}, func(m *Model) *numberField { return &m.crt.qty }, func(m *Model) int {
			l := m.cartSelected()
			return l.qty + m.maxBuy(l.product)
		}, func(m *Model) string { return m.crt.err }, false, true},
		{"cart order", func(m *Model) {
			fillCart(t, m)
			m.Update(key("c"))
			m.Update(key("down"))
			m.Update(key("down"))
			m.Update(key("enter"))
		}, func(m *Model) *numberField { return &m.crt.qty }, func(m *Model) int {
			l := m.cartSelected()
			return m.w.Stock(l.city, l.product)
		}, func(m *Model) string { return m.crt.err }, false, true},
		{"target", func(m *Model) {
			m.Update(key("5"))
			m.Update(key("]"))
			m.onRoutes = true
			m.Update(key("R"))
			m.Update(key("enter"))
			m.Update(key("enter")) // units
		}, func(m *Model) *numberField { return &m.tgt.units }, func(m *Model) int { return m.w.Capacity(m.targetRoute().To) },
			func(m *Model) string { return m.tgt.err }, false, false},
		{"fund", func(m *Model) {
			m.w.Player.CleanCash = 45_001 // under what fills goodwill: the cash is the line
			m.Update(key("7"))
			m.Update(key("f"))
		}, func(m *Model) *numberField { return &m.fnd.amt }, func(m *Model) int { return m.maxFund(m.fundCity()) },
			func(m *Model) string { return m.fnd.err }, true, true},
		{"fast", func(m *Model) { m.Update(key("F")) }, func(m *Model) *numberField { return &m.fst.days }, func(m *Model) int { return fastDaysMax },
			func(m *Model) string { return m.fst.err }, false, true},
	}
	shortcuts := "m max  h half  ↑↓ ±1  pgup pgdn ±10"
	for _, f := range fields {
		m := richModel(t, 80, 24)
		f.open(m)
		fld := f.field(m)
		mode := m.mode
		mx := f.max(m)
		if mx < 3 {
			t.Fatalf("%s: a max of %d is too small to test", f.name, mx)
		}
		if f.name == "sell" && mx%2 == 0 {
			t.Fatalf("sell: max %d is even; half's rounding needs an odd one", mx)
		}
		suffix := fmt.Sprintf("/ %d max", mx)
		if f.money {
			suffix = "/ " + money(mx) + " max"
		}
		if v := fld.Value(); v != "" && (f.name == "sell" || f.name == "buy" || f.name == "fund" || f.name == "fast") {
			t.Fatalf("%s: the field opens with %q", f.name, v)
		}
		// The cart opens on the line's quantity and the target on the
		// target set: clear it first.
		fld.SetValue("")
		want := func(step, v string) {
			t.Helper()
			if m.mode != mode || fld.Value() != v {
				t.Fatalf("%s: after %s the field is %q (mode %v), want %q", f.name, step, fld.Value(), m.mode, v)
			}
		}
		m.Update(key("m"))
		want("m", strconv.Itoa(mx))
		view := stripANSI(m.View())
		if !strings.Contains(view, suffix) {
			t.Errorf("%s: the field does not show %q:\n%s", f.name, suffix, view)
		}
		if !strings.Contains(view, shortcuts) {
			t.Errorf("%s: the footer does not list %q:\n%s", f.name, shortcuts, view)
		}
		m.Update(key("up"))
		want("m, ↑", strconv.Itoa(mx))
		m.Update(key("pgup"))
		want("m, ↑, pgup", strconv.Itoa(mx))
		m.Update(key("down"))
		want("m, ↓", strconv.Itoa(mx-1))
		fld.SetValue("")
		m.Update(key("h"))
		want("h", strconv.Itoa(mx/2))
		fld.SetValue("")
		m.Update(key("a"))
		want("a", strconv.Itoa(mx))
		fld.SetValue("")
		m.Update(key("pgdown"))
		want("pgdn", "0")
		m.Update(key("pgdown"))
		want("pgdn, pgdn", "0")
		m.Update(key("down"))
		want("pgdn, pgdn, ↓", "0")
		m.Update(key("pgup"))
		want("0, pgup", strconv.Itoa(min(10, mx)))
		// Typing is digits and backspace, nothing else.
		fld.SetValue("")
		for _, k := range []string{"1", "x", "2", "backspace", "3"} {
			m.Update(key(k))
		}
		want("typing 1 x 2 ⌫ 3", "13")
		// A typed number over max: refused by the game as it always was,
		// or, for a target, set as it always was.
		fld.SetValue(strconv.Itoa(mx + 1))
		m.Update(key("enter"))
		if f.name == "sell" || f.name == "buy" {
			m.Update(key("enter")) // the dial step, or the buy's once, commits
		}
		if f.name == "sell" {
			m.Update(key("enter")) // the sale's once (#114) commits
		}
		switch {
		case f.refused && (m.mode != mode || f.err(m) == ""):
			t.Errorf("%s: %d over a max of %d was not refused: mode %v err %q status %q", f.name, mx+1, mx, m.mode, f.err(m), m.status)
		case !f.refused && (m.mode != modePlay || f.err(m) != ""):
			t.Errorf("%s: %d over a max of %d was refused: mode %v err %q", f.name, mx+1, mx, m.mode, f.err(m))
		}
		if f.name == "target" {
			r := m.targetRoute()
			if got := m.w.Route(r.ID).Target[m.w.Products[m.cursor]]; got != mx+1 {
				t.Errorf("target: set to %d, want %d", got, mx+1)
			}
		}
	}
	// A field that can take nothing shows no max, blank or typed in.
	f := newNumberField("blank = max")
	for _, v := range []string{"", "5"} {
		f.SetValue(v)
		if view := stripANSI(f.View()); strings.Contains(view, "/ ") {
			t.Errorf("a field with a max of 0 at %q shows one: %q", v, view)
		}
	}
	// The product step lists no shortcut.
	m := richModel(t, 80, 24)
	m.Update(key("s"))
	if view := stripANSI(m.View()); strings.Contains(view, "m max") {
		t.Errorf("the product step lists the field's keys:\n%s", view)
	}
}
