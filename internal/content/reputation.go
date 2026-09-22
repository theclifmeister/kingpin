package content

import "fmt"

// ReputationConfig mirrors reputation.toml: where the three axes come
// from, how they fade, and what the other sims read off them.
type ReputationConfig struct {
	Reputation ReputationTuning `toml:"reputation"`
	Fear       FearSources      `toml:"fear"`
	Respect    RespectSources   `toml:"respect"`
	Notoriety  NotorietySources `toml:"notoriety"`
	Effects    ReputationFX     `toml:"effects"`
}

type ReputationTuning struct {
	Baseline float64 `toml:"baseline"` // every axis fades toward this
	Decay    float64 `toml:"decay"`    // fraction of the gap to baseline closed per day
	Total    float64 `toml:"total"`    // the three never add up to more than this
	Band     float64 `toml:"band"`     // ReputationShifted fires on crossing a multiple of this
}

type FearSources struct {
	StrikeTaken float64 `toml:"strike_taken"`
	StrikeHeld  float64 `toml:"strike_held"`
	PushHeld    float64 `toml:"push_held"`
	Boost       float64 `toml:"boost"` // a rival corner's takings your enforcers took (#70)
}

type RespectSources struct {
	GenerousPay float64 `toml:"generous_pay"`
	Payoff      float64 `toml:"payoff"`
	ShortPay    float64 `toml:"short_pay"`
	DealKept    float64 `toml:"deal_kept"` // per day of a truce or split kept
}

type NotorietySources struct {
	Units       float64  `toml:"units"` // a point per this many units sold in a day
	Headline    float64  `toml:"headline"`
	Sources     []string `toml:"headline_sources"` // journal sources whose headlines are about you
	StrikeTaken float64  `toml:"strike_taken"`
	StrikeHeld  float64  `toml:"strike_held"`
	Overdose    float64  `toml:"overdose"` // an overdose on your corner (#47)
	Body        float64  `toml:"body"`     // a death on your corner, either side (#46): the paper names you
}

// ReputationFX is what each axis does at 100; each sim scales the knob it
// owns by the axis and never reads another sim's.
type ReputationFX struct {
	FearPushCut        float64 `toml:"fear_push_cut"`
	FearHeatFloor      float64 `toml:"fear_heat_floor"`
	RespectLoyaltyCut  float64 `toml:"respect_loyalty_cut"`
	RespectSupplierCut float64 `toml:"respect_supplier_cut"`
	NotorietyHireCut   float64 `toml:"notoriety_hire_cut"`
	NotorietyHeat      float64 `toml:"notoriety_heat"`
	FearDeal           float64 `toml:"fear_deal"`       // rivals: added to the chance a proposal is accepted
	RespectTrust       float64 `toml:"respect_trust"`   // rivals: extra on the trust a kept deal-day earns
	RivalClaimCut      float64 `toml:"rival_claim_cut"` // rivals: fraction cut from the chance it claims a corner
	PoachPriceCut      float64 `toml:"poach_price_cut"` // rivals: fraction cut from the price of buying off a head of its muscle (#70)
}

// Scale is v at axis 0 and v times (1 + full) at axis 100: how an effect
// that adds grows with an axis.
func Scale(axis, full float64) float64 { return 1 + full*clamp01(axis/100) }

// Cut is 1 at axis 0 and 1 - full at axis 100: how an effect that takes
// away grows with an axis.
func Cut(axis, full float64) float64 { return 1 - full*clamp01(axis/100) }

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// validate checks the [reputation] table: a total and a band to count
// in, and a decay that is a fraction.
func (r ReputationConfig) validate() error {
	if t := r.Reputation; t.Total <= 0 || t.Band <= 0 || t.Decay < 0 || t.Decay > 1 {
		return fmt.Errorf("bad [reputation] table %+v", t)
	}
	return nil
}
