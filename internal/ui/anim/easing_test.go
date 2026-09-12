package anim

import (
	"math"
	"testing"
)

// The easing table against a few known values of the standard curves:
// every curve is 0 at 0 and 1 at 1, the in curves are under the line
// and the out curves over it at a quarter, and the midpoints are the
// ones the formulas give.
func TestEasing(t *testing.T) {
	curves := map[string]Easing{
		"linear":  Linear,
		"in_sine": InSine, "out_sine": OutSine, "in_out_sine": InOutSine,
		"in_quad": InQuad, "out_quad": OutQuad, "in_out_quad": InOutQuad,
		"in_cubic": InCubic, "out_cubic": OutCubic, "in_out_cubic": InOutCubic,
		"in_expo": InExpo, "out_expo": OutExpo, "in_out_expo": InOutExpo,
		"in_elastic": InElastic, "out_elastic": OutElastic, "in_out_elastic": InOutElastic,
		"in_bounce": InBounce, "out_bounce": OutBounce, "in_out_bounce": InOutBounce,
	}
	for name, f := range curves {
		if f(0) != 0 || math.Abs(f(1)-1) > 1e-12 {
			t.Errorf("%s: ends at %v and %v", name, f(0), f(1))
		}
		q := f(0.25)
		switch name[:3] {
		case "in_":
			if name[:6] != "in_out" && q >= 0.25 {
				t.Errorf("%s(0.25) = %v, not under the line", name, q)
			}
		case "out":
			if q <= 0.25 {
				t.Errorf("%s(0.25) = %v, not over the line", name, q)
			}
		}
	}
	near := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 1e-6 {
			t.Errorf("%s = %.6f, want %.6f", name, got, want)
		}
	}
	near("in_quad(0.5)", InQuad(0.5), 0.25)
	near("out_quad(0.5)", OutQuad(0.5), 0.75)
	near("in_out_quad(0.25)", InOutQuad(0.25), 0.125)
	near("in_out_quad(0.75)", InOutQuad(0.75), 0.875)
	near("in_cubic(0.5)", InCubic(0.5), 0.125)
	near("in_out_cubic(0.5)", InOutCubic(0.5), 0.5)
	near("in_sine(0.5)", InSine(0.5), 1-math.Sqrt2/2)
	near("in_out_sine(0.5)", InOutSine(0.5), 0.5)
	near("in_expo(0.5)", InExpo(0.5), 1.0/32)
	near("out_expo(0.5)", OutExpo(0.5), 1-1.0/32)
	near("out_bounce(0.5)", OutBounce(0.5), 0.765625)
	near("in_out_bounce(0.5)", InOutBounce(0.5), 0.5)
	near("out_elastic(0.5)", OutElastic(0.5), 1.015625)
	near("in_elastic(0.5)", InElastic(0.5), -0.015625)
	if InOutElastic(0.5) != 0.5 && math.Abs(InOutElastic(0.5)-0.5) > 1e-6 {
		t.Errorf("in_out_elastic(0.5) = %v", InOutElastic(0.5))
	}
}
