package crew_test

import (
	"math"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/gametest"
	"github.com/theclifmeister/kingpin/internal/sim/crew"
)

// The lab (#48): with the lab asset owned and standing in its city a
// chemist's cook there is lab_mul the batch, lands at lab_quality where
// that is over the chemist's, and the precursors cost lab_cost_mul of
// the cook_cost; nowhere else, and not while it stands idle.
func TestTheLabMultipliesTheCook(t *testing.T) {
	cfg := content.MustLoad()
	lab := cfg.Assets.ByEffect(content.AssetLab)
	if lab == nil {
		t.Fatal("no lab in the file")
	}
	w := game.NewWorld(7, []game.StartingCity{{ID: lab.City, Name: "Lab City", Products: []game.StartingProduct{gametest.A}}, {ID: "other", Name: "Other", Products: []game.StartingProduct{gametest.A}}}, 1000, 100)
	s := crew.New(cfg)
	w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Doc", Role: game.RoleChemist, Skill: 50, Loyalty: 80}}
	batch, quality := s.Batch(w), s.ChemistQuality(w)
	if s.BatchIn(w, lab.City) != batch || s.QualityIn(w, lab.City) != quality || s.CookCostIn(w, lab.City, 900) != 900 || s.Lab(w, lab.City) != nil {
		t.Fatal("the lab reads without the asset")
	}
	w.Assets = []game.Asset{{ID: lab.ID, Name: lab.Name, City: lab.City}}
	if s.Lab(w, lab.City) == nil || s.Lab(w, "other") != nil {
		t.Fatal("the lab reads in the wrong city")
	}
	if got := s.BatchIn(w, lab.City); got != int(math.Round(float64(batch)*lab.LabMul)) {
		t.Fatalf("batch %d, want %d x %.1f", got, batch, lab.LabMul)
	}
	if got := s.QualityIn(w, lab.City); got != math.Max(quality, lab.LabQuality) {
		t.Fatalf("quality %.0f, want %.0f", got, math.Max(quality, lab.LabQuality))
	}
	if got := s.CookCostIn(w, lab.City, 900); got != int(math.Round(900*lab.LabCostMul)) {
		t.Fatalf("cost %d, want %d", got, int(math.Round(900*lab.LabCostMul)))
	}
	if s.BatchIn(w, "other") != batch || s.QualityIn(w, "other") != quality || s.CookCostIn(w, "other", 900) != 900 {
		t.Fatal("the lab reads in the other city")
	}
	w.Crew.Members[0].Skill = 100 // a chemist better than the lab keeps their quality
	if got := s.QualityIn(w, lab.City); got != s.ChemistQuality(w) || got < lab.LabQuality {
		t.Fatalf("quality %.0f with a skill-100 chemist", got)
	}
	w.Assets[0].FrozenUntil = w.Day + 5
	if s.Lab(w, lab.City) != nil || s.BatchIn(w, lab.City) != s.Batch(w) {
		t.Fatal("an idle lab cooks")
	}
	w.Crew.Members = nil
	w.Assets[0].FrozenUntil = 0
	if s.BatchIn(w, lab.City) != 0 || s.QualityIn(w, lab.City) != 0 {
		t.Fatal("the lab cooks with no chemist")
	}
}
