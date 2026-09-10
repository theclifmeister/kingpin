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
