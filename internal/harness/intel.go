package harness

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Intel (#45). The harness policies read the world, not the file: they
// tune the sims, they are not players. The one exception is Informed,
// whose point is the file: it is the value of the intel in numbers.

// Informed is Diplomat with a cop on the payroll (#45): whenever the
// heat where it stands is over InformedHeat and the file holds no cop's
// word on the police there that still speaks of a night to come, it
// pays a cop cop_price; and the day the word says the police can move
// again at the sting rung or above (the first night the cooldown lets
// them, or any night after while the word lives), it lies low whatever
// the margin, so a sting that comes finds it not dealing and adds no
// page (#27). Otherwise it plays as the diplomat does. A wrong word
// (cop_accuracy) is a night it sells into a sting.
func Informed(cfg *content.Config, lieLowAt float64, corners int) Policy {
	diplomat := Diplomat(cfg, lieLowAt, corners)
	tun := cfg.Intel.Intel
	return func(w *game.World) {
		diplomat(w)
		if w.Over != nil {
			return
		}
		here := w.Here()
		level, from, known := game.Known(w).Response(here.ID)
		if here.Heat > InformedHeat && (!known || from < w.Day+1) && w.Today.Cop == nil && w.Player.DirtyCash >= InformedMargin*tun.CopPrice {
			_ = w.PayCop(tun.CopPrice)
		}
		if known && content.Rank(level) >= content.Rank(content.Sting) && from <= w.Day+1 {
			w.SetLieLow(true)
		}
	}
}

// InformedHeat is the heat over which the informed player pays a cop,
// and InformedMargin how many times the price it keeps in hand.
const (
	InformedHeat   = 50.0
	InformedMargin = 3
)

// NoIntel returns a copy of cfg with the feed boxed (#45): no faction
// plants a lie, so a run that pays no cop and plants no spy is
// byte-for-byte the run before the feature (TestNoIntelIsTheOldRun);
// the observation facts are written off events already emitted, with
// no dice, and change nothing a policy reads.
func NoIntel(cfg *content.Config) *content.Config {
	boxed := *cfg
	boxed.Intel.Intel.FeedChance = 0
	return &boxed
}
