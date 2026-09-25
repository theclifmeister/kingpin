package sparkline

import "testing"

func TestRender(t *testing.T) {
	if got := Render([]float64{1, 2, 3, 4, 5, 6, 7, 8}, 8); got != "▁▂▃▄▅▆▇█" {
		t.Fatalf("got %q", got)
	}
	if got := Render([]float64{5, 5, 5}, 3); got != "▅▅▅" {
		t.Fatalf("flat: %q", got)
	}
	if got := Render([]float64{1, 2, 3, 4}, 2); got != "▁█" {
		t.Fatalf("window: %q", got)
	}
	// A move under MinSpan reads flat about the middle, not as a crash
	// (#473: a -0% move drew `█▁`); one over it uses the whole height.
	if got := Render([]float64{100, 99.6}, 2); got != "▅▄" {
		t.Fatalf("a small move: %q", got)
	}
	if got := Render([]float64{100, 80}, 2); got != "█▁" {
		t.Fatalf("a real fall: %q", got)
	}
	if Render(nil, 5) != "" || Render([]float64{1}, 0) != "" {
		t.Fatal("empty cases")
	}
}

func TestBar(t *testing.T) {
	got := Bar(0.5, 10, []float64{0.3, 0.75})
	if len([]rune(got)) != 10 {
		t.Fatalf("width: %q", got)
	}
	if got != "█████░░┆░░" {
		t.Fatalf("got %q", got)
	}
}
