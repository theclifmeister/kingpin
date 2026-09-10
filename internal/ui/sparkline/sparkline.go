// Package sparkline renders a series of values as Unicode block characters.
package sparkline

import "strings"

var blocks = []rune("▁▂▃▄▅▆▇█")

// Render draws the last width values of vs, scaled between their min and max.
// A flat series renders as a mid-height line.
func Render(vs []float64, width int) string {
	if width <= 0 || len(vs) == 0 {
		return ""
	}
	if len(vs) > width {
		vs = vs[len(vs)-width:]
	}
	lo, hi := vs[0], vs[0]
	for _, v := range vs {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	var b strings.Builder
	for _, v := range vs {
		idx := len(blocks) / 2
		if hi > lo {
			idx = int((v - lo) / (hi - lo) * float64(len(blocks)-1))
		}
		b.WriteRune(blocks[idx])
	}
	return b.String()
}

// Bar draws a horizontal gauge of width cells filled to frac (0..1), with
// tick marks at the given fractions.
func Bar(frac float64, width int, marks []float64) string {
	if width <= 0 {
		return ""
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac*float64(width) + 0.5)
	cells := make([]rune, width)
	for i := range cells {
		if i < filled {
			cells[i] = '█'
		} else {
			cells[i] = '░'
		}
	}
	for _, m := range marks {
		i := int(m * float64(width))
		if i >= 0 && i < width && i >= filled {
			cells[i] = '┆'
		}
	}
	return string(cells)
}
