package news_test

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/gametest"
	"github.com/theclifmeister/kingpin/internal/sim"
	"github.com/theclifmeister/kingpin/internal/sim/news"
)

// reportOnly is every kind with no headline template on purpose (#274):
// bookkeeping the morning report prints and the ticker would only
// repeat, the informant's that are deliberately silent (the informant
// is hidden), the card's that carry their own text, and the loop's own.
// A new kind goes here or gets a template in headlines.toml;
// TestEveryEmittedEventHasTemplate fails on one that does neither and
// on one that does both.
var reportOnly = map[string]bool{
	"PriceMove": true, "CrewPaid": true, "LieutenantActed": true,
	"RivalUndercut": true, "CashLaundered": true, "CrewPaidOff": true,
	"WholesaleBought": true, "SupplyBought": true, "SupplyShort": true, "StandingShort": true,
	"ShipmentSent": true, "ShipmentArrived": true, "CityFunded": true,
	"PlayerUndercut":      true,
	"CrewTurnedInformant": true, "LieutenantFlipped": true, // deliberately silent: the informant is hidden
	"DilemmaDrawn": true, "DilemmaAnswered": true, // they carry their own text, the card's
	"FrontInvested": true, "Reserved": true, "AssetUpkeepPaid": true, // #192, #195, #351
	"CrewBailed": true, "CrewRecovered": true, "KinLooking": true, // #46
	"RivalScouted": true, "PoliceTipped": true, // #70
	"ReignBroken": true, // #227
	"DealEnded":   true, "TributePaid": true,
	"CampaignBacked": true,                                                                    // #193
	"BribeAccepted":  true, "BribeRefused": true, "LeadFound": true, "CheckpointBought": true, // #42
	"ContractAccepted": true, "ContractExpired": true,
	"SupplierBought": true, "CreditTaken": true, "DebtPaid": true, // #72
	"HouseCompromised": true, "StockMoved": true, "RentPaid": true, // #73
	"StockCut": true, "CookOrdered": true, "Cooked": true, // #47
	"ExportShipped": true,                   // #391
	"DeedBought":    true, "DeedRent": true, // #194
	"Taxed":         true, // #231
	"ClaimDeterred": true, // #233
	"TrustSpread":   true, // #43
	"AssetFrozen":   true, // #48
	"IntelGained":   true, // #45
	"CaptainActed":  true, // #346
	// Found unlisted (#274): neither templated nor named report-only
	// before events.All; the loop's own, never a line of news.
	"DayEnded": true, "Headline": true, "GameOver": true,
}

// templateKeys is the headline keys a kind writes where they are not
// just its own name: a kind picks one by what happened (a seizure, a
// slump, a firing that was an informant), by its level or axis, or by
// its gate. A kind not here writes the key of its own name.
func templateKeys(cfg *content.Config) map[string][]string {
	keys := map[string][]string{
		"PriceShock":          {"PriceShock", "PriceShockSeized", "PriceSlump"},
		"Unlocked":            {"UnlockedProduct", "UnlockedFront", "UnlockedConnect", "UnlockedRole", "UnlockedAsset"}, // by its Gate (#148)
		"PlayerSold":          {"PlayerSoldBig", "PlayerSoldZero"},
		"HeatChanged":         {"HeatWarning"},
		"CrewFired":           {"CrewFired", "CrewFiredInformant"},
		"CrewShot":            {"CrewShot", "CrewKilled", "MuscleKilled"}, // #46
		"LieutenantWalked":    {"LieutenantWalked", "LieutenantWalkedRival"},
		"CornerLost":          {"CornerLost", "CornerCrackdown"},
		"CornerTaken":         {"CornerTaken", "CornerHanded", "RivalClaimed"},
		"CornerStruck":        {"CornerStruckTaken", "CornerStruckHeld", "RivalRouted"}, // the war order (#229)
		"WarEscalated":        {"WarOpen", "WarCrackdown"},
		"RivalBoosted":        {"RivalBoosted", "RivalBoostedHeld"}, // #70
		"FactionPushed":       {"FactionPushed", "FactionTook"},     // #43
		"RivalAbsorbed":       {"RivalAbsorbed", "RivalScattered"},
		"RivalLeaderArrested": {"RivalLeaderArrested", "RivalLeaderKilled"},
		"ReputationShifted": {"ReputationFearUp", "ReputationFearDown", "ReputationRespectUp", "ReputationRespectDown",
			"ReputationNotorietyUp", "ReputationNotorietyDown"},
		"DAElected":       {"DAElected", "DAReElected", "DABought"}, // #193
		"ChiefReplaced":   {"ChiefReplaced", "ChiefReplacedDA", "ChiefReplacedCampaign"},
		"PressureShifted": {"PressureShiftedUp", "PressureShiftedDown"},
		"SpyFound":        {"SpyFound", "SpyShot"},                // #45
		"IntelFalse":      {"IntelFalseRoute", "IntelFalseStash"}, // #45
	}
	// Every rung of the police response, the task force's (#48) included.
	for _, r := range cfg.Heat.Responses {
		keys["Enforcement"] = append(keys["Enforcement"], "Enforcement"+capital(r.Level))
	}
	// The world's incidents (#44): Incident for a row without its own,
	// and every row's key beside it.
	keys["Incident"] = []string{"Incident"}
	for _, inc := range cfg.Incidents.Table {
		keys["Incident"] = append(keys["Incident"], inc.Key())
	}
	return keys
}

// Every event kind (events.All, #274) has a headline template or is
// named report-only, otherwise the ticker goes silent on something that
// matters without anybody having decided it should: every key a kind
// writes has a template, a report-only kind has no template and no
// keys, the two lists name only real kinds, and every template in
// headlines.toml is one some kind writes.
func TestEveryEmittedEventHasTemplate(t *testing.T) {
	cfg := content.MustLoad()
	n, err := news.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	keys := templateKeys(cfg)
	kinds := map[string]bool{}
	written := map[string]bool{}
	for _, e := range events.All {
		k := e.Kind()
		kinds[k] = true
		if reportOnly[k] {
			if n.HasTemplate(k) {
				t.Errorf("%s is report-only but has a headline template", k)
			}
			if _, ok := keys[k]; ok {
				t.Errorf("%s is report-only but writes template keys", k)
			}
			continue
		}
		ks, ok := keys[k]
		if !ok {
			ks = []string{k}
		}
		for _, key := range ks {
			written[key] = true
			if !n.HasTemplate(key) {
				t.Errorf("no headline template for %s (kind %s): add one, or list the kind in reportOnly", key, k)
			}
		}
	}
	for k := range reportOnly {
		if !kinds[k] {
			t.Errorf("report-only %s is not an event kind", k)
		}
	}
	for k := range keys {
		if !kinds[k] {
			t.Errorf("template keys listed for %s, which is not an event kind", k)
		}
	}
	for key := range cfg.Headlines.Templates {
		if !written[key] {
			t.Errorf("headline template %s is written by no event kind", key)
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
		return gametest.TickOn(w, day, evs...)
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
