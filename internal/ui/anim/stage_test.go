package anim

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// TestStageScene: the frame is the modal's rows, the title on line 0
// and the body under it; the title is printing over the first third,
// the body blank until the last, and the frame at the end is the title
// in the accent over the body plain, every line the one handed in.
func TestStageScene(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0)
	defer lipgloss.SetColorProfile(profile)
	title, body := "STAGE 2 · CREW", []string{"A blurb.", "Some prose.", "", "OPENED", "  a thing", "", "NEXT", "The next line."}
	s := Stage(title, body, theme.Money, Seed(1, 3, "stage"))
	h := 1 + len(body)
	third := StageLength / 3
	if s.Done(StageLength-Frame) || !s.Done(StageLength) {
		t.Error("Done is not at StageLength")
	}
	for _, at := range []time.Duration{0, third / 2, third, 2*third - Frame} {
		f := s.Frame(at, 72, h)
		if len(f) != h {
			t.Fatalf("at %v: %d lines, want %d", at, len(f), h)
		}
		for i, l := range f[1:] {
			if l != "" {
				t.Errorf("at %v: body line %d is %q before the wipe", at, i, stripANSI(l))
			}
		}
	}
	if got := stripANSI(s.Frame(third/2, 72, h)[0]); got == "" || got == title {
		t.Errorf("at half the print the title reads %q", got)
	}
	end := s.Frame(StageLength, 72, h)
	if stripANSI(end[0]) != title || !strings.Contains(end[0], theme.Fg(theme.Money).Render("STAGE")) {
		t.Errorf("the settled title is %q", end[0])
	}
	for i, l := range end[1:] {
		if l != body[i] {
			t.Errorf("the settled body line %d is %q, want %q", i, l, body[i])
		}
	}
	// A short frame is cut, a tall one padded.
	if f := s.Frame(StageLength, 72, 3); len(f) != 3 || strings.TrimSpace(stripANSI(f[0])) != title {
		t.Errorf("cut to 3 lines: %q", f)
	}
	if f := s.Frame(StageLength, 72, 20); len(f) != 20 || f[19] != "" {
		t.Errorf("padded to 20 lines: %d, last %q", len(f), f[19])
	}
	// Two seeds throw the beams' dice differently; the same seed the same.
	same, differ := true, false
	for at := time.Duration(0); at < StageLength; at += Frame {
		a := strings.Join(Stage(title, body, theme.Money, Seed(1, 3, "stage")).Frame(at, 72, h), "\n")
		b := strings.Join(Stage(title, body, theme.Money, Seed(2, 3, "stage")).Frame(at, 72, h), "\n")
		same = same && a == strings.Join(s.Frame(at, 72, h), "\n")
		differ = differ || a != b
	}
	if !same || !differ {
		t.Errorf("same seed same frames %v, other seed other frames %v", same, differ)
	}
}
