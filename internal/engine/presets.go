package engine

import (
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
)

// Operation presets (#357, docs/presets.md): a preset is a named list
// of the routine's existing commands, and nothing more. The built-ins
// are presets.toml's rows, each a rule over the routine as it stands
// ("every standing order at the quiet dial"); a saved one is the
// routine as it stood the day the player saved it (game.Preset, kept in
// the profile by the front end and handed over with UsePresets).
// PresetCommands resolves either into the commands that set it against
// the run now, PresetDiff runs them on a copy of the run and says what
// would change, and ApplyPreset issues them through the session's own
// commands, so applying a preset is exactly issuing its commands by
// hand (TestPresetIsItsCommands).

// ErrNoPreset is what a preset call returns for an id or name no preset
// has.
var ErrNoPreset = errors.New("no preset by that name")

// Command is one of the routine's session commands as a preset issues
// it: Op is the command's name on the wire (place_standing, set_pay…),
// the rest its parameters, a dial by name. Session.Do issues one.
type Command struct {
	Op      string `json:"op"`
	City    string `json:"city,omitempty"`
	Product string `json:"product,omitempty"`
	Route   string `json:"route,omitempty"`
	N       int    `json:"n,omitempty"`    // the quantity, the units or the days; place_standing's game.AllUnits (-1) is the whole stash (#503)
	Dial    string `json:"dial,omitempty"` // the dial by name
	On      bool   `json:"on,omitempty"`   // set_lie_low's
}

// The ops a preset issues: the session commands of the routine, by
// their names on the wire.
const (
	OpPlaceStanding  = "place_standing"
	OpCancelStanding = "cancel_standing"
	OpSetSupply      = "set_supply"
	OpClearSupply    = "clear_supply"
	OpSetLaunderDial = "set_launder_dial"
	OpSetPay         = "set_pay"
	OpSetRoute       = "set_route"
	OpSetRouteTarget = "set_route_target"
	OpSetRouteDays   = "set_route_days"
	OpSetLieLow      = "set_lie_low"
)

// standingN is the quantity a preset re-places a standing order at: its
// units, or game.AllUnits for one kept at the whole stash (#503), so a
// preset never freezes "all" at the number it read.
func standingN(o game.SellOrder) int {
	if o.All {
		return game.AllUnits
	}
	return o.Qty
}

// Ops is every op a preset issues, in the order a preset issues them.
func Ops() []string {
	return []string{OpCancelStanding, OpClearSupply, OpSetSupply, OpPlaceStanding, OpSetLaunderDial, OpSetPay, OpSetRoute, OpSetRouteTarget, OpSetRouteDays, OpSetLieLow}
}

// Do issues one command through the session command of its name, as a
// front end calling it by hand would.
func (s *Session) Do(c Command) error {
	switch c.Op {
	case OpPlaceStanding:
		d, ok := events.ParseDial(c.Dial)
		if !ok {
			return game.ErrBadDial
		}
		return s.PlaceStanding(c.City, c.Product, c.N, d)
	case OpCancelStanding:
		s.CancelStanding(c.City, c.Product)
		return nil
	case OpSetSupply:
		return s.SetSupply(c.City, c.Product, c.N)
	case OpClearSupply:
		s.ClearSupply(c.City, c.Product)
		return nil
	case OpSetLaunderDial:
		d, ok := events.ParseLaunder(c.Dial)
		if !ok {
			return game.ErrBadDial
		}
		return s.SetLaunderDial(d)
	case OpSetPay:
		p, ok := events.ParsePay(c.Dial)
		if !ok {
			return game.ErrBadDial
		}
		return s.SetPay(p)
	case OpSetRoute:
		d, ok := events.ParseRouteDial(c.Dial)
		if !ok {
			return game.ErrBadDial
		}
		return s.SetRoute(c.Route, d)
	case OpSetRouteTarget:
		return s.SetRouteTarget(c.Route, c.Product, c.N)
	case OpSetRouteDays:
		return s.SetRouteDays(c.Route, c.Product, c.N)
	case OpSetLieLow:
		s.SetLieLow(c.On)
		return nil
	}
	return fmt.Errorf("no command %q", c.Op)
}

// Preset is one preset in the list: a built-in's id, or a saved one's
// name as its id, the name, a line of copy, and whether it is saved.
type Preset struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Blurb string `json:"blurb"`
	Saved bool   `json:"saved"`
}

// UsePresets hands the session the presets the front end keeps (the
// TUI's profile): Presets lists them after the built-ins and the
// preset calls find them by name. A built-in wins a name both have.
func (s *Session) UsePresets(saved []game.Preset) { s.saved = saved }

// Presets is every preset: the built-ins in the file's order, then the
// saved ones in the order they were saved. It is never null on the
// wire.
func (s *Session) Presets() []Preset {
	out := []Preset{}
	for _, p := range s.cfg.Presets.Presets {
		out = append(out, Preset{ID: p.ID, Name: p.Name, Blurb: p.Blurb})
	}
	for _, p := range s.saved {
		if s.builtin(p.Name) != nil {
			continue
		}
		out = append(out, Preset{ID: p.Name, Name: p.Name, Blurb: savedBlurb(p), Saved: true})
	}
	return out
}

// savedBlurb is a saved preset's line: the day it was saved and what
// it holds.
func savedBlurb(p game.Preset) string {
	parts := []string{fmt.Sprintf("saved day %d", p.Day), plural(len(p.Standing), "standing order"), plural(len(p.Supply), "contract")}
	running := 0
	for _, r := range p.Routes {
		if r.Dial.On() {
			running++
		}
	}
	parts = append(parts, plural(running, "route")+" running", "launder "+p.Launder.String(), "pay "+p.Pay.String())
	return strings.Join(parts, ", ")
}

func plural(n int, noun string) string { return format.Plural(n, noun) }

// builtin is the built-in with the id, else the name (any case), or nil.
func (s *Session) builtin(id string) *content.PresetConfig {
	for i, p := range s.cfg.Presets.Presets {
		if p.ID == id {
			return &s.cfg.Presets.Presets[i]
		}
	}
	for i, p := range s.cfg.Presets.Presets {
		if strings.EqualFold(p.Name, id) {
			return &s.cfg.Presets.Presets[i]
		}
	}
	return nil
}

// find is the preset under id or name: its list entry and its commands
// against the run now.
func (s *Session) find(id string) (Preset, []Command, error) {
	if p := s.builtin(id); p != nil {
		return Preset{ID: p.ID, Name: p.Name, Blurb: p.Blurb}, s.builtinCommands(*p), nil
	}
	for _, p := range s.saved {
		if strings.EqualFold(p.Name, id) {
			return Preset{ID: p.Name, Name: p.Name, Blurb: savedBlurb(p), Saved: true}, s.savedCommands(p), nil
		}
	}
	return Preset{}, nil, ErrNoPreset
}

// PresetCommands is the commands the preset under id (or name) issues
// against the run as it stands: only those that set something to what
// it is not, in the order they run (Ops). It is never null on the wire.
func (s *Session) PresetCommands(id string) ([]Command, error) {
	_, cmds, err := s.find(id)
	if cmds == nil {
		cmds = []Command{}
	}
	return cmds, err
}

// builtinCommands is a presets.toml row as commands: each rule over
// every setting of yours it names.
func (s *Session) builtinCommands(p content.PresetConfig) []Command {
	w := s.w
	var cmds []Command
	each := func(f func(city, product string)) {
		for _, cid := range w.CityOrder {
			for _, pid := range w.Products {
				f(cid, pid)
			}
		}
	}
	switch p.Standing {
	case "":
	case content.PresetCancel:
		each(func(cid, pid string) {
			if _, ok := w.YourStanding(cid, pid); ok {
				cmds = append(cmds, Command{Op: OpCancelStanding, City: cid, Product: pid})
			}
		})
	default:
		d, _ := events.ParseDial(p.Standing)
		each(func(cid, pid string) {
			if o, ok := w.YourStanding(cid, pid); ok && o.Dial != d {
				cmds = append(cmds, Command{Op: OpPlaceStanding, City: cid, Product: pid, N: standingN(o), Dial: p.Standing})
			}
		})
	}
	if p.Supply == content.PresetClear {
		each(func(cid, pid string) {
			if _, ok := w.Supplied(cid, pid); ok {
				cmds = append(cmds, Command{Op: OpClearSupply, City: cid, Product: pid})
			}
		})
	}
	if d, ok := events.ParseLaunder(p.Launder); ok && d != w.Laundering.Dial {
		cmds = append(cmds, Command{Op: OpSetLaunderDial, Dial: p.Launder})
	}
	if d, ok := events.ParsePay(p.Pay); ok && d != w.Crew.Pay {
		cmds = append(cmds, Command{Op: OpSetPay, Dial: p.Pay})
	}
	if d, ok := events.ParseRouteDial(p.Routes); ok {
		for _, r := range s.cfg.Routes.Routes {
			if cur := w.Route(r.ID).Dial; cur.On() && cur != d {
				cmds = append(cmds, Command{Op: OpSetRoute, Route: r.ID, Dial: p.Routes})
			}
		}
	}
	if p.LieLow && !w.Today.LieLow {
		cmds = append(cmds, Command{Op: OpSetLieLow, On: true})
	}
	return cmds
}

// Snapshot is the routine as it stands, as a saved preset under name:
// every standing order and supply contract of yours, the launder and
// pay dials, and every route running or keeping a target. The front end
// keeps it (the TUI in the profile); it touches nothing.
func (s *Session) Snapshot(name string) game.Preset {
	w := s.w
	p := game.Preset{Name: name, Day: w.Day, Launder: w.Laundering.Dial, Pay: w.Crew.Pay}
	for _, cid := range w.CityOrder {
		for _, pid := range w.Products {
			if o, ok := w.YourStanding(cid, pid); ok {
				p.Standing = append(p.Standing, o)
			}
			if c, ok := w.Supplied(cid, pid); ok {
				p.Supply = append(p.Supply, c)
			}
		}
	}
	for _, r := range s.cfg.Routes.Routes {
		if rs := w.Route(r.ID); rs.Dial.On() || rs.HasTargets() {
			p.Routes = append(p.Routes, game.RoutePreset{ID: r.ID, Dial: rs.Dial, Target: maps.Clone(rs.Target), Days: maps.Clone(rs.Days)})
		}
	}
	return p
}

// savedCommands is a saved preset as commands: the routine set back to
// it, skipping what this run has not got (a city not reached, a product
// not there, a route not open).
func (s *Session) savedCommands(p game.Preset) []Command {
	w := s.w
	var cmds []Command
	stand := map[string]game.SellOrder{}
	for _, o := range p.Standing {
		stand[game.OrderKey(o.City, o.Product)] = o
	}
	supply := map[string]game.SupplyContract{}
	for _, c := range p.Supply {
		supply[game.SupplyKey(c.City, c.Product)] = c
	}
	for _, cid := range w.CityOrder {
		for _, pid := range w.Products {
			if _, ok := w.YourStanding(cid, pid); ok {
				if _, keep := stand[game.OrderKey(cid, pid)]; !keep {
					cmds = append(cmds, Command{Op: OpCancelStanding, City: cid, Product: pid})
				}
			}
		}
	}
	for _, cid := range w.CityOrder {
		for _, pid := range w.Products {
			if _, ok := w.Supplied(cid, pid); ok {
				if _, keep := supply[game.SupplyKey(cid, pid)]; !keep {
					cmds = append(cmds, Command{Op: OpClearSupply, City: cid, Product: pid})
				}
			}
		}
	}
	for _, c := range p.Supply {
		if w.Product(c.City, c.Product) == nil {
			continue
		}
		if cur, ok := w.Supplied(c.City, c.Product); ok && cur.Units == c.Units {
			continue
		}
		cmds = append(cmds, Command{Op: OpSetSupply, City: c.City, Product: c.Product, N: c.Units})
	}
	for _, o := range p.Standing {
		if w.Product(o.City, o.Product) == nil {
			continue
		}
		if cur, ok := w.YourStanding(o.City, o.Product); ok && cur.Qty == o.Qty && cur.Dial == o.Dial && cur.All == o.All {
			continue
		}
		cmds = append(cmds, Command{Op: OpPlaceStanding, City: o.City, Product: o.Product, N: standingN(o), Dial: o.Dial.String()})
	}
	if p.Launder != w.Laundering.Dial {
		cmds = append(cmds, Command{Op: OpSetLaunderDial, Dial: p.Launder.String()})
	}
	if p.Pay != w.Crew.Pay {
		cmds = append(cmds, Command{Op: OpSetPay, Dial: p.Pay.String()})
	}
	routes := map[string]game.RoutePreset{}
	for _, r := range p.Routes {
		routes[r.ID] = r
	}
	for _, r := range s.cfg.Routes.Routes {
		if !s.set.Logistics.Open(w, r) || w.Cities[r.From] == nil || w.Cities[r.To] == nil {
			continue
		}
		want := routes[r.ID]
		cur := w.Route(r.ID)
		if cur.Dial != want.Dial {
			cmds = append(cmds, Command{Op: OpSetRoute, Route: r.ID, Dial: want.Dial.String()})
		}
		for _, pid := range w.Products {
			switch u, d := want.Target[pid], want.Days[pid]; {
			case d > 0:
				if cur.Days[pid] != d {
					cmds = append(cmds, Command{Op: OpSetRouteDays, Route: r.ID, Product: pid, N: d})
				}
			case u > 0:
				if cur.Target[pid] != u || cur.Days[pid] > 0 {
					cmds = append(cmds, Command{Op: OpSetRouteTarget, Route: r.ID, Product: pid, N: u})
				}
			case cur.Target[pid] > 0 || cur.Days[pid] > 0:
				cmds = append(cmds, Command{Op: OpSetRouteTarget, Route: r.ID, Product: pid})
			}
		}
	}
	return cmds
}

// Review is what a preset changes: one Change a setting that moves, in
// the routine's order, the commands the rules would refuse (each
// skipped, the rest still run), and how many of the routine's settings
// stay as they are, folded into a count.
type Review struct {
	Preset  Preset    `json:"preset"`
	Changes []Change  `json:"changes"`
	Refused []Refusal `json:"refused"`
	Same    int       `json:"same"`
}

// Refusal is a command the rules refuse, and why in the game's words.
type Refusal struct {
	Command Command `json:"command"`
	Why     string  `json:"why"`
}

// Change is one setting a preset moves, from what it is to what it
// would be, in words ("40 normal" → "40 quiet", "keep at 120" → "none",
// "careful"). Setting is standing, supply, launder, pay, route, target
// or lie_low; City and Product name a standing order or a contract,
// Route and Product a route's dial or target. Was and Now are a
// standing order's before and after (nil where there is none), for a
// front end's estimate of its take and heat; Cost and CostTo a
// contract's buy tomorrow morning as market.Sim.Plan lays it out now
// and after. Dropped is how many of tonight's orders lying low drops.
type Change struct {
	Setting string          `json:"setting"`
	City    string          `json:"city,omitempty"`
	Product string          `json:"product,omitempty"`
	Route   string          `json:"route,omitempty"`
	From    string          `json:"from"`
	To      string          `json:"to"`
	Was     *game.SellOrder `json:"was,omitempty"`
	Now     *game.SellOrder `json:"now,omitempty"`
	Cost    int             `json:"cost,omitempty"`
	CostTo  int             `json:"cost_to,omitempty"`
	Dropped int             `json:"dropped,omitempty"`
}

// PresetDiff is what applying the preset under id (or name) would
// change, read by running its commands on a copy of the run: the
// Review ApplyPreset would return, and the run untouched.
func (s *Session) PresetDiff(id string) (Review, error) {
	p, cmds, err := s.find(id)
	if err != nil {
		return Review{}, err
	}
	if s.w.Over != nil {
		return Review{}, game.ErrGameOver
	}
	b, err := game.Encode(s.w)
	if err != nil {
		return Review{}, err
	}
	w, err := game.Decode(b)
	if err != nil {
		return Review{}, err
	}
	return (&Session{cfg: s.cfg, set: s.set, w: w}).run(p, cmds), nil
}

// ApplyPreset issues the preset's commands, each through the session
// command of its name, and returns what moved: PresetDiff's review, now
// true of the run. A command the rules refuse is skipped and listed.
func (s *Session) ApplyPreset(id string) (Review, error) {
	p, cmds, err := s.find(id)
	if err != nil {
		return Review{}, err
	}
	if s.w.Over != nil {
		return Review{}, game.ErrGameOver
	}
	return s.run(p, cmds), nil
}

// run issues the commands on the session's world and reads what moved.
func (s *Session) run(p Preset, cmds []Command) Review {
	w := s.w
	before := s.routineOf(w)
	r := Review{Preset: p, Changes: []Change{}, Refused: []Refusal{}}
	for _, c := range cmds {
		if err := s.Do(c); err != nil {
			r.Refused = append(r.Refused, Refusal{Command: c, Why: err.Error()})
		}
	}
	r.Changes, r.Same = s.diff(before, s.routineOf(w))
	return r
}

// routine is the routine's settings as a preset reads them, and the
// morning's contract buys as the market plans them.
type routine struct {
	standing map[string]game.SellOrder
	supply   map[string]game.SupplyContract
	cost     map[string]int // the contract's buy tomorrow morning, by SupplyKey
	launder  events.Launder
	pay      events.Pay
	routes   map[string]game.RouteSetting
	lieLow   bool
	orders   int // tonight's orders
	cities   []string
	products []string
}

func (s *Session) routineOf(w *game.World) routine {
	r := routine{
		standing: maps.Clone(w.Standing), supply: maps.Clone(w.Supply), cost: map[string]int{},
		launder: w.Laundering.Dial, pay: w.Crew.Pay, routes: map[string]game.RouteSetting{},
		lieLow: w.Today.LieLow, orders: len(w.Today.Orders), cities: w.CityOrder, products: w.Products,
	}
	for _, pl := range s.set.Market.Plan(w) {
		if pl.Lieutenant == "" {
			r.cost[game.SupplyKey(pl.Contract.City, pl.Contract.Product)] = pl.Cost
		}
	}
	for id, rs := range w.Routes {
		rs.Target, rs.Days = maps.Clone(rs.Target), maps.Clone(rs.Days)
		r.routes[id] = rs
	}
	return r
}

// diff is every setting that moved from a to b, in the routine's order
// (the standing orders and the contracts in city then ladder order, the
// launder and pay dials, the routes in the file's order with their
// targets, lying low), and how many of the settings either holds did
// not move.
func (s *Session) diff(a, b routine) ([]Change, int) {
	changes := []Change{}
	settings := 0
	for _, cid := range a.cities {
		for _, pid := range a.products {
			k := game.OrderKey(cid, pid)
			was, okA := a.standing[k]
			now, okB := b.standing[k]
			if okA || okB {
				settings++
			}
			if okA != okB || was != now {
				c := Change{Setting: "standing", City: cid, Product: pid, From: orderWords(was, okA), To: orderWords(now, okB)}
				if okA {
					c.Was = &was
				}
				if okB {
					c.Now = &now
				}
				changes = append(changes, c)
			}
		}
	}
	for _, cid := range a.cities {
		for _, pid := range a.products {
			k := game.SupplyKey(cid, pid)
			was, okA := a.supply[k]
			now, okB := b.supply[k]
			if okA || okB {
				settings++
			}
			if okA != okB || was.Units != now.Units {
				changes = append(changes, Change{Setting: "supply", City: cid, Product: pid, From: supplyWords(was, okA), To: supplyWords(now, okB), Cost: a.cost[k], CostTo: b.cost[k]})
			}
		}
	}
	settings += 2
	if a.launder != b.launder {
		changes = append(changes, Change{Setting: "launder", From: a.launder.String(), To: b.launder.String()})
	}
	if a.pay != b.pay {
		changes = append(changes, Change{Setting: "pay", From: a.pay.String(), To: b.pay.String()})
	}
	for _, r := range s.cfg.Routes.Routes {
		ra, rb := a.routes[r.ID], b.routes[r.ID]
		if ra.Dial.On() || rb.Dial.On() {
			settings++
		}
		if ra.Dial != rb.Dial {
			changes = append(changes, Change{Setting: "route", Route: r.ID, From: ra.Dial.String(), To: rb.Dial.String()})
		}
		for _, pid := range a.products {
			wa, wb := targetWords(ra, pid), targetWords(rb, pid)
			if wa != "none" || wb != "none" {
				settings++
			}
			if wa != wb {
				changes = append(changes, Change{Setting: "target", Route: r.ID, Product: pid, From: wa, To: wb})
			}
		}
	}
	if a.lieLow || b.lieLow {
		settings++
	}
	if a.lieLow != b.lieLow {
		c := Change{Setting: "lie_low", From: onOff(a.lieLow), To: onOff(b.lieLow)}
		if b.lieLow {
			c.Dropped = a.orders - b.orders
		}
		changes = append(changes, c)
	}
	return changes, settings - len(changes)
}

func orderWords(o game.SellOrder, ok bool) string {
	if !ok {
		return "none"
	}
	return strconv.Itoa(o.Qty) + " " + o.Dial.String()
}

func supplyWords(c game.SupplyContract, ok bool) string {
	if !ok {
		return "none"
	}
	return "keep at " + strconv.Itoa(c.Units)
}

func targetWords(rs game.RouteSetting, product string) string {
	if d := rs.Days[product]; d > 0 {
		return plural(d, "day")
	}
	if u := rs.Target[product]; u > 0 {
		return plural(u, "unit")
	}
	return "none"
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}
