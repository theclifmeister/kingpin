package anim

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// TestSequence: two parts in boxes play one after the other, the first
// holding its last frame while the second runs, the second drawing
// nothing before its turn; Done at the sum; the frame at the end is
// both settled, exactly what each part settles to on its own, flush
// left where the box is the text's size; a part after a still (Over 0)
// starts at once; and an X indents.
func TestSequence(t *testing.T) {
	title, prose := NewText("TITLE"), NewText("one line\nand two")
	a := Decrypt(title, theme.Money, 600*time.Millisecond, Seed(1, 1, "a"))
	b := Wipe(prose, theme.Text, 400*time.Millisecond, Seed(1, 1, "b"))
	s := Sequence(
		Part{Scene: a, Over: 600 * time.Millisecond, W: title.Width(), H: 1},
		Part{Scene: b, Over: 400 * time.Millisecond, Y: 2, W: prose.Width(), H: 2},
	)
	if s.Done(time.Second-Frame) || !s.Done(time.Second) {
		t.Fatal("Done is not at the sum of the lengths")
	}
	w, h := 20, 4
	tw, pw := title.Width(), prose.Width()
	for at := time.Duration(0); at < 600*time.Millisecond; at += Frame {
		f := s.Frame(at, w, h)
		if len(f) != h {
			t.Fatalf("%v: %d lines, want %d", at, len(f), h)
		}
		if f[0] != a.Frame(at, tw, 1)[0] {
			t.Errorf("%v: the first box is not the first part's frame", at)
		}
		if f[1] != "" || f[2] != "" || f[3] != "" {
			t.Errorf("%v: a part yet to start drew something: %q", at, f[1:])
		}
	}
	for at := 600 * time.Millisecond; at <= time.Second; at += Frame {
		f := s.Frame(at, w, h)
		if f[0] != a.Frame(600*time.Millisecond, tw, 1)[0] {
			t.Errorf("%v: the first part did not hold its last frame", at)
		}
		if bf := b.Frame(at-600*time.Millisecond, pw, 2); f[2] != bf[0] || f[3] != bf[1] {
			t.Errorf("%v: the second box is not the second part's frame from its own start", at)
		}
	}
	end := s.Frame(time.Second, w, h)
	want := append(Still(title, theme.Money).Frame(0, tw, 1), "")
	want = append(want, Still(prose, theme.Text).Frame(0, pw, 2)...)
	if strings.Join(end, "\n") != strings.Join(want, "\n") {
		t.Errorf("the end is not every part settled:\n%q\n%q", end, want)
	}
	if got := stripANSI(strings.Join(end, "\n")); got != "TITLE\n\none line\nand two" {
		t.Errorf("the boxes are not flush left: %q", got)
	}
	// A still first, over 0: the next part starts at t = 0, indented by
	// its X. A box past the canvas is cut, and a canvas of no rows is
	// no lines.
	s = Sequence(Part{Scene: Still(title, theme.Money), W: tw, H: 1}, Part{Scene: b, Over: 400 * time.Millisecond, X: 3, Y: 3, W: pw, H: 2})
	at := 200 * time.Millisecond
	if f := s.Frame(at, w, 4); f[0] != want[0] || f[3] != "   "+b.Frame(at, pw, 2)[0] || f[2] != "" {
		t.Errorf("a still did not give way at once, or X did not indent: %q", f)
	}
	if f := s.Frame(time.Second, 6, 5); len(f) != 5 || stripANSI(f[0]) != "TITLE" || lipgloss.Width(f[3]) > 6 || lipgloss.Width(f[4]) > 6 {
		t.Errorf("a box past the right edge was not cut to the canvas: %q", f)
	}
	if f := s.Frame(0, w, 0); len(f) != 0 {
		t.Errorf("no rows drew %d lines", len(f))
	}
	if !s.Done(400*time.Millisecond) || s.Done(399*time.Millisecond) {
		t.Error("Done is not the second part's length")
	}
}
