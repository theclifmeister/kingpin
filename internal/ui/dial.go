// The dial convention (#88): every notch of a dial in a row, the chosen
// one in brackets and the accent, and what the sell dial's notches do.
// The sell dialog, the crew's pay, the ledger's wash, the law's fund and
// the routes' target draw their dials through it (#275: out of
// dialogs.go).

package ui

import (
	"strings"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// dialRow draws the sell dial as `quiet  normal  [aggressive]`.
func dialRow(d events.Dial) string {
	return dialCells([]string{"quiet", "normal", "aggressive"}, int(d))
}

// dialCells is the dial convention (#88): every notch in a row two
// spaces apart, the chosen one bracketed in the accent (theme.Dial), as
// `quiet  [normal]  aggressive`. The sell, pay and launder dials draw
// through it.
func dialCells(notches []string, on int) string {
	cells := make([]string, len(notches))
	for i, n := range notches {
		if i == on {
			n = "[" + n + "]"
		}
		cells[i] = theme.Dial(i == on).Render(n)
	}
	return strings.Join(cells, "  ")
}

// dialBlurb is what the dial does, short enough for the modal's width
// after the heat figure.
func dialBlurb(d events.Dial) string {
	switch d {
	case events.DialQuiet:
		return "half the volume, small discount, barely a ripple"
	case events.DialAggressive:
		return "push past demand, premium first, then the crash"
	default:
		return "sell to demand at market price"
	}
}
