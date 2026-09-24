package engine_test

import (
	"slices"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/harness"
)

// TestEveryLeadHasAnAct (#354): every lead line of a played run carries
// an act on one of the screens an alert lands on, and the subject it
// names is there: the corner on the map, the member on the roster (a
// post picker opens on a corner). The view carries the lead as the
// report has it, and the report's sections in ReportSections' one
// order, every one of them.
func TestEveryLeadHasAnAct(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	s, err := engine.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := s.NewRun(7, game.Start{})
	policy := harness.Boss(cfg, 40, "")
	screens := []string{engine.ScreenDashboard, engine.ScreenMarket, engine.ScreenCrew, engine.ScreenMap, engine.ScreenLedger, engine.ScreenRivals}
	kinds := map[string]bool{}
	for d := 0; d < 120 && w.Over == nil; d++ {
		policy(w)
		s.EndDay()
		v := s.View()
		if len(v.Report.Lead) != len(w.Report.Lead) {
			t.Fatalf("day %d: the view's lead %+v, the report's %+v", w.Day, v.Report.Lead, w.Report.Lead)
		}
		for i, l := range v.Report.Lead {
			kinds[l.Kind] = true
			if l.Text != w.Report.Lead[i].Text || !slices.Contains(content.DigestKinds, l.Kind) {
				t.Errorf("day %d line %d: %+v", w.Day, i+1, l)
			}
			if !slices.Contains(screens, l.Act.Screen) {
				t.Errorf("day %d %s: the screen %q is none an alert lands on", w.Day, l.Kind, l.Act.Screen)
			}
			switch l.Act.Subject {
			case engine.SubjectCorner:
				if w.Corner(l.Corner) == nil {
					t.Errorf("day %d %s: corner %q is not on the map", w.Day, l.Kind, l.Corner)
				}
			case engine.SubjectMember:
				if w.Crew.Member(l.Member) == nil {
					t.Errorf("day %d %s: member %d is not on the roster", w.Day, l.Kind, l.Member)
				}
			case engine.SubjectHouse:
				if w.House(l.House) == nil {
					t.Errorf("day %d %s: house %q is not on the books", w.Day, l.Kind, l.House)
				}
			case "":
			default:
				t.Errorf("day %d %s: subject %q", w.Day, l.Kind, l.Act.Subject)
			}
			if l.Act.Mode == engine.ModePost && l.Act.Subject != engine.SubjectCorner {
				t.Errorf("day %d %s: a post picker on no corner", w.Day, l.Kind)
			}
		}
		var ids []string
		for _, sec := range v.Report.Sections {
			ids = append(ids, sec.ID)
		}
		var want []string
		for _, sec := range engine.ReportSections(w.Report) {
			want = append(want, sec.ID)
		}
		if !slices.Equal(ids, want) || len(ids) != 14 {
			t.Fatalf("day %d: the view's sections %v, want %v", w.Day, ids, want)
		}
	}
	if len(kinds) < 4 {
		t.Errorf("120 days of the boss led with %d kinds: %v", len(kinds), kinds)
	}
}
