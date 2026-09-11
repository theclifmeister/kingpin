// Package format is how the game writes a number: money, prices, day
// counts and the words around them. The UI and the morning report draw
// on the same three money formats, so a sum reads the same on the
// dashboard as it does in the report.
package format

import (
	"fmt"
	"math"
	"strings"
)

// Arrow is what a change is written with: `14 → 17`, never `->`.
const Arrow = "→"

// Money formats an integer dollar amount with thousands separators:
// itemised figures (a fee, a wage, a purchase) stay exact.
func Money(n int) string {
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

// Cash formats a dollar amount the way a headline would: exact with
// separators under $10K, then $45K, $1.2M, $34B. Totals on the dashboard,
// title bar, report and run summary use it; itemised figures stay exact.
func Cash(n int) string {
	if n > -10_000 && n < 10_000 {
		return Money(n)
	}
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	v := float64(n)
	units := []string{"K", "M", "B", "T"}
	i := 0
	v /= 1000
	// Round before choosing the unit so $999,600 reads $1.0M, not $1000K.
	for i < len(units)-1 && math.Round(v) >= 1000 {
		v /= 1000
		i++
	}
	if v < 10 {
		return fmt.Sprintf("%s$%.1f%s", sign, v, units[i])
	}
	return fmt.Sprintf("%s$%.0f%s", sign, v, units[i])
}

// Price formats a per-unit price: cents matter on a $20 bag, not on a
// $2,500 one.
func Price(v float64) string {
	if v < 1000 {
		return fmt.Sprintf("$%.2f", v)
	}
	return Money(int(math.Round(v)))
}

// Plural is n of a thing: `1 corner`, `3 corners`, `0 corners`.
func Plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
