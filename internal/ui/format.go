package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/format"
)

// money, cash and price are the three money formats, one per column
// kind, as format fixes them: money keeps itemised amounts exact, cash
// compacts totals ($45K, $1.2M) and price is per unit.
func money(n int) string { return format.Money(n) }

func cash(n int) string { return format.Cash(n) }

func price(v float64) string { return format.Price(v) }

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
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	rs := []rune(s)
	if width == 1 {
		return string(rs[:1])
	}
	out := rs
	for lipgloss.Width(string(out))+1 > width && len(out) > 0 {
		out = out[:len(out)-1]
	}
	return string(out) + "…"
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

func pct(from, to float64) float64 {
	if from == 0 {
		return 0
	}
	return (to - from) / from * 100
}
