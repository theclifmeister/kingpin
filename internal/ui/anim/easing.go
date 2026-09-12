package anim

import "math"

// Easing maps progress in [0, 1] to eased progress: how a movement
// starts and stops. The table is TTE's (NOTICE; ttfx's
// src/utils/easing.rs is the reference, the standard CSS curves): in,
// out and in-out for sine, quad, cubic, expo, elastic and bounce, and
// Linear. An effect picks the curve the original uses where the shape
// depends on it and a gentler one where the frame rate does (a wipe
// that eases in over thirty frames reveals nothing for the first
// several).
type Easing func(p float64) float64

// Linear is no easing.
func Linear(p float64) float64 { return p }

func InSine(p float64) float64    { return 1 - math.Cos(p*math.Pi/2) }
func OutSine(p float64) float64   { return math.Sin(p * math.Pi / 2) }
func InOutSine(p float64) float64 { return -(math.Cos(math.Pi*p) - 1) / 2 }

func InQuad(p float64) float64  { return p * p }
func OutQuad(p float64) float64 { return 1 - (1-p)*(1-p) }
func InOutQuad(p float64) float64 {
	if p < 0.5 {
		return 2 * p * p
	}
	return 1 - math.Pow(-2*p+2, 2)/2
}

func InCubic(p float64) float64  { return p * p * p }
func OutCubic(p float64) float64 { return 1 - math.Pow(1-p, 3) }
func InOutCubic(p float64) float64 {
	if p < 0.5 {
		return 4 * p * p * p
	}
	return 1 - math.Pow(-2*p+2, 3)/2
}

func InExpo(p float64) float64 {
	if p == 0 {
		return 0
	}
	return math.Pow(2, 10*p-10)
}
func OutExpo(p float64) float64 {
	if p == 1 {
		return 1
	}
	return 1 - math.Pow(2, -10*p)
}
func InOutExpo(p float64) float64 {
	switch {
	case p == 0:
		return 0
	case p == 1:
		return 1
	case p < 0.5:
		return math.Pow(2, 20*p-10) / 2
	}
	return (2 - math.Pow(2, -20*p+10)) / 2
}

func InElastic(p float64) float64 {
	const c4 = 2 * math.Pi / 3
	switch p {
	case 0:
		return 0
	case 1:
		return 1
	}
	return -math.Pow(2, 10*p-10) * math.Sin((p*10-10.75)*c4)
}
func OutElastic(p float64) float64 {
	const c4 = 2 * math.Pi / 3
	switch p {
	case 0:
		return 0
	case 1:
		return 1
	}
	return math.Pow(2, -10*p)*math.Sin((p*10-0.75)*c4) + 1
}
func InOutElastic(p float64) float64 {
	const c5 = 2 * math.Pi / 4.5
	switch {
	case p == 0:
		return 0
	case p == 1:
		return 1
	case p < 0.5:
		return -(math.Pow(2, 20*p-10) * math.Sin((20*p-11.125)*c5)) / 2
	}
	return (math.Pow(2, -20*p+10)*math.Sin((20*p-11.125)*c5))/2 + 1
}

func InBounce(p float64) float64  { return 1 - OutBounce(1-p) }
func OutBounce(p float64) float64 { return outBounce(p) }
func InOutBounce(p float64) float64 {
	if p < 0.5 {
		return (1 - outBounce(1-2*p)) / 2
	}
	return (1 + outBounce(2*p-1)) / 2
}

func outBounce(p float64) float64 {
	const n1, d1 = 7.5625, 2.75
	switch {
	case p < 1/d1:
		return n1 * p * p
	case p < 2/d1:
		p -= 1.5 / d1
		return n1*p*p + 0.75
	case p < 2.5/d1:
		p -= 2.25 / d1
		return n1*p*p + 0.9375
	}
	p -= 2.625 / d1
	return n1*p*p + 0.984375
}

// clamp01 holds p to [0, 1].
func clamp01(p float64) float64 { return math.Max(0, math.Min(1, p)) }
