package ui

import (
	"errors"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// A number field (#112) is every quantity the game asks for: the buy
// and sell dialogs' quantity, the cart's, the route target's units and
// the fund dialog's amount. One helper takes the keys and draws the
// field, so every field has the same four shortcuts beside the digits
// and backspace: m is the most the field can take, h half of it, ↑↓ ±1
// and pgup pgdn ±10 (#241: no alias; a is accept and abandon), every
// shortcut clamped to [0, max]. Typing is not clamped: a typed number
// over max is refused where it always was, by the game (`Only 3 Weed in
// Eastside.`), and a blank still means what it did (the most for a buy,
// a sale or a fund; none for a target). The caller sets max before it
// hands the field a key or draws it, because max moves with the world
// (the stash, the cash, the capacity).
type numberField struct {
	in    textinput.Model
	max   int  // what the field can take; the shortcuts clamp to it
	min   int  // the least a number typed may be (#526: the till's float); 0 for most fields, where blank means what it means
	money bool // the field is dollars: max reads `$45,000`
	fresh bool // the value was set, not typed: the first digit replaces it (#426)
}

// newNumberField is a field with its placeholder, what a blank means;
// the field is as wide as the placeholder, so the max sits close after
// the number. The text input's Width counts its prompt and the cursor
// cell (bubbles v1: the placeholder is cut to `Width - prompt - 1` with
// an ellipsis), so the width is the prompt's plus the placeholder's and
// the placeholder renders whole (#234).
func newNumberField(placeholder string) numberField {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 9
	ti.Prompt = "> "
	ti.Width = lipgloss.Width(ti.Prompt) + lipgloss.Width(placeholder)
	return numberField{in: ti}
}

// Value is the typed text, blank included.
func (f numberField) Value() string { return f.in.Value() }

// SetValue sets the typed text, the cursor after it.
func (f *numberField) SetValue(s string) {
	f.in.SetValue(s)
	f.in.CursorEnd()
	f.fresh = s != ""
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

// errNotAWholeNumber is what a field says of text that does not read as
// a quantity: the one refusal every dialog shows for it (#275).
var errNotAWholeNumber = errors.New("enter a whole number above zero")

// Read is the quantity the field reads (#275): blank for a blank, what
// the dialog takes it to mean (one level, the price, the most allowed),
// and errNotAWholeNumber for text that does not read or reads as zero.
func (f numberField) Read(blank int) (int, error) {
	if strings.TrimSpace(f.in.Value()) == "" {
		return blank, nil
	}
	n, ok := f.Number()
	if !ok || n <= 0 {
		return 0, errNotAWholeNumber
	}
	return n, nil
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
	case "m":
		f.Set(f.top())
	case "h":
		f.Set(f.clamp(f.top() / 2))
	case "up":
		f.Set(f.clamp(n + 1))
	case "down":
		f.Set(f.clamp(n - 1))
	case "pgup":
		f.Set(f.clamp(n + 10))
	case "pgdown":
		f.Set(f.clamp(n - 10))
	case "backspace", "delete", "left", "right":
		f.fresh = false
		var cmd tea.Cmd
		f.in, cmd = f.in.Update(k)
		return cmd
	default:
		// Digits typed or pasted (#426: a paste is one message with
		// every rune, and was dropped whole, leaving blank = max). A
		// value the dialog or a shortcut set is replaced, not appended
		// to: 472 then 250 is 250, not 472250.
		if digits := string(k.Runes); k.Type == tea.KeyRunes && digits != "" && strings.Trim(digits, "0123456789") == "" {
			if f.fresh {
				f.in.SetValue("")
				f.fresh = false
			}
			var cmd tea.Cmd
			f.in, cmd = f.in.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: k.Runes})
			return cmd
		}
	}
	return nil
}

// top is the field's max, never under its min (#526: the till's max
// read the dirty cash in hand, under the float it stated).
func (f numberField) top() int { return max(f.max, f.min) }

// clamp holds a shortcut's result to [min, max].
func (f numberField) clamp(n int) int { return max(f.min, min(n, f.top())) }

// Outside is a typed number outside the field's range (#526): -1 under
// its min, 1 over its max, 0 inside it or blank. The dialog refuses or
// clamps it, as a buy's quantity over the stash is set to what fits.
func (f numberField) Outside() int {
	n, ok := f.Number()
	switch {
	case !ok || strings.TrimSpace(f.in.Value()) == "":
		return 0
	case n < f.min:
		return -1
	case n > f.top():
		return 1
	}
	return 0
}

// View is the field followed by ` / 340 max` in Subtle; the suffix is
// left off when the field can take nothing. A field with a min reads
// its range, `$50,000 … $50,000,000` (#526), never a max under the min.
func (f numberField) View() string {
	v := f.in.View()
	if f.top() <= 0 {
		return v
	}
	num := func(n int) string {
		if f.money {
			return money(n)
		}
		return strconv.Itoa(n)
	}
	if f.min > 0 {
		return v + "  " + theme.Subtle.Render(num(f.min)+" … "+num(f.top()))
	}
	return v + "  " + theme.Subtle.Render("/ "+num(f.max)+" max")
}
