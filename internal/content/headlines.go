package content

// HeadlinesConfig mirrors headlines.toml.
type HeadlinesConfig struct {
	FlavourChance float64             `toml:"flavour_chance"`
	Templates     map[string][]string `toml:"templates"`
	Flavour       []string            `toml:"flavour"`
	Swagger       []string            `toml:"swagger"` // the boss's headlines (#233): flavour that names you, while the city is yours
}
