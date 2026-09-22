package content

import (
	"fmt"
	"slices"
)

// CharactersConfig mirrors characters.toml (#50): the starts a run can
// be given and the one toggle beside them. A character is a start and
// nothing more (docs/profile.md): applied once by sim.NewWorldWith,
// read by no sim. The first row is the default, the run as it is.
type CharactersConfig struct {
	Characters []CharacterConfig `toml:"character"`
	HardDA     ToggleConfig      `toml:"hard_da"`
}

// CharacterConfig is one character: its id, the name and blurb the
// picker prints, the rule that unlocks it and its start.
type CharacterConfig struct {
	ID     string      `toml:"id"`
	Name   string      `toml:"name"`
	Blurb  string      `toml:"blurb"`
	Unlock UnlockRule  `toml:"unlock"`
	Start  StartConfig `toml:"start"`
}

// ToggleConfig is a start that is not a character (hard_da): its name,
// blurb and rule.
type ToggleConfig struct {
	Name   string     `toml:"name"`
	Blurb  string     `toml:"blurb"`
	Unlock UnlockRule `toml:"unlock"`
}

// UnlockRule is when the profile opens a character or a toggle: from
// the first run (Always), on a run that ended by Ending (a cause of
// endings.toml), or on a run that reached Stage (an id of
// progression.toml) or one past it. Exactly one is set.
type UnlockRule struct {
	Always bool   `toml:"always"`
	Ending string `toml:"ending"`
	Stage  string `toml:"stage"`
}

// Free reports whether the rule opens from the first run.
func (r UnlockRule) Free() bool { return r.Always }

// StartConfig is what a character puts on the world on day 0: crew on
// the payroll by role, with a name from the pool's lists and their
// stats off the character's own stream; nodes of the tree owned;
// products on the ladder in every city; the city and corner you stand
// on (home's starting corner given back to the street); the reputation
// you arrive with; and, with KnowChief, the chief's temper in the file.
type StartConfig struct {
	Crew       []string        `toml:"crew"`
	Upgrades   []string        `toml:"upgrades"`
	Products   []string        `toml:"products"`
	City       string          `toml:"city"`
	Corner     string          `toml:"corner"`
	Corners    []string        `toml:"corners"` // corners held on day 0 with the start crew posted on them, in order (#232)
	Reputation StartReputation `toml:"reputation"`
	KnowChief  bool            `toml:"know_chief"`
}

// StartReputation is the three axes a character arrives with, 0..100.
type StartReputation struct {
	Fear      float64 `toml:"fear"`
	Respect   float64 `toml:"respect"`
	Notoriety float64 `toml:"notoriety"`
}

// Empty reports whether the start changes nothing: the default's.
func (s StartConfig) Empty() bool {
	return len(s.Crew) == 0 && len(s.Upgrades) == 0 && len(s.Products) == 0 && s.City == "" && s.Corner == "" && len(s.Corners) == 0 && s.Reputation == StartReputation{} && !s.KnowChief
}

// Character returns the row with id, or nil.
func (c CharactersConfig) Character(id string) *CharacterConfig {
	return find(c.Characters, func(e *CharacterConfig) bool { return e.ID == id })
}

// Default is the first row: the run as it is. An empty id reads as it.
func (c CharactersConfig) Default() *CharacterConfig {
	if len(c.Characters) == 0 {
		return nil
	}
	return &c.Characters[0]
}

// IsDefault reports whether id is the default character: the first
// row's id, or empty.
func (c CharactersConfig) IsDefault(id string) bool {
	return id == "" || (c.Default() != nil && c.Default().ID == id)
}

// validate checks the file hangs together: ids unique and named, the
// first row unlocked always with an empty start (it is the run as it
// is), every rule exactly one of its three with a known cause or
// stage, every role, node, product, city and corner a thing the other
// files have, a corner named with its city and in it, and the
// reputation in 0..100.
func (c CharactersConfig) validate(crew CrewConfig, up UpgradesConfig, mk MarketConfig, city CityConfig, prog ProgressionConfig) error {
	if len(c.Characters) == 0 {
		return fmt.Errorf("no characters")
	}
	seen := map[string]bool{}
	for i, ch := range c.Characters {
		if ch.ID == "" || ch.Name == "" || ch.Blurb == "" {
			return fmt.Errorf("character %d needs an id, a name and a blurb", i)
		}
		if seen[ch.ID] {
			return fmt.Errorf("character %q is defined twice", ch.ID)
		}
		seen[ch.ID] = true
		if err := ch.Unlock.validate(prog); err != nil {
			return fmt.Errorf("character %q: %w", ch.ID, err)
		}
		if i == 0 && (!ch.Unlock.Always || !ch.Start.Empty()) {
			return fmt.Errorf("character %q is the first row, the default: always unlocked, no start", ch.ID)
		}
		s := ch.Start
		for _, role := range s.Crew {
			if _, ok := crew.Role[role]; !ok {
				return fmt.Errorf("character %q: unknown crew role %q", ch.ID, role)
			}
		}
		for _, id := range s.Upgrades {
			if up.Upgrade(id) == nil {
				return fmt.Errorf("character %q: unknown upgrade %q", ch.ID, id)
			}
		}
		for _, id := range s.Products {
			if mk.Product(id) == nil {
				return fmt.Errorf("character %q: unknown product %q", ch.ID, id)
			}
		}
		if (s.City == "") != (s.Corner == "") {
			return fmt.Errorf("character %q: a city and a corner go together", ch.ID)
		}
		if s.City != "" {
			cc := city.City(s.City)
			if cc == nil {
				return fmt.Errorf("character %q: unknown city %q", ch.ID, s.City)
			}
			found := false
			for _, k := range cc.Corners {
				found = found || k.ID == s.Corner
			}
			if !found {
				return fmt.Errorf("character %q: no corner %q in %s", ch.ID, s.Corner, s.City)
			}
		}
		// The corners held on day 0 (#232): in the start city (home
		// with none named), each once, and free: not the corner you
		// stand on, the character's or the file's start.
		startCity, standing := s.City, s.Corner
		if startCity == "" {
			startCity, standing = city.Home().ID, city.Territory.Start
		}
		held := map[string]bool{}
		for _, id := range s.Corners {
			if held[id] {
				return fmt.Errorf("character %q: corner %q held twice", ch.ID, id)
			}
			held[id] = true
			if id == standing {
				return fmt.Errorf("character %q: corner %q is the one you stand on", ch.ID, id)
			}
			found := false
			for _, k := range city.City(startCity).Corners {
				found = found || k.ID == id
			}
			if !found {
				return fmt.Errorf("character %q: no corner %q in %s", ch.ID, id, startCity)
			}
		}
		for _, v := range []float64{s.Reputation.Fear, s.Reputation.Respect, s.Reputation.Notoriety} {
			if v < 0 || v > 100 {
				return fmt.Errorf("character %q: reputation %v outside 0..100", ch.ID, v)
			}
		}
	}
	if c.HardDA.Name == "" || c.HardDA.Blurb == "" {
		return fmt.Errorf("[hard_da] needs a name and a blurb")
	}
	if err := c.HardDA.Unlock.validate(prog); err != nil {
		return fmt.Errorf("[hard_da]: %w", err)
	}
	return nil
}

// validate checks a rule is exactly one thing and names a cause or a
// stage the files have.
func (r UnlockRule) validate(prog ProgressionConfig) error {
	n := 0
	if r.Always {
		n++
	}
	if r.Ending != "" {
		n++
		if !slices.Contains(Causes, r.Ending) {
			return fmt.Errorf("unknown ending %q", r.Ending)
		}
	}
	if r.Stage != "" {
		n++
		if prog.Index(r.Stage) < 0 {
			return fmt.Errorf("unknown stage %q", r.Stage)
		}
	}
	if n != 1 {
		return fmt.Errorf("an unlock is exactly one of always, ending or stage: %+v", r)
	}
	return nil
}

// Index is the tier's place in the ladder (0 the first), or -1 for an
// id the file lacks.
func (p ProgressionConfig) Index(id string) int {
	for i, t := range p.Tiers {
		if t.ID == id {
			return i
		}
	}
	return -1
}
