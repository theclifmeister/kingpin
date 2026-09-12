package anim

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// TestBust: the title strobes red twice over the first steps and rests
// in gold; the level is on the tape from the first frame and settles
// at the report's indent in red; the loss starts burning at its moment
// and the whole line reads as the report's at the end; the frame is h
// rows whatever h; Done at BustLength; the stash raid glitches longer
// and shows one frame in purple; the dice tell two seeds apart.
func TestBust(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0)
	defer lipgloss.SetColorProfile(profile)
	const title, level, loss = "MORNING REPORT · DAY 42", "RAID", ": lost 40 Weed and $2,000 in Eastside"
	// A canvas renders runs between the blanks: the first word says the
	// colour.
	red, gold := theme.Fg(theme.Heat).Render("MORNING"), theme.Fg(theme.Money).Render("MORNING")
	s := Bust(title, level, loss, false, Seed(1, 0, "bust"))
	if s.Done(BustLength-Frame) || !s.Done(BustLength) {
		t.Errorf("the scene is not %v long", BustLength)
	}
	strobes := 0
	for step := 0; step < 4; step++ {
		f := s.Frame(time.Duration(step)*bustStrobe+Frame/2, 72, 2)
		if step%2 == 0 && strings.Contains(f[0], red) {
			strobes++
		} else if step%2 == 1 && !strings.Contains(f[0], gold) {
			t.Errorf("strobe step %d: the title is %q, not gold", step, f[0])
		}
	}
	if strobes != 2 {
		t.Errorf("the title strobed red %d times, want 2", strobes)
	}
	last := s.Frame(BustLength, 72, 2)
	if !strings.Contains(last[0], gold) || strings.Contains(last[0], red) {
		t.Errorf("the title rests in %q, not gold", last[0])
	}
	if got := stripANSI(last[1]); got != "  "+level+loss {
		t.Errorf("the last frame's line is %q", got)
	}
	if heat := strings.TrimSuffix(theme.Fg(theme.Heat).Render("R"), "R\x1b[0m"); !strings.Contains(last[1], heat+level) {
		t.Errorf("the level does not settle in red: %q", last[1])
	}
	before := stripANSI(s.Frame(bustBurnAt-Frame, 72, 2)[1])
	if strings.Contains(before, "lost") || strings.Contains(before, "Weed") {
		t.Errorf("the loss is up before it burns: %q", before)
	}
	if after := stripANSI(s.Frame(bustBurnAt+Frame, 72, 2)[1]); len(after) <= len(before) {
		t.Errorf("the loss is not on its line once it burns: %q", after)
	}
	for _, h := range []int{0, 1, 2, 5} {
		if n := len(s.Frame(BustLength/2, 72, h)); n != h {
			t.Errorf("%d rows asked, %d given", h, n)
		}
	}
	// The stash raid: the tape still torn where the plain raid's has
	// settled, and exactly one frame in purple.
	stash := Bust(title, level, loss, true, Seed(1, 0, "bust"))
	tape := func(sc Scene, at time.Duration) string { return stripANSI(sc.(*bust).level.Frame(at, 20, 1)[0]) }
	if at := bustGlitch + 2*Frame; tape(s, at) != tape(s, at+Frame) {
		t.Errorf("at %v the plain raid's tape is still moving", at)
	}
	purple, moving := 0, 0
	open := strings.TrimSuffix(theme.Fg(theme.Rivals).Render("R"), "R\x1b[0m")
	for at := time.Duration(0); at < BustLength; at += Frame {
		if strings.Contains(stash.Frame(at, 72, 2)[1], open) {
			purple++
		}
		if at > bustGlitch && at < bustGlitchStash && tape(stash, at) != tape(stash, at+Frame) {
			moving++
		}
	}
	if purple != 1 {
		t.Errorf("%d frames in purple, want 1", purple)
	}
	if moving == 0 {
		t.Error("the stash raid's tape settles as early as the plain raid's")
	}
	if got := stripANSI(stash.Frame(BustLength, 72, 2)[1]); got != "  "+level+loss {
		t.Errorf("the stash raid's last frame is %q", got)
	}
	// The dice.
	a, b := bustScene(3), bustScene(4)
	same := true
	for at := time.Duration(0); at < BustLength; at += Frame {
		same = same && strings.Join(a.Frame(at, 72, 2), "\n") == strings.Join(b.Frame(at, 72, 2), "\n")
	}
	if same {
		t.Error("two seeds rendered the same frames")
	}
}
