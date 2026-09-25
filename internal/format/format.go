// Package format is how the game writes a number: money, prices, day
// counts and the words around them. The UI and the morning report draw
// on the same three money formats, so a sum reads the same on the
// dashboard as it does in the report.
package format

import (
	"fmt"
	"math"
	"strings"
	"time"
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

// Signed is Money for a change, its sign written either way: `+$1,200`,
// `-$300`, `$0` (#351: the cash flow's lines).
func Signed(n int) string {
	if n > 0 {
		return "+" + Money(n)
	}
	return Money(n)
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

// CashWeight is what a pile of dollars weighs in hundred-dollar bills
// (#392), a bill being a gram: `under a kilo`, `450 kg`, `1.5 tonnes`,
// `12 tonnes`. The ledger and the report say it once the pile is big
// enough to be a problem of storage.
func CashWeight(n int) string {
	kg := float64(n) / 100 / 1000
	switch {
	case kg < 1:
		return "under a kilo"
	case math.Round(kg) < 1000:
		return fmt.Sprintf("%.0f kg", kg)
	case kg < 9950:
		return fmt.Sprintf("%.1f tonnes", kg/1000)
	}
	return fmt.Sprintf("%.0f tonnes", kg/1000)
}

// Price formats a per-unit price: cents matter on a $20 bag, not on a
// $2,500 one.
func Price(v float64) string {
	if v < 1000 {
		return fmt.Sprintf("$%.2f", v)
	}
	return Money(int(math.Round(v)))
}

// Pct is a fraction written as a percent to prec decimals (#275):
// Pct(0.25, 0) is `25%`, Pct(0.045, 1) `4.5%`. It is `%.0f%%` of the
// fraction times a hundred, so a site that wrote that reads the same.
func Pct(frac float64, prec int) string { return fmt.Sprintf("%.*f%%", prec, frac*100) }

// PctBand is a band of fractions as percents to prec decimals (#464):
// PctBand(0.3, 0.45, 0) is `30–45%`, and one percent where the two ends
// write the same, PctBand(0.3, 0.301, 0) `30%`. The odds the file lets
// you read go through it, the strike picker's and the map's alike.
func PctBand(lo, hi float64, prec int) string {
	a, b := Pct(lo, prec), Pct(hi, prec)
	if a == b {
		return a
	}
	return strings.TrimSuffix(a, "%") + "–" + b
}

// Times is a multiplier to prec decimals (#275): Times(1.25, 2) is
// `×1.25`, Times(1.5, 1) `×1.5`.
func Times(x float64, prec int) string { return fmt.Sprintf("×%.*f", prec, x) }

// TimesSig is a multiplier to sig significant figures, trailing zeros
// dropped (#275): TimesSig(1.5, 3) is `×1.5`, TimesSig(2, 3) `×2`.
func TimesSig(x float64, sig int) string { return fmt.Sprintf("×%.*g", sig, x) }

// Ago is how long since something happened, the way the start menu dates
// a save: `just now` under a minute, then `5m ago`, `2h ago`, `3d ago`.
func Ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d/time.Hour))
	}
	return fmt.Sprintf("%dd ago", int(d/(24*time.Hour)))
}

// A is a noun with its indefinite article: `a runner`, `an enforcer`,
// `an Eastside outfit` (a name's capital counts, #148).
func A(noun string) string {
	if noun != "" && strings.ContainsRune("aeiouAEIOU", rune(noun[0])) {
		return "an " + noun
	}
	return "a " + noun
}

// Plural is n of a thing: `1 corner`, `3 corners`, `0 corners`. It
// knows the irregulars the game counts (`city` to `cities`, `person` to
// `people`, `box` to `boxes`); a noun of two words is pluralised on its
// last (`1 more day`, `2 more days`).
func Plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %s", n, Plurals(noun))
}

// Plurals is the plural of a noun on its own: `corners`, `cities`,
// `people`.
func Plurals(noun string) string {
	if i := strings.LastIndex(noun, " "); i >= 0 {
		return noun[:i+1] + Plurals(noun[i+1:])
	}
	if p, ok := irregular[noun]; ok {
		return p
	}
	switch {
	case strings.HasSuffix(noun, "y") && len(noun) > 1 && !strings.ContainsRune("aeiou", rune(noun[len(noun)-2])):
		return noun[:len(noun)-1] + "ies"
	case strings.HasSuffix(noun, "s"), strings.HasSuffix(noun, "x"), strings.HasSuffix(noun, "z"),
		strings.HasSuffix(noun, "ch"), strings.HasSuffix(noun, "sh"):
		return noun + "es"
	}
	return noun + "s"
}

var irregular = map[string]string{
	"person": "people",
	"police": "police",
	"crew":   "crew",
	"cash":   "cash",
	"stock":  "stock",
}
