package news_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/sim/news"
)

// Every event the market, territory, rivals, crew, heat and laundering
// sims can emit must have a headline template, otherwise the ticker goes
// silent on something that matters. RivalUndercut, CashLaundered and
// CrewPaidOff are report-only bookkeeping, like PriceMove and CrewPaid;
// CrewTurnedInformant is deliberately silent, the informant is hidden;
// DilemmaDrawn and DilemmaAnswered carry their own text, the card's.
func TestEveryEmittedEventHasTemplate(t *testing.T) {
	cfg := content.MustLoad()
	n, err := news.New(cfg.Headlines, cfg.Dilemmas)
	if err != nil {
		t.Fatal(err)
	}
	required := []string{
		"PriceShock", "PriceSlump", "ProductUnlocked",
		"PlayerSoldBig", "PlayerSoldZero",
		"EnforcementPatrol", "EnforcementSting", "EnforcementRaid", "EnforcementArrest",
		"LaidLow", "HeatWarning",
		"CrewHired", "CrewFired", "CrewFiredInformant", "CrewQuit", "CrewSkimmed",
		"CrewDefected", "InvestigationRun",
		"CornerClaimed", "CornerLost", "CornerRobbed", "CornerCrackdown",
		"RivalMovedIn", "RivalClaimed", "CornerTaken", "CornerHanded", "RivalPushed",
		"CornerStruckTaken", "CornerStruckHeld", "RivalRouted",
		"RivalTippedPolice", "WarOpen", "WarCrackdown",
		"UpgradeBought", "FallGuyBurned",
		"FrontBought", "FrontAudited", "FrontFrozen",
		"ReputationFearUp", "ReputationFearDown", "ReputationRespectUp", "ReputationRespectDown",
		"ReputationNotorietyUp", "ReputationNotorietyDown",
	}
	for _, r := range cfg.Heat.Responses {
		required = append(required, "Enforcement"+capital(r.Level))
	}
	for _, k := range required {
		if !n.HasTemplate(k) {
			t.Errorf("no headline template for %s", k)
		}
	}
	if len(cfg.Headlines.Flavour) < 10 {
		t.Errorf("only %d flavour headlines; the ticker will repeat", len(cfg.Headlines.Flavour))
	}
}

func capital(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-'a'+'A') + s[1:]
}
