package ui

import (
	"bytes"
	"encoding/gob"
	"os"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The UI suite's shared helpers (#276): one way to build a test model,
// feed it keys and play its days, and one way to read a rendered frame
// back as text (splitView for MAIN and the pane). A helper one file uses
// stays in that file; one two files want lives here. The fixtures that
// set up a run (richModel, tableModel, the scenes' mornings) stay with
// the tests they were written for.

// ---- Seeds (#292) ----

// fixtureSeed is the seed the UI tests play (#292). It was the wall
// clock's until a test that held on most worlds failed on one in eighty,
// and nobody could play that world again.
const fixtureSeed = 1

// testSeed is where a test's runs start (#292): fixtureSeed, or
// KINGPIN_TEST_SEED to look at other worlds (a number, or random for the
// wall clock's). A test that fails says which seed it played, so the
// failure replays.
func testSeed(t *testing.T) uint64 {
	t.Helper()
	seed := uint64(fixtureSeed)
	switch v := os.Getenv("KINGPIN_TEST_SEED"); v {
	case "":
	case "random":
		seed = newSeed()
	default:
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			t.Fatalf("KINGPIN_TEST_SEED=%q: a number, or random", v)
		}
		seed = n
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("played seed %d: KINGPIN_TEST_SEED=%d replays it", seed, seed)
		}
	})
	return seed
}

// testSeeds is the Options.Seeds a test model plays (#292): testSeed,
// then the next and the next for every further run the test starts, so
// the UI suite plays the same worlds on every run.
func testSeeds(t *testing.T) func() uint64 {
	t.Helper()
	next := testSeed(t)
	return func() uint64 {
		s := next
		next++
		return s
	}
}

// ---- Models ----

// duel is the file with one faction in the run (#43): the rival at
// home alone, the run every fixture here was pinned on. The table's
// screens are tested on tableModel.
func duel() *content.Config {
	cfg := content.MustLoad()
	cfg.Rivals.Factions.Min, cfg.Rivals.Factions.Max = 1, 1
	return cfg
}

// sizedModel is New on the file and options given, sized to w by h:
// the one place a test model is made. It sets no KINGPIN_HOME and no
// seeds, so a caller opening a second model on the same save dir (a
// continue, a reload) gets the dir it is in and the options it passed.
func sizedModel(t *testing.T, cfg *content.Config, opts Options, w, h int) *Model {
	t.Helper()
	m, err := New(cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

// newTestModel is a fresh install at w by h with animation off (#152):
// no test and no README capture holds a scene unless it asks for one
// (newAnimModel).
func newTestModel(t *testing.T, w, h int) *Model {
	t.Helper()
	return newModelWith(t, w, h, Options{Anim: false})
}

// newModelWith is newTestModel with the options given, its runs on the
// test seeds unless the options name their own.
func newModelWith(t *testing.T, w, h int, opts Options) *Model {
	t.Helper()
	if opts.Seeds == nil {
		opts.Seeds = testSeeds(t)
	}
	t.Setenv("KINGPIN_HOME", t.TempDir())
	return sizedModel(t, duel(), opts, w, h)
}

// newModelIn is a second model on the same save dir as m, for continuing
// m's slot the way a fresh start of the game would.
func newModelIn(t *testing.T, m *Model, w, h int) *Model {
	t.Helper()
	return sizedModel(t, m.cfg, Options{Anim: false}, w, h)
}

// gobCopy is w through gob and back: a save's round trip, for comparing
// two worlds decoded (gob writes a map in whatever order it walks it, so
// two equal worlds need not be equal bytes).
func gobCopy(t *testing.T, w *game.World) *game.World {
	t.Helper()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(w); err != nil {
		t.Fatal(err)
	}
	var out game.World
	if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return &out
}

// ---- Keys and days ----

// key is the tea.KeyMsg for a key as the key table names it: one rune,
// or a named key (enter, esc, tab, the arrows, shift+tab, pgdown,
// pgup); any other string goes as runes, which msg.String() reads back
// as the string (so key("ctrl+c") reaches the handler as ctrl+c).
func key(s string) tea.KeyMsg {
	if len(s) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// endDay presses n and, if the news sim dealt a dilemma card overnight,
// answers it with the highlighted choice and reads the outcome, so the
// caller lands on the morning report the way it did before cards. Cards
// come at the seed's whim from day 5 on, and test seeds are wall-clock.
func endDay(t *testing.T, m *Model) {
	t.Helper()
	m.Update(key("n"))
	skipScene(m)
	if m.mode == modeStage {
		// A tier entered overnight (#149) opens its stage before the card.
		assertFits(t, m.View(), m.width, m.height, "stage")
		m.Update(key("enter"))
		skipScene(m)
	}
	if m.mode == modeCard {
		assertFits(t, m.View(), m.width, m.height, "dilemma card")
		m.Update(key("1")) // a digit picks, enter decides (#461)
		m.Update(key("enter"))
		if m.mode != modeCard || !m.cardDone {
			t.Fatalf("answering the card: mode %v done %v", m.mode, m.cardDone)
		}
		assertFits(t, m.View(), m.width, m.height, "dilemma outcome")
		m.Update(key("enter"))
	}
}

// skipScene ends the interstitial a morning opened on, if one is up (a
// fixture with animation on: the card's scene, #154), so the keys
// after it are the modal's; any key does, consumed, so esc it is. A
// report's scene resolves and holds instead (#203): the keys after it
// are the report's too, and its close ends the hold.
func skipScene(m *Model) {
	if m.scene != nil && !m.scene.Idle && !m.scene.Holding() {
		m.Update(key("esc"))
	}
}

// fast presses F and runs up to days days.
func fast(t *testing.T, m *Model, days int) {
	t.Helper()
	m.Update(key("F"))
	if m.mode != modeConfirmFast {
		t.Fatalf("F: mode %v, status %q", m.mode, m.status)
	}
	m.amt.SetValue(strconv.Itoa(days))
	m.Update(key("enter"))
}

// closeMorning answers a card with its first choice and closes the
// outcome and the report, so the next key lands on the play screen.
func closeMorning(t *testing.T, m *Model) {
	t.Helper()
	if m.mode == modeStage {
		m.Update(key("enter"))
	}
	if m.mode == modeCard {
		m.Update(key("1"))
		m.Update(key("enter"))
		m.Update(key("enter"))
	}
	if m.mode == modeReport {
		m.Update(key("enter"))
	}
	if m.mode != modePlay {
		t.Fatalf("after the morning: mode %v", m.mode)
	}
}

// ---- Reading a frame ----

// stripANSI removes escape sequences so a render can be read as text.
func stripANSI(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			in = true
		case in && r == 'm':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// viewLines is the view's rows, ANSI stripped: row 0 the title bar, row
// h-1 the status bar.
func viewLines(m *Model) []string {
	return strings.Split(stripANSI(m.View()), "\n")
}

// splitView splits a play-mode frame into MAIN and the pane, row by row,
// ANSI stripped (#85): the body rows between the title bar and the
// status bar (the details strip left out under paneMinWidth), each row's
// MAIN cells and, where the pane sits beside MAIN, its last paneWidth
// cells, borders and padding kept. It is the one reader of a frame's
// parts; mainText and paneRender are its halves as text.
func splitView(m *Model) (main, pane []string) {
	ls := strings.Split(m.View(), "\n")
	end := len(ls) - 1
	if !m.paneShown() && m.width < paneMinWidth {
		end-- // the strip
	}
	for i := 1; i < end; i++ {
		rs := []rune(stripANSI(ls[i]))
		if m.paneShown() && len(rs) >= paneWidth {
			pane = append(pane, string(rs[len(rs)-paneWidth:]))
			rs = rs[:len(rs)-paneWidth]
		}
		main = append(main, string(rs))
	}
	return main, pane
}

// mainText is MAIN's text read off a view: splitView's MAIN rows, each
// trimmed on the right.
func mainText(m *Model) string {
	main, _ := splitView(m)
	for i, l := range main {
		main[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(main, "\n")
}

// paneRender is the details pane's text read off a view at a width from
// the pane's: splitView's pane rows, borders trimmed.
func paneRender(m *Model) string {
	_, pane := splitView(m)
	for i, l := range pane {
		pane[i] = strings.TrimSpace(strings.Trim(l, "│╭╮╰╯─ "))
	}
	return strings.Join(pane, "\n")
}

// paneText is the pane's sections rendered plain, one line each, for
// asserting what the inspector carries; it reads m.details(), not the
// frame, so it holds at any width.
func paneText(m *Model) string {
	var ls []string
	for _, s := range m.details() {
		ls = append(ls, s.title)
		for _, l := range s.lines {
			ls = append(ls, stripANSI(l))
		}
	}
	return strings.Join(ls, "\n")
}

// paneProse is paneText with each section's lines run together, so a
// wrapped note reads whole.
func paneProse(m *Model) string {
	var b strings.Builder
	for _, s := range m.details() {
		b.WriteString(s.title + "\n")
		var ls []string
		for _, l := range s.lines {
			ls = append(ls, strings.TrimSpace(stripANSI(l)))
		}
		b.WriteString(spaces.ReplaceAllString(strings.Join(ls, " "), " ") + "\n")
	}
	return b.String()
}

// stripLine is the details strip of a view at a width under the pane's:
// row h-2, `▸ …  ␣ more`.
func stripLine(m *Model) string {
	return strings.TrimRight(viewLines(m)[m.height-2], " ")
}

// footerKeys is the modal's footer as `key label` pairs, for reading
// what a step lists.
func footerKeys(m *Model) string {
	var keys []string
	for _, b := range m.modalFooter() {
		keys = append(keys, b.key+" "+b.label)
	}
	return strings.Join(keys, "  ")
}

// modalBox finds the modal in a view: its top border's line index, its
// width, and its lines from the top border to the bottom one.
func modalBox(t *testing.T, view string) (top, width int, box []string) {
	t.Helper()
	ls := strings.Split(view, "\n")
	top = -1
	for i, l := range ls {
		p := strings.TrimSpace(stripANSI(l))
		switch {
		case strings.HasPrefix(p, "╔"):
			if top >= 0 {
				t.Fatalf("two modals in the view:\n%s", stripANSI(view))
			}
			top, width = i, lipgloss.Width(p)
		case strings.HasPrefix(p, "╚"):
			if top < 0 {
				t.Fatalf("a bottom border with no top:\n%s", stripANSI(view))
			}
			box = ls[top : i+1]
			return top, width, box
		}
	}
	t.Fatalf("no modal in the view:\n%s", stripANSI(view))
	return
}

// reportLine is the first line of the report modal's body.
func reportLine(t *testing.T, m *Model) string {
	t.Helper()
	if m.mode != modeReport {
		t.Fatalf("mode %v, not the report: status %q", m.mode, m.status)
	}
	_, _, box := modalBox(t, m.View())
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(stripANSI(box[3])), "║"))
}

// assertFits fails a view taller than h or with a row wider than w, and
// a modal wider than the screen, which wraps its border.
func assertFits(t *testing.T, view string, w, h int, what string) {
	t.Helper()
	ls := strings.Split(view, "\n")
	if len(ls) > h {
		t.Errorf("%s: %d lines > height %d", what, len(ls), h)
	}
	for i, l := range ls {
		if lw := lipgloss.Width(l); lw > w {
			t.Errorf("%s: line %d is %d cells wide > %d: %q", what, i, lw, w, l)
		}
		// A modal wider than the screen wraps its border onto the next
		// line: the top-right corner then starts a line of its own, a
		// one-cell overflow leaves a lone corner, and a body line loses
		// its closing bar.
		p := strings.TrimSpace(stripANSI(l))
		switch {
		case strings.HasPrefix(p, "═") && strings.HasSuffix(p, "╗") && !strings.HasPrefix(p, "╔"),
			p == "╗" || p == "╝",
			strings.Contains(p, "║") && !strings.HasSuffix(p, "║"):
			t.Errorf("%s: a modal wider than %d wrapped at line %d: %q", what, w, i, p)
		}
	}
}
