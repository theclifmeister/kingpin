package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The copy, colour and status pass (#88): real plurals, one arrow, one
// set of status words, one meaning per colour, no style built outside
// theme, and every empty state one sentence in Subtle.

// sourceLines yields every line of the non-test Go sources in dirs that
// is not a comment, with its file and line number.
func sourceLines(t *testing.T, dirs ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, dir := range dirs {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for i, l := range strings.Split(string(src), "\n") {
				if strings.HasPrefix(strings.TrimSpace(l), "//") {
					continue
				}
				out[f+":"+itoa(i+1)] = l
			}
		}
	}
	return out
}

func itoa(n int) string { return strconv.Itoa(n) }

// pluralHack is a string literal with a `(s)` in it.
var pluralHack = regexp.MustCompile(`"(?:[^"\\]|\\.)*\(s\)(?:[^"\\]|\\.)*"`)

// TestNoPluralHacks: no `(s)` in the UI's or the report's sources, and
// in every rendered screen of the fixture no `(s)`, no `->`, no double
// space after a middle dot and no `TITLE  count` (a caps title followed
// by two spaces and a figure).
func TestNoPluralHacks(t *testing.T) {
	for where, l := range sourceLines(t, ".", filepath.Join("..", "sim", "news")) {
		if pluralHack.MatchString(l) {
			t.Errorf("%s: a (s) plural: %s", where, strings.TrimSpace(l))
		}
	}
	titleCount := regexp.MustCompile(`(?m)^[A-Z]{2,}(?: · \S+)?  +\d`)
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		richFixture(t, sz, func(m *Model, view, what string) {
			plain := stripANSI(view)
			for _, bad := range []string{"(s)", "->", " ·  "} {
				if strings.Contains(plain, bad) {
					t.Errorf("%dx%d %s: %q in the render:\n%s", sz[0], sz[1], what, bad, plain)
				}
			}
			if hit := titleCount.FindString(plain); hit != "" {
				t.Errorf("%dx%d %s: a count two spaces after a caps title, %q:\n%s", sz[0], sz[1], what, hit, plain)
			}
		})
	}
}

// TestThemeIsTheOnlyStylist: no view file builds a style of its own;
// every lipgloss.NewStyle() in internal/ui is in theme/.
func TestThemeIsTheOnlyStylist(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, l := range strings.Split(string(src), "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), "//") || strings.Contains(l, `"lipgloss.NewStyle()"`) {
				continue // this test's own words
			}
			if strings.Contains(l, "lipgloss.NewStyle()") {
				t.Errorf("%s:%d builds a style outside theme: %s", f, i+1, strings.TrimSpace(l))
			}
		}
	}
}

// TestStatusKinds: the status bar (row h-1) colours the message by its
// kind, which every site sets: a refusal in Warning, a confirmation in
// Body, the tell that somebody is talking in Bad; and a refusal reads
// `Can't …` or `Nothing to …` with a full stop.
func TestStatusKinds(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0) // termenv.TrueColor
	defer lipgloss.SetColorProfile(profile)
	m := newTestModel(t, 120, 40)
	statusRow := func() string {
		rows := strings.Split(m.View(), "\n")
		return rows[len(rows)-1]
	}
	check := func(what string, style lipgloss.Style, prefixes ...string) {
		t.Helper()
		if m.status == "" {
			t.Fatalf("%s: no status", what)
		}
		if !strings.HasSuffix(m.status, ".") {
			t.Errorf("%s: the status has no full stop: %q", what, m.status)
		}
		if len(prefixes) > 0 {
			ok := false
			for _, p := range prefixes {
				ok = ok || strings.HasPrefix(m.status, p)
			}
			if !ok {
				t.Errorf("%s: the status is not in the refusal's register %v: %q", what, prefixes, m.status)
			}
		}
		if row := statusRow(); !strings.Contains(row, style.Render(m.status)) {
			t.Errorf("%s: %q is not rendered in its kind's style on row h-1:\n%s", what, m.status, row)
		}
	}
	// Refusals: with nothing to sell, a hire with the cursor nowhere, a
	// proposal with no rival, a key the screen does not take.
	m.Update(key("s"))
	if m.mode != modePlay {
		t.Fatalf("the sell dialog opened with nothing to sell")
	}
	check("a sell with nothing to sell", theme.Warning, "Nothing to ")
	m.Update(key("4"))
	m.Update(key("f")) // the cursor is on a candidate: nobody to fire
	check("a fire with the cursor on a candidate", theme.Warning, "Can't ")
	m.Update(key("h"))
	check("a hire", theme.Body)
	m.Update(key("8"))
	m.Update(key("d"))
	check("a proposal with no rival", theme.Warning, "Nothing to ", "Can't ")
	m.Update(key("1"))
	m.Update(key("w"))
	check("a key the screen does not take", theme.Warning)
	// A confirmation: the buy.
	m.Update(key("b"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("esc")) // the dialog stays open for the next line (#103)
	check("a buy", theme.Body)
	// A danger: the morning after the tell has shown twice, with the
	// informant still on the payroll (the count resets once nobody is
	// talking) and the wages covered.
	m.w.Crew.Members = append(m.w.Crew.Members, game.CrewMember{ID: 900, Name: "Rat", Role: "runner", Skill: 50, Loyalty: 50, Nerve: 50, Wage: 40, Informant: true})
	m.w.Player.DirtyCash += 10_000
	m.w.Heat.Leaks = 2
	endDay(t, m)
	m.Update(key("enter"))
	if m.status != "Somebody is talking. Investigate on the crew screen (4)." {
		t.Fatalf("the morning's status: %q", m.status)
	}
	check("somebody talking", theme.Bad)
	// A dialog error is in the same register: sentence case, a full stop.
	m.Update(key("b"))
	m.Update(key("enter"))
	m.dlg.qty.SetValue("x")
	m.Update(key("enter"))
	if m.dlg.err != "Enter a whole number above zero." {
		t.Errorf("the dialog error: %q", m.dlg.err)
	}
	m.Update(key("esc"))
	m.w.Player.DirtyCash = 0
	m.Update(key("b"))
	m.Update(key("enter"))
	if m.dlg.err != "Can't afford or hold any." {
		t.Errorf("the dialog error with no cash: %q", m.dlg.err)
	}
}

// TestEmptyStates: every empty state the bare fixture shows (no crew,
// no fronts, no deals, no offers, no journal, no rival, an empty stash)
// is a sentence in Subtle: sentence case, ending in a full stop, and
// naming its key the legend's way where it has one. The rival's and
// the paper's `Yet.` lines stay: they are the voice.
func TestEmptyStates(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0) // termenv.TrueColor
	defer lipgloss.SetColorProfile(profile)
	// Each empty state, and whether the rival has to be in town for
	// the screen to show it.
	want := []struct {
		text  string
		rival bool
	}{
		// The dashboard's RIVALS panel wraps its line at 120 columns,
		// so only the first sentence is looked for whole.
		{"Nobody on the payroll. Pick a face below and press h.", false},
		{"Nobody right now.", false},
		{"No fronts yet. A front washes dirty cash clean; press b to buy one.", false},
		{"Nobody is contesting the city yet.", false},
		{"The paper has nothing to say about you. Yet.", false},
		{"Nobody is asking. Offers come here and lapse in a few days.", false},
		{"Nobody is looking at you. Yet.", false},
		{"No deals. Press d to propose one.", true},
		{"Nothing on the table.", true},
	}
	sentenceRE := regexp.MustCompile(`^[A-Z][^.]*\.( [A-Z][^.]*\.)?$`)
	for _, w := range want {
		if !sentenceRE.MatchString(w.text) {
			t.Errorf("the empty state %q is not a sentence in sentence case with a full stop", w.text)
		}
	}
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		for _, rival := range []bool{false, true} {
			m := newTestModel(t, sz[0], sz[1])
			m.w.Crew.Members, m.w.Crew.Candidates = nil, nil
			for _, id := range m.w.Products {
				m.w.Stash(m.w.Player.Location)[id] = 0
			}
			if rival {
				m.w.Rival.Arrived = 1
			}
			seen := map[string]bool{}
			for _, s := range []string{"1", "2", "3", "4", "5", "6", "7", "8"} {
				m.Update(key(s))
				view := m.View()
				plain := stripANSI(view)
				if s == "1" && sz[0] >= paneMinWidth && !rival && !strings.Contains(plain, "Nobody is contesting the city.") {
					t.Errorf("%dx%d: the dashboard's RIVALS panel has no rival line", sz[0], sz[1])
				}
				for _, w := range want {
					if w.rival != rival || !strings.Contains(plain, w.text) {
						continue
					}
					seen[w.text] = true
					// In Subtle: the line carries the text (or, for one
					// that names a key, its opening words) in Subtle.
					head := w.text
					if i := strings.Index(head, "ress "); i >= 0 {
						head = head[:i+len("ress ")]
					}
					if !strings.Contains(view, theme.Subtle.Render(head)) && !strings.Contains(view, theme.Subtle.Render(w.text)) {
						t.Errorf("%dx%d screen %s: the empty state %q is not in Subtle", sz[0], sz[1], s, w.text)
					}
				}
			}
			for _, w := range want {
				if w.rival == rival && !seen[w.text] {
					t.Errorf("%dx%d rival %v: the empty state %q is not shown", sz[0], sz[1], rival, w.text)
				}
			}
		}
	}
}
