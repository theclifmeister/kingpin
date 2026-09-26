package news

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
)

// The report's numbers show what the stop and the panel say (#522): a
// pressure band crossed shows the move even where rounding would hide it
// ("Eastside pressure 25 → 25" beside "pressure up in Eastside"), and
// the war reads on the panel's scale, against the crackdown line
// ("loud (58/100)" beside the panel's 52/80).
func TestReportNumbersShowTheChange(t *testing.T) {
	if got := pressureLine("Eastside", 24.8, 25.2); !strings.Contains(got, "24.8 → 25.2") {
		t.Errorf("a band crossed by a hair reads %q", got)
	}
	if got := pressureLine("Eastside", 20, 31); !strings.Contains(got, "20 → 31") {
		t.Errorf("a band crossed reads %q", got)
	}
	got := warLine(events.WarEscalated{Stage: events.StageOpen, War: 52, Crackdown: 80})
	if !strings.Contains(got, "(52/80)") || strings.Contains(got, "/100") {
		t.Errorf("the war reads %q, want it on the panel's 80", got)
	}
}
