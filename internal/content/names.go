package content

import "fmt"

// NamesConfig mirrors names.toml.
type NamesConfig struct {
	Crew        []string `toml:"crew"`
	Rivals      []string `toml:"rivals"`
	Chiefs      []string `toml:"chiefs"`
	DAs         []string `toml:"das"`
	Celebrities []string `toml:"celebrities"` // who an incident names (#44): the star who overdosed
	Reporters   []string `toml:"reporters"`   // ... and the byline on the profile
	Chemists    []string `toml:"chemists"`    // the chemist's names (#47), a pool of their own so the crew's roll as it did
	Drivers     []string `toml:"drivers"`     // the driver's names (#46), the same pattern
}

// Pool is a name pool by the name an incident's `names` field gives it
// (#44): celebrities, reporters, chiefs, das, rivals; nil for one the
// file does not have.
func (n NamesConfig) Pool(name string) []string {
	switch name {
	case "celebrities":
		return n.Celebrities
	case "reporters":
		return n.Reporters
	case "chiefs":
		return n.Chiefs
	case "das":
		return n.DAs
	case "rivals":
		return n.Rivals
	}
	return nil
}

// validate checks every pool a sim draws from has names in it. The crew
// roster can grow past max_crew by a lieutenant's people, and every city
// can have one; the pool of crew names has to cover that and the
// candidates on top, so it reads the crew and the city files.
func (n NamesConfig) validate(crew CrewConfig, city CityConfig) error {
	roster := crew.Crew.MaxCrew + len(city.Cities)*crew.Role["lieutenant"].Crew
	if len(n.Crew) < roster+crew.Crew.Candidates {
		return fmt.Errorf("only %d crew names", len(n.Crew))
	}
	if len(n.Rivals) == 0 {
		return fmt.Errorf("no rival names")
	}
	if len(n.Chiefs) == 0 || len(n.DAs) == 0 {
		return fmt.Errorf("no chief or DA names")
	}
	if len(n.Chemists) == 0 {
		return fmt.Errorf("no chemist names")
	}
	if len(n.Drivers) == 0 {
		return fmt.Errorf("no driver names")
	}
	return nil
}

// validateApart checks a rival leader never shares a name with a
// connect or anyone the crew pools can deal (#425): a playtest had the
// rival Cass taking corners while the connect Cass sold at 55% of
// street, and every line naming Cass read both ways.
func (n NamesConfig) validateApart(sup SuppliersConfig) error {
	taken := map[string]string{}
	for _, s := range sup.Deck {
		taken[s.Name] = "a connect"
	}
	for _, pool := range [][]string{n.Crew, n.Chemists, n.Drivers} {
		for _, name := range pool {
			taken[name] = "a crew name"
		}
	}
	for _, r := range n.Rivals {
		if what, ok := taken[r]; ok {
			return fmt.Errorf("rival %q is also %s", r, what)
		}
	}
	return nil
}
