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
		crew                 named // members lost, with what happened to them
		down                 named // members laid up, back in days (#423)
		seized               int   // the police's takes tonight
		units, cash          int   // what they took
		pages                int   // what went in the DA's file
		lostCity, movedFirst string
		scouts               named // a faction's scouts or recruiters in a city (#341)
		probe                *game.Line
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
				if ev.Dead {
					crew.addMember(ev.Name+" (killed)", ev.ID)
				} else { // laid up, back in days: not lost (#423)
					down.addMember(fmt.Sprintf("%s (shot, %s)", ev.Name, format.Plural(ev.Days, "day")), ev.ID)
				}
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
		case events.InvestigationOpened:
			if probe == nil {
				l := investigationLine(ev)
				probe = &l
			}
		case events.RivalScouting:
			scouts.add(fmt.Sprintf("%s's scouts are in %s", ev.Rival, w.CityName(ev.City)), ev.City)
		case events.RivalRecruiting:
			scouts.add(fmt.Sprintf("%s's crew is hiring in %s", ev.Rival, w.CityName(ev.City)), ev.City)
		}
	}

	if n := lost.len(); n > 0 {
		l := cornerLine(w, fmt.Sprintf("Lost %s: %s.", cornerCount(n), lost.words()), lost.first())
		if l.City == "" {
			l.City = lostCity
		}
		add("corner_lost", float64(n), l)
	}
	if n := crew.len() + down.len(); n > 0 {
		// Lost is gone for good or in a cell; a member shot and alive
		// is laid up, and says so (#423).
		var parts []string
		switch k := crew.len(); {
		case k == 1:
			parts = append(parts, fmt.Sprintf("Lost %s.", crew.words()))
		case k > 1:
			parts = append(parts, fmt.Sprintf("Lost %d of the crew: %s.", k, crew.words()))
		}
		if down.len() > 0 {
			parts = append(parts, fmt.Sprintf("Laid up: %s.", down.words()))
		}
		l := game.Line{Text: strings.Join(parts, " "), Act: game.Act{Screen: game.ScreenCrew}}
		for _, id := range append(slices.Clone(crew.members), down.members...) {
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
	if probe != nil {
		add("investigation", 1, *probe)
	}
	if n := scouts.len(); n > 0 {
		add("scouts", float64(n), game.Line{Text: capitalize(scouts.joined("; ")) + ".", Act: game.Act{Screen: game.ScreenRivals}, City: scouts.first()})
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
	// works, and the runners and enforcers fit to work with no post.
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
	// An enforcer with no post is idle too (#419): a corner lost from
	// under one sends them home, and only the runner used to say so.
	var runners, guards named
	for _, m := range w.Crew.Members {
		if !m.Fit(w.Day) || w.PostOf(m.ID) != nil {
			continue
		}
		switch {
		case m.Role == game.RoleRunner:
			runners.addMember(m.Name, m.ID)
		case m.Role == game.RoleEnforcer && w.GuardOf(m.ID) == nil:
			guards.addMember(m.Name, m.ID)
		}
	}
	if n := runners.len() + guards.len(); n > 0 {
		var text string
		switch {
		case guards.len() == 0:
			text = fmt.Sprintf("%s are idle: %s.", capitalize(format.Plural(n, "runner")), runners.words())
		case runners.len() == 0:
			text = fmt.Sprintf("%s are idle: %s.", capitalize(format.Plural(n, "enforcer")), guards.words())
		default:
			noun := func(k int, s string) string {
				if k == 1 {
					return s
				}
				return format.Plurals(s)
			}
			text = fmt.Sprintf("%d of the crew are idle: %s (%s), %s (%s).", n, runners.words(), noun(runners.len(), "runner"), guards.words(), noun(guards.len(), "enforcer"))
		}
		if n == 1 && guards.len() == 0 {
			text = fmt.Sprintf("A runner is idle: %s.", runners.words())
		} else if n == 1 {
			text = fmt.Sprintf("An enforcer is idle: %s.", guards.words())
		}
		first := append(slices.Clone(runners.members), guards.members...)[0]
		add("idle_runner", float64(n), game.Line{Text: text, Act: game.Act{Screen: game.ScreenCrew, Subject: game.OnMember}, Member: first})
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
// (purchases and investments) and into the offshore account (#422):
// money turned into stock, a front or the account is not money lost, and a buying day would read as a crash. It says
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
	act := game.Act{Screen: game.ScreenLedger}
	if swing < 0 {
		if road := s.roadHint(w); road != "" {
			text += " " + road
			act = game.Act{Screen: game.ScreenMap}
		}
	}
	return game.Line{Text: text, Act: act}, math.Min(math.Abs(swing), cfg.SwingCap)
}

// roadHint is the falling profit line's pointer to the road (#446): a
// run at Territory with no route on and a week no better than the one
// before it has met the one city's ceiling (every policy that stays home
// flattens after day 70; the ones that run the road to the second city
// do not), so the line says where the next step is. Empty otherwise.
func (s *Sim) roadHint(w *game.World) string {
	if t := s.pcfg.Tier(w.Tier()); t == nil || t.ID != "territory" || len(w.CityOrder) < 2 {
		return ""
	}
	for _, rs := range w.Routes {
		if rs.Dial.On() {
			return ""
		}
	}
	week := s.cfg.Digest.Week
	if len(w.Flows) < 2*week {
		return ""
	}
	last, before := 0, 0
	for i, f := range w.Flows[len(w.Flows)-2*week:] {
		if i < week {
			before += profit(f)
		} else {
			last += profit(f)
		}
	}
	if last > before {
		return ""
	}
	far := w.CityOrder[1]
	if far == w.Home().ID {
		far = w.CityOrder[0]
	}
	return fmt.Sprintf("The corners here have a ceiling: the road to %s is on the map (5).", w.CityName(far))
}

// profit is a night's net less its purchases and investments, and less
// what went offshore (#422): money moved to your own account is not a
// night's loss ("The night made -$34K" on a night the street made
// +$23K and $51K went to the account).
func profit(f game.CashFlow) int {
	return f.Net() - f.Line(game.FlowPurchases).Total() - f.Line(game.FlowInvestments).Total() - f.Line(game.FlowOffshore).Total()
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

// investigationLine is the police opening an investigation (#343),
// its act where the target is, as the alert's: the corner on the map,
// the product on the market, the house on the ledger.
func investigationLine(ev events.InvestigationOpened) game.Line {
	when := "tonight"
	if d := ev.Due - ev.Day; d > 1 {
		when = "in " + format.Plural(d, "day")
	} else if d == 1 {
		when = "tomorrow night"
	}
	l := game.Line{Text: fmt.Sprintf("The police opened an investigation on %s: they come %s.", ev.Name, when), City: ev.City}
	switch ev.Lead {
	case game.LeadCorner:
		l.Act, l.Corner = game.Act{Screen: game.ScreenMap, Subject: game.OnCorner}, ev.Target
	case game.LeadHouse:
		l.Act, l.House = game.Act{Screen: game.ScreenLedger, Subject: game.OnHouse}, ev.Target
	default:
		l.Act = game.Act{Screen: game.ScreenMarket}
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

// joined is every name, joined by sep.
func (n named) joined(sep string) string { return strings.Join(n.names, sep) }

// words are the names, the first digestNames of them and a count of the
// rest: `Oak St, 5th Ave and 2 more`.
func (n named) words() string {
	if len(n.names) <= digestNames {
		return strings.Join(n.names, ", ")
	}
	return strings.Join(n.names[:digestNames], ", ") + fmt.Sprintf(" and %d more", len(n.names)-digestNames)
}
