package rivals_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The rival's state has one writer (#144): no sim but this one assigns
// into *w.Rival(), and none takes its address. The crew sim's hand-off is
// the queue on its own state (Crew.Leads) and the corners a lieutenant
// walks over, and the rivals sim reads both. The exceptions are the
// lieutenant's walk, whose flip is booked the night the corners change
// hands (lieutenant.go says why); the list can only shrink.
func TestRivalStateHasOneWriter(t *testing.T) {
	root := filepath.Join("..", "..", "..", "internal", "sim")
	allowed := map[string]bool{
		"crew/lieutenant.go: w.Faction(to.ID).Flips++":          true,
		"crew/lieutenant.go: w.Faction(to.ID).LastFlip = t.Day": true,
		"crew/lieutenant.go: w.Faction(to.ID).Observed = true":  true,
	}
	var (
		// an assignment into a faction's state (every sim calls the
		// world w): `w.Rival().Cash -= n`, `w.Rivals[i].Flips++`,
		// `w.Faction(id).Observed = true`, `w.Rivals = x`
		write = regexp.MustCompile(`\bw\.(?:Rival\(\)|Rivals(?:\[[^\]]*\])?|Faction\([^)]*\))(?:\.[\w.\[\]()]*)?\s*(?:\+\+|--|[-+*/]?=[^=])`)
		// an alias taken into the slice itself: `&w.Rivals[i]`. A
		// pointer read (`r := w.Rival()`, `w.Faction(id)`, a range over
		// w.Rivals) is the world handing a sim a *RivalState the way it
		// hands a *City, and writing through it is the same rule by
		// convention (TestSimsWriteOnlyTheirOwnState says so).
		alias = regexp.MustCompile(`&w\.Rivals\b`)
	)
	var offenders []string
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if rel == "rivals" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, l := range strings.Split(string(src), "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), "//") {
				continue
			}
			if !write.MatchString(l) && !alias.MatchString(l) {
				continue
			}
			key := filepath.ToSlash(rel) + ": " + strings.TrimSpace(l)
			if allowed[key] {
				seen[key] = true
				continue
			}
			offenders = append(offenders, filepath.ToSlash(rel)+":"+strconv.Itoa(i+1)+": "+strings.TrimSpace(l))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Errorf("a faction's state written outside internal/sim/rivals (hand it over through an event or the writer's own state):\n  %s", strings.Join(offenders, "\n  "))
	}
	for key := range allowed {
		if !seen[key] {
			t.Errorf("exception no longer in the source, drop it from the list: %s", key)
		}
	}
}
