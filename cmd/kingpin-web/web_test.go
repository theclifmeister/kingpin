package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/harness"
	"github.com/theclifmeister/kingpin/internal/protocol"
)

// TestWebClient (#328): the site built as `go run ./cmd/kingpin-web`
// builds it, its modules run under Node against the WebAssembly engine
// (testdata/play.mjs). The client speaks this build's protocol and view
// and refuses another; its animation table is engine.CueKinds, no more
// and no less; the reference seed played with the autopilot reaches an
// ending; and on it, three more seeds and 120 nights of the harness's
// boss every cue that came names something on the map, animates and
// draws, and every day's map draws. A cue none of them gave is made up
// on the boss's last morning and drawn too, so all 17 are.
func TestWebClient(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the site")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("no node on the runner: CI plays the client under it")
		}
		t.Skip("no node to run the client")
	}
	t.Parallel()
	site := t.TempDir()
	if err := Build(site); err != nil {
		t.Fatal(err)
	}
	kinds, err := json.Marshal(engine.CueKinds())
	if err != nil {
		t.Fatal(err)
	}
	alertKinds, err := json.Marshal(engine.AlertKinds())
	if err != nil {
		t.Fatal(err)
	}
	boss := filepath.Join(t.TempDir(), "boss.json")
	bossNights(t, boss)
	exits := filepath.Join(t.TempDir(), "exits.json")
	exitSaves(t, exits)
	run := exec.Command(node, filepath.Join("testdata", "play.mjs"), site, string(kinds), boss, string(alertKinds), exits)
	run.Env = append(os.Environ(), "KINGPIN_HOME="+t.TempDir())
	raw, err := run.Output()
	if err != nil {
		var stderr []byte
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = ee.Stderr
		}
		t.Fatalf("node: %v\n%s", err, stderr)
	}
	type runResult struct {
		Seed int            `json:"seed"`
		Day  int            `json:"day"`
		Over string         `json:"over"`
		Seen map[string]int `json:"seen"`
	}
	var got struct {
		Engine       struct{ Protocol, View int }
		Accepts      bool        `json:"accepts"`
		RefusesOther bool        `json:"refusesOther"`
		Missing      []string    `json:"missing"`
		Extra        []string    `json:"extra"`
		Problems     []string    `json:"problems"`
		Reference    runResult   `json:"reference"`
		More         []runResult `json:"more"`
		Boss         runResult   `json:"boss"`
		Synthetic    []string    `json:"synthetic"`
		AlertsMiss   []string    `json:"alertsMissing"`
		AlertsExtra  []string    `json:"alertsExtra"`
		AlertsWorded int         `json:"alertsWorded"`
		AlertsLinked int         `json:"alertsLinked"`
		Exits        map[string]struct {
			Open bool   `json:"open"`
			Over string `json:"over"`
		} `json:"exits"`
		CashOut struct{ Dirty, Clean, Fee int } `json:"cashOut"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("node printed %s: %v", raw, err)
	}
	if got.Engine.Protocol != protocol.Version || got.Engine.View != engine.ViewVersion {
		t.Errorf("the module says protocol %d view %d, the build %d %d", got.Engine.Protocol, got.Engine.View, protocol.Version, engine.ViewVersion)
	}
	if !got.Accepts {
		t.Errorf("the client does not speak this build's protocol %d or view %d: update SUPPORTED in web/js/session.js with the client", protocol.Version, engine.ViewVersion)
	}
	if !got.RefusesOther {
		t.Error("the client took an engine of a protocol it does not know")
	}
	if len(got.Missing)+len(got.Extra) > 0 {
		t.Errorf("the animation table: no animation for %v, an animation for no cue %v", got.Missing, got.Extra)
	}
	for _, p := range got.Problems {
		t.Error(p)
	}
	if len(got.AlertsMiss)+len(got.AlertsExtra) > 0 {
		t.Errorf("the alerts' words: none for %v, words for no kind %v", got.AlertsMiss, got.AlertsExtra)
	}
	if got.AlertsWorded == 0 || got.AlertsLinked == 0 {
		t.Errorf("%d alerts worded and %d linked to a panel over every run: the page draws none", got.AlertsWorded, got.AlertsLinked)
	}
	// The ways out (#405): every ending the player claims, through the
	// page's own exits.
	for id, cause := range map[string]string{"retire": content.CauseRetired, "vanish": content.CauseVanished, "crown": content.CauseKingpin, "straight": content.CauseBusinessman} {
		if e := got.Exits[id]; !e.Open || e.Over != cause {
			t.Errorf("the %s save: the page read it open %v and the claim ended the run %q, want %s", id, e.Open, e.Over, cause)
		}
	}
	if c := got.CashOut; c.Clean != 1000 || c.Fee <= 0 || c.Dirty != 1000-c.Fee {
		t.Errorf("the cash-out: %+v, want 1,000 clean out and 1,000 less the fee dirty in", c)
	}
	if got.Reference.Over == "" {
		t.Errorf("seed 7 with the autopilot: no ending by day %d", got.Reference.Day)
	}
	seen := map[string]int{}
	for _, r := range append([]runResult{got.Reference, got.Boss}, got.More...) {
		for k, n := range r.Seen {
			seen[k] += n
		}
	}
	var names []string
	for k := range seen {
		names = append(names, k)
	}
	sort.Strings(names)
	t.Logf("seed 7 ended day %d (%s); cues animated from the engine: %v; made up for the rest: %v", got.Reference.Day, got.Reference.Over, seen, got.Synthetic)
	for _, k := range []string{"corner_claimed", "sale", "police", "market", "run", "shipment", "crew_joined"} {
		if seen[k] == 0 {
			t.Errorf("the autopilot's four seeds and the boss's nights and no %s cue animated (%v)", k, names)
		}
	}
	if len(seen)+len(got.Synthetic) < len(engine.CueKinds()) {
		t.Errorf("%d cues animated from the engine and %d made up, of %d", len(seen), len(got.Synthetic), len(engine.CueKinds()))
	}
}

// night is a morning's view and the cues of the night before it.
type night struct {
	View engine.View  `json:"view"`
	Cues []engine.Cue `json:"cues"`
}

// bossNights writes 120 days of the harness's boss on seed 3 to path as
// nights: a run that ships, hires, posts and fights, which the
// autopilot's greedy dealer never does, so the client animates the cues
// of a run that has them.
func bossNights(t *testing.T, path string) {
	t.Helper()
	cfg := content.MustLoad()
	s, err := engine.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var cues []engine.Cue
	s.Subscribe(func(e events.Event) {
		if c, ok := engine.CueOf(e); ok {
			cues = append(cues, c)
		}
	})
	w := s.NewRun(3, game.Start{})
	var policy harness.Policy
	for _, p := range harness.Policies {
		if p.Name == "boss" {
			policy = p.Make(cfg, harness.PolicyOpts{})
		}
	}
	var nights []night
	for d := 0; d < 120 && w.Over == nil; d++ {
		if w.Dilemmas.Pending != nil {
			_, _ = s.Choose(0)
		}
		policy(w)
		cues = nil
		s.EndDay()
		nights = append(nights, night{View: s.View(), Cues: cues})
	}
	b, err := json.Marshal(nights)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// exitSaves writes, to path, a save for each way out the player claims
// (#405) with that one open, on seed 7's first morning: the account
// and the quiet days (with clean cash for the cash-out), the new
// identity, the reign, and the businessman's streak. The page imports
// each and takes it.
func exitSaves(t *testing.T, path string) {
	t.Helper()
	cfg := content.MustLoad()
	saves := map[string]string{}
	for id, open := range map[string]func(*game.World){
		"retire": func(w *game.World) {
			w.Offshore, w.QuietDays = cfg.Laundering.Offshore.RetireCash, cfg.Laundering.Offshore.RetireDays
			w.Player.CleanCash = 50_000
		},
		"vanish":   func(w *game.World) { w.Upgrades["identity"] = true },
		"crown":    func(w *game.World) { w.Reign = max(1, w.Day) },
		"straight": func(w *game.World) { w.LegitDays = cfg.Laundering.Businessman.LegitDays },
	} {
		s, err := engine.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		w := s.NewRun(7, game.Start{})
		if w.Upgrades == nil {
			w.Upgrades = map[string]bool{}
		}
		open(w)
		b, err := s.ExportSave()
		if err != nil {
			t.Fatal(err)
		}
		saves[id] = base64.StdEncoding.EncodeToString(b)
	}
	raw, err := json.Marshal(saves)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}
