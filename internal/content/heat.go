package content

import "fmt"

// HeatConfig mirrors heat.toml.
type HeatConfig struct {
	Heat          HeatTuning          `toml:"heat"`
	Investigation InvestigationTuning `toml:"investigation"`
	Responses     []ResponseConfig    `toml:"response"`
}

// InvestigationTuning is the targeted investigation (#343): with it on,
// the police who would send a blind sting open an investigation on the
// biggest source of the city's heat (a corner, a product, a house) and
// the sting lands on that target alone, LeadDays later. Off, the sting
// is the blind one it always was and nothing is tallied.
type InvestigationTuning struct {
	Enabled        bool    `toml:"enabled"`
	WindowDays     int     `toml:"window_days"`      // the tally of heat by source forgets 1/window_days of itself a day
	LeadDays       int     `toml:"lead_days"`        // the nights between the investigation opening and the hit
	ClosedHeatDrop float64 `toml:"closed_heat_drop"` // the city's heat a hit that found its target takes off: the sacrifice pays
	Evidence       int     `toml:"evidence"`         // the pages a hit on a target in use files; 0 is the sting's
}

// On reports whether investigations open at all.
func (i InvestigationTuning) On() bool { return i.Enabled && i.WindowDays > 0 && i.LeadDays > 0 }

type HeatTuning struct {
	Decay              float64 `toml:"decay"`
	LieLowMultiplier   float64 `toml:"lie_low_multiplier"`
	SaleHeat           float64 `toml:"sale_heat"`
	StreetUnits        float64 `toml:"street_units"`
	DirtyCashThreshold int     `toml:"dirty_cash_threshold"`
	DirtyCashHeat      float64 `toml:"dirty_cash_heat"`
	DirtyCashCover     float64 `toml:"dirty_cash_cover"` // a front covers this many times its price of the pile
	CooldownDays       int     `toml:"cooldown_days"`
	EvidenceArrest     int     `toml:"evidence_arrest"`
	CrewHeat           float64 `toml:"crew_heat"`
	InformantDays      int     `toml:"informant_days"`
	InformantEvidence  int     `toml:"informant_evidence"`
	InformantHeat      float64 `toml:"informant_heat"`
	TipEvidence        float64 `toml:"tip_evidence"` // chance a tip on a rival corner (#70) files a page anyway
	SloppySkill        int     `toml:"sloppy_skill"`
	SloppyHeat         float64 `toml:"sloppy_heat"`
	AuditHeat          float64 `toml:"audit_heat"`         // heat an audited front adds the morning after
	AuditEvidence      int     `toml:"audit_evidence"`     // evidence an audit adds when the front was run greedy
	StructureEvidence  int     `toml:"structure_evidence"` // pages per lot of clean cash moved offshore over the lot in a day (#195)
	TaskforceCash      int     `toml:"taskforce_cash"`     // dirty cash over which the task force can form with no asset owned (#48); 0 is never on cash alone
	BustDays           int     `toml:"bust_days"`          // days a bust that took product stays on the record the connects read (#72)
	FallHeat           float64 `toml:"fall_heat"`          // the fall guy (#49): every city's heat is capped here once he takes the case ...
	FallCash           float64 `toml:"fall_cash"`          // ... and this share of the dirty cash and of the clean goes on making it stick
}

// The response ladder's levels (#144): the names heat.toml's
// [[response]] tables carry, the heat sim dispatches on, an
// Enforcement event reports and every reader compares against. They
// are constants so that no copy of the ladder is a string literal;
// Levels is the ladder in order, for a check that the file has every
// rung and a walk that wants them ranked. A patrol caps the street; a
// sting and a raid take stock and cash and, on a day you dealt, file a
// page; a task force (#48) forms only against an asset or a pile over
// taskforce_cash, is announced a day ahead and seizes an asset; an
// arrest ends the run unless a fall guy takes it.
const (
	Patrol    = "patrol"
	Sting     = "sting"
	Raid      = "raid"
	TaskForce = "taskforce"
	Arrest    = "arrest"
)

// Levels is the response ladder from the lightest touch to the end of
// the run, in the order the thresholds climb.
var Levels = []string{Patrol, Sting, Raid, TaskForce, Arrest}

// Rank is a level's place on the ladder, 1 for a patrol to 5 for an
// arrest; 0 for a name that is not a level.
func Rank(level string) int {
	for i, l := range Levels {
		if l == level {
			return i + 1
		}
	}
	return 0
}

// ResponseConfig is one rung of the ladder: the heat a city reaches for
// its police to answer at this level, and what the answer does.
type ResponseConfig struct {
	Level     string  `toml:"level"`
	Threshold float64 `toml:"threshold"`
	CapDays   int     `toml:"cap_days"`
	Cap       float64 `toml:"cap"`
	StockLoss float64 `toml:"stock_loss"`
	CashLoss  float64 `toml:"cash_loss"`
	HeatDrop  float64 `toml:"heat_drop"`
	Evidence  int     `toml:"evidence"`
	Cooldown  int     `toml:"cooldown_days"` // this rung's own cooldown, flat (#48: the task force's is long and federal, no chief's to shorten); 0 is [heat] cooldown_days
}

// validate checks the ladder reads as one: every rung named once, no
// rung the code does not know, and the thresholds climbing in the
// ladder's order.
func (h HeatConfig) validate() error {
	if t := h.Heat; t.BustDays < 1 || t.FallHeat < 0 || t.FallHeat > 100 || t.FallCash < 0 || t.FallCash > 1 {
		return fmt.Errorf("[heat] bust_days %d must be positive, fall_heat %.0f in 0..100 and fall_cash %.2f in 0..1", t.BustDays, t.FallHeat, t.FallCash)
	}
	if i := h.Investigation; i.Enabled && (i.WindowDays < 1 || i.LeadDays < 1 || i.ClosedHeatDrop < 0 || i.Evidence < 0) {
		return fmt.Errorf("[investigation] window_days %d and lead_days %d must be positive, closed_heat_drop %.1f and evidence %d not negative", i.WindowDays, i.LeadDays, i.ClosedHeatDrop, i.Evidence)
	}
	at := map[string]float64{}
	for _, r := range h.Responses {
		if Rank(r.Level) == 0 {
			return fmt.Errorf("response %q is not a level %v", r.Level, Levels)
		}
		if _, dup := at[r.Level]; dup {
			return fmt.Errorf("response %q listed twice", r.Level)
		}
		at[r.Level] = r.Threshold
	}
	for i, l := range Levels {
		if _, ok := at[l]; !ok {
			return fmt.Errorf("no [[response]] for %s", l)
		}
		if i > 0 && at[l] <= at[Levels[i-1]] {
			return fmt.Errorf("%s fires at %.0f, not above %s at %.0f", l, at[l], Levels[i-1], at[Levels[i-1]])
		}
	}
	return nil
}
