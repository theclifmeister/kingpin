package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/ui/sparkline"
)

// money, cash and price are the three money formats, one per column
// kind, as format fixes them: money keeps itemised amounts exact, cash
// compacts totals ($45K, $1.2M) and price is per unit.
func money(n int) string { return format.Money(n) }

func cash(n int) string { return format.Cash(n) }

func price(v float64) string { return format.Price(v) }

// fare is a route's fare a unit: whole dollars as money prints them
// (`$8`), cents through price where the tree has cut one under a dollar
// (`$0.50`, #119).
func fare(v float64) string {
	if v == float64(int(v)) {
		return money(int(v))
	}
	return price(v)
}

// plural is n of a thing: `1 corner`, `3 corners`.
func plural(n int, noun string) string { return format.Plural(n, noun) }

// fit pads or truncates s to exactly width visible cells.
func fit(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w > width {
		return truncate(s, width)
	}
	return s + strings.Repeat(" ", width-w)
}

// truncate cuts s to width cells, adding an ellipsis when it had to cut.
// It cuts between escape sequences, never through one, and keeps the
// ones after the cut, so a styled string cut short still closes.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return ansi.Truncate(s, 1, "")
	}
	return ansi.Truncate(s, width, "…")
}

// lines joins non-empty strings with newlines.
func lines(ss ...string) string {
	out := ss[:0:0]
	for _, s := range ss {
		if s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, "\n")
}

// pctText is a percentage as the table's kPct column prints it (#244):
// one decimal under ten, none from ten (`4.5%`, `12%`), so a percent in
// the pane reads as the same percent in a table.
func pctText(f float64) string {
	if f < 10 && f > -10 {
		return fmt.Sprintf("%.1f%%", f)
	}
	return fmt.Sprintf("%.0f%%", f)
}

// barText is a bar with its number after it, styled as one (#244):
// `████░░░░ 45`. The suffix is the caller's (` 45`, ` 45/60`, ` 67%`),
// the marks the ticks on the bar (a loyalty's line), so every bar in
// MAIN and the pane is drawn the way the table's gauge cell is.
func barText(frac float64, width int, marks []float64, suffix string, style lipgloss.Style) string {
	return style.Render(sparkline.Bar(frac, width, marks) + suffix)
}

// clamp holds a cursor inside a list of n and returns it: the last row
// once the list shrank under it, 0 for an empty list (#244: every
// selection clamps both ends the same way).
func clamp(cur *int, n int) int {
	*cur = max(0, min(*cur, n-1))
	return *cur
}
