package harness

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
)

// PolicyOpts is everything cmd/balance's flags hand a scripted policy
// (#274): one struct so every constructor in Policies takes the same
// arguments, and a policy that reads none of them ignores it. The zero
// value is not the tool's default (ForceWarn and DialQuiet are the
// zeros); DefaultPolicyOpts is.
type PolicyOpts struct {
	LieLow     float64      // the heat the managed-style policies lie low at; 0 or under is each policy's own default
	Corners    int          // corners territory, war, diplomat, informed and pricewar work, counting yours
	Force      events.Force // how hard war strikes
	Undercut   events.Dial  // the dial pricewar undercuts at
	Lieutenant string       // the temper delegated and boss force on the lieutenant they hire; "" as generated
	Margin     float64      // boss's levels margin (BossAt)
	Houses     int          // houses stashed keeps a city; 0 is StashHouses
	Fronts     bool         // whether stashed buys fronts to pay the rent
}

// DefaultPolicyOpts is what cmd/balance passes with no flag set: three
// corners, war at push, pricewar at normal, BossMargin, stashed with its
// fronts.
func DefaultPolicyOpts() PolicyOpts {
	return PolicyOpts{Corners: 3, Force: events.ForcePush, Undercut: events.DialNormal, Margin: BossMargin, Fronts: true}
}

// at is the lie-low threshold a policy plays at: the override when one
// is set, else the policy's own default.
func (o PolicyOpts) at(def float64) float64 {
	if o.LieLow > 0 {
		return o.LieLow
	}
	return def
}

// NamedPolicy is one entry of the registry: the name cmd/balance's
// -policy takes and the constructor it plays.
type NamedPolicy struct {
	Name string
	Make func(cfg *content.Config, o PolicyOpts) Policy
}

// Policies is every scripted policy cmd/balance can play, in the order
// its -help lists them (#274). It is the one list: the tool looks a
// policy up here and refuses a name that is not in it, the help text
// is generated from it, and TestPolicyListsMatchTheRegistry holds
// docs/harness.md and CLAUDE.md to it, so the three can no longer drift
// the way the old switch, its help string and the docs did (33, 31 and
// "thirty-three" names against 35 cases).
var Policies = []NamedPolicy{
	{"idle", func(*content.Config, PolicyOpts) Policy { return Idle }},
	{"hide", func(*content.Config, PolicyOpts) Policy { return Hide }},
	{"quiet", func(cfg *content.Config, _ PolicyOpts) Policy { return Trader(cfg, events.DialQuiet) }},
	{"normal", func(cfg *content.Config, _ PolicyOpts) Policy { return Trader(cfg, events.DialNormal) }},
	{"aggressive", func(cfg *content.Config, _ PolicyOpts) Policy { return Trader(cfg, events.DialAggressive) }},
	{"careful", func(cfg *content.Config, o PolicyOpts) Policy { return Careful(cfg, o.at(35)) }},
	{"managed", func(cfg *content.Config, o PolicyOpts) Policy { return Managed(cfg, o.at(50)) }},
	{"upgraded", func(cfg *content.Config, o PolicyOpts) Policy { return Upgraded(cfg, o.at(40)) }},
	{"crewed", func(cfg *content.Config, o PolicyOpts) Policy { return Crewed(cfg, o.at(40)) }},
	{"vigilant", func(cfg *content.Config, o PolicyOpts) Policy { return Vigilant(cfg, o.at(40)) }},
	{"territory", func(cfg *content.Config, o PolicyOpts) Policy { return Territory(cfg, o.at(40), o.Corners) }},
	{"war", func(cfg *content.Config, o PolicyOpts) Policy { return Warlike(cfg, o.at(40), o.Corners, o.Force) }},
	{"warlord", func(cfg *content.Config, o PolicyOpts) Policy { return Warlord(cfg, o.at(40)) }},
	{"diplomat", func(cfg *content.Config, o PolicyOpts) Policy { return Diplomat(cfg, o.at(40), o.Corners) }},
	{"laundered", func(cfg *content.Config, o PolicyOpts) Policy { return Laundered(cfg, o.at(40)) }},
	{"funded", func(cfg *content.Config, o PolicyOpts) Policy { return Funded(cfg, o.at(40)) }},
	{"corrupt", func(cfg *content.Config, o PolicyOpts) Policy { return Corrupt(cfg, o.at(40)) }},
	{"favoured", func(cfg *content.Config, o PolicyOpts) Policy { return Favoured(cfg, o.at(40)) }},
	{"distributor", func(cfg *content.Config, o PolicyOpts) Policy { return Distributor(cfg, o.at(40)) }},
	{"driven", func(cfg *content.Config, o PolicyOpts) Policy { return Driven(cfg, o.at(40)) }},
	{"delegated", func(cfg *content.Config, o PolicyOpts) Policy { return Delegated(cfg, o.at(40), o.Lieutenant) }},
	{"dealer", func(cfg *content.Config, o PolicyOpts) Policy { return Dealer(cfg, o.at(40)) }},
	{"stocked", func(cfg *content.Config, o PolicyOpts) Policy { return Stocked(cfg, o.at(40)) }},
	{"routine", func(cfg *content.Config, o PolicyOpts) Policy { return Routine(cfg, o.at(40)) }},
	{"leveraged", func(cfg *content.Config, o PolicyOpts) Policy { return Leveraged(cfg, o.at(40)) }},
	{"boss", func(cfg *content.Config, o PolicyOpts) Policy { return BossAt(cfg, o.at(40), o.Lieutenant, o.Margin) }},
	{"pricewar", func(cfg *content.Config, o PolicyOpts) Policy { return Pricewar(cfg, o.at(40), o.Corners, o.Undercut) }},
	{"stashed", func(cfg *content.Config, o PolicyOpts) Policy { return Stashed(cfg, o.at(40), o.Houses, o.Fronts) }},
	{"saboteur", func(cfg *content.Config, o PolicyOpts) Policy { return Saboteur(cfg, o.at(40)) }},
	{"tipster", func(cfg *content.Config, o PolicyOpts) Policy { return Tipster(cfg, o.at(40)) }},
	{"cook", func(cfg *content.Config, o PolicyOpts) Policy { return Cook(cfg, o.at(40)) }},
	{"retiree", func(cfg *content.Config, o PolicyOpts) Policy { return Retiree(cfg, o.at(40)) }},
	{"cartel", func(cfg *content.Config, o PolicyOpts) Policy { return Cartel(cfg, o.at(40)) }},
	{"reckless", func(cfg *content.Config, _ PolicyOpts) Policy { return Reckless(cfg) }},
	{"informed", func(cfg *content.Config, o PolicyOpts) Policy { return Informed(cfg, o.at(40), o.Corners) }},
}

// PolicyNames is the registry's names in its order.
func PolicyNames() []string {
	names := make([]string, len(Policies))
	for i, p := range Policies {
		names[i] = p.Name
	}
	return names
}

// PolicyNamed is the registry's entry for name, or false when there is
// none.
func PolicyNamed(name string) (NamedPolicy, bool) {
	for _, p := range Policies {
		if p.Name == name {
			return p, true
		}
	}
	return NamedPolicy{}, false
}
