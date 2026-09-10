package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// money formats an integer dollar amount with thousands separators.
func money(n int) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-$" + b.String()
	}
	return "$" + b.String()
}

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

// clampLines keeps at most n lines of s.
func clampLines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	ls := strings.Split(s, "\n")
	if len(ls) > n {
		ls = ls[:n]
	}
	return strings.Join(ls, "\n")
}

func pct(from, to float64) float64 {
	if from == 0 {
		return 0
	}
	return (to - from) / from * 100
}
