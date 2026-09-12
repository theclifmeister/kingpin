package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/sparkline"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// colKind is what every cell of a column is, and so how it is written:
// one number format per column, text on the left and numbers on the
// right. A nil cell is `-` in a number column and blank in a text one.
type colKind int

const (
	kText  colKind = iota // a name, a word, a status
	kInt                  // a count: 152
	kCash                 // a total: $45K, $1.2M (cash())
	kMoney                // an itemised amount: $25,000 (money())
	kPrice                // a per-unit price: $19.63, $2,308 (price())
	kPct                  // a share: 15%, or 0.8% under ten
	kDays                 // a duration, 3d, or a day of the run, d0
	kBar                  // a gauge with its number after it, or a sparkline
	kDial                 // an order at a dial, 40 aggr., or a dial's name
)

// col is one column of a table: the title over it, the kind of every
// cell under it and, for a bar, how many cells the bar is drawn in.
// Every other column sizes itself to the widest of its title and cells.
type col struct {
	title string
	kind  colKind
	width int
}

// The cell values a kind takes beyond the plain int, float64 and string.

// styled draws a cell in its own style; the selected row is drawn in
// Selected across the row and ignores it.
type styled struct {
	st lipgloss.Style
	v  any
}

// approx marks an estimate: ~40.
type approx struct{ v any }

// signed writes a change with its sign: +8%, -3.
type signed struct{ v any }

// gauge is a bar cell: the fraction filled, the tick marks, and the
// number written after the bar.
type gauge struct {
	frac  float64
	marks []float64
	n     float64
}

// spark is a sparkline cell over a series, with a mark after it when
// the series is under a shock.
type spark struct {
	vs   []float64
	mark string
}

// order is a dial cell: units queued at a dial, ↻ when it is a standing
// order of yours (#114), (lt) when it is the lieutenant's.
type order struct {
	qty      int
	dial     string
	lt       bool
	standing bool
}

// day is a day of the run in a days column: d0.
type day int

// mark is a row's own sign in the gutter when the cursor is not on it
// (the tree's ✓, ○ and ·); it goes first in the row and is not a cell.
type mark string

// tableHook, when set, sees every table rendered: the columns with the
// widths they were drawn at and the lines. Tests use it to check that
// every cell reads as its column's kind.
var tableHook func(cols []col, lines []string)

// dialShort is a sell dial's name in a cell.
func dialShort(d fmt.Stringer) string {
	s := d.String()
	if s == "aggressive" {
		return "aggr."
	}
	return s
}

// cellText writes one value by its column's kind, unwrapping the
// markers, and returns the style the value carried.
func cellText(k colKind, width int, v any) (string, *lipgloss.Style) {
	var st *lipgloss.Style
	prefix := ""
	sign := false
	for {
		switch x := v.(type) {
		case styled:
			st, v = &x.st, x.v
			continue
		case approx:
			prefix, v = "~", x.v
			continue
		case signed:
			sign, v = true, x.v
			continue
		}
		break
	}
	if v == nil {
		if k == kText {
			return "", st
		}
		return "-", st
	}
	var s string
	switch k {
	case kText:
		s = fmt.Sprint(v)
	case kInt, kCash, kMoney, kPrice, kPct, kDays:
		if !isNumber(v) {
			// A value the column cannot read is a bug: show it as one
			// rather than as a zero, and checkTable catches it.
			return "?" + fmt.Sprint(v), st
		}
	}
	switch k {
	case kInt:
		n := toInt(v)
		if sign {
			s = fmt.Sprintf("%+d", n)
		} else {
			s = fmt.Sprintf("%d", n)
		}
	case kCash:
		s = cash(toInt(v))
	case kMoney:
		s = money(toInt(v))
	case kPrice:
		s = price(toFloat(v))
	case kPct:
		f := toFloat(v)
		switch {
		case sign:
			s = fmt.Sprintf("%+.0f%%", f)
		case f < 10 && f > -10:
			s = fmt.Sprintf("%.1f%%", f)
		default:
			s = fmt.Sprintf("%.0f%%", f)
		}
	case kDays:
		if d, ok := v.(day); ok {
			s = fmt.Sprintf("d%d", int(d))
		} else {
			s = fmt.Sprintf("%dd", toInt(v))
		}
	case kBar:
		switch x := v.(type) {
		case gauge:
			s = sparkline.Bar(x.frac, width, x.marks) + fmt.Sprintf(" %.0f", x.n)
		case spark:
			s = sparkline.Render(x.vs, width)
			if x.mark != "" {
				s += " " + x.mark
			}
		default:
			s = fmt.Sprint(v)
		}
	case kDial:
		switch x := v.(type) {
		case order:
			s = fmt.Sprintf("%d %s", x.qty, x.dial)
			switch {
			case x.lt:
				s += " (lt)"
			case x.standing:
				s += " ↻"
			}
		case fmt.Stringer:
			s = x.String()
		default:
			s = fmt.Sprint(v)
		}
	}
	return prefix + s, st
}

// isNumber says whether a number column can read v.
func isNumber(v any) bool {
	switch v.(type) {
	case int, float64, day:
		return true
	}
	return false
}

func toInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case float64:
		return int(x + 0.5)
	case day:
		return int(x)
	}
	return 0
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case int:
		return float64(x)
	case float64:
		return x
	}
	return 0
}

// left says whether a kind is written from the left: text, bars and
// dials are; numbers line up on the right.
func (k colKind) left() bool {
	return k == kText || k == kBar || k == kDial
}

// table renders cols over rows as a header in Subtle and one line per
// row: a two-cell gutter (▸ on the cursor's row, which is drawn in
// Selected across the row), then the cells two spaces apart. Every
// column is as wide as the widest of its title and its cells; when the
// lines would be wider than width (0 for no limit) the last text column
// gives up the difference and its cells are cut with …. A cursor under
// zero selects nothing.
func table(cols []col, rows [][]any, cursor, width int) []string {
	type cell struct {
		s  string
		st *lipgloss.Style
	}
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = lipgloss.Width(c.title)
	}
	cells := make([][]cell, len(rows))
	marks := make([]string, len(rows))
	for r, row := range rows {
		if len(row) > 0 {
			if m, ok := row[0].(mark); ok {
				marks[r] = string(m)
				row = row[1:]
			}
		}
		cells[r] = make([]cell, len(cols))
		for i, c := range cols {
			var v any
			if i < len(row) {
				v = row[i]
			}
			s, st := cellText(c.kind, c.width, v)
			cells[r][i] = cell{s, st}
			widths[i] = max(widths[i], lipgloss.Width(s))
		}
	}
	// The last text column absorbs an overflow.
	total := 2 + 2*(len(cols)-1)
	for _, w := range widths {
		total += w
	}
	if width > 0 && total > width {
		for i := len(cols) - 1; i >= 0; i-- {
			if cols[i].kind == kText {
				widths[i] = max(3, widths[i]-(total-width))
				break
			}
		}
	}
	drawn := make([]col, len(cols))
	for i, c := range cols {
		drawn[i] = col{c.title, c.kind, widths[i]}
	}
	place := func(k colKind, s string, w int) string {
		if k.left() {
			return fit(s, w)
		}
		if lw := lipgloss.Width(s); lw < w {
			return strings.Repeat(" ", w-lw) + s
		}
		return truncate(s, w)
	}
	var out []string
	var h []string
	for i, c := range cols {
		h = append(h, place(c.kind, c.title, widths[i]))
	}
	out = append(out, theme.Subtle.Render("  "+strings.Join(h, "  ")))
	for r := range rows {
		var parts []string
		var plain []string
		for i, c := range cells[r] {
			s := place(cols[i].kind, c.s, widths[i])
			plain = append(plain, s)
			if c.st != nil {
				s = c.st.Render(s)
			}
			parts = append(parts, s)
		}
		switch {
		case r == cursor:
			out = append(out, theme.Gold.Render("▸ ")+theme.Selected.Render(strings.Join(plain, "  ")))
		case marks[r] != "":
			out = append(out, fit(marks[r], 2)+strings.Join(parts, "  "))
		default:
			out = append(out, "  "+strings.Join(parts, "  "))
		}
	}
	if width > 0 {
		for i, l := range out {
			out[i] = truncate(l, width)
		}
	}
	if tableHook != nil {
		tableHook(drawn, out)
	}
	return out
}

// tableWidth is how wide table would draw cols over rows with no limit,
// for a caller sizing a sparkline to what the width leaves.
func tableWidth(cols []col, rows [][]any) int {
	hook := tableHook
	tableHook = nil
	defer func() { tableHook = hook }()
	w := 0
	for _, l := range table(cols, rows, -1, 0) {
		w = max(w, lipgloss.Width(l))
	}
	return w
}
