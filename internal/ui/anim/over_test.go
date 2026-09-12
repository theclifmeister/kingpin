package anim

import (
	"strings"
	"testing"
	"time"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// TestLayerHeld (#156): a layer paints later scenes over earlier ones
// cell by cell and is done when all are; Held is a scene's frame at
// one moment, done at once. (Sequence is #157's, TestSequence.)
func TestLayerHeld(t *testing.T) {
	// A layer: the text over the curtain, the text's cells winning.
	bars := Curtain(Text{}, theme.Heat, time.Second, nil)
	word := Still(NewText("XY"), theme.Text)
	l := Layer(Held(bars, time.Second), word)
	if !l.Done(0) || plain(l.Frame(0, 4, 1)) != "█XY█\n" {
		t.Errorf("the word over the held curtain: %q done %v", plain(l.Frame(0, 4, 1)), l.Done(0))
	}
	l = Layer(bars, word)
	if l.Done(time.Second-Frame) || !l.Done(time.Second) {
		t.Error("a layer is done when its slowest scene is")
	}
	if strings.TrimSpace(plain(l.Frame(0, 4, 2))) != "XY" {
		t.Errorf("at t = 0 the curtain has not fallen and the word stands: %q", plain(l.Frame(0, 4, 2)))
	}
	// Held: the curtain half fallen, held there.
	h := Held(bars, 500*time.Millisecond)
	if !h.Done(0) || plain(h.Frame(0, 2, 4)) != plain(bars.Frame(500*time.Millisecond, 2, 4)) || plain(h.Frame(time.Hour, 2, 4)) != plain(h.Frame(0, 2, 4)) {
		t.Error("Held is not the frame at the moment, held")
	}
}

// TestEndingScenes (#156): each ending's scene is OverLength long,
// opens and closes as its cause says, and fits the modal's canvas.
func TestEndingScenes(t *testing.T) {
	in := Indicted(7, "Indictment lands", Seed(1, 1, "over"))
	ar := Arrested(Seed(1, 1, "over"))
	br := Broke("peak cash  $1\nwages      $2\n \nMarco  Dee", Seed(1, 1, "over"))
	for name, s := range map[string]Scene{"indicted": in, "arrested": ar, "broke": br} {
		if s.Done(OverLength-Frame) || !s.Done(OverLength) {
			t.Errorf("%s: not done at OverLength", name)
		}
		for _, at := range []time.Duration{0, OverLength / 3, OverLength / 2, OverLength} {
			if f := s.Frame(at, 72, 15); len(f) != 15 {
				t.Errorf("%s at %v: %d lines", name, at, len(f))
			}
		}
	}
	// Indicted: the file's pages are all there before the stamp lands,
	// and the stamp lies across the file at the end.
	if f := plain(in.Frame(overPrint, 72, 15)); !strings.Contains(f, "page 1") || !strings.Contains(f, "page 7") {
		t.Errorf("indicted at the end of the print:\n%s", f)
	}
	if f := plain(in.Frame(OverLength, 72, 15)); !strings.Contains(f, "Indictment lands") || !strings.Contains(f, "page 1") || !strings.Contains(f, "page 7") {
		t.Errorf("indicted at the end:\n%s", f)
	}
	if f := plain(Indicted(0, "x", Seed(1, 1, "over")).Frame(OverLength, 72, 15)); !strings.Contains(f, "x") || strings.Contains(f, "page 2") {
		t.Errorf("a file of no pages prints one:\n%s", f)
	}
	if f := plain(Indicted(40, "x", Seed(1, 1, "over")).Frame(OverLength, 72, 15)); strings.Contains(f, "page 13") {
		t.Errorf("a file deeper than OverPagesMax printed past it:\n%s", f)
	}
	// Arrested: bars over the whole frame once fallen, the word on
	// them, and the bars lifted at the end with the word standing.
	if f := plain(ar.Frame(overFall, 72, 15)); strings.Count(f, " ") != 0 || len([]rune(f)) != 72*15+15 {
		t.Errorf("arrested: the curtain has not covered the frame at %v:\n%s", overFall, f)
	}
	if f := plain(ar.Frame(overFall+overWord/2, 72, 15)); strings.Count(f, " ") != 0 {
		t.Errorf("arrested: a gap in the bars under the word at %v:\n%s", overFall+overWord/2, f)
	}
	if f := plain(ar.Frame(OverLength, 72, 15)); strings.Count(f, "█") != strings.Count(Arrest, "█") {
		t.Errorf("arrested at the end: %d blocks, want the word's %d", strings.Count(f, "█"), strings.Count(Arrest, "█"))
	}
	if tx := NewText(Arrest); tx.Height() != 6 || tx.Width() != 60 {
		t.Errorf("the word's art is %dx%d", tx.Width(), tx.Height())
	}
	// Broke: the figures stand at the start, are gone at the fall's
	// end, and the nought stands at the end.
	if f := plain(br.Frame(0, 72, 15)); !strings.Contains(f, "peak cash  $1") || !strings.Contains(f, "Marco  Dee") {
		t.Errorf("broke at the start:\n%s", f)
	}
	if f := plain(br.Frame(overLoss, 72, 15)); strings.TrimSpace(f) != "" {
		t.Errorf("broke: the figures are still there at the end of the fall:\n%s", f)
	}
	if f, want := plain(br.Frame(OverLength, 72, 15)), plain(Still(NewText(Zero), theme.Heat).Frame(0, 72, 15)); f != want {
		t.Errorf("broke at the end:\n%s", f)
	}
}
