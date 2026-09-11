package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// A number field (#112) is every quantity the game asks for: the buy
// and sell dialogs' quantity, the cart's, the route target's units and
// the fund dialog's amount. One helper takes the keys and draws the
// field, so every field has the same four shortcuts beside the digits
// and backspace: m is the most the field can take (a an alias, where
// "all" is the word), h half of it, ↑↓ ±1 and pgup pgdn ±10, every
// shortcut clamped to [0, max]. Typing is not clamped: a typed number
// over max is refused where it always was, by the game (`Only 3 Weed in
// Eastside.`), and a blank still means what it did (the most for a buy,
// a sale or a fund; none for a target). The caller sets max before it
// hands the field a key or draws it, because max moves with the world
// (the stash, the cash, the capacity).
type numberField struct {
	in    textinput.Model
	max   int  // what the field can take; the shortcuts clamp to it
	money bool // the field is dollars: max reads `$45,000`
}

// newNumberField is a field with its placeholder, what a blank means;
// the field is as wide as the placeholder, so the max sits close after
// the number.
func newNumberField(placeholder string) numberField {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 9
	ti.Width = len(placeholder)
	ti.Prompt = "> "
	return numberField{in: ti}
}

// Value is the typed text, blank included.
func (f numberField) Value() string { return f.in.Value() }

// SetValue sets the typed text, the cursor after it.
func (f *numberField) SetValue(s string) {
	f.in.SetValue(s)
	f.in.CursorEnd()
}

// Set writes a number into the field.
func (f *numberField) Set(n int) { f.SetValue(strconv.Itoa(n)) }

// Number is the typed text as a number: 0 for a blank, false for text
// that does not read as a whole number.
func (f numberField) Number() (int, bool) {
	s := strings.TrimSpace(f.in.Value())
	if s == "" {
		return 0, true
	}
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= 0
}

// Focus and Blur are the text input's: the cursor shows while the field
// is the step the dialog is on.
func (f *numberField) Focus() tea.Cmd { return f.in.Focus() }
func (f *numberField) Blur()          { f.in.Blur() }

// Update takes a key: the shortcuts move the number within [0, max],
// digits and backspace edit the text (left and right move within it),
// and anything else is left alone, so a letter never lands in a
// quantity.
func (f *numberField) Update(k tea.KeyMsg) tea.Cmd {
	key := k.String()
	n, ok := f.Number()
	if !ok {
		n = 0
	}
	switch key {
	case "m", "a":
		f.Set(f.max)
	case "h":
		f.Set(f.max / 2)
	case "up":
		f.Set(f.clamp(n + 1))
	case "down":
		f.Set(f.clamp(n - 1))
	case "pgup":
		f.Set(f.clamp(n + 10))
	case "pgdown":
		f.Set(f.clamp(n - 10))
	case "backspace", "delete", "left", "right":
		var cmd tea.Cmd
		f.in, cmd = f.in.Update(k)
		return cmd
	default:
		if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
			var cmd tea.Cmd
			f.in, cmd = f.in.Update(k)
			return cmd
		}
	}
	return nil
}

// clamp holds a shortcut's result to [0, max].
func (f numberField) clamp(n int) int { return max(0, min(n, f.max)) }

// View is the field followed by ` / 340 max` in Subtle; the suffix is
// left off when the field can take nothing.
func (f numberField) View() string {
	v := f.in.View()
	if f.max <= 0 {
		return v
	}
	mx := strconv.Itoa(f.max)
	if f.money {
		mx = money(f.max)
	}
	return v + "  " + theme.Subtle.Render("/ "+mx+" max")
}
