package sim_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestSimsNeverImportEachOther: no package under internal/sim/ imports
// another under internal/sim/ (#144). Events and World state are the only
// cross-sim channel; a query two sims share (news.Eligible was one, read
// by the market's buyers) lives in game beside FoldEffects. The check
// parses the import block of every non-test Go file under internal/sim/*
// (this package, the assembler, is exempt: it imports every sim by
// design) and names the file and the edge on a hit.
func TestSimsNeverImportEachOther(t *testing.T) {
	const prefix = "github.com/theclifmeister/kingpin/internal/sim/"
	dirs, err := filepath.Glob("*")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, dir := range dirs {
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			continue
		}
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			src, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			checked++
			for _, imp := range src.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if strings.HasPrefix(path, prefix) {
					t.Errorf("%s imports %s: sims never import each other", f, strings.TrimPrefix(path, prefix))
				}
			}
		}
	}
	if checked < 10 {
		t.Fatalf("only %d sim files checked; the glob is wrong", checked)
	}
}

// TestSimsWriteOnlyTheirOwnState (#144): a package under internal/sim/
// assigns into its own state on the World and nothing else. Its own
// state is the field the map in docs/README.md gives it (the heat sim
// w.Heat and a city's Heat, the law w.Law and a city's Pressure and
// Goodwill, the market the markets, the connects, the contracts and the
// buyers, the default quality and a corner's Repeat, #47, its field on
// the territory's corner, ...); what every sim shares is the till (w.Player.DirtyCash,
// CleanCash) and w.Stats; w.Over is listed for the sims that own an
// ending (#49, docs/endings.md: heat, crew, rivals, laundering), each
// written through World.End in the owner's step. Stock
// moves only through the accessors (TestStashHasNoWriters). The check
// is a grep over every non-test file: a direct assignment through w
// (`w.Rival().Cash -= n`, `w.Offers = nil`, `w.Cities[c].Heat = v`) or an
// alias taken into it (`r := w.Rival()`, `m := &w.Crew.Members[i]`);
// what a sim writes through a pointer it was handed (a *City from
// w.Cities, a *House from w.Fullest) is the same rule by convention,
// and the docs say which sim writes which. The corner is the one such
// pointer the tree checks (#274, corners_test.go): it is the territory
// sim's, a change of holder is game.(*Corner).Hand
// (TestCornerOwnerIsHanded), and every other package's write into one
// is a row of cornerWriters (TestCornerWritersAreDeclared): the
// market's Repeat and its Squeeze on the rival's corners, the rivals'
// Squeeze on yours and Starved/StarvedDay, the lieutenant's Since, and
// the hand-overs the rivals and the lieutenant make. The exceptions are the
// cross-sim writes the docs rule on, each named here so the list can
// only shrink: the lieutenant's walk books the rival's flip the night
// the corners change hands (crew/lieutenant.go, #144 PR 2), the audit
// flips an accountant without an event (laundering.go, #29), the
// connect's collector hurts your best enforcer (market/suppliers.go,
// #72). The file (World.Intel, #45) is nobody's row on purpose: a fact
// is written through World.Learn (LearnBooks, Unlearn, Expose) by
// whichever sim owns the truth it states, a call like AddStock that
// the grep does not see, and an assignment into w.Intel from any sim
// is an offender here.
func TestSimsWriteOnlyTheirOwnState(t *testing.T) {
	owned := map[string][]string{
		"world":      {"Incidents"}, // its effects apply in game.ApplyIncident, the one place that knows the keys (#44)
		"market":     {"Cities.Market", "Contracts", "Buyers", "Suppliers", "Markup", "Supply", "Standing", "BaseQuality", "Cities.Corners.Repeat"},
		"logistics":  {"Shipments", "Logistics", "Routes"},
		"territory":  {"Cities.Corners", "Houses"},
		"rivals":     {"Rivals", "Rival", "Faction", "Offers", "Over", "Reign", "War"},                               // the endings it owns (#49): kingpin (the reign's stamp since #227), taken_out, the table's betrayed; the war order it ends (#229)
		"crew":       {"Crew", "Delegated", "DelegatedSupply", "Over"},                                               // broke, and the lieutenant's betrayed (#49)
		"heat":       {"Heat", "Cities.Heat", "Houses", "FallsTaken", "Over"},                                        // indicted, arrested, and vanished through the exit plans (#49)
		"law":        {"Law", "Cities.Pressure", "Cities.Goodwill", "Cities.Campaign", "Cities.Corners.Deed"},        // the forfeiture takes a deed through w.SeizeDeed (#194)
		"laundering": {"Laundering", "Fronts", "Offshore", "QuietDays", "Assets", "AssetsLost", "LegitDays", "Over"}, // the assets are clean money (#48): the task force names what it takes, this sim books it; businessman (#49)
		"reputation": {"Player.Reputation"},
		"news":       {"Journal", "Report", "Dilemmas", "Progression", "Flows"}, // the cash flow's history (#351)
	}
	shared := []string{"Player.DirtyCash", "Player.CleanCash", "Stats"}
	allowed := map[string]bool{
		"crew/lieutenant.go: w.Faction(to.ID).Flips++":          true,
		"crew/lieutenant.go: w.Faction(to.ID).LastFlip = t.Day": true,
		"crew/lieutenant.go: w.Faction(to.ID).Observed = true":  true,
		"laundering/laundering.go: m := &w.Crew.Members[i]":     true,
		"market/suppliers.go: m := &w.Crew.Members[i]":          true,
	}
	var (
		// `w.Field[...].Sub... op= ` and `&w.Field[...].Sub`; a call in
		// the path (`w.Rival().X`, `w.Faction(id).X`, #43) is a
		// selector like any other
		write = regexp.MustCompile(`\bw(\.[A-Z]\w*(?:\([^)]*\))?(?:\[[^\]]*\])?(?:\.\w+(?:\([^)]*\))?(?:\[[^\]]*\])?)*)\s*(?:\+\+|--|[-+*/]?=[^=])`)
		alias = regexp.MustCompile(`&w(\.[A-Z]\w*(?:\([^)]*\))?(?:\[[^\]]*\])?(?:\.\w+(?:\([^)]*\))?(?:\[[^\]]*\])?)*)`)
		index = regexp.MustCompile(`\[[^\]]*\]|\([^)]*\)`)
	)
	path := func(sel string) string {
		return strings.TrimPrefix(index.ReplaceAllString(sel, ""), ".")
	}
	permits := func(sim, p string) bool {
		for _, a := range append(owned[sim], shared...) {
			if p == a || strings.HasPrefix(p, a+".") {
				return true
			}
		}
		return false
	}
	var offenders []string
	seen := map[string]bool{}
	checked := 0
	err := filepath.WalkDir(".", func(file string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
			return err
		}
		rel := filepath.ToSlash(file)
		sim, _, ok := strings.Cut(rel, "/")
		if !ok {
			return nil // the assembler
		}
		if _, known := owned[sim]; !known {
			t.Errorf("%s: a sim the owned list does not know; give it a row", rel)
			return nil
		}
		src, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		checked++
		for i, l := range strings.Split(string(src), "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), "//") {
				continue
			}
			var hits []string
			for _, m := range write.FindAllStringSubmatch(l, -1) {
				hits = append(hits, m[1])
			}
			for _, m := range alias.FindAllStringSubmatch(l, -1) {
				hits = append(hits, m[1])
			}
			for _, sel := range hits {
				p := path(sel)
				if permits(sim, p) {
					continue
				}
				key := rel + ": " + strings.TrimSpace(l)
				if allowed[key] {
					seen[key] = true
					continue
				}
				offenders = append(offenders, rel+":"+strconv.Itoa(i+1)+": "+strings.TrimSpace(l)+"  (w."+p+" is not the "+sim+" sim's)")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 10 {
		t.Fatalf("only %d sim files checked; the walk is wrong", checked)
	}
	if len(offenders) > 0 {
		t.Errorf("a sim writes another's state (hand it over through an event, or the writer's own state):\n  %s", strings.Join(offenders, "\n  "))
	}
	for key := range allowed {
		if !seen[key] {
			t.Errorf("exception no longer in the source, drop it from the list: %s", key)
		}
	}
}
