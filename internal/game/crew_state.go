package game

import "github.com/theclifmeister/kingpin/internal/events"

// CrewState is the player's crew: the roster, the hiring pool and the pay
// dial. HiredToday, FiredToday and PaidOffToday are per-day scratch the
// clock clears.
type CrewState struct {
	Members      []CrewMember
	Candidates   []CrewMember
	Pay          events.Pay
	NextID       int
	PoolDay      int // day the candidate pool last rotated
	LastSkim     int // day skimming was last reported; 0 means never
	Exposed      int // member an investigation named as the informant; 0 nobody (they may be gone)
	Investigated int // investigations that found nobody since the last that did; each makes the next more likely to
	HiredToday   []CrewMember
	FiredToday   []CrewMember
	PaidOffToday []Payoff
	Offered      map[string]bool // roles announced as looking for work (#148): accountant, lieutenant, chemist; nil is none
	Leads        []Lead          // who went over to the rival last night, for the rivals sim to act on next step (#144); the crew sim writes it fresh every step and nothing else writes it
	Cooks        []Cook          // the chemist's lots on their way (#47), in the order ordered; Cook queues them, the crew sim lands them
	NextCook     int             // the last cook's id
	BailedToday  []Payoff        // bail put down today (#46), per-day scratch the clock clears; the crew sim reports it
	Fallen       []Fallen        // the crew shot dead on your corners (#46), oldest first: the run summary reads it
	Named        []string        // every name on the payroll this run, in the order hired (#425): the pool never deals one again while it has others; nil before the field
}

// Fallen is a member of the crew shot dead on a corner (#46): who, and
// where and when they fell.
type Fallen struct {
	ID         int
	Name       string
	Role       string
	Age        int
	Day        int
	Corner     string
	CornerName string
}

// The crew's roles, as crew.toml's [role.*] tables name them (#274):
// runners sell, enforcers guard and strike, accountants wash through the
// fronts. The specialists' roles are declared beside their helpers.
const (
	RoleRunner     = "runner"
	RoleEnforcer   = "enforcer"
	RoleAccountant = "accountant"
)

// RoleChemist is the role of the crew member who makes quality (#47):
// cuts keep more with one on the payroll, and only they cook.
const RoleChemist = "chemist"

// Chemist is the best chemist on the payroll, or nil.
func (c *CrewState) Chemist() *CrewMember {
	return c.Best(RoleChemist)
}

// Cooking is what is on its way to a city's stash of a product (#47).
func (c CrewState) Cooking(city, product string) int {
	n := 0
	for _, k := range c.Cooks {
		if k.City == city && k.Product == product {
			n += k.Units
		}
	}
	return n
}

// CrewMember is one person on the payroll (or in the hiring pool). Stats are
// 0..100. Units, Wage and Fee are fixed when the candidate is generated so
// the world never needs crew tuning to price them. Informant is the hidden
// flag: the roster never shows it, the report does, in its own way. A
// lieutenant has a Personality (violent, greedy, careful, steady), fixed
// when they are generated and hidden until Observed, and runs the City
// they are assigned to.
type CrewMember struct {
	ID          int
	Name        string
	Role        string // runner, enforcer, accountant, lieutenant, chemist
	Skill       int
	Loyalty     float64
	Greed       int
	Nerve       int
	Units       int            // sell capacity this member adds
	Wage        int            // daily wage at fair pay
	Fee         int            // signing fee
	Hired       int            // day hired
	Informant   bool           // talking to the police; only firing them stops it
	Personality string         // a lieutenant's: violent, greedy, careful, steady
	City        string         // the city a lieutenant runs; empty when unassigned
	Assigned    int            // day the lieutenant was last given a city
	Stickups    map[string]int // a lieutenant's: each corner's Robbed in their city the day they took it (#497), so robbed_off counts only theirs; nil is none (the pre-#497 member)
	Observed    bool           // the lieutenant has been on the job long enough for the report to name their personality
	Former      string         // the faction a candidate in the pool used to run with (#43): a fragmented faction's muscle, at a discount; "" for anyone else

	// Crew life (#46). Age is years, seeded at generation (MigrateAges
	// for a save from before it); Growth is the skill on its way, the
	// fraction under a point. Kin are the ids of the cousin, partner or
	// friend on the payroll or in the pool who remembers what you do to
	// this one. JailedUntil and WoundedUntil are the day they are back:
	// jailed or wounded on every day before it, off the corner and
	// selling and guarding nothing. Bailed says the release tomorrow is
	// one you paid for. Zero values are the pre-#46 member.
	Age          int
	Growth       float64
	Kin          []int
	JailedUntil  int
	WoundedUntil int
	Bailed       bool

	// Intel (#45): Undercover is the faction a spy is under with ("" is
	// nobody, the member at work), UndercoverDay the day they went, the
	// clock their reports run on. A spy works nothing for you and is
	// off the roster's counts until they come back.
	Undercover    string
	UndercoverDay int

	// Veterans (#346). Trait is what they showed at crew.toml [traits]
	// days of service, "" before then or with the table boxed; Lived is
	// what they lived through, each of raid, shot and jail once, the
	// words the trait's draw weighs (written only while traits are on).
	// Captain is the city they run crew care for ("" nobody's), and
	// Budget the pay-off budget a night they have there. Zero values
	// are the pre-#346 member.
	Trait   string
	Lived   []string
	Captain string
	Budget  int
}

// StickupsSince is a corner's stick-ups since the lieutenant took its
// city (#497): its Robbed less the count it had then, or all of it
// once the territory sim has forgotten the old ones.
func (m CrewMember) StickupsSince(c Corner) int {
	if b := m.Stickups[c.ID]; b <= c.Robbed {
		return c.Robbed - b
	}
	return c.Robbed
}

// HasLived reports whether the member lived through word (#346).
func (m CrewMember) HasLived(word string) bool {
	for _, l := range m.Lived {
		if l == word {
			return true
		}
	}
	return false
}

// Captain returns the member running crew care for a city (#346), or
// nil.
func (c *CrewState) Captain(city string) *CrewMember {
	if city == "" {
		return nil
	}
	for i := range c.Members {
		if m := &c.Members[i]; m.Captain == city {
			return m
		}
	}
	return nil
}

// Jailed reports whether the member is in a cell on day.
func (m CrewMember) Jailed(day int) bool { return m.JailedUntil > day }

// Wounded reports whether the member is laid up on day.
func (m CrewMember) Wounded(day int) bool { return m.WoundedUntil > day }

// Fit reports whether the member can work on day: neither jailed nor
// wounded.
func (m CrewMember) Fit(day int) bool { return !m.Jailed(day) && !m.Wounded(day) && m.Undercover == "" }

// Working reports whether the member is at work at all: the crew sim
// clears JailedUntil and WoundedUntil the morning they are back, so
// either set is a member in a cell or laid up whatever the day, which
// is how the best chemist or fixer is chosen without one.
func (m CrewMember) Working() bool {
	return m.JailedUntil == 0 && m.WoundedUntil == 0 && m.Undercover == ""
}

// IsKin reports whether the two are kin (#46).
func (m CrewMember) IsKin(id int) bool {
	for _, k := range m.Kin {
		if k == id {
			return true
		}
	}
	return false
}

// RoleDriver is the crew member who rides a route's shipments (#46).
const RoleDriver = "driver"

// Driver returns the member with id if they are a driver on the
// payroll, or nil.
func (c *CrewState) Driver(id int) *CrewMember {
	if m := c.Member(id); m != nil && m.Role == RoleDriver {
		return m
	}
	return nil
}

// Lieutenant reports whether the member is a lieutenant.
func (m CrewMember) Lieutenant() bool { return m.Role == RoleLieutenant }

// Runs reports whether the member is a lieutenant running a city.
func (m CrewMember) Runs() bool { return m.Lieutenant() && m.City != "" }

// RoleLieutenant is the role of a crew member who runs a city for the
// player.
const RoleLieutenant = "lieutenant"

// Informants counts the members talking to the police.
func (c CrewState) Informants() int {
	n := 0
	for _, m := range c.Members {
		if m.Informant {
			n++
		}
	}
	return n
}

// Payoff is a member paid to stay loyal today, for the report.
type Payoff struct {
	ID    int
	Name  string
	Cost  int
	Clean int // of Cost, what came out of the clean pile (#351), for the report's flow
}

// InvestigationOrder is the player asking questions of the crew tonight,
// paid up front and resolved by the crew sim at end of day.
type InvestigationOrder struct {
	Cost  int
	Clean int // of Cost, what came out of the clean pile (#351), for the report's flow
}

// Runners counts the runners at work.
func (c CrewState) Runners() int { return c.Role(RoleRunner) }

// Role counts the members of a role at work: on the payroll and neither
// in a cell nor laid up (#46, Working), so an enforcer in a cell
// deters nobody, goes on no strike and guards nothing. OnPayroll
// counts them all.
func (c CrewState) Role(role string) int {
	n := 0
	for _, m := range c.Members {
		if m.Role == role && m.Working() {
			n++
		}
	}
	return n
}

// OnPayroll counts the members of a role, at work or not.
func (c CrewState) OnPayroll(role string) int {
	n := 0
	for _, m := range c.Members {
		if m.Role == role {
			n++
		}
	}
	return n
}

// Member returns the roster entry with id, or nil.
func (c *CrewState) Member(id int) *CrewMember {
	return find(c.Members, func(e *CrewMember) bool { return e.ID == id })
}

// Best is the most skilled member of a role at work (not jailed, wounded
// or undercover), or nil; the first on the roster wins a tie (#275). The
// chemist and the fixer are Best of their role.
func (c *CrewState) Best(role string) *CrewMember {
	return c.top(role, true)
}

// Strongest is the most skilled member of a role on the books, at work or
// not, or nil; the first on the roster wins a tie (#275). The odds that
// read the best enforcer (the investigation, the scout) have always
// counted a jailed or wounded one: Best would move them.
func (c *CrewState) Strongest(role string) *CrewMember {
	return c.top(role, false)
}

func (c *CrewState) top(role string, working bool) *CrewMember {
	var best *CrewMember
	for i := range c.Members {
		if m := &c.Members[i]; m.Role == role && (!working || m.Working()) && (best == nil || m.Skill > best.Skill) {
			best = m
		}
	}
	return best
}

// Lieutenant returns the member running a city, or nil.
func (c *CrewState) Lieutenant(city string) *CrewMember {
	for i := range c.Members {
		if m := &c.Members[i]; m.Runs() && m.City == city {
			return m
		}
	}
	return nil
}

// RoleFixer is the crew member who knows who takes an envelope (#42).
const RoleFixer = "fixer"

// Fixer returns the most skilled fixer on the payroll, or nil: the one
// whose word the law sim weighs on a bribe.
func (c *CrewState) Fixer() *CrewMember {
	return c.Best(RoleFixer)
}

// Lieutenants counts the members running a city.
func (c CrewState) Lieutenants() int {
	n := 0
	for _, m := range c.Members {
		if m.Runs() {
			n++
		}
	}
	return n
}
