package ui

import (
	"os"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
)

// handledKeys are the keys keyPlay's switch handled the day the key
// table replaced it (#80): what the table must list, and all it may.
var handledKeys = []string{
	"q", " ", "ctrl+s", "?", "1", "2", "3", "4", "5", "6", "7", "8", "tab", "shift+tab",
	"n", "enter", "u", "r", "R", "b", "s", "t", "g", "[", "]", "x", "y", "l", "N",
	"h", "f", "p", "d", "i", "$", "c", "e", "a", "w",
	"up", "k", "down", "j", "left", "right", "pgup", "pgdown",
}

var pointerRE = regexp.MustCompile(`^[A-Z][a-z]+(?: [a-z]+)* on the \w+ screen \(\d\)(?: or the \w+ screen \(\d\))?\.(?: [A-Z][a-z]+(?: [a-z]+)* on the \w+ screen \(\d\)\.)*$`)

// Every key keyPlay handled is in the table, the table lists no other,
// every binding is listed somewhere with a label of one or two
// lowercase words, no screen has two bindings of its own for one key,
// and a letter pressed on a screen that does not take it is refused
// with the one pointer format (the arrows and the paging keys are
// silent).
func TestEveryKeyIsInTheTable(t *testing.T) {
	m := newTestModel(t, 120, 40)
	handled := map[string]bool{}
	for _, k := range handledKeys {
		handled[k] = true
	}
	accepted := map[string]bool{}
	for _, b := range bindings {
		for _, k := range rawKeys(b) {
			accepted[k] = true
			if !handled[k] {
				t.Errorf("the table lists %q (%s %s), which keyPlay never handled", k, b.key, b.label)
			}
		}
		// #109: screens is the only thing that lists a binding, so every
		// one the pane shows names its screens, global or not, and the
		// quiet ones name none.
		if !b.quiet && len(b.screens) == 0 {
			t.Errorf("%s %s is listed nowhere", b.key, b.label)
		}
		if b.quiet && (b.screens != nil || !b.global) {
			t.Errorf("%s %s is quiet and named for screens, or not global", b.key, b.label)
		}
		if words := strings.Fields(b.label); len(words) == 0 || len(words) > 3 || b.label != strings.ToLower(b.label) {
			t.Errorf("%s: the label %q is not one or two lowercase words", b.key, b.label)
		}
		if b.help == "" || b.do == nil {
			t.Errorf("%s %s has no help or nothing to do", b.key, b.label)
		}
	}
	for _, k := range handledKeys {
		if !accepted[k] {
			t.Errorf("keyPlay handled %q and the table does not list it", k)
		}
	}
	for s := screen(0); s < screenCount; s++ {
		m.switchScreen(s)
		// Two bindings of a screen's own for one key must be told apart
		// by a `when` (the market's `x decline` on the buyers, `x cancel
		// order` on the table).
		own := map[string]binding{}
		for _, b := range bindings {
			if !b.names(s) {
				continue
			}
			for _, k := range rawKeys(b) {
				if prev, dup := own[k]; dup && b.when == nil && prev.when == nil {
					t.Errorf("%s: %q is both %s and %s", screenOf[s], k, prev.label, b.label)
				}
				own[k] = b
			}
		}
		for _, k := range handledKeys {
			m.mode, m.status = modePlay, ""
			_, found, ownHere := m.lookup(k)
			m.Update(key(k))
			if m.mode == modePlay || m.mode == modeConfirmEnd || m.mode == modeHelp || m.mode == modeDetails {
				m.mode = modePlay
			}
			switch {
			case found, ownHere, len(k) != 1:
				if m.status != "" && pointerRE.MatchString(m.status) {
					t.Errorf("%s: %q is taken here and was pointed away: %q", screenOf[s], k, m.status)
				}
			default:
				if p := pointer(k); p != "" && (m.status != p || !pointerRE.MatchString(p)) {
					t.Errorf("%s: %q gave %q, want the pointer %q", screenOf[s], k, m.status, p)
				}
			}
			if m.quitting {
				m.quitting = false
			}
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
}

// The pane's KEYS section is exactly keysFor(screen), in order, with
// the table's labels, on every screen: `n end day` first, `? help`
// last, `q quit` in neither, and nothing listed that does not name the
// screen (#109: a key is listed where it is used).
func TestLegendMatchesTable(t *testing.T) {
	m := newTestModel(t, 300, 60)
	for s := screen(0); s < screenCount; s++ {
		m.switchScreen(s)
		m.status = ""
		keys := m.keysFor(s)
		if len(keys) < 3 || keys[0].key != "n" || keys[0].label != "end day" || keys[len(keys)-1].key != "?" {
			t.Errorf("%s: keysFor is %v", screenOf[s], keys)
		}
		for _, b := range keys {
			if b.key == "q" {
				t.Errorf("%s: q is in the pane's KEYS", screenOf[s])
			}
			if !b.names(s) {
				t.Errorf("%s: the pane lists %s %s, which does not name the screen", screenOf[s], b.key, b.label)
			}
		}
		// The pane's KEYS rows, read off the render: every key and its
		// label in order, two a row.
		rows := strings.Split(stripANSI(m.View()), "\n")
		var text string
		in := false
		for _, r := range rows {
			rs := []rune(r)
			if len(rs) < paneWidth {
				continue
			}
			cell := strings.TrimSpace(strings.Trim(string(rs[len(rs)-paneWidth:]), "│╰─╯"))
			switch {
			case cell == "KEYS":
				in = true
			case in && strings.HasPrefix(string(rs[len(rs)-paneWidth:]), "╰"):
				in = false
			case in:
				text += " " + cell
			}
		}
		text = spaces.ReplaceAllString(text, " ")
		at := 0
		for _, b := range keys {
			pair := b.key + " " + m.labelOf(b)
			i := strings.Index(text[at:], pair)
			if i < 0 {
				t.Errorf("%s: the pane's KEYS lack %q after %q in %q", screenOf[s], pair, text[:at], text)
				continue
			}
			at += i + len(pair)
		}
		if rest := strings.TrimSpace(text[at:]); rest != "" {
			t.Errorf("%s: the pane's KEYS carry %q past the table", screenOf[s], rest)
		}
	}
}

// Every global is listed on the screens #109 gives it and works on
// every screen, listed or not; the screens a global names are exactly
// the issue's.
func TestGlobalsAreListedWhereUsed(t *testing.T) {
	want := map[string][]screen{
		"n":   everywhere,
		"↑↓":  listScreens,
		"[ ]": on(screenMarket, screenMap),
		"b":   on(screenDashboard, screenMarket),
		"s":   on(screenDashboard, screenMarket),
		"x":   on(screenDashboard, screenMarket),
		"l":   on(screenDashboard),
		"p":   on(screenCrew),
		"d":   on(screenLedger),
		"g":   on(screenDashboard, screenMap),
		"r":   on(screenDashboard, screenJournal),
		"␣":   everywhere,
		"?":   everywhere,
	}
	m := newTestModel(t, 120, 40)
	for _, b := range bindings {
		if !b.global || b.quiet {
			continue
		}
		ws, ok := want[b.key]
		if !ok {
			t.Errorf("%s %s is a global the test does not know", b.key, b.label)
			continue
		}
		for s := screen(0); s < screenCount; s++ {
			listed := false
			for _, x := range ws {
				listed = listed || x == s
			}
			if b.names(s) != listed {
				t.Errorf("%s %s names %s: %v, want %v", b.key, b.label, screenOf[s], b.names(s), listed)
			}
			// Pressed anywhere, the key is taken: by the global or by the
			// screen's own binding for it, never pointed away.
			m.switchScreen(s)
			m.status = ""
			for _, k := range rawKeys(b) {
				if _, found, _ := m.lookup(k); !found {
					t.Errorf("%s: %q is not taken by any binding", screenOf[s], k)
				}
			}
		}
	}
}

// The help modal's rows and the README's rows are the bindings, one
// each, with the table's labels, and no help row is cut at 80 columns.
func TestHelpMatchesKeys(t *testing.T) {
	m := newTestModel(t, 80, 24)
	lines := m.helpLines()
	var rows []string
	for _, l := range lines {
		if p := stripANSI(l); p != "" && p != strings.ToUpper(p) && !strings.HasPrefix(p, "WORDS") {
			rows = append(rows, p)
		}
		if p := stripANSI(l); p == "WORDS" {
			break
		}
	}
	if len(rows) != len(bindings) {
		t.Errorf("help has %d rows for %d bindings:\n%s", len(rows), len(bindings), strings.Join(rows, "\n"))
	}
	for _, b := range bindings {
		want := stripANSI(helpRow(b.key, b.label, b.help))
		if len([]rune(b.help)) > helpSentenceW {
			t.Errorf("the help for %s %s runs to %d characters, over %d", b.key, b.label, len([]rune(b.help)), helpSentenceW)
		}
		n := 0
		for _, r := range rows {
			if r == want {
				n++
			}
		}
		if n != 1 {
			t.Errorf("help has %d rows for %s %s, want one: %q", n, b.key, b.label, want)
		}
		if lipgloss.Width(want) > m.modalInner() {
			t.Errorf("the help row for %s is %d wide, over the modal's %d: %q", b.key, lipgloss.Width(want), m.modalInner(), want)
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 80})
	m.Update(key("?"))
	view := stripANSI(m.View())
	for _, r := range rows {
		if !strings.Contains(view, r) {
			t.Errorf("the help modal lacks %q", r)
		}
	}
	if strings.Contains(view, "…") {
		t.Errorf("a help row is cut:\n%s", view)
	}
	readme := ReadmeKeys()
	for _, b := range bindings {
		cell := "| `" + b.key + "` | " + strings.ReplaceAll(b.label, "<city>", `\<city\>`) + " | " + b.help + " |"
		if strings.Count(readme, cell) != 1 {
			t.Errorf("the README table has %d rows for %s %s", strings.Count(readme, cell), b.key, b.label)
		}
	}
	if n := strings.Count(readme, "\n| `"); n != len(bindings) {
		t.Errorf("the README table has %d rows for %d bindings", n, len(bindings))
	}
}

// README.md's key table is what cmd/keys prints from the bindings.
func TestReadmeMatchesKeys(t *testing.T) {
	b, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := ReadmeKeysSection(string(b))
	if !ok {
		t.Fatalf("README.md has no %s / %s markers", ReadmeKeysBegin, ReadmeKeysEnd)
	}
	if got != ReadmeKeys() {
		t.Errorf("README.md's key table is stale: run `go run ./cmd/keys -w`\ngot:\n%s\nwant:\n%s", got, ReadmeKeys())
	}
}

// keyHints are what a key hint outside the legend, the pane and the
// modal footers looks like: the old two-part pointers, `press x`, a
// `·`-joined key list, and a key in parens (the plural's `(s)` aside),
// which is allowed only as the tail of `on the <screen> (<digit>)`.
var keyHints = []*regexp.Regexp{
	regexp.MustCompile(`\(\w+, \w+\)`),
	regexp.MustCompile(`on the tree`),
	regexp.MustCompile(`(?i)\bpress [^ ]{1,6}\b`),
	regexp.MustCompile(`(^|\s)[a-z] [a-z]+ · [a-z] [a-z]+`),
}

var (
	parenKey        = regexp.MustCompile(`\(([a-rt-zA-Z]|\d)\)`)
	screenPointerRE = regexp.MustCompile(`on the \w+ screen \(\d\)$`)
	tutorialRE      = regexp.MustCompile(`No sales queued\. Press s[^\n│]*|[Pp]ress [a-z] to [a-z]+ one\.`)
)

// No screen, modal or status carries a key hint outside the legend, the
// pane and the modal footers, and every pointer to a screen is spelled
// `on the <screen> (<digit>)`. The tutorial line and the empty states
// that name their key the legend's way (emptyState: `No deals. Press d
// to propose one.`) are the exceptions.
func TestNoKeyHintsOutsideTheLegend(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		richFixture(t, sz, func(m *Model, view, what string) {
			plain := tutorialRE.ReplaceAllString(stripANSI(view), "")
			for _, re := range keyHints {
				if hit := re.FindString(plain); hit != "" {
					t.Errorf("%dx%d %s: a key hint %q outside the legend:\n%s", sz[0], sz[1], what, hit, plain)
				}
			}
			for _, i := range parenKey.FindAllStringIndex(plain, -1) {
				if !screenPointerRE.MatchString(plain[max(0, i[0]-30):i[1]]) {
					t.Errorf("%dx%d %s: a key or a pointer not spelled `on the <screen> (<digit>)`: %q", sz[0], sz[1], what, plain[max(0, i[0]-30):i[1]])
				}
			}
		})
	}
}

// q in play mode saves and quits, as it did when the switch handled it.
func TestQuitKeySaves(t *testing.T) {
	m := newTestModel(t, 80, 24)
	m.w.Player.DirtyCash = 4321
	_, cmd := m.Update(key("q"))
	if cmd == nil || !m.quitting {
		t.Fatalf("q: cmd %v quitting %v", cmd, m.quitting)
	}
	w, err := game.Load()
	if err != nil || w.Player.DirtyCash != 4321 {
		t.Fatalf("q did not save: %v %+v", err, w)
	}
}
