package anim

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// TestMorning: the counter reads yesterday's number at the first frame
// and today's at the end, with the changing digit off the row between
// (the roll), at today's width; the title row is empty at the first
// frame and the whole title in theme.Money at the end; the frame is h
// rows whatever h; Done at MorningLength; and the same on every seed,
// the slide and the wipe throwing none.
func TestMorning(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0)
	defer lipgloss.SetColorProfile(profile)
	s := Morning(41, 42, "MORNING REPORT · DAY 42", Seed(1, 0, "morning"))
	if s.Done(MorningLength-Frame) || !s.Done(MorningLength) {
		t.Errorf("the scene is not %v long", MorningLength)
	}
	first, last := s.Frame(0, 40, 2), s.Frame(MorningLength, 40, 2)
	if got := stripANSI(first[0]); got != "41" {
		t.Errorf("the first frame's counter is %q, want 41", got)
	}
	if got := stripANSI(last[0]); got != "42" {
		t.Errorf("the last frame's counter is %q, want 42", got)
	}
	if first[1] != "" {
		t.Errorf("the first frame's title is %q, want nothing", first[1])
	}
	if got := stripANSI(last[1]); got != "MORNING REPORT · DAY 42" {
		t.Errorf("the last frame's title is %q", got)
	}
	if !strings.Contains(last[1], theme.Fg(theme.Money).Render("MORNING")) {
		t.Errorf("the settled title is not in gold: %q", last[1])
	}
	rolled := false
	for at := Frame; at < MorningLength; at += Frame {
		f := s.Frame(at, 40, 2)
		if c := stripANSI(f[0]); c == "4" {
			rolled = true
		} else if c != "41" && c != "42" {
			t.Errorf("at %v the counter reads %q", at, c)
		}
		if w := lipgloss.Width(f[0]); w > 2 {
			t.Errorf("at %v the counter is %d wide", at, w)
		}
	}
	if !rolled {
		t.Error("the last digit never left the row: no roll")
	}
	// The number growing a digit: 9 to 10 at the new width.
	s = Morning(9, 10, "MORNING REPORT · DAY 10", nil)
	if got := stripANSI(s.Frame(MorningLength, 40, 2)[0]); got != "10" {
		t.Errorf("9 rolls to %q, want 10", got)
	}
	for _, h := range []int{0, 1, 2, 5} {
		if n := len(s.Frame(MorningLength/2, 40, h)); n != h {
			t.Errorf("%d rows asked, %d given", h, n)
		}
	}
	// No dice: another seed is the same scene.
	a, b := morningScene(3), morningScene(4)
	for at := time.Duration(0); at <= MorningLength; at += Frame {
		if strings.Join(a.Frame(at, 80, 2), "\n") != strings.Join(b.Frame(at, 80, 2), "\n") {
			t.Fatalf("at %v two seeds differ", at)
		}
	}
}
