package engine

import (
	"fmt"
	"math"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Chip is one thing a choice on a dilemma card does, in words (#358):
// what the TUI draws under the choice's label and the view carries
// beside it. Tone is how it reads for you: ToneGain, ToneCost, ToneLine
// (it pushes you over a line the world already knows) or ToneNote.
type Chip struct {
	Text string `json:"text"`
	Tone string `json:"tone"`
}

// The chips' tones.
const (
	ToneGain = "gain"
	ToneCost = "cost"
	ToneLine = "line"
	ToneNote = "note"
)

// Hidden is the one chip a card with hide = true shows under every
// choice: its drama is the unknown.
const Hidden = "costs you something"

// ChoiceChips is what each choice on c does, a list of chips a choice,
// in game.Gauges' order: the figures are game.Card.Preview's, so what
// the card shows is what the choice does. Where a move crosses a line
// the world already knows, the chip says so in words and not in dice:
// the heat where you stand over (or back under) a rung of the ladder
// (Heat.Ladder, the lines the dashboard's gauge marks), a member's
// loyalty over or under the skim line, a grudge the rival will act on.
// A choice that moves the reputation axes past the street's total gets
// a note that the reputation sim evens it out tonight. A card with Hide
// shows Hidden under every choice; a choice that moves nothing says so.
func ChoiceChips(cfg *content.Config, r Rules, w *game.World, c *game.Card) [][]Chip {
	out := make([][]Chip, len(c.Choices))
	for i := range c.Choices {
		if c.Hide {
			out[i] = []Chip{{Text: Hidden, Tone: ToneNote}}
			continue
		}
		ch, err := c.Preview(w, i)
		if err != nil {
			continue // a key the world does not know: Choose refuses it too
		}
		out[i] = chips(cfg, r, w, c, ch)
		if len(out[i]) == 0 {
			out[i] = []Chip{{Text: "changes nothing", Tone: ToneNote}}
		}
	}
	return out
}

// chips is one choice's changes in words.
func chips(cfg *content.Config, r Rules, w *game.World, c *game.Card, changes []game.Change) []Chip {
	var out []Chip
	var loyalty []game.Change
	for _, ch := range changes {
		if ch.Key == "loyalty" {
			loyalty = append(loyalty, ch)
		}
	}
	rep := false
	for _, ch := range changes {
		d := ch.Delta()
		switch ch.Key {
		case "dirty_cash", "clean_cash":
			out = append(out, Chip{Text: strings.TrimSuffix(ch.Key, "_cash") + " " + signedCash(int(d)), Tone: good(d > 0)})
		case "heat":
			out = append(out, heatChip(r, w, ch))
		case "loyalty": // the members' moves are contiguous: all of them at the first
			if loyalty != nil {
				out = append(out, loyaltyChips(r, w, loyalty)...)
				loyalty = nil
			}
		case "war":
			out = append(out, Chip{Text: "war with " + rivalName(w, c) + " " + signed(d), Tone: good(d < 0)})
		case "grudge":
			if d > 0 {
				out = append(out, Chip{Text: "grudge " + signed(d) + ": " + rivalName(w, c) + " remember", Tone: ToneLine})
			} else {
				out = append(out, Chip{Text: "grudge " + signed(d), Tone: ToneGain})
			}
		case "rival_muscle":
			out = append(out, Chip{Text: "their muscle " + signed(d), Tone: good(d < 0)})
		case "rival_cash":
			out = append(out, Chip{Text: "their cash " + signedCash(int(d)), Tone: good(d < 0)})
		case "stock":
			out = append(out, Chip{Text: "stock " + signed(d), Tone: good(d > 0)})
		case "corners":
			if k := w.Corner(c.Corner); d < 0 && k != nil {
				out = append(out, Chip{Text: "give up " + k.Name, Tone: ToneCost})
			} else {
				out = append(out, Chip{Text: "corners " + signed(d), Tone: good(d > 0)})
			}
		case "owes":
			if d > 0 {
				out = append(out, Chip{Text: "you'll owe " + favours(int(d)), Tone: ToneLine})
			} else {
				out = append(out, Chip{Text: favours(int(-d)) + " repaid", Tone: ToneGain})
			}
		case "fear", "respect":
			out = append(out, Chip{Text: ch.Key + " " + signed(d), Tone: good(d > 0)})
			rep = rep || d > 0
		case "notoriety":
			out = append(out, Chip{Text: ch.Key + " " + signed(d), Tone: good(d < 0)})
			rep = rep || d > 0
		}
	}
	if rep && cfg != nil {
		sum := 0.0
		for _, a := range game.Axes {
			v := *w.Player.Reputation.Axis(a)
			for _, ch := range changes {
				if ch.Key == a {
					v = ch.To
				}
			}
			sum += v
		}
		if t := cfg.Reputation.Reputation.Total; t > 0 && sum > t {
			out = append(out, Chip{Text: "more than the street can hold: it evens out tonight", Tone: ToneNote})
		}
	}
	return out
}

// heatChip is the heat where you stand: over the highest rung of the
// ladder it crosses going up, back under the lowest going down, else
// the move.
func heatChip(r Rules, w *game.World, ch game.Change) Chip {
	d := ch.Delta()
	if r.Heat != nil {
		ladder := r.Heat.Ladder(w, w.Here())
		if d > 0 {
			for i := len(ladder) - 1; i >= 0; i-- {
				if l := ladder[i]; ch.From < l.Threshold && ch.To >= l.Threshold {
					return Chip{Text: fmt.Sprintf("heat %.0f %s %.0f: over the %s line", ch.From, format.Arrow, ch.To, rung(l.Level)), Tone: ToneLine}
				}
			}
		} else {
			for _, l := range ladder {
				if ch.To < l.Threshold && ch.From >= l.Threshold {
					return Chip{Text: fmt.Sprintf("heat %.0f %s %.0f: back under the %s line", ch.From, format.Arrow, ch.To, rung(l.Level)), Tone: ToneGain}
				}
			}
		}
	}
	return Chip{Text: "heat " + signed(d), Tone: good(d < 0)}
}

// loyaltyChips is the loyalty moves: the member's by name when one
// moves, the crew's as one chip when more do (a range when the clamps
// made them differ), and a chip for every member the move takes over or
// under the skim line.
func loyaltyChips(r Rules, w *game.World, ls []game.Change) []Chip {
	line := 0.0
	if r.Crew != nil {
		line = r.Crew.Tuning().SkimThreshold
	}
	crossed := func(ch game.Change) (Chip, bool) {
		name := memberName(w, ch.Member)
		switch {
		case line > 0 && ch.From >= line && ch.To < line:
			return Chip{Text: fmt.Sprintf("%s's loyalty %.0f %s %.0f: under the skim line", name, ch.From, format.Arrow, ch.To), Tone: ToneLine}, true
		case line > 0 && ch.From < line && ch.To >= line:
			return Chip{Text: fmt.Sprintf("%s's loyalty %.0f %s %.0f: over the skim line", name, ch.From, format.Arrow, ch.To), Tone: ToneGain}, true
		}
		return Chip{}, false
	}
	if len(ls) == 1 {
		if chip, ok := crossed(ls[0]); ok {
			return []Chip{chip}
		}
		d := ls[0].Delta()
		return []Chip{{Text: memberName(w, ls[0].Member) + "'s loyalty " + signed(d), Tone: good(d > 0)}}
	}
	lo, hi := ls[0].Delta(), ls[0].Delta()
	for _, ch := range ls[1:] {
		lo, hi = math.Min(lo, ch.Delta()), math.Max(hi, ch.Delta())
	}
	text := "crew loyalty " + signed(hi)
	if lo < 0 {
		text = "crew loyalty " + signed(lo)
	}
	if lo != hi {
		text = "crew loyalty up to " + signed(hi)
		if lo < 0 {
			text = "crew loyalty down to " + signed(lo)
		}
	}
	out := []Chip{{Text: text, Tone: good(lo >= 0 && hi > 0)}} // the tone reads as the text does: "down to" is a cost (#384)
	for _, ch := range ls {
		if chip, ok := crossed(ch); ok {
			out = append(out, chip)
		}
	}
	return out
}

// good is a move's tone: a gain when it helps you, a cost when not.
func good(ok bool) string {
	if ok {
		return ToneGain
	}
	return ToneCost
}

// signed is a gauge's move with its sign, `+8` or `−8`: whole numbers,
// a tenth when the move is under one.
func signed(v float64) string {
	s := fmt.Sprintf("%.0f", math.Abs(v))
	if math.Abs(v) < 1 {
		s = fmt.Sprintf("%.1f", math.Abs(v))
	}
	if v < 0 {
		return "−" + s
	}
	return "+" + s
}

// signedCash is a sum with its sign, `−$12K`, as a headline gives it.
func signedCash(n int) string {
	if n < 0 {
		return "−" + format.Cash(-n)
	}
	return "+" + format.Cash(n)
}

// favours is a count of favours in words: `a favour`, `2 favours`.
func favours(n int) string {
	if n == 1 {
		return "a favour"
	}
	return format.Plural(n, "favour")
}

// rung is a level in words.
func rung(level string) string {
	if level == content.TaskForce {
		return "task force"
	}
	return level
}

// rivalName is the crew of the faction the card is about (#385) in
// words, read without making one.
func rivalName(w *game.World, c *game.Card) string {
	if len(w.Rivals) == 0 {
		return "the rival"
	}
	return w.FactionName(c.Faction)
}

func memberName(w *game.World, id int) string {
	if m := w.Crew.Member(id); m != nil {
		return m.Name
	}
	return "somebody"
}
