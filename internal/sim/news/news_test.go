package news_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/sim/news"
)

// Every event the market, logistics, territory, rivals, crew, heat, law
// and laundering sims can emit must have a headline template, otherwise
// the ticker goes silent on something that matters. RivalUndercut,
// CashLaundered, CrewPaidOff, WholesaleBought, SupplyBought,
// SupplyShort, StandingShort, ShipmentSent, ShipmentArrived and CityFunded are
// report-only bookkeeping, like PriceMove, CrewPaid and LieutenantActed;
// CrewTurnedInformant and LieutenantFlipped are deliberately silent, the
// informant is hidden; DilemmaDrawn and DilemmaAnswered carry their own
// text, the card's.
func TestEveryEmittedEventHasTemplate(t *testing.T) {
	cfg := content.MustLoad()
	n, err := news.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	required := []string{
		"PriceShock", "PriceShockSeized", "PriceSlump", "ShipmentSeized",
		"UnlockedProduct", "UnlockedFront", "UnlockedConnect", "UnlockedRole", // Unlocked, by its Gate (#148)
		"PlayerSoldBig", "PlayerSoldZero",
		"EnforcementPatrol", "EnforcementSting", "EnforcementRaid", "EnforcementArrest",
		"LaidLow", "HeatWarning",
		"CrewHired", "CrewFired", "CrewFiredInformant", "CrewQuit", "CrewSkimmed",
		"CrewDefected", "InvestigationRun", "LieutenantWalked", "LieutenantWalkedRival",
		"CornerClaimed", "CornerLost", "CornerRobbed", "CornerCrackdown",
		"RivalMovedIn", "RivalEyeing", "RivalOutbid", "RivalClaimed", "CornerTaken", "CornerHanded", "RivalPushed",
		"RivalAbandoned", // PlayerUndercut is report-only
		"CornerStruckTaken", "CornerStruckHeld", "RivalRouted",
		"RivalTippedPolice", "WarOpen", "WarCrackdown",
		"RivalBoosted", "RivalBoostedHeld", "RivalRaided", "RivalMusclePoached", // RivalScouted and PoliceTipped are report-only (#70)
		"DealOffered", "DealAccepted", "DealRefused", "DealBroken", // DealEnded and TributePaid are report-only
		"UpgradeBought", "FallGuyBurned",
		"FrontBought", "FrontAudited", "FrontFrozen",
		"ReputationFearUp", "ReputationFearDown", "ReputationRespectUp", "ReputationRespectDown",
		"ReputationNotorietyUp", "ReputationNotorietyDown",
		"DAElected", "DAReElected", "ChiefReplaced", "ChiefReplacedDA", "PressureShiftedUp", "PressureShiftedDown", // CityFunded is report-only
		"DABought", "CampaignLost", "CampaignHedged", "ChiefReplacedCampaign", // CampaignBacked is report-only (#193)
		"ContractOffered", "ContractDelivered", "ContractFailed", // ContractAccepted and ContractExpired are report-only
		"DebtLate", "SupplierFrozen", "SupplierWarned", "SupplierCollected", // SupplierBought, CreditTaken and DebtPaid are report-only (#72)
		"TierReached",                                            // #147
		"HouseBought", "HouseRobbed", "HouseRaided", "HouseLost", // HouseCompromised, StockMoved and RentPaid are report-only (#73)
		"Overdose", // StockCut and Cooked are report-only (#47)
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
