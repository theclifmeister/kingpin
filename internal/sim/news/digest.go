package news

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The morning's lead (#354): the night ranked by consequence, the
// biggest changes first, each a line in words and the act that answers
// it (#352's shape, game.Act). It is the news sim's own reading of the
// tick's events, the world as it stands and the cash flow's history,
// weighed by headlines.toml [digest]: no dice, and no sim reads it.

// digestNames is how many names a lead line lists before `and N more`.
const digestNames = 3

// lead is the night's lead: every kind of change that happened scored
// its weight times its count, the biggest [digest] lines of them,
// biggest first, a tie to the kind DigestKinds lists first. flow is
// tonight's cash flow; w.Flows is still the nights before it.
func (s *Sim) lead(w *game.World, t *game.Tick, flow game.CashFlow) []game.Line {
	cfg := s.cfg.Digest
	type scored struct {
		line  game.Line
		score float64
		rank  int
	}
	var all []scored
	add := func(kind string, n float64, l game.Line) {
		if n <= 0 {
			return
		}
		l.Kind = kind
		all = append(all, scored{l, cfg.Weights[kind] * n, slices.Index(content.DigestKinds, kind)})
	}

	var (
		lost, won, moved     named // corners
		crew                 named // members, with what happened to them
		seized               int   // the police's takes tonight
		units, cash          int   // what they took
		pages                int   // what went in the DA's file
		lostCity, movedFirst string
	)
	for _, e := range t.Events() {
		switch ev := e.(type) {
		case events.CornerLost:
			if ev.Owner == game.OwnerPlayer {
				lost.add(ev.Name, ev.Corner)
			}
		case events.CornerTaken:
			if ev.From == game.OwnerPlayer {
				lost.add(ev.Name, ev.Corner)
			}
		case events.LieutenantWalked:
			for _, c := range ev.Corners {
				lost.add(c, "")
			}
			if lostCity == "" {
				lostCity = ev.City
			}
			crew.addMember(ev.Name+" (walked)", ev.ID)
		case events.CornerClaimed:
			won.add(ev.Name, ev.Corner)
		case events.CornerStruck:
			if ev.Taken {
				won.add(ev.Name, ev.Corner)
			}
		case events.RivalMovedIn:
			moved.add(ev.Name, ev.Corner)
			if movedFirst == "" {
				movedFirst = ev.Rival
			}
		case events.CrewQuit:
			crew.addMember(ev.Name+" (quit)", ev.ID)
		case events.CrewArrested:
			crew.addMember(ev.Name+" (arrested)", ev.ID)
		case events.CrewShot:
			if !ev.Theirs {
				how := " (shot)"
				if ev.Dead {
					how = " (killed)"
				}
				crew.addMember(ev.Name+how, ev.ID)
			}
		case events.CrewDefected:
			crew.addMember(ev.Name+" (defected)", ev.ID)
		case events.Enforcement:
			pages += ev.Evidence
			if ev.Level == content.Patrol || ev.Level == content.Arrest {
				continue
			}
			if n := sumUnits(ev.StockLost); n > 0 || ev.CashLost > 0 {
				seized++
				units += n
				cash += ev.CashLost
			}
		case events.ShipmentSeized:
			seized++
			units += ev.Units
		case events.FrontAudited:
			if ev.Seized > 0 {
				seized++
				cash += ev.Seized
			}
		case events.RaidFellThrough:
			pages += ev.Evidence
		case events.BribeBackfired:
			pages += ev.Evidence
		case events.LeadsFiled:
			pages += ev.Evidence
		}
	}

	if n := lost.len(); n > 0 {
		l := cornerLine(w, fmt.Sprintf("Lost %s: %s.", cornerCount(n), lost.words()), lost.first())
		if l.City == "" {
			l.City = lostCity
		}
		add("corner_lost", float64(n), l)
	}
	if n := crew.len(); n > 0 {
		l := game.Line{Text: fmt.Sprintf("Lost %d of the crew: %s.", n, crew.words()), Act: game.Act{Screen: game.ScreenCrew}}
		if n == 1 {
			l.Text = fmt.Sprintf("Lost %s.", crew.words())
		}
		for _, id := range crew.members {
			if w.Crew.Member(id) != nil { // on the roster still: a cell, a bed
				l.Act.Subject, l.Member = game.OnMember, id
				break
			}
		}
		add("crew_lost", float64(n), l)
	}
	if seized > 0 {
		var took []string
		if units > 0 {
			took = append(took, format.Plural(units, "unit"))
		}
		if cash > 0 {
			took = append(took, format.Cash(cash))
		}
		add("seizure", float64(seized), game.Line{Text: fmt.Sprintf("The police took %s.", strings.Join(took, " and ")), Act: game.Act{Screen: game.ScreenDashboard}})
	}
	if pages > 0 {
		add("pages", float64(pages), game.Line{Text: fmt.Sprintf("The DA filed %s on you.", format.Plural(pages, "page")), Act: game.Act{Screen: game.ScreenDashboard}})
	}
	if l, n := s.flowLine(w, flow); n > 0 {
		add("flow", n, l)
	}
	if n := moved.len(); n > 0 {
		who := "A new crew"
		if movedFirst != "" {
			who = movedFirst + "'s crew"
		}
		add("faction", float64(n), cornerLine(w, fmt.Sprintf("%s moved in: %s.", who, moved.words()), moved.first()))
	}
	if n := won.len(); n > 0 {
		add("corner_won", float64(n), cornerLine(w, fmt.Sprintf("Took %s: %s.", cornerCount(n), won.words()), won.first()))
	}

	// The standing trouble (#345): the corners you hold that nobody
	// works, and the runners fit to work with no post.
	var idle named
	for _, c := range w.Corners() {
		if c.Held() && !c.Worked() {
			idle.add(c.Name, c.ID)
		}
	}
	if n := idle.len(); n > 0 {
		l := cornerLine(w, fmt.Sprintf("Nobody works %s: %s.", cornerCount(n), idle.words()), idle.first())
		l.Act.Mode = game.ModePost
		add("idle_corner", float64(n), l)
	}
	var runners named
	for _, m := range w.Crew.Members {
		if m.Role == game.RoleRunner && m.Fit(w.Day) && w.PostOf(m.ID) == nil {
			runners.addMember(m.Name, m.ID)
		}
	}
	if n := runners.len(); n > 0 {
		text := fmt.Sprintf("%s are idle: %s.", capitalize(format.Plural(n, "runner")), runners.words())
		if n == 1 {
			text = fmt.Sprintf("A runner is idle: %s.", runners.words())
		}
		add("idle_runner", float64(n), game.Line{Text: text, Act: game.Act{Screen: game.ScreenCrew, Subject: game.OnMember}, Member: runners.members[0]})
	}

	sort.SliceStable(all, func(i, j int) bool {
		if all[i].score != all[j].score {
			return all[i].score > all[j].score
		}
		return all[i].rank < all[j].rank
	})
	var out []game.Line
	for _, sc := range all[:min(len(all), cfg.Lines)] {
		out = append(out, sc.line)
	}
	return out
}

// flowLine is the cash flow's swing (#351): tonight's profit against
// the average of the [digest] week of nights before it, as a share of
// that average, from min_swing and capped at swing_cap. Profit is the
// night's net less what went into product and the operation
// (purchases and investments): money turned into stock or a front is
// not money lost, and a buying day would read as a crash. It says
// nothing before there is a week to read, or when the week made
// nothing either way.
func (s *Sim) flowLine(w *game.World, flow game.CashFlow) (game.Line, float64) {
	cfg := s.cfg.Digest
	if len(w.Flows) < cfg.Week {
		return game.Line{}, 0
	}
	sum := 0
	for _, f := range w.Flows[len(w.Flows)-cfg.Week:] {
		sum += profit(f)
	}
	avg := float64(sum) / float64(cfg.Week)
	if math.Abs(avg) < 1 {
		return game.Line{}, 0
	}
	net := profit(flow)
	swing := (float64(net) - avg) / math.Abs(avg)
	if math.Abs(swing) < cfg.MinSwing {
		return game.Line{}, 0
	}
	week := signedCash(int(math.Round(avg)))
	var text string
	switch {
	case avg > 0 && net >= 0 && swing >= 1:
		ratio, prec := float64(net)/avg, 1
		if ratio >= 10 {
			prec = 0
		}
		text = fmt.Sprintf("Profit ran %s the week's: %s against %s a night.", format.Times(ratio, prec), signedCash(net), week)
	case avg > 0 && net >= 0:
		dir := "rose"
		if swing < 0 {
			dir = "fell"
		}
		text = fmt.Sprintf("Profit %s %s on the week: %s against %s a night.", dir, format.Pct(math.Abs(swing), 0), signedCash(net), week)
	default:
		text = fmt.Sprintf("The night made %s against the week's %s a night.", signedCash(net), week)
	}
	return game.Line{Text: text, Act: game.Act{Screen: game.ScreenLedger}}, math.Min(math.Abs(swing), cfg.SwingCap)
}

// profit is a night's net less its purchases and investments.
func profit(f game.CashFlow) int {
	return f.Net() - f.Line(game.FlowPurchases).Total() - f.Line(game.FlowInvestments).Total()
}

// signedCash is a change of cash the way a headline writes it:
// `+$4,200`, `-$45K`.
func signedCash(n int) string {
	if n > 0 {
		return "+" + format.Cash(n)
	}
	return format.Cash(n)
}

// cornerLine is a line whose act is the map on the corner (the first a
// line names), in the corner's city.
func cornerLine(w *game.World, text, corner string) game.Line {
	l := game.Line{Text: text, Act: game.Act{Screen: game.ScreenMap}}
	if c := w.Corner(corner); c != nil {
		l.Act.Subject, l.Corner, l.City = game.OnCorner, c.ID, c.City
	}
	return l
}

// cornerCount is n corners in words: `a corner`, `3 corners`.
func cornerCount(n int) string {
	if n == 1 {
		return "a corner"
	}
	return format.Plural(n, "corner")
}

// sumUnits is the units in a take.
func sumUnits(m map[string]int) int {
	n := 0
	for _, q := range m {
		n += q
	}
	return n
}

// named is the things a lead line lists, in the order the night named
// them, with the ids its act opens on.
type named struct {
	names   []string
	ids     []string
	members []int
}

func (n *named) add(name, id string) {
	n.names = append(n.names, name)
	n.ids = append(n.ids, id)
}

func (n *named) addMember(name string, id int) {
	n.names = append(n.names, name)
	n.members = append(n.members, id)
}

func (n named) len() int { return len(n.names) }

// first is the first id the line names, or "".
func (n named) first() string {
	for _, id := range n.ids {
		if id != "" {
			return id
		}
	}
	return ""
}

// words are the names, the first digestNames of them and a count of the
// rest: `Oak St, 5th Ave and 2 more`.
func (n named) words() string {
	if len(n.names) <= digestNames {
		return strings.Join(n.names, ", ")
	}
	return strings.Join(n.names[:digestNames], ", ") + fmt.Sprintf(" and %d more", len(n.names)-digestNames)
}
