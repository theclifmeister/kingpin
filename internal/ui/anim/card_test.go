package anim

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// TestCard: the card's scene is the title decrypting on row 0 over
// CardTitleLength, the prose wiping in from row 2 over CardProseLength,
// wrapped at the frame's width, flush left; the same seed the same
// frames, another seed other churn; the last frame the title in the
// accent and the prose plain, held.
func TestCard(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(profile)
	over := CardTitleLength + CardProseLength
	a, b, c := Card(sampleTitle, sampleText, Seed(1, 3, "card")), Card(sampleTitle, sampleText, Seed(1, 3, "card")), Card(sampleTitle, sampleText, Seed(2, 3, "card"))
	if a.Done(over-Frame) || !a.Done(over) {
		t.Fatal("Done is not the two lengths")
	}
	same, differ := true, false
	for at := time.Duration(0); at <= over; at += Frame {
		fa, fb, fc := a.Frame(at, 72, 8), b.Frame(at, 72, 8), c.Frame(at, 72, 8)
		same = same && strings.Join(fa, "\n") == strings.Join(fb, "\n")
		differ = differ || strings.Join(fa, "\n") != strings.Join(fc, "\n")
		if len(fa) != 8 || fa[1] != "" {
			t.Fatalf("%v: %d lines, row 1 %q", at, len(fa), fa[1])
		}
		if at < CardTitleLength && strings.Join(fa[2:], "") != "" {
			t.Errorf("%v: the prose is up before the title has decrypted", at)
		}
		if at >= CardTitleLength && stripANSI(fa[0]) != sampleTitle {
			t.Errorf("%v: the title is %q", at, stripANSI(fa[0]))
		}
	}
	if !same || !differ {
		t.Errorf("the same seed the same frames: %v; another seed other frames: %v", same, differ)
	}
	// The end: the title in the accent, the prose plain, at the width.
	end := a.Frame(over, 72, 8)
	if want := Still(NewText(sampleTitle), theme.Warn).Frame(0, len(sampleTitle), 1)[0]; end[0] != want {
		t.Errorf("the title settled as %q, not %q", end[0], want)
	}
	if got, want := strings.Join(end[2:], "\n"), Wrap(sampleText, 72)+"\n\n\n"; got != want {
		t.Errorf("the prose is\n%q\nnot\n%q", got, want)
	}
	if strings.Join(a.Frame(over+time.Second, 72, 8), "\n") != strings.Join(end, "\n") {
		t.Error("the end is not held")
	}
	// Wrapped at the frame's width: narrower is taller.
	lines := func(f []string) int {
		return strings.Count(strings.TrimRight(strings.Join(f[2:], "\n"), "\n"), "\n") + 1
	}
	if narrow := a.Frame(over, 40, 12); lines(narrow) <= lines(end) || lipgloss.Width(narrow[2]) > 40 {
		t.Errorf("the prose did not rewrap at 40: %q", narrow)
	}
}
