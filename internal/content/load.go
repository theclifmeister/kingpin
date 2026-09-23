// Package content loads the tuning data (markets, cities, routes, heat,
// crew, rivals, upgrades, laundering, reputation, law, headlines, dilemmas) that
// lives in TOML files embedded in the binary. Balance changes never need
// code changes.
package content

import (
	"embed"
	"fmt"

	"github.com/BurntSushi/toml"
)

//go:embed *.toml
var files embed.FS

// Config is everything the simulations need to be constructed.
type Config struct {
	Market      MarketConfig
	City        CityConfig
	Routes      RoutesConfig
	Heat        HeatConfig
	Crew        CrewConfig
	Rivals      RivalsConfig
	Laundering  LaunderingConfig
	Names       NamesConfig
	Upgrades    UpgradesConfig
	Reputation  ReputationConfig
	Law         LawConfig
	Headlines   HeadlinesConfig
	Dilemmas    DilemmasConfig
	Buyers      BuyersConfig
	Suppliers   SuppliersConfig
	Progression ProgressionConfig
	Houses      HousesConfig
	Incidents   IncidentsConfig
	Assets      AssetsConfig
	Intel       IntelConfig
	Endings     EndingsConfig
	Characters  CharactersConfig
}

// Load parses the embedded TOML files, then checks them (#275): every
// file is decoded first, in the table's order, so that a check which
// reads another file (the names pool against the crew and the cities,
// the incidents against the map, the characters against five files)
// sees it whole. The checks then run in dependency order, a file after
// every file it reads, each error prefixed with the file it is about.
// A file owns its checks in its own validate(); the one that joins two
// files, a route that opens with an asset, is listed under the file it
// is about (routes.toml) after both are checked.
func Load() (*Config, error) {
	var c Config
	for _, f := range []struct {
		name string
		dest any
	}{
		{"market.toml", &c.Market},
		{"city.toml", &c.City},
		{"routes.toml", &c.Routes},
		{"heat.toml", &c.Heat},
		{"crew.toml", &c.Crew},
		{"rivals.toml", &c.Rivals},
		{"laundering.toml", &c.Laundering},
		{"names.toml", &c.Names},
		{"upgrades.toml", &c.Upgrades},
		{"reputation.toml", &c.Reputation},
		{"law.toml", &c.Law},
		{"headlines.toml", &c.Headlines},
		{"dilemmas.toml", &c.Dilemmas},
		{"buyers.toml", &c.Buyers},
		{"suppliers.toml", &c.Suppliers},
		{"progression.toml", &c.Progression},
		{"houses.toml", &c.Houses},
		{"incidents.toml", &c.Incidents},
		{"intel.toml", &c.Intel},
		{"assets.toml", &c.Assets},
		{"endings.toml", &c.Endings},
		{"characters.toml", &c.Characters},
	} {
		if err := decode(f.name, f.dest); err != nil {
			return nil, err
		}
	}
	for _, v := range []struct {
		name  string
		check func() error
	}{
		{"market.toml", c.Market.validate},
		{"city.toml", c.City.validate},
		{"routes.toml", func() error { return c.Routes.validate(c.City) }},
		{"heat.toml", c.Heat.validate},
		{"crew.toml", func() error { return c.Crew.validate(c.Market) }},
		{"rivals.toml", c.Rivals.validate},
		{"laundering.toml", c.Laundering.validate},
		{"names.toml", func() error { return c.Names.validate(c.Crew, c.City) }},
		{"upgrades.toml", c.Upgrades.validate},
		{"reputation.toml", c.Reputation.validate},
		{"law.toml", c.Law.validate},
		{"dilemmas.toml", c.Dilemmas.validate},
		{"buyers.toml", func() error { return c.Buyers.validate(c.Market, c.City) }},
		{"suppliers.toml", func() error { return c.Suppliers.validate(c.Market, c.City) }},
		{"progression.toml", c.Progression.validate},
		{"houses.toml", func() error { return c.Houses.validate(c.City) }},
		{"incidents.toml", func() error { return c.Incidents.validate(c.City, c.Market, c.Routes, c.Names) }},
		{"intel.toml", c.Intel.validate},
		{"assets.toml", func() error { return c.Assets.validate(c.City) }},
		{"routes.toml", func() error { return c.Routes.validateAssets(c.Assets) }},
		{"endings.toml", c.Endings.validate},
		{"headlines.toml", c.Headlines.validate},
		{"characters.toml", func() error { return c.Characters.validate(c.Crew, c.Upgrades, c.Market, c.City, c.Progression) }},
	} {
		if err := v.check(); err != nil {
			return nil, fmt.Errorf("%s: %w", v.name, err)
		}
	}
	return &c, nil
}

// MustLoad is Load for tests and main; it panics on error.
func MustLoad() *Config {
	c, err := Load()
	if err != nil {
		panic(err)
	}
	return c
}

func decode(name string, v any) error {
	b, err := files.ReadFile(name)
	if err != nil {
		return err
	}
	return decodeBytes(name, b, v)
}

// decodeBytes parses one file's bytes into v. It is decode without the
// embedded read, so a test can feed a file that is not in the box.
func decodeBytes(name string, b []byte, v any) error {
	md, err := toml.Decode(string(b), v)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	// A key nobody reads, a misspelled tuning number or an effect name
	// nobody folds, would silently do nothing: every file is strict.
	if keys := md.Undecoded(); len(keys) > 0 {
		return fmt.Errorf("%s: unknown key %s", name, keys[0])
	}
	return nil
}

// find returns the first row of s that match accepts, or nil: the one
// find-by-id loop every table's lookup shares (#275). The pointer is into
// s, the table's own slice. game has its own; content imports nothing of
// game's.
func find[T any](s []T, match func(*T) bool) *T {
	for i := range s {
		if match(&s[i]) {
			return &s[i]
		}
	}
	return nil
}
