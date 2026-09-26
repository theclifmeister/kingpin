package news

import (
	"strings"
	"testing"
	"text/template"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The report says what happened (#502): each line is checked against the
// event it words.
func TestReportLinesAgreeWithWhatHappened(t *testing.T) {
	w := &game.World{}

	// A robbery asks for an enforcer only when none was posted; with one
	// posted it names them and their odds.
	bare := robberyLine(w, events.CornerRobbed{Name: "The Docks", Cash: 400, Odds: 0.04})
	if !strings.Contains(bare, "An enforcer on the corner would have helped.") {
		t.Errorf("an unguarded robbery: %q", bare)
	}
	guarded := robberyLine(w, events.CornerRobbed{Name: "The Docks", Cash: 400, Enforcer: "Ike", Odds: 0.012, Bare: 0.04})
	if strings.Contains(guarded, "would have helped") || !strings.Contains(guarded, "Ike was on the corner") || !strings.Contains(guarded, "4.0% to 1.2%") {
		t.Errorf("a robbery with Ike posted: %q", guarded)
	}

	// A lieutenant's cut is apart from the skim: the skim line says it
	// is on top of the cut, and never counts it.
	skim := skimLine(events.CrewSkimmed{Amount: 850, Cuts: 850})
	if !strings.Contains(skim, "$850 of the takings never made it back, on top of the $850 the lieutenants kept as their cut") {
		t.Errorf("a skim beside a cut: %q", skim)
	}
	if skim := skimLine(events.CrewSkimmed{Amount: 300}); strings.Contains(skim, "cut") {
		t.Errorf("a skim with no lieutenant: %q", skim)
	}

	// A night nothing was washed does not say it washed $0.
	if got := washLine(events.CashLaundered{Upkeep: 150, Earned: 400}); strings.Contains(got, "$0") || strings.Contains(got, "0 fronts") || !strings.HasPrefix(got, "Nothing washed: upkeep -$150") {
		t.Errorf("a night with nothing washed: %q", got)
	}
	if got := washLine(events.CashLaundered{Amount: 2_000, Fronts: 1, Upkeep: 150}); got != "Washed $2,000 clean through 1 front, upkeep -$150" {
		t.Errorf("a night's wash: %q", got)
	}

	// The wash line under a contract's short names the till it bought
	// on when the day spent it down after the wash.
	w.Flows = []game.CashFlow{{Day: 1, Lines: []game.FlowLine{{Cat: game.FlowLaundering, Pools: game.Pools{Dirty: -2_000, Clean: 2_000}}}, Closing: game.Pools{Dirty: 168_479}}}
	if got := tookLine(w, events.SupplyShort{Why: "cash", Till: 168_479}); got != "  the wash took $2,000 last night and left the till $168,479" {
		t.Errorf("the wash took it: %q", got)
	}
	if got := tookLine(w, events.SupplyShort{Why: "cash", Till: 18_000}); got != "  the till was down to $18,000 when it bought" {
		t.Errorf("the day spent it: %q", got)
	}
	if got := tookLine(w, events.SupplyShort{Why: "room", Till: 18_000}); got != "" {
		t.Errorf("a short of room: %q", got)
	}

	// A name with its own article gets no second: `the {{.Route}}` is
	// `{{the .Route}}`, and The Channel reads once.
	if got := article("Coast guard boards the {{.Route}}. The {{.Corner}} is quiet"); got != "Coast guard boards {{the .Route}}. {{The .Corner}} is quiet" {
		t.Errorf("the rewrite: %q", got)
	}
	tm := template.Must(template.New("x").Funcs(articles).Parse(article("Coast guard boards the {{.Route}}")))
	var b strings.Builder
	if err := tm.Execute(&b, data{Route: "The Channel"}); err != nil || b.String() != "Coast guard boards The Channel" {
		t.Errorf("the Channel: %q %v", b.String(), err)
	}
}
