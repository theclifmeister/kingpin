package news_test

import (
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/sim/news"
)

// Every event the market, territory, crew and heat sims can emit must have a headline
// template, otherwise the ticker goes silent on something that matters.
func TestEveryEmittedEventHasTemplate(t *testing.T) {
	cfg := content.MustLoad()
	n, err := news.New(cfg.Headlines)
	if err != nil {
		t.Fatal(err)
	}
	required := []string{
		"PriceShock", "PriceSlump", "ProductUnlocked",
		"PlayerSoldBig", "PlayerSoldZero",
		"EnforcementPatrol", "EnforcementSting", "EnforcementRaid", "EnforcementArrest",
		"LaidLow", "HeatWarning",
		"CrewHired", "CrewFired", "CrewQuit", "CrewSkimmed",
		"CornerClaimed", "CornerLost", "CornerRobbed",
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
