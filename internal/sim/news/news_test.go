package news_test

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/news"
)

// Every event the market, logistics, territory, rivals, crew, heat, law
// and laundering sims can emit must have a headline template, otherwise
// the ticker goes silent on something that matters. RivalUndercut,
// CashLaundered, FrontInvested, Reserved, CrewPaidOff, WholesaleBought, SupplyBought,
// SupplyShort, StandingShort, ShipmentSent, ShipmentArrived and CityFunded are
// report-only bookkeeping, like PriceMove, CrewPaid and LieutenantActed;
// CrewTurnedInformant and LieutenantFlipped are deliberately silent, the
// informant is hidden; DilemmaDrawn and DilemmaAnswered carry their own
// text, the card's; an Incident's key is the row's (#44).
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
		"CrewArrested", "CrewReleased", "CrewShot", "CrewKilled", "CrewRetired", "MuscleKilled", // CrewBailed, CrewRecovered and KinLooking are report-only (#46)
		"CornerClaimed", "CornerLost", "CornerRobbed", "CornerCrackdown",
		"RivalMovedIn", "RivalEyeing", "RivalOutbid", "RivalClaimed", "CornerTaken", "CornerHanded", "RivalPushed",
		"RivalAbandoned",                                                   // PlayerUndercut is report-only
		"CornerStruckTaken", "CornerStruckHeld", "RivalRouted", "WarEnded", // the war order (#229)
		"RivalTippedPolice", "WarOpen", "WarCrackdown",
		"RivalBoosted", "RivalBoostedHeld", "RivalRaided", "RivalMusclePoached", // RivalScouted and PoliceTipped are report-only (#70)
		"ReignBegan",                                               // ReignBroken is report-only (#227)
		"DealOffered", "DealAccepted", "DealRefused", "DealBroken", // DealEnded and TributePaid are report-only
		"UpgradeBought", "FallGuyBurned",
		"FrontBought", "FrontAudited", "FrontFrozen", "FrontGrew", // FrontInvested (#192) and Reserved (#195) are report-only
		"ReputationFearUp", "ReputationFearDown", "ReputationRespectUp", "ReputationRespectDown",
		"ReputationNotorietyUp", "ReputationNotorietyDown",
		"DAElected", "DAReElected", "ChiefReplaced", "ChiefReplacedDA", "PressureShiftedUp", "PressureShiftedDown", // CityFunded is report-only
		"DABought", "CampaignLost", "CampaignHedged", "ChiefReplacedCampaign", // CampaignBacked is report-only (#193)
		"BribeBackfired", "LeadsFiled", "OfficialsCold", // BribeAccepted, BribeRefused, LeadFound and CheckpointBought are report-only (#42)
		"RaidFellThrough",                                        // the favour (#228)
		"ContractOffered", "ContractDelivered", "ContractFailed", // ContractAccepted and ContractExpired are report-only
		"DebtLate", "SupplierFrozen", "SupplierWarned", "SupplierCollected", // SupplierBought, CreditTaken and DebtPaid are report-only (#72)
		"TierReached",                                            // #147
		"HouseBought", "HouseRobbed", "HouseRaided", "HouseLost", // HouseCompromised, StockMoved and RentPaid are report-only (#73)
		"Overdose",                  // StockCut and Cooked are report-only (#47)
		"DeedsBought", "DeedSeized", // DeedBought and DeedRent are report-only (#194); Taxed is report-only (#231); ClaimDeterred is report-only (#233)
		"SpyPlanted", "SpyFound", "SpyShot", "IntelFalseRoute", "IntelFalseStash", // IntelGained is report-only (#45)
	}
	for _, r := range cfg.Heat.Responses {
		required = append(required, "Enforcement"+capital(r.Level))
	}
	// The world's incidents (#44): Incident for a row without its own,
	// and every row's key beside it.
	required = append(required, "Incident")
	for _, inc := range cfg.Incidents.Table {
		required = append(required, inc.Key())
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

// The paper names you and the morning opens on you (#233): the title
// is a dealer with nobody on the payroll, a crew with somebody, the
// boss of home while the city is yours in the kingpin's sense; the
// swagger headlines name the boss off their own stream and never the
// dealer; the TIER section opens with the night's homage (before the
// reign's line has it), the pushes held off and the claims your name
// turned, and says nothing on a night with nothing to say.
func TestThePaperNamesYou(t *testing.T) {
	cfg := content.MustLoad()
	n, err := news.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Headlines.Swagger) < 3 {
		t.Fatalf("%d swagger headlines", len(cfg.Headlines.Swagger))
	}
	w := sim.NewWorld(cfg, 4)
	tk := func(day int, evs ...events.Event) *game.Tick {
		t := &game.Tick{Day: day, RNG: game.RNGFor(w.Seed, day), Seed: w.Seed}
		for _, e := range evs {
			t.Emit(e)
		}
		return t
	}
	w.Day = 4
	n.Step(w, tk(5))
	if w.Report == nil || len(w.Report.Tier) != 0 {
		t.Fatalf("a night with nothing to say opened on something: %v", w.Report.Tier)
	}
	// The boss: every faction gone, six of ten held; a swagger headline
	// turns up within a few mornings and names the city, and the flavour
	// rolled on the home stream is the flavour it always was.
	for _, r := range w.Rivals {
		r.Arrived, r.Fragmented = 1, 1
	}
	home := w.Home()
	for i := 0; i < 6; i++ {
		home.Corners[i].Owner, home.Corners[i].Runner = game.OwnerPlayer, 1
	}
	named := false
	for day := 6; day <= 40 && !named; day++ {
		w.Day = day - 1
		n.Step(w, tk(day))
		for _, h := range w.Journal {
			if h.Day == day && strings.Contains(h.Text, "the boss of "+home.Name) {
				named = true
			}
		}
	}
	if !named {
		t.Fatal("the paper never named the boss")
	}
	// The morning's opening lines.
	w.Day = 40
	rival := w.Rival()
	evs := []events.Event{
		events.TributePaid{Day: 41, Rival: rival.Leader, Faction: rival.Faction(), Amount: 4_100, ToYou: true},
		events.TributePaid{Day: 41, Rival: "Sal", Faction: "f2", Amount: 900, ToYou: true},
		events.RivalPushed{Day: 41, Corner: home.Corners[0].ID, Name: home.Corners[0].Name, Rival: "Vasquez", Faction: "f3"},
		events.RivalPushed{Day: 41, Corner: home.Corners[1].ID, Name: home.Corners[1].Name, Rival: "Vasquez", Faction: "f3"},
		events.CrewShot{Day: 41, Rival: "Vasquez", Faction: "f3", Theirs: true, Dead: true},
		events.ClaimDeterred{Day: 41, City: home.ID, Rival: "Sal", Faction: "f2"},
	}
	n.Step(w, tk(41, evs...))
	got := strings.Join(w.Report.Tier, "\n")
	for _, want := range []string{"2 crews paid homage last night: $5,000.", "Vasquez's crew pushed on your front line twice and were held off, losing 1 head.", "Nobody set up on a free corner in " + home.Name + ": your name kept them out."} {
		if !strings.Contains(got, want) {
			t.Errorf("the opening lacks %q:\n%s", want, got)
		}
	}
	// Under the reign the homage is the reign's line, said once.
	w.Reign = 30
	w.Day = 41
	n.Step(w, tk(42, evs[0]))
	got = strings.Join(w.Report.Tier, "\n")
	if !strings.Contains(got, "REIGN: day 13 of the reign") || strings.Contains(got, "paid homage last night") {
		t.Errorf("under the reign:\n%s", got)
	}
}
