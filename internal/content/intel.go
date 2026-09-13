package content

import "fmt"

// IntelConfig mirrors intel.toml (#45): what the file of facts fades
// at, what a push tells you, what a cop, a spy and a lie are worth.
type IntelConfig struct {
	Intel IntelTuning `toml:"intel"`
}

// IntelTuning is the [intel] table. StaleRate is what a fact's
// confidence loses a day and Forget the line under which it is dropped;
// ObserveConfidence and ObserveBand are what a push, a strike or a boost
// reveals of a faction's muscle (a band ObserveBand heads wide at that
// confidence); CopPrice buys a cop's word at CopAccuracy (less money,
// less often, in proportion), a wrong word off by up to CopSlip days or
// a rung; a spy files every SpyDays nights, true at SpyAccuracy for a
// skill of SpySkill (scaled linearly by skill), a wrong count off by up
// to SpySlip heads, and every report is found at SpyFound by the
// faction's temper, TurnShare of those found coming home turned and the
// rest shot; a faction trusting you under FeedTrust plants a lie at
// FeedChance a night at FeedConfidence, a fed road claiming FeedRisk a
// day.
type IntelTuning struct {
	StaleRate         float64            `toml:"stale_rate"`
	Forget            float64            `toml:"forget"`
	ObserveConfidence float64            `toml:"observe_confidence"`
	ObserveBand       int                `toml:"observe_band"`
	CopPrice          int                `toml:"cop_price"`
	CopAccuracy       float64            `toml:"cop_accuracy"`
	CopSlip           int                `toml:"cop_slip"`
	SpyDays           int                `toml:"spy_days"`
	SpyAccuracy       float64            `toml:"spy_accuracy"`
	SpySkill          int                `toml:"spy_skill"`
	SpySlip           int                `toml:"spy_slip"`
	TurnShare         float64            `toml:"turn_share"`
	FeedTrust         float64            `toml:"feed_trust"`
	FeedChance        float64            `toml:"feed_chance"`
	FeedConfidence    float64            `toml:"feed_confidence"`
	FeedRisk          float64            `toml:"feed_risk"`
	SpyFound          map[string]float64 `toml:"spy_found"`
}

// Accuracy is how often a cop paid amount is right: cop_accuracy at
// cop_price, less in proportion under it, never more.
func (t IntelTuning) Accuracy(amount int) float64 {
	if t.CopPrice <= 0 || amount >= t.CopPrice {
		return t.CopAccuracy
	}
	return t.CopAccuracy * float64(amount) / float64(t.CopPrice)
}

// SpyOdds is how often a spy of the skill reports true: spy_accuracy at
// spy_skill, scaled by the skill, never over 0.98.
func (t IntelTuning) SpyOdds(skill int) float64 {
	if t.SpySkill <= 0 {
		return t.SpyAccuracy
	}
	return min(0.98, t.SpyAccuracy*float64(skill)/float64(t.SpySkill))
}

// Found is a report's chance of the spy being found under a faction of
// the temper.
func (t IntelTuning) Found(personality string) float64 { return t.SpyFound[personality] }

// validate checks the table: shares in 0..1, days and heads positive.
func (c IntelConfig) validate() error {
	t := c.Intel
	for name, v := range map[string]float64{"stale_rate": t.StaleRate, "forget": t.Forget, "observe_confidence": t.ObserveConfidence, "cop_accuracy": t.CopAccuracy, "spy_accuracy": t.SpyAccuracy, "turn_share": t.TurnShare, "feed_chance": t.FeedChance, "feed_confidence": t.FeedConfidence, "feed_risk": t.FeedRisk} {
		if v < 0 || v > 1 {
			return fmt.Errorf("[intel] %s %v (0..1)", name, v)
		}
	}
	if t.ObserveBand < 1 || t.SpyDays < 1 || t.CopPrice < 0 || t.SpySkill < 0 || t.CopSlip < 0 || t.SpySlip < 0 || t.FeedTrust < 0 {
		return fmt.Errorf("bad [intel] table %+v", t)
	}
	for _, p := range Personalities {
		if v, ok := t.SpyFound[p]; !ok || v < 0 || v > 1 {
			return fmt.Errorf("[intel.spy_found] needs %s in 0..1", p)
		}
	}
	return nil
}
