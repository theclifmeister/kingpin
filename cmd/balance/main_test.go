package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/harness"
	"github.com/theclifmeister/kingpin/internal/sim"
)

// TestEveryPolicyPlays plays every registered policy five days on one
// seed through the tally the tool uses and prints the summary to
// nowhere (#274): a constructor that panics on the tool's default
// options, a counter that goes negative, or a summary line that indexes
// an empty slice fails here instead of in a balance run.
func TestEveryPolicyPlays(t *testing.T) {
	cfg := content.MustLoad()
	const days = 5
	for _, np := range harness.Policies {
		t.Run(np.Name, func(t *testing.T) {
			p := np.Make(cfg, harness.DefaultPolicyOpts())
			tl := newTally(cfg)
			w := sim.NewWorldWith(cfg, 1, harness.Character(cfg, ""))
			tl.table.counts[len(w.Rivals)]++
			res, err := harness.Play(cfg, w, days, func(w *game.World) { tl.morning(w); p(w) }, harness.Options{Incidents: true})
			if err != nil {
				t.Fatal(err)
			}
			tl.add(res, days, false)
			if len(tl.played) != 1 || tl.played[0] < 1 || tl.played[0] > days {
				t.Errorf("played %v, want one run of 1..%d days", tl.played, days)
			}
			if tl.peaks[0] < 0 {
				t.Errorf("peak cash %d", tl.peaks[0])
			}
			for name, n := range map[string]int{
				"robberies": tl.robberies, "robbed": tl.robbed, "incidents": tl.firedRuns,
				"tips": tl.rival.tips, "strikes": tl.rival.strikes, "undercuts": tl.rival.undercuts,
				"scouts": tl.books.scouts, "boosts": tl.books.boosts,
				"informants": tl.crew.informants, "bodies": tl.crew.bodies,
				"laundered": tl.wash.laundered, "audits": tl.wash.audits, "assets": tl.wash.assetsBought,
				"bribes": tl.law.bribes, "elections": tl.law.elections,
				"shipments": tl.trade.shipments, "rent": tl.trade.rent, "deeds": tl.trade.deedsBought,
				"cops": tl.intel.copsPaid, "spies": tl.intel.spies,
			} {
				if n < 0 {
					t.Errorf("%s = %d, a count cannot go negative", name, n)
				}
			}
			if got := len(tl.endings); got != 1 {
				t.Errorf("endings %v, want the one run", tl.endings)
			}
			stdout := os.Stdout
			null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			os.Stdout = null
			defer func() { os.Stdout = stdout; null.Close() }()
			tl.print(summary{policy: np.Name, pace: "on", credit: "on", runs: 1, days: days, incidents: true})
		})
	}
}

// TestPolicyNamesAreUnique holds the registry to one entry a name, so
// PolicyNamed can never shadow a policy with an earlier one.
func TestPolicyNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, n := range harness.PolicyNames() {
		if seen[n] {
			t.Errorf("%q registered twice", n)
		}
		seen[n] = true
		if _, ok := harness.PolicyNamed(n); !ok {
			t.Errorf("PolicyNamed(%q) misses it", n)
		}
	}
	if _, ok := harness.PolicyNamed("bosss"); ok {
		t.Error("PolicyNamed finds a name that is not registered")
	}
}

// TestPolicyListsMatchTheRegistry holds the docs to harness.Policies
// (#274): docs/harness.md lists every policy in the registry's order
// and CLAUDE.md names how many there are in words, first to last. They
// drifted to 31 and "thirty-three" against 35 while the list lived in
// a switch.
func TestPolicyListsMatchTheRegistry(t *testing.T) {
	names := harness.PolicyNames()
	doc, err := os.ReadFile("../../docs/harness.md")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile("`cmd/balance` policies: `([^`]*)`").FindSubmatch(doc)
	if m == nil {
		t.Fatal("docs/harness.md has no \"`cmd/balance` policies: `…`\" list")
	}
	if got, want := string(m[1]), strings.Join(names, " | "); got != want {
		t.Errorf("docs/harness.md lists\n  %s\nthe registry is\n  %s", got, want)
	}
	claude, err := os.ReadFile("../../CLAUDE.md")
	if err != nil {
		t.Fatal(err)
	}
	want := "plays one of " + inWords(len(names)) + " scripted policies (`" + names[0] + "` to `" + names[len(names)-1] + "`"
	if !strings.Contains(string(claude), want) {
		t.Errorf("CLAUDE.md does not say %q", want)
	}
}

// inWords spells n (under a hundred) the way CLAUDE.md does.
func inWords(n int) string {
	ones := []string{"", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten",
		"eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	tens := []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
	if n < 20 {
		return ones[n]
	}
	if n%10 == 0 {
		return tens[n/10]
	}
	return tens[n/10] + "-" + ones[n%10]
}
