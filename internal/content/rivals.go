package content

import (
	"fmt"

	"github.com/theclifmeister/kingpin/internal/events"
)

// RivalsConfig mirrors rivals.toml.
type RivalsConfig struct {
	Rivals      RivalsTuning                 `toml:"rivals"`
	Pace        PaceTuning                   `toml:"pace"`
	Pricewar    PricewarTuning               `toml:"pricewar"`
	Diplomacy   DiplomacyTuning              `toml:"diplomacy"`
	Books       BooksTuning                  `toml:"books"`
	Boost       BoostTuning                  `toml:"boost"`
	Tip         TipTuning                    `toml:"tip"`
	Poach       PoachTuning                  `toml:"poach"`
	Factions    FactionsTuning               `toml:"factions"`
	Expansion   ExpansionTuning              `toml:"expansion"`
	Endings     RivalEndingsTuning           `toml:"endings"`
	War         WarTuning                    `toml:"war"`
	Weight      WeightTuning                 `toml:"weight"`
	Deal        map[string]DealConfig        `toml:"deal"`
	Personality map[string]PersonalityConfig `toml:"personality"`
	Force       map[string]ForceConfig       `toml:"force"`
}

// RivalEndingsTuning is the endings the rivals sim owns (#49, [endings]):
// the run ends kingpin once World.Dominant has held for DominantDays
// days (the day the last faction fell or bowed, read off the table's
// stamps: no counter) with more than KingpinShare of home's corners
// held, taken out the night a faction's push takes the
// last corner you hold anywhere while its war with you is open and the
// enforcers at work are under TakenOutMuscle, and betrayed the night a
// faction breaks a deal with you of its own accord while another's war
// with you is open (BetrayWar, the war line). Zero boxes each.
type RivalEndingsTuning struct {
	DominantDays   int     `toml:"dominant_days"`
	KingpinShare   float64 `toml:"kingpin_share"` // ... holding more than this share of home's corners: the weather can empty a table, a city nobody holds has no kingpin
	ReignGrace     int     `toml:"reign_grace"`   // mornings a reign rides under the share before it breaks (#399); 0 breaks it the first
	TakenOutMuscle int     `toml:"taken_out_muscle"`
	BetrayWar      float64 `toml:"betray_war"`
}

// RivalsTuning is the rival's economy and its fight. The money is priced
// in the ladder's unit (#139): a corner-day, what a standard corner at
// home earns it in a day (margin of the street value the corner moves in
// the products the street there sells), so its costs climb the ladder
// with its take and a price war or a drain is felt whatever the tier.
type RivalsTuning struct {
	ArriveDay          int     `toml:"arrive_day"`
	StartCash          float64 `toml:"start_cash"`   // corner-days it arrives with
	StartMuscle        int     `toml:"start_muscle"` // the heads it arrives with, and never lets go
	Margin             float64 `toml:"margin"`       // of the street value its corners move, as daily income
	MuscleWage         float64 `toml:"muscle_wage"`  // what a corner's guard (muscle_per_corner heads) costs it a day, in corner-days; a head earns that over muscle_per_corner
	MuscleFee          float64 `toml:"muscle_fee"`   // corner-days to recruit a head
	ClaimCost          float64 `toml:"claim_cost"`   // corner-days to set up on a free corner
	SupplierMin        float64 `toml:"supplier_min"`
	SupplierMax        float64 `toml:"supplier_max"`
	PushFlip           float64 `toml:"push_flip"`
	PushWar            float64 `toml:"push_war"`
	UndercutPrice      float64 `toml:"undercut_price"`
	TipHeat            float64 `toml:"tip_heat"`
	ObserveDays        int     `toml:"observe_days"`
	RegroupDays        int     `toml:"regroup_days"`
	WarDecay           float64 `toml:"war_decay"`
	WarThreshold       float64 `toml:"war_threshold"`
	CrackdownThreshold float64 `toml:"crackdown_threshold"`
	CrackdownCorners   int     `toml:"crackdown_corners"`
	CrackdownHeat      float64 `toml:"crackdown_heat"`
	CrackdownMuscle    float64 `toml:"crackdown_muscle"`
	OutbidGrudge       int     `toml:"outbid_grudge"` // grudge a claim it telegraphed and lost to a body on the corner adds (#69)
}

// PaceTuning is how fast the rival takes the city (#60): its claim chance
// scaled by the player's share of home's corners between ClaimScaleMin
// (nothing held) and ClaimScaleMax (all of it), a cooldown after any claim,
// and a grace period after it arrives in which it never sets up on a
// corner the player has ever worked.
type PaceTuning struct {
	ClaimScaleMin float64 `toml:"claim_scale_min"`
	ClaimScaleMax float64 `toml:"claim_scale_max"`
	ClaimCooldown int     `toml:"claim_cooldown"`
	ArriveGrace   int     `toml:"arrive_grace"`
}

// PricewarTuning is the price war (#68): what working a corner cheap next
// to a rival's takes off it, what it costs the player, and how the rival
// answers. The market sim reads the first three (steal, price_cut, glut)
// when it resolves the night's orders at home; the rivals sim the rest.
type PricewarTuning struct {
	Steal        float64 `toml:"steal"`         // share of the rival corner's demand taken at normal by a player holding all its neighbours
	PriceCut     float64 `toml:"price_cut"`     // discount off street price the undercut units sell at
	Glut         float64 `toml:"glut"`          // multiplier on the undercut units' price impact
	War          float64 `toml:"war"`           // war heat per undercut day
	Grudge       bool    `toml:"grudge"`        // a corner starved to the line adds to the rival's grudge
	PricewarDays int     `toml:"pricewar_days"` // days squeezed running before the rival answers
	PricewarPush float64 `toml:"pricewar_push"` // multiplier on push_chance for the answer
}

// BooksTuning is scouting the rival's books (#70): what a look costs and
// the odds it reads them, in the shape of crew.toml's investigation, and
// how long the snapshot stays fresh.
type BooksTuning struct {
	ScoutCost  int     `toml:"scout_cost"`  // dirty cash, then clean
	ScoutBase  float64 `toml:"scout_base"`  // chance a scout reads the books with no enforcer on the payroll
	ScoutSkill float64 `toml:"scout_skill"` // added at enforcer skill 100; scales by the best enforcer's skill
	ScoutLearn float64 `toml:"scout_learn"` // added per scout that read nothing since the last that did
	StaleDays  int     `toml:"stale_days"`  // days after which the snapshot is stale
}

// BoostTuning is the enforcers robbing a rival corner's takings (#70):
// what it takes, what it draws and what it costs the enforcers, at any
// force; the odds are the force's, as a strike's.
type BoostTuning struct {
	Take       float64 `toml:"take"`        // share of the corner's day of income taken
	Heat       float64 `toml:"heat"`        // heat drawn, times the corner's heat
	War        float64 `toml:"war"`         // what it adds to the war
	Loyalty    float64 `toml:"loyalty"`     // the toll on an enforcer with no nerve when it lands
	FailLoss   float64 `toml:"fail_loss"`   // the toll when it fails
	FailMuscle int     `toml:"fail_muscle"` // a failed boost against more muscle than this hurts an enforcer ...
	FailHurt   int     `toml:"fail_hurt"`   // ... by this much skill
}

// TipTuning is tipping the police on a rival corner (#70): what it does
// to the rival's own heat, when the police act on it, and what it costs
// at the table.
type TipTuning struct {
	Heat         float64 `toml:"heat"`          // Rival.Heat per tip
	PushHeat     float64 `toml:"push_heat"`     // Rival.Heat per push it makes while its heat is over zero
	Decay        float64 `toml:"decay"`         // fraction of Rival.Heat that fades a day
	PoliceNotice float64 `toml:"police_notice"` // over this the police take the corner last tipped
	RaidMuscle   float64 `toml:"raid_muscle"`   // fraction of its muscle a raid takes
	RaidDays     int     `toml:"raid_days"`     // days between raids: the heat builds meanwhile
	Trust        float64 `toml:"trust"`         // off Rival.Trust per tip
	Grudge       int     `toml:"grudge"`        // grudge a tip adds
}

// PoachTuning is buying off the rival's muscle (#70): the price of a head
// and the odds the money sends them home.
type PoachTuning struct {
	MusclePrice float64 `toml:"muscle_price"` // corner-days a head costs ...
	CashShare   float64 `toml:"cash_share"`   // ... plus this share of the rival's chest per head it keeps
	Odds        float64 `toml:"odds"`         // chance the order lands
	Grudge      int     `toml:"grudge"`       // grudge a failed order adds
	AwayDays    int     `toml:"away_days"`    // days before a head bought off or arrested is back in the pool it hires from, one head at a time
}

// FactionsTuning is the table (#43): how many factions a run has and
// how they deal with each other and with your crew. Min and Max bound
// the count by seed (1 and 1 is the duel, byte-for-byte the run before
// #43); AbsorbDays is how long a faction stands with no corners before
// the one that took its last absorbs it; AllyTrust is the trust a
// defensive faction gains in whoever the expansionist pushed on, per
// push, AllyLine where it stands with them and AllyShare the share of
// its front-line muscle it lends their pushes; GrudgeTrust is what a
// faction loses in one that took a corner off it; PushHeat is the heat
// on the city a faction-on-faction push draws. PoachCash is the chest,
// in corner-days, over which a faction poaches your crew, PoachMul the
// wages it offers as a multiple of theirs, PoachChance per day it can,
// PoachLine the loyalty under which the member goes and PoachDip what
// one who stayed loses. LeaderArrestHeat is the faction's heat at which
// the police take its leader, FragmentDays how long its corners take
// to drift, ShockMul and ShockDays the price spike on its city's
// products, FragmentDiscount the cut on the fee of its muscle in your
// pool. TributeCorners is what your enforcers take off a faction before
// it offers homage, HomageCut the share of its take it offers and
// HomageChance per day it qualifies.
type FactionsTuning struct {
	Min              int     `toml:"min"`
	Max              int     `toml:"max"`
	Away             float64 `toml:"away"`        // chance a faction after the first lives in the other city (0: the whole table at home)
	AwayMax          int     `toml:"away_max"`    // the most factions that live away from home
	ArriveGap        int     `toml:"arrive_gap"`  // days between one faction's arrival and the next's: the second comes arrive_gap days after arrive_day
	SharedPace       bool    `toml:"shared_pace"` // the factions in a city claim and push on you at the duel's pace between them: each at its chance over their number
	TableShare       float64 `toml:"table_share"` // the most of a city's corners the table holds between them before it stops claiming free ones (0: no cap); past it factions grow only off each other
	AbsorbDays       int     `toml:"absorb_days"`
	AllyTrust        float64 `toml:"ally_trust"`
	AllyLine         float64 `toml:"ally_line"`
	AllyShare        float64 `toml:"ally_share"`
	GrudgeTrust      float64 `toml:"grudge_trust"`
	PushHeat         float64 `toml:"push_heat"`
	PoachCash        float64 `toml:"poach_cash"`
	PoachMul         float64 `toml:"poach_mul"`
	PoachChance      float64 `toml:"poach_chance"`
	PoachLine        float64 `toml:"poach_line"`
	PoachDip         float64 `toml:"poach_dip"`
	LeaderArrestHeat float64 `toml:"leader_arrest_heat"`
	FragmentDays     int     `toml:"fragment_days"`
	ShockMul         float64 `toml:"shock_mul"`
	ShockDays        int     `toml:"shock_days"`
	FragmentDiscount float64 `toml:"fragment_discount"`
	TributeCorners   int     `toml:"tribute_corners"`
	HomageCut        float64 `toml:"homage_cut"`
	HomageChance     float64 `toml:"homage_chance"`
	StrandDays       int     `toml:"strand_days"`       // days a faction stands landless, whatever its chest, or a seat at home stays in the wings past its day, before it scatters (#389; 0: never)
	SuccessionMuscle float64 `toml:"succession_muscle"` // the share of a faction's muscle that walks when the world kills its leader and a successor takes over (#389)
	SettleDays       int     `toml:"settle_days"`       // days a claim must stand before it ends a run-out faction's landless spell: one retaken sooner leaves its clock running from the first rout (#495; 0: any claim restarts it)
}

// ExpansionTuning is the table following the money (#341, [expansion]):
// the rivals sim keeps the player's take a city over the last
// WindowDays days, and a city where no faction lives whose window
// crosses TakeMin draws one: a seat still in the wings, or a cell of
// the strongest faction at home when none is. It scouts (day 0),
// recruits ScoutDays later (Bite of the hiring pool's best faces, the
// city's crew poachable, a tribute offered), and arrives ArriveDays
// after that. A hit on the scouts sets it back SetbackDays, once; a
// window back under TakeMin before it recruits sends it home. Enabled
// false boxes it: the window is still kept (bookkeeping, no dice) and
// nothing is drawn.
type ExpansionTuning struct {
	Enabled     bool `toml:"enabled"`
	TakeMin     int  `toml:"take_min"`     // the take in a city over window_days that draws a faction
	WindowDays  int  `toml:"window_days"`  // the days the take is summed over
	ScoutDays   int  `toml:"scout_days"`   // days from the scouts to the recruiting
	ArriveDays  int  `toml:"arrive_days"`  // days from the recruiting to the arrival
	SetbackDays int  `toml:"setback_days"` // days a hit on the scouts sets it back, once
	Bite        int  `toml:"bite"`         // the hiring pool's best faces it takes the night it recruits
}

type PersonalityConfig struct {
	Trust           float64 `toml:"trust"`        // trust in the player at the start of a run
	DealBias        float64 `toml:"deal_bias"`    // added to the chance it accepts any proposal
	Betrayal        float64 `toml:"betrayal"`     // chance per live deal per day it breaks one itself
	OfferChance     float64 `toml:"offer_chance"` // chance per day it puts a deal on the table when its situation calls for one
	ClaimChance     float64 `toml:"claim_chance"`
	MaxShare        float64 `toml:"max_share"`     // the most of home's corners it sets up on, as a share of the map
	PushPastCap     float64 `toml:"push_past_cap"` // multiplier on push_chance once it holds its share; 0 stops
	Grow            string  `toml:"grow"`          // biggest, adjacent, random
	PushChance      float64 `toml:"push_chance"`
	MusclePerCorner float64 `toml:"muscle_per_corner"`
	Undercut        float64 `toml:"undercut"`
	TipChance       float64 `toml:"tip_chance"`
	Defence         float64 `toml:"defence"`
}

type ForceConfig struct {
	Attack  float64 `toml:"attack"`
	Flip    float64 `toml:"flip"`
	Heat    float64 `toml:"heat"`
	War     float64 `toml:"war"`
	Loyalty float64 `toml:"loyalty"`
	Trust   float64 `toml:"trust"` // what a strike at this force costs the rival's trust in you
}

// DiplomacyTuning is the table: how deals are offered, judged, kept and
// broken. The rival's answer is a chance built from a base per deal kind,
// the terms asked, its trust, its personality and the war, plus what the
// player's reputation adds (reputation.toml [effects]).
type DiplomacyTuning struct {
	OfferDays      int       `toml:"offer_days"`      // days a rival offer stays on the table
	TrustKept      float64   `toml:"trust_kept"`      // trust per day of a live deal
	AcceptTrust    float64   `toml:"accept_trust"`    // added to the chance at trust 100
	AcceptWar      float64   `toml:"accept_war"`      // added to the chance at war 100: a loud war makes peace attractive
	BetrayalFloor  float64   `toml:"betrayal_floor"`  // trust after the player breaks a deal
	DistrustDays   int       `toml:"distrust_days"`   // days after a betrayal the rival takes no deal and offers none
	BetrayalSpread float64   `toml:"betrayal_spread"` // trust every other faction loses (Phase 4)
	JointTrust     float64   `toml:"joint_trust"`     // trust a joint shipment needs (#30)
	TruceDays      []int     `toml:"truce_days"`      // the three lengths a truce can be proposed at
	TributeCuts    []float64 `toml:"tribute_cuts"`    // the three cuts of the player's daily street value in what the rival sells (rivals.Sim.TributeBase) a tribute can be
	TributeMin     int       `toml:"tribute_min"`     // a tribute is never under this a day
	LowCashDays    int       `toml:"low_cash_days"`   // an expansionist that cannot pay its muscle this long offers a truce
	UpperHand      float64   `toml:"upper_hand"`      // an opportunist with this many times the muscle on the front line demands tribute
	SplitFair      float64   `toml:"split_fair"`      // share of the city's demand the rival lets the player's side of a split have at trust 0 ...
	SplitTrust     float64   `toml:"split_trust"`     // ... plus this much at trust 100
	SplitTerms     float64   `toml:"split_terms"`     // the split's terms are (fair - ask) times this, clamped to -1..1 (#275)
}

// WeightTuning is rivals.toml [weight] (#275): what a body on the street
// weighs. Strength counts an enforcer at work StrengthBase + skill /
// StrengthSkill, over Posted where he guards a corner; Guard counts the
// enforcer on a pushed corner GuardEnforcer + skill / GuardSkill, the
// player working it GuardYou and a runner GuardRunner. The fields stand
// where the literals stood, divisors as divisors, so every weight is
// the float it was.
type WeightTuning struct {
	StrengthBase  float64 `toml:"strength_base"`
	StrengthSkill float64 `toml:"strength_skill"`
	Posted        float64 `toml:"posted"`
	GuardEnforcer float64 `toml:"guard_enforcer"`
	GuardSkill    float64 `toml:"guard_skill"`
	GuardYou      float64 `toml:"guard_you"`
	GuardRunner   float64 `toml:"guard_runner"`
}

// DealConfig is what the rival thinks of one deal kind: the base chance
// it accepts, and how much the terms move it (the easy option adds terms,
// the hard one takes it away).
type DealConfig struct {
	Base  float64 `toml:"base"`
	Terms float64 `toml:"terms"`
}

// Personalities are the rival personalities in a fixed order, so a pick
// by seed is reproducible.
var Personalities = []string{"expansionist", "defensive", "opportunist", "chaotic"}

// ForceFor returns the tuning for a force dial position.
func (r RivalsConfig) ForceFor(f events.Force) ForceConfig { return r.Force[f.String()] }

// WarTuning is rivals.toml's [war] (#229): the dial the war order's
// strikes go in at, warn, push or hit; anything else boxes the war
// (harness.NoWar), so a run that never declares one is the run before.
type WarTuning struct {
	Dial string `toml:"dial"`
}

// Force is the dial as events.Force, and whether the table is on.
func (w WarTuning) Force() (events.Force, bool) { return events.ParseForce(w.Dial) }

// DealKinds are the deals that can be proposed, in the order the UI
// lists them. The joint shipment waits on routes (#30).
var DealKinds = []string{"truce", "tribute", "split"}

// validate checks the rivals file reads as one: a [personality.X] table
// per personality and a [force.X] per force the sims index directly,
// then the diplomacy table: a [deal.X] table per kind, three options for
// the truce and the tribute, and sane days.
func (r RivalsConfig) validate() error {
	for _, p := range Personalities {
		if _, ok := r.Personality[p]; !ok {
			return fmt.Errorf("no [personality.%s] table", p)
		}
	}
	for _, f := range events.ForceNames() {
		if _, ok := r.Force[f]; !ok {
			return fmt.Errorf("no [force.%s] table", f)
		}
	}
	d := r.Diplomacy
	for _, k := range DealKinds {
		if _, ok := r.Deal[k]; !ok {
			return fmt.Errorf("no [deal.%s] table", k)
		}
	}
	if len(d.TruceDays) != 3 || len(d.TributeCuts) != 3 {
		return fmt.Errorf("truce_days and tribute_cuts need three options each, got %d and %d", len(d.TruceDays), len(d.TributeCuts))
	}
	if d.OfferDays < 1 || d.DistrustDays < 1 {
		return fmt.Errorf("offer_days %d and distrust_days %d must be positive", d.OfferDays, d.DistrustDays)
	}
	if d.SplitTerms <= 0 {
		return fmt.Errorf("[diplomacy] split_terms %.2f must be positive: a fairer ask is an easier one", d.SplitTerms)
	}
	// The weights (#275): every body weighs something, and the divisors
	// divide.
	if wt := r.Weight; wt.StrengthBase <= 0 || wt.StrengthSkill <= 0 || wt.Posted <= 0 || wt.GuardEnforcer <= 0 || wt.GuardSkill <= 0 || wt.GuardYou <= 0 || wt.GuardRunner <= 0 {
		return fmt.Errorf("[weight] every weight and divisor must be positive: %+v", wt)
	}
	if t := r.Rivals; t.Margin <= 0 || t.MuscleWage <= 0 {
		return fmt.Errorf("margin %.2f and muscle_wage %.2f must be positive", t.Margin, t.MuscleWage)
	}
	for name, p := range r.Personality {
		if p.MusclePerCorner <= 0 {
			return fmt.Errorf("[personality.%s] muscle_per_corner %.2f must be positive: the wage per head is muscle_wage over it", name, p.MusclePerCorner)
		}
	}
	// The books (#70): a boost takes a share, a tip's notice line is
	// reachable, a head has a price and the snapshot goes stale.
	if b := r.Boost; b.Take < 0 || b.Take > 1 {
		return fmt.Errorf("[boost] take %.2f must be in 0..1", b.Take)
	}
	if tp := r.Tip; tp.PoliceNotice <= 0 || tp.PoliceNotice > 100 || tp.Heat <= 0 || tp.RaidMuscle < 0 || tp.RaidMuscle > 1 {
		return fmt.Errorf("[tip] police_notice %.0f must be in 1..100, heat %.0f positive and raid_muscle %.2f in 0..1", tp.PoliceNotice, tp.Heat, tp.RaidMuscle)
	}
	if p := r.Poach; p.MusclePrice <= 0 || p.Odds < 0 || p.Odds > 1 {
		return fmt.Errorf("[poach] muscle_price %.2f must be positive and odds %.2f in 0..1", p.MusclePrice, p.Odds)
	}
	if r.Books.StaleDays < 1 || r.Books.ScoutCost < 0 {
		return fmt.Errorf("[books] stale_days %d must be positive and scout_cost %d not negative", r.Books.StaleDays, r.Books.ScoutCost)
	}
	// The table (#43): a count, a line the police act at, a drift.
	if f := r.Factions; f.Min < 1 || f.Max < f.Min {
		return fmt.Errorf("[factions] min %d must be positive and max %d at least min", f.Min, f.Max)
	}
	if e := r.Expansion; e.Enabled && (e.TakeMin <= 0 || e.WindowDays < 1 || e.ScoutDays < 1 || e.ArriveDays < 1 || e.SetbackDays < 0 || e.Bite < 0) {
		return fmt.Errorf("[expansion] take_min %d, window_days %d, scout_days %d and arrive_days %d must be positive, setback_days %d and bite %d not negative", e.TakeMin, e.WindowDays, e.ScoutDays, e.ArriveDays, e.SetbackDays, e.Bite)
	}
	if f := r.Factions; f.SettleDays < 0 {
		return fmt.Errorf("[factions] settle_days %d must be 0 or more", f.SettleDays)
	}
	if f := r.Factions; f.StrandDays < 0 || (f.StrandDays > 0 && f.StrandDays < f.AbsorbDays) || f.SuccessionMuscle < 0 || f.SuccessionMuscle > 1 {
		return fmt.Errorf("[factions] strand_days %d must be 0 or at least absorb_days %d, succession_muscle %.2f in 0..1", f.StrandDays, f.AbsorbDays, f.SuccessionMuscle)
	}
	if f := r.Factions; f.LeaderArrestHeat <= 0 || f.LeaderArrestHeat > 100 || f.FragmentDays < 1 || f.AbsorbDays < 1 {
		return fmt.Errorf("[factions] leader_arrest_heat %.0f must be in 1..100, fragment_days %d and absorb_days %d positive", f.LeaderArrestHeat, f.FragmentDays, f.AbsorbDays)
	}
	return nil
}
