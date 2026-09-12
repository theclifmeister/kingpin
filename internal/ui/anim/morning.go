package anim

import (
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// Morning is the day rolling over (#159), the most frequent scene and
// so the shortest: the title bar's day counter slides from yesterday's
// number to today's, the old digits rolling up and out of the bar as
// the new roll in from below (a slide reversed under a slide, on a
// canvas three rows tall of which the bar shows the middle, so a digit
// that does not change stands still as an odometer's does), and the
// report's title, `MORNING REPORT · DAY 42`, wipes in from the left,
// MorningLength in all. The frame is two rows, each on a canvas of its
// own width so both sit flush left where they are drawn: row 0 the
// counter, today's digits wide, which the model puts in the title bar
// after `Day `; row 1 the title row at the width given, which the
// model puts in the report's title bar. The counter settles plain, as
// the bar draws the day, and the title in theme.Money, the modal's
// title colour. No dice: the slide and the wipe throw none, so the
// registry's entry says so.
func Morning(from, to int, title string, rng *rand.Rand) Scene {
	old, now, head := NewText(strconv.Itoa(from)), NewText(strconv.Itoa(to)), NewText(title)
	return &morning{
		digits: now.Width(),
		roll: Layer(
			Reverse(SlideFrom(Down))(old, "", MorningLength, rng),
			SlideFrom(Up)(now, "", MorningLength, rng),
		),
		title: WipeFrom(Right)(head, theme.Money, MorningLength, rng),
		width: head.Width(),
	}
}

// MorningLength is how long the morning's scene runs, the counter and
// the title together.
const MorningLength = 250 * time.Millisecond

// rollRows is the counter's canvas: the row the bar shows and one above
// and below it for the digits on their way.
const rollRows = 3

type morning struct {
	digits int   // the counter's width: today's digits
	roll   Scene // the counter: the old number leaving over the new arriving
	title  Scene // the report's title row
	width  int   // the title's width
}

func (s *morning) Done(t time.Duration) bool { return t >= MorningLength }

func (s *morning) Frame(t time.Duration, w, h int) []string {
	out := make([]string, 0, h)
	if h > 0 {
		out = append(out, s.roll.Frame(t, s.digits, rollRows)[rollRows/2])
	}
	if h > 1 {
		out = append(out, s.title.Frame(t, min(w, s.width), 1)...)
	}
	for len(out) < h {
		out = append(out, "")
	}
	return out[:h]
}

// morningScene is the registry's morning: day 41 rolling over to 42.
func morningScene(seed uint64) Scene {
	return Morning(41, 42, "MORNING REPORT · DAY 42", Seed(seed, 0, "morning"))
}
