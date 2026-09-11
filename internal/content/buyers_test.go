package content

import (
	"strings"
	"testing"
)

// A key in buyers.toml that nobody reads fails at start-up, the way it
// does for the dilemma deck: a typo would otherwise silently do nothing.
func TestBuyersRefuseUnknownKey(t *testing.T) {
	good := `
[buyers]
min_gap = 3
max_gap = 7
offer_days = 3
blacklist_days = 30
respect = 3
notoriety_penalty = 5
keep_days = 14

[[buyer]]
id = "x"
name = "a tester"
pitch = "{{.Name}} wants {{.Units}} {{.Product}}."
city = "any"
product = "any"
size = [1.0, 2.0]
days = [2, 4]
premium = [1.2, 1.5]
penalty = 5
penalty_cash = 0.2
heat = 0.4
`
	var cfg BuyersConfig
	if err := decodeBytes("buyers.toml", []byte(good), &cfg); err != nil {
		t.Fatal(err)
	}
	if err := cfg.validate(MustLoad().Market, MustLoad().City); err != nil {
		t.Fatal(err)
	}
	bad := good + "premuim_cash = 0.5\n"
	if err := decodeBytes("buyers.toml", []byte(bad), &BuyersConfig{}); err == nil || !strings.Contains(err.Error(), "unknown key") {
		t.Fatalf("a misspelt key was accepted: %v", err)
	}
	bad = strings.Replace(good, "heat = 0.4", "heat = 0.4\n[buyer.trigger]\ncorner = 3", 1)
	if err := decodeBytes("buyers.toml", []byte(bad), &BuyersConfig{}); err == nil || !strings.Contains(err.Error(), "unknown key") {
		t.Fatalf("a misspelt trigger field was accepted: %v", err)
	}
}

// The deck in the box reads as a deck: every band a pair, every city and
// product known, and it refuses what is not.
func TestBuyersValidate(t *testing.T) {
	cfg := MustLoad()
	if len(cfg.Buyers.Deck) < 5 {
		t.Fatalf("only %d buyers in the deck", len(cfg.Buyers.Deck))
	}
	for _, bad := range []func(b *BuyerConfig){
		func(b *BuyerConfig) { b.City = "atlantis" },
		func(b *BuyerConfig) { b.Product = "snow" },
		func(b *BuyerConfig) { b.Size = []float64{2, 1} },
		func(b *BuyerConfig) { b.Days = []int{0, 3} },
		func(b *BuyerConfig) { b.Premium = []float64{1.5} },
		func(b *BuyerConfig) { b.Heat = 0 },
		func(b *BuyerConfig) { b.PenaltyCash = 2 },
	} {
		deck := BuyersConfig{Buyers: cfg.Buyers.Buyers, Deck: append([]BuyerConfig(nil), cfg.Buyers.Deck...)}
		bad(&deck.Deck[0])
		if err := deck.validate(cfg.Market, cfg.City); err == nil {
			t.Errorf("a bad buyer was accepted: %+v", deck.Deck[0])
		}
	}
}
