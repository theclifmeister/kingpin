package content

import (
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/events"
)

// TestTemperDialIsTheSellDial (#275): a temperament's dial is read
// through the sell dial's own names (events.ParseDial), so the file's
// words land on the notch they name, a word the dial does not know is
// refused at load with the names it could have been, and the steady
// default sells at normal.
func TestTemperDialIsTheSellDial(t *testing.T) {
	cfg := MustLoad()
	for _, name := range LieutenantPersonalities {
		p := cfg.Crew.Lieutenant.Temper(name)
		if got := p.SellDial().String(); got != p.Dial {
			t.Errorf("%s: dial %q sells at %q", name, p.Dial, got)
		}
	}
	if got := cfg.Crew.Lieutenant.Temper("nobody").SellDial(); got != events.DialNormal {
		t.Errorf("the default temper sells at %v, want normal", got)
	}
	bad := cfg.Crew
	bad.Lieutenant.Personality = map[string]LieutenantPersonality{}
	for k, v := range cfg.Crew.Lieutenant.Personality {
		bad.Lieutenant.Personality[k] = v
	}
	p := bad.Lieutenant.Personality["steady"]
	p.Dial = "loud"
	bad.Lieutenant.Personality["steady"] = p
	err := bad.validate(cfg.Market)
	if err == nil || !strings.Contains(err.Error(), `"loud"`) || !strings.Contains(err.Error(), "aggressive") {
		t.Fatalf("a temper dial the sell dial does not know: %v", err)
	}
}
