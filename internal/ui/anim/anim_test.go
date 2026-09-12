package anim

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// The canvas renders runs of one colour through theme.Fg, plain cells
// plain, trailing blanks dropped, and drops what is set outside it.
func TestCanvasRuns(t *testing.T) {
	c := NewCanvas(6, 2)
	c.Set(0, 0, 'a', theme.Money)
	c.Set(1, 0, 'b', theme.Money)
	c.Set(2, 0, 'c', "")
	c.Set(4, 0, 'd', theme.Dim)
	c.Set(9, 1, 'x', theme.Dim) // outside: dropped
	c.Set(-1, 0, 'x', theme.Dim)
	ls := c.Lines()
	if len(ls) != 2 || ls[1] != "" {
		t.Fatalf("lines: %q", ls)
	}
	want := theme.Fg(theme.Money).Render("ab") + "c " + theme.Fg(theme.Dim).Render("d")
	if ls[0] != want {
		t.Errorf("row 0:\n%q\nwant\n%q", ls[0], want)
	}
	if lipgloss.Width(ls[0]) != 5 {
		t.Errorf("row 0 is %d wide", lipgloss.Width(ls[0]))
	}
}

// The text is centred, its cells read in order, blanks are no cell.
func TestTextCentres(t *testing.T) {
	tx := NewText("\nab\n c\n")
	if tx.Width() != 2 || tx.Height() != 2 {
		t.Fatalf("size %dx%d", tx.Width(), tx.Height())
	}
	if x, y := tx.Origin(80, 24); x != 39 || y != 11 {
		t.Errorf("origin %d,%d", x, y)
	}
	cells := tx.Cells()
	if len(cells) != 3 || cells[0] != (Cell{0, 0, 'a'}) || cells[2] != (Cell{1, 1, 'c'}) {
		t.Errorf("cells %+v", cells)
	}
}

// Seed is one stream per (seed, day, name), and never the game's.
func TestSeedIsItsOwn(t *testing.T) {
	a, b := Seed(1, 2, "title"), Seed(1, 2, "title")
	if a.Uint64() != b.Uint64() {
		t.Error("the same seed, day and name gave different streams")
	}
	seen := map[uint64]bool{}
	for _, r := range []struct {
		seed uint64
		day  int
		name string
	}{{1, 2, "title"}, {2, 2, "title"}, {1, 3, "title"}, {1, 2, "card"}} {
		v := Seed(r.seed, r.day, r.name).Uint64()
		if seen[v] {
			t.Errorf("%+v repeats another stream's first draw", r)
		}
		seen[v] = true
	}
}

// Decrypt resolves the text in its length: nothing at t = 0 but the
// first typed cells, ciphertext in Dim on the way, the text itself in
// the accent at the end and after, and every frame the text's height
// and no wider than the canvas.
func TestDecryptResolves(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0) // termenv.TrueColor: the colours are the point
	defer lipgloss.SetColorProfile(profile)
	art := "KIN\nGPI"
	s := Decrypt(NewText(art), theme.Money, time.Second, Seed(1, 0, "test"))
	gold := theme.Fg(theme.Money)
	if s.Done(time.Second-Frame) || !s.Done(time.Second) {
		t.Error("Done is not at the length")
	}
	plain := func(ls []string) string {
		var b strings.Builder
		for _, l := range ls {
			b.WriteString(stripANSI(l) + "\n")
		}
		return b.String()
	}
	end := s.Frame(time.Second, 20, 4)
	if len(end) != 4 {
		t.Fatalf("%d lines", len(end))
	}
	if p := plain(end); !strings.Contains(p, "KIN") || !strings.Contains(p, "GPI") {
		t.Errorf("the text did not settle:\n%s", p)
	}
	if !strings.Contains(end[1], gold.Render("KIN")) || !strings.Contains(end[2], gold.Render("GPI")) {
		t.Errorf("the settled text is not one run of the accent: %q", end)
	}
	if got := s.Frame(3*time.Second, 20, 4); strings.Join(got, "\n") != strings.Join(end, "\n") {
		t.Error("the frame moves after the end")
	}
	// On the way: something in Dim, and the flash in Text once a cell settles.
	dim, flashed := false, false
	for at := time.Duration(0); at < time.Second; at += Frame {
		f := strings.Join(s.Frame(at, 20, 4), "\n")
		dim = dim || strings.Contains(f, "\x1b[38;2;108;108;108m")
		flashed = flashed || strings.Contains(f, theme.Fg(theme.Text).Render("K"))
		for _, l := range s.Frame(at, 20, 4) {
			if lipgloss.Width(l) > 20 {
				t.Errorf("at %v a line is %d wide", at, lipgloss.Width(l))
			}
		}
	}
	if !dim || !flashed {
		t.Errorf("no ciphertext in Dim (%v) or no flash in Text (%v) on the way", dim, flashed)
	}
	// A canvas too small cuts the text rather than wrapping it.
	for _, l := range s.Frame(time.Second, 2, 1) {
		if lipgloss.Width(l) > 2 {
			t.Errorf("a 2x1 canvas drew %q", l)
		}
	}
}

// The player: an interstitial is over on Done; a loop rests, then
// restarts on Next with the clock from zero, and its pass counts.
func TestPlayerLoops(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	p := &Player{Scene: Decrypt(NewText("A"), theme.Money, 100*time.Millisecond, Seed(1, 0, "t"))}
	if p.Tick(now) || p.T() != 0 {
		t.Fatal("the first tick is over, or not at zero")
	}
	if p.Tick(now.Add(50 * time.Millisecond)) {
		t.Fatal("over before Done")
	}
	if !p.Tick(now.Add(100 * time.Millisecond)) {
		t.Fatal("not over at Done")
	}
	passes := 0
	loop := &Player{
		Scene: Decrypt(NewText("A"), theme.Money, 100*time.Millisecond, Seed(1, 0, "t")),
		Idle:  true,
		Rest:  time.Second,
		Next: func(pass int) Scene {
			passes = pass
			return Decrypt(NewText("A"), theme.Money, 100*time.Millisecond, Seed(1, pass, "t"))
		},
	}
	for at := time.Duration(0); at < 1100*time.Millisecond; at += Frame {
		if loop.Tick(now.Add(at)) {
			t.Fatalf("the loop was over at %v", at)
		}
	}
	if passes != 0 || loop.Pass() != 0 {
		t.Fatalf("restarted inside the rest: pass %d", passes)
	}
	if loop.Tick(now.Add(1200*time.Millisecond)) || passes != 1 || loop.Pass() != 1 || loop.T() != 0 {
		t.Fatalf("after the rest: pass %d T %v", passes, loop.T())
	}
	if len(loop.Frame(10, 1)) != 1 {
		t.Error("Frame is not the scene's")
	}
	// Next returning nil ends the loop, once the pass has rested.
	loop.Next = func(int) Scene { return nil }
	if loop.Tick(now.Add(1400*time.Millisecond)) || !loop.Tick(now.Add(3*time.Second)) {
		t.Error("a loop whose Next gives nothing is over before its rest, or not after")
	}
}

// The registry names every scene once and each makes on a seed.
func TestScenesRegistry(t *testing.T) {
	names := map[string]bool{}
	for _, sc := range Scenes() {
		if names[sc.Name] || sc.Name == "" || sc.New == nil || sc.New(1) == nil {
			t.Errorf("registry entry %q is bad or repeated", sc.Name)
		}
		names[sc.Name] = true
	}
	if !names["title"] {
		t.Error("the title is not registered")
	}
	if tx := NewText(Kingpin); tx.Height() != 6 || tx.Width() > 50 {
		t.Errorf("the art is %dx%d, want 6 rows and at most 50 columns", tx.Width(), tx.Height())
	}
}

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
