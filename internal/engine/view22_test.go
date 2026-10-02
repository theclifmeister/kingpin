package engine_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/game"
)

// TestViewCarriesTheReportPoints (#560): view 22 carries where a report
// line points, each section's points by line and a lead line's at, as
// the report has them.
func TestViewCarriesTheReportPoints(t *testing.T) {
	t.Parallel()
	s, w := freshSession(t)
	act := game.Act{Screen: game.ScreenLedger}
	w.Report = &game.DayReport{
		Day:      3,
		Unlocked: []string{"Heroin is on offer.", "The Laundromat is open to you: it washes over the till."},
		Points:   []game.Point{{Kind: "front", Text: "The Laundromat is open to you: it washes over the till.", At: 29, Act: act}},
		Lead:     []game.Line{{Kind: "flow", Text: "The next step is the road to Bayport.", Act: game.Act{Screen: game.ScreenMap}, At: 36}},
	}
	v := s.View()
	if l := v.Report.Lead[0]; l.At != 36 {
		t.Errorf("the lead line: %+v", l)
	}
	for _, sec := range v.Report.Sections {
		if sec.Points == nil {
			t.Errorf("%s: points is nil", sec.ID)
		}
		if sec.ID != "unlocked" {
			if len(sec.Points) != 0 {
				t.Errorf("%s: %+v", sec.ID, sec.Points)
			}
			continue
		}
		if len(sec.Points) != 1 || sec.Points[0].Line != 1 || sec.Points[0].At != 29 || sec.Points[0].Kind != "front" || sec.Points[0].Act != act {
			t.Errorf("unlocked: %+v", sec.Points)
		}
	}
}
