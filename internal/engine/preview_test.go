package engine_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/harness"
)

// dice are the kinds that say a night rolled something the preview
// cannot know (engine.PreviewUnknown): a robbery, the police, an audit,
// a skim, the price war, the road seized, the crew's own nights, the
// rivals' moves that pay or take, the weather, and the wholesale lots
// bought at tomorrow's price.
var dice = map[string]bool{
	"CornerRobbed": true, "HouseRobbed": true, "Enforcement": true, "FallGuyBurned": true,
	"FrontAudited": true, "CrewSkimmed": true, "PlayerUndercut": true, "ShipmentSeized": true,
	"CrewShot": true, "CrewArrested": true, "CrewQuit": true, "CrewDefected": true, "CrewPoached": true, "LieutenantWalked": true,
	"RivalBoosted": true, "RivalMusclePoached": true, "SupplierCollected": true, "ContractFailed": true,
	"Incident": true, "WholesaleBought": true,
}

// TestPreviewAgreesWithAQuietNight (#353): every policy the harness
// plays, eighty nights of seed 7. On a night that rolled nothing the
// preview cannot know, the closing it projects is the closing the
// report finds, dirty and clean, to the dollar; and it is on most
// nights, so the check is not vacuous.
func TestPreviewAgreesWithAQuietNight(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	quiet, nights := 0, 0
	for _, np := range harness.Policies {
		policy := np.Make(cfg, harness.DefaultPolicyOpts())
		s, err := engine.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		w := s.NewRun(7, game.Start{})
		for d := 0; d < 80 && w.Over == nil; d++ {
			policy(w)
			p := s.Preview()
			evs := s.EndDay()
			if w.Over != nil {
				break
			}
			nights++
			rolled := ""
			for _, e := range evs {
				if k := fmt.Sprintf("%T", e)[len("events."):]; dice[k] {
					rolled = k
				}
			}
			if rolled != "" {
				continue
			}
			quiet++
			got := w.Report.Flow.Closing
			if p.Flow.Closing.Dirty != got.Dirty || p.Flow.Closing.Clean != got.Clean {
				t.Errorf("%s, day %d: projected %+v, the night closed on %+v", np.Name, w.Day, p.Flow.Closing, got)
			}
		}
	}
	if quiet*2 < nights {
		t.Fatalf("%d quiet nights of %d: the check is thin", quiet, nights)
	}
	t.Logf("%d quiet nights of %d agree", quiet, nights)
}

// TestPreviewNeverWritesTheWorld (#353): the world's JSON is the same
// before and after a preview, every morning of the boss's run and the
// distributor's (the road, the fronts, the crew, the contracts).
func TestPreviewNeverWritesTheWorld(t *testing.T) {
	t.Parallel()
	cfg := content.MustLoad()
	for _, policy := range []harness.Policy{harness.Boss(cfg, 40, ""), harness.Distributor(cfg, 40), harness.Trader(cfg, events.DialAggressive)} {
		s, err := engine.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if s.Preview() != nil {
			t.Fatal("a preview before a run")
		}
		w := s.NewRun(3, game.Start{})
		for d := 0; d < 60 && w.Over == nil; d++ {
			policy(w)
			before, err := json.Marshal(w)
			if err != nil {
				t.Fatal(err)
			}
			p := s.Preview()
			after, _ := json.Marshal(w)
			if string(before) != string(after) {
				t.Fatalf("day %d: the preview wrote the world", w.Day)
			}
			if p == nil || p.Day != w.Day+1 || len(p.Flow.Lines) != len(game.FlowCats) {
				t.Fatalf("day %d: preview %+v", w.Day, p)
			}
			// The alerts are the morning's, each with the act that
			// answers it (#352), so a front end jumps from the preview.
			got, _ := json.Marshal(p.Alerts)
			want, _ := json.Marshal(append([]engine.Alert{}, s.Alerts()...))
			if string(got) != string(want) {
				t.Fatalf("day %d: the preview's alerts %s, the morning's %s", w.Day, got, want)
			}
			for _, a := range p.Alerts {
				if a.Act.Screen == "" {
					t.Fatalf("day %d: %s has no act", w.Day, a.Key)
				}
			}
			s.EndDay()
		}
	}
}
