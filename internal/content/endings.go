package content

import (
	"fmt"
	"strings"
	"text/template"
)

// EndingsConfig mirrors endings.toml (#49): one row a cause of
// game.Ending, the kingpin's reign and the summary's timeline. It is the
// summary's file, read by the UI alone: the thresholds that detect an
// ending live in the file of the sim that owns it (laundering.toml
// [businessman], rivals.toml [endings], crew.toml [lieutenant]
// betray_share), so nothing here moves a sim's dice.
type EndingsConfig struct {
	Kingpin KingpinConfig  `toml:"kingpin"`
	Summary SummaryConfig  `toml:"summary"`
	Endings []EndingConfig `toml:"ending"`
}

// KingpinConfig is the kingpin's epilogue: ReignYears is how long the
// reign reads at heat 0 and pressure 0 at home the morning it ends; the
// years are ReignYears x (1 - (heat + pressure) / 200), rounded, one at
// least (Reign).
type KingpinConfig struct {
	ReignYears int `toml:"reign_years"`
}

// SummaryConfig is the summary's timeline: the top Lines headlines of
// the run by the Weight of their source, shown in the order they
// happened; a source not in Weight weighs nothing and is left out.
type SummaryConfig struct {
	Lines  int                `toml:"lines"`
	Weight map[string]float64 `toml:"weight"`
	Quiet  []string           `toml:"quiet"` // headline templates (headlines.toml keys) never in THE STORY, whatever their source weighs (#424)
}

// validateQuiet checks every quiet key is a headline template.
func (s SummaryConfig) validateQuiet(h HeadlinesConfig) error {
	for _, k := range s.Quiet {
		if len(h.Templates[k]) == 0 {
			return fmt.Errorf("summary: quiet %q is no headline template", k)
		}
	}
	return nil
}

// EndingConfig is one cause: the title the summary prints, whether the
// run counts as won (the summary's colour and the scene's accent), the
// epilogue as a text/template over Epilogue, and the line the ending's
// scene plays for the causes on the exit scene.
type EndingConfig struct {
	Cause    string `toml:"cause"`
	Title    string `toml:"title"`
	Won      bool   `toml:"won"`
	Epilogue string `toml:"epilogue"`
	Scene    string `toml:"scene"`
}

// The causes of game.Ending (#49): the three the run had, the one #195
// landed, and the five this issue adds. Every one has a row in
// endings.toml (validate) and a scripted scenario in the harness that
// reaches it (TestEveryEndingIsReachable).
const (
	CauseIndicted    = "indicted"
	CauseArrested    = "arrested"
	CauseBroke       = "broke"
	CauseRetired     = "retired"
	CauseBusinessman = "businessman"
	CauseKingpin     = "kingpin"
	CauseBetrayed    = "betrayed"
	CauseTakenOut    = "taken_out"
	CauseVanished    = "vanished"
)

// Causes is every cause, in the order the docs table them.
var Causes = []string{CauseIndicted, CauseArrested, CauseBroke, CauseRetired, CauseBusinessman, CauseKingpin, CauseBetrayed, CauseTakenOut, CauseVanished}

// Epilogue is what an epilogue template can name: the numbers the
// summary reads off the world, the money formatted by the caller.
type Epilogue struct {
	Days     int    // the day the run ended
	City     string // home
	Here     string // where you stood
	Offshore string // the account, formatted
	Left     string // the pile left behind, formatted
	Bodies   int
	Fronts   string // `3 businesses`
	Corners  int
	Name     string // the lieutenant who flipped (betrayed)
	Leader   string // the faction's leader (taken_out, betrayed by the table)
	DA       string
	Chief    string
	Pages    int    // the DA's file
	Years    string // the kingpin's reign, `4 years`
	Reign    string // the reign lived before the crown was taken (#227), `31 days`
	Hot      bool   // the heat over the pressure at home at the end
}

// Ending returns the row for the cause, or nil.
func (e EndingsConfig) Ending(cause string) *EndingConfig {
	return find(e.Endings, func(x *EndingConfig) bool { return x.Cause == cause })
}

// Won reports whether the cause counts as won; false for one the file
// does not know.
func (e EndingsConfig) Won(cause string) bool {
	if row := e.Ending(cause); row != nil {
		return row.Won
	}
	return false
}

// Title is the cause's title, or the cause in capitals for one the file
// does not know.
func (e EndingsConfig) Title(cause string) string {
	if row := e.Ending(cause); row != nil && row.Title != "" {
		return row.Title
	}
	return strings.ToUpper(strings.ReplaceAll(cause, "_", " "))
}

// Render fills the cause's epilogue with d; empty for a cause the file
// does not know, and the template's error for one that will not render.
func (e EndingsConfig) Render(cause string, d Epilogue) (string, error) {
	row := e.Ending(cause)
	if row == nil {
		return "", nil
	}
	t, err := template.New(cause).Parse(row.Epilogue)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := t.Execute(&b, d); err != nil {
		return "", err
	}
	return b.String(), nil
}

// Reign is how many years the kingpin's reign lasted: ReignYears at
// heat 0 and pressure 0 at home the morning it ends, down to one as
// they rise to 100 each. No dice: the epilogue is a function of the
// world as it stood.
func (k KingpinConfig) Reign(heat, pressure float64) int {
	years := float64(k.ReignYears) * (1 - (heat+pressure)/200)
	return max(1, int(years+0.5))
}

// validate checks every cause has one row with a title and an epilogue
// that parses, no row names a cause the game lacks, and the summary
// asks for at least a line.
func (e EndingsConfig) validate() error {
	seen := map[string]bool{}
	for _, row := range e.Endings {
		known := false
		for _, c := range Causes {
			known = known || c == row.Cause
		}
		if !known {
			return fmt.Errorf("ending %q is not a cause the game has", row.Cause)
		}
		if seen[row.Cause] {
			return fmt.Errorf("ending %q listed twice", row.Cause)
		}
		seen[row.Cause] = true
		if row.Title == "" || row.Epilogue == "" || row.Scene == "" {
			return fmt.Errorf("ending %q needs a title, an epilogue and a scene line", row.Cause)
		}
		if _, err := template.New(row.Cause).Parse(row.Epilogue); err != nil {
			return fmt.Errorf("ending %q: %w", row.Cause, err)
		}
	}
	for _, c := range Causes {
		if !seen[c] {
			return fmt.Errorf("no ending row for %q", c)
		}
	}
	if e.Summary.Lines < 1 {
		return fmt.Errorf("[summary] lines %d: at least one", e.Summary.Lines)
	}
	if e.Kingpin.ReignYears < 1 {
		return fmt.Errorf("[kingpin] reign_years %d: at least one", e.Kingpin.ReignYears)
	}
	return nil
}
