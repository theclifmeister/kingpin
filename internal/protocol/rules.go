package protocol

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The quotes (#325): every method of engine.Rules is served as
// rules.<sim>.<method> in snake_case (rules.market.capacity,
// rules.crew.investigate_cost), so a front end prices a move before it
// makes it with the numbers the TUI reads. A quote changes nothing and
// needs a run.
//
// The world is the server's: a rule's *game.World is the run's, never a
// parameter. A thing of the world goes by its id and is resolved against
// the run (ruleParams): a corner, a faction, a supplier, a city, a
// house, a deed (its corner), a front (owned or on offer), a crew member
// (on the payroll or in the pool), a contract and a route. A deal goes as
// an object (DealParams). A day is today or tomorrow, the two a front end
// asks about: a later one would read what the run has not yet told the
// player. A result that would carry the truth goes by id instead
// (ruleResults), and the rules that hand over a truth whole are in
// unservedRules.

// DealParams is game.Deal as the wire spells it: a deal proposed to the
// faction the rule is asked about.
type DealParams struct {
	Kind  string      `json:"kind"`
	Terms TermsParams `json:"terms"`
}

// unservedRules are the rules the wire does not carry, and why:
// TestEveryRuleIsClassed fails on one that is neither served nor here.
var unservedRules = map[string]string{
	"Logistics.Route": "the route's config holds its risk, the truth: the view carries the route and the risk the file knows (rules.logistics.routes_open gives the ids)",
}

// ruleParam is how one kind of parameter goes on the wire: the type the
// schema shows, the name it goes by when the rule's own is a letter,
// and how the id is resolved against the run.
type ruleParam struct {
	wire    reflect.Type
	name    string
	resolve func(w *game.World, s *engine.Session, raw json.RawMessage) (reflect.Value, error)
}

var (
	worldType = reflect.TypeFor[*game.World]()
	stringT   = reflect.TypeFor[string]()
	intT      = reflect.TypeFor[int]()
)

// byID decodes an id of type T and looks it up, refusing one the run
// does not have.
func byID[T any](what string, find func(w *game.World, s *engine.Session, id T) (any, bool)) func(*game.World, *engine.Session, json.RawMessage) (reflect.Value, error) {
	return func(w *game.World, s *engine.Session, raw json.RawMessage) (reflect.Value, error) {
		var id T
		if err := json.Unmarshal(raw, &id); err != nil {
			return reflect.Value{}, fmt.Errorf("a %s goes by its id", what)
		}
		v, ok := find(w, s, id)
		if !ok {
			return reflect.Value{}, fmt.Errorf("no %s %v", what, id)
		}
		return reflect.ValueOf(v), nil
	}
}

// ruleParams are the world's things a rule takes, by Go type.
var ruleParams = map[reflect.Type]ruleParam{
	reflect.TypeFor[*game.Corner](): {stringT, "corner", byID("corner", func(w *game.World, _ *engine.Session, id string) (any, bool) {
		c := w.Corner(id)
		return c, c != nil
	})},
	reflect.TypeFor[game.Corner](): {stringT, "corner", byID("corner", func(w *game.World, _ *engine.Session, id string) (any, bool) {
		if c := w.Corner(id); c != nil {
			return *c, true
		}
		return nil, false
	})},
	reflect.TypeFor[*game.RivalState](): {stringT, "faction", byID("faction", func(w *game.World, _ *engine.Session, id string) (any, bool) {
		if id == "" {
			return nil, false
		}
		r := w.Faction(id)
		return r, r != nil
	})},
	reflect.TypeFor[*game.Supplier](): {stringT, "supplier", byID("supplier", func(w *game.World, _ *engine.Session, id string) (any, bool) {
		sup := w.Supplier(id)
		return sup, sup != nil
	})},
	reflect.TypeFor[*game.City](): {stringT, "city", byID("city", func(w *game.World, _ *engine.Session, id string) (any, bool) {
		c := w.City(id)
		return c, c != nil
	})},
	reflect.TypeFor[*game.House](): {stringT, "house", byID("house", func(w *game.World, _ *engine.Session, id string) (any, bool) {
		h := w.House(id)
		return h, h != nil
	})},
	reflect.TypeFor[*game.Deed](): {stringT, "corner", byID("deeded corner", func(w *game.World, _ *engine.Session, id string) (any, bool) {
		if c := w.Corner(id); c != nil && c.Deed != nil {
			return c.Deed, true
		}
		return nil, false
	})},
	reflect.TypeFor[game.Front](): {stringT, "front", byID("front", func(w *game.World, s *engine.Session, id string) (any, bool) {
		if f := w.Front(id); f != nil {
			return *f, true
		}
		for _, o := range s.Rules().Laundering.Offers() {
			if o.ID == id {
				return game.Front{ID: o.ID, Name: o.Name}, true
			}
		}
		return nil, false
	})},
	reflect.TypeFor[game.CrewMember](): {intT, "member", byID("crew member", func(w *game.World, _ *engine.Session, id int) (any, bool) {
		for _, pool := range [][]game.CrewMember{w.Crew.Members, w.Crew.Candidates} {
			for _, m := range pool {
				if m.ID == id {
					return m, true
				}
			}
		}
		return nil, false
	})},
	reflect.TypeFor[game.Contract](): {intT, "contract", byID("contract", func(w *game.World, _ *engine.Session, id int) (any, bool) {
		if c := w.Contract(id); c != nil {
			return *c, true
		}
		return nil, false
	})},
	reflect.TypeFor[content.RouteConfig](): {stringT, "route", byID("route", func(_ *game.World, s *engine.Session, id string) (any, bool) {
		if r := s.Rules().Logistics.Route(id); r != nil {
			return *r, true
		}
		return nil, false
	})},
	reflect.TypeFor[content.LaneConfig](): {stringT, "lane", byID("lane", func(_ *game.World, s *engine.Session, id string) (any, bool) {
		if l := s.Rules().Logistics.Lane(id); l != nil {
			return *l, true
		}
		return nil, false
	})},
	reflect.TypeFor[game.Deal](): {reflect.TypeFor[DealParams](), "deal", func(_ *game.World, _ *engine.Session, raw json.RawMessage) (reflect.Value, error) {
		var d DealParams
		if err := json.Unmarshal(raw, &d); err != nil {
			return reflect.Value{}, err
		}
		t := d.Terms
		return reflect.ValueOf(game.Deal{Kind: d.Kind, Terms: game.Terms{Days: t.Days, PerDay: t.PerDay, Corners: t.Corners, Route: t.Route, Units: t.Units}}), nil
	}},
}

// ruleResult is how one kind of result goes on the wire: by id, where
// the value itself would carry the truth or a pointer into the run.
type ruleResult struct {
	wire reflect.Type
	id   func(v reflect.Value) any
}

var ruleResults = map[reflect.Type]ruleResult{
	reflect.TypeFor[*game.Corner](): {stringT, func(v reflect.Value) any {
		if v.IsNil() {
			return ""
		}
		return v.Interface().(*game.Corner).ID
	}},
	reflect.TypeFor[*game.City](): {stringT, func(v reflect.Value) any {
		if v.IsNil() {
			return ""
		}
		return v.Interface().(*game.City).ID
	}},
	reflect.TypeFor[[]*game.RivalState](): {reflect.TypeFor[[]string](), func(v reflect.Value) any {
		ids := []string{}
		for _, r := range v.Interface().([]*game.RivalState) {
			ids = append(ids, r.Faction())
		}
		return ids
	}},
	reflect.TypeFor[[]content.RouteConfig](): {reflect.TypeFor[[]string](), func(v reflect.Value) any {
		ids := []string{}
		for _, r := range v.Interface().([]content.RouteConfig) {
			ids = append(ids, r.ID)
		}
		return ids
	}},
	reflect.TypeFor[[]content.LaneConfig](): {reflect.TypeFor[[]string](), func(v reflect.Value) any {
		ids := []string{}
		for _, l := range v.Interface().([]content.LaneConfig) {
			ids = append(ids, l.ID)
		}
		return ids
	}},
}

// rulesType is engine.Rules: a field an interface for each sim.
var rulesType = reflect.TypeFor[engine.Rules]()

// ruleKey is a rule as ruleNames and unservedRules key it: Market.Capacity.
func ruleKey(sim, name string) string { return sim + "." + name }

// ruleMethodName is a rule's name on the wire: rules.market.capacity.
func ruleMethodName(sim, name string) string { return "rules." + snake(sim) + "." + snake(name) }

// rulesServed is every rule on the wire, as {sim, method} in order.
func rulesServed() [][2]string {
	var out [][2]string
	for i := 0; i < rulesType.NumField(); i++ {
		f := rulesType.Field(i)
		for j := 0; j < f.Type.NumMethod(); j++ {
			name := f.Type.Method(j).Name
			if _, no := unservedRules[ruleKey(f.Name, name)]; !no {
				out = append(out, [2]string{f.Name, name})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return ruleKey(out[i][0], out[i][1]) < ruleKey(out[j][0], out[j][1]) })
	return out
}

func init() {
	for _, r := range rulesServed() {
		register(ruleMethod(r[0], r[1]))
	}
}

// ruleMethod serves one rule: the run's world in place of the
// *game.World, the world's things by id, a dial by name, a day today or
// tomorrow; a (value, ok) answer refused when not ok, two values as an
// object named after the rule's results.
func ruleMethod(sim, name string) method {
	f, _ := rulesType.FieldByName(sim)
	fn, _ := f.Type.MethodByName(name)
	t := fn.Type
	key := ruleKey(sim, name)
	sig := ruleNames[key]
	m := method{name: ruleMethodName(sim, name), run: true}

	type in struct {
		world bool
		typ   reflect.Type // the Go type the rule takes
		rp    *ruleParam
		day   bool
	}
	var ins []in
	for i := 0; i < t.NumIn(); i++ {
		pt := t.In(i)
		pname := fmt.Sprintf("arg%d", i+1)
		if i < len(sig.params) {
			pname = sig.params[i]
		}
		if pt == worldType {
			ins = append(ins, in{world: true, typ: pt})
			continue
		}
		wire := pt
		var rp *ruleParam
		if p, ok := ruleParams[pt]; ok {
			rp = &p
			wire, pname = p.wire, p.name
		} else if dialNames(pt) != nil {
			pname = snake(pt.Name())
		}
		ins = append(ins, in{typ: pt, rp: rp, day: pname == "day" && pt == intT})
		m.params = append(m.params, param{pname, wire})
	}

	var resultNames []string
	outs := t.NumOut()
	okResult := outs == 2 && t.Out(1).Kind() == reflect.Bool
	switch {
	case outs == 1 || okResult:
		m.result = t.Out(0)
		if rr, ok := ruleResults[m.result]; ok {
			m.result = rr.wire
		}
	case outs > 1:
		resultNames = sig.results
		fields := make([]reflect.StructField, outs)
		for i := range fields {
			n := fmt.Sprintf("r%d", i+1)
			if i < len(resultNames) {
				n = resultNames[i]
			}
			fields[i] = reflect.StructField{Name: exported(n), Type: t.Out(i), Tag: reflect.StructTag(`json:"` + snake(n) + `"`)}
		}
		m.result = reflect.StructOf(fields)
	}

	m.call = func(s *Server, ps []json.RawMessage) (any, error) {
		if len(ps) != len(m.params) {
			return nil, &Error{Code: CodeInvalidParams, Message: fmt.Sprintf("%s takes %d params, got %d", m.name, len(m.params), len(ps))}
		}
		w := s.sess.World()
		args := make([]reflect.Value, 0, len(ins))
		k := 0
		for _, a := range ins {
			if a.world {
				args = append(args, reflect.ValueOf(w))
				continue
			}
			p := m.params[k]
			raw := ps[k]
			k++
			var v reflect.Value
			var err error
			if a.rp != nil {
				v, err = a.rp.resolve(w, s.sess, raw)
			} else {
				v, err = decodeArg(raw, a.typ)
			}
			if err != nil {
				return nil, &Error{Code: CodeInvalidParams, Message: fmt.Sprintf("%s: %s: %v", m.name, p.name, err)}
			}
			if a.day && v.Int() != int64(w.Day) && v.Int() != int64(w.Day+1) {
				return nil, &Error{Code: CodeInvalidParams, Message: fmt.Sprintf("%s: day %d: a quote reads today (%d) or tomorrow", m.name, v.Int(), w.Day)}
			}
			args = append(args, v)
		}
		out := reflect.ValueOf(s.sess.Rules()).FieldByName(sim).MethodByName(name).Call(args)
		switch {
		case okResult:
			if !out[1].Bool() {
				return nil, fmt.Errorf("nothing by that id")
			}
			return wireResult(out[0]), nil
		case len(out) == 1:
			return wireResult(out[0]), nil
		case len(out) > 1:
			v := reflect.New(m.result).Elem()
			for i, o := range out {
				v.Field(i).Set(o)
			}
			return v.Interface(), nil
		}
		return nil, nil
	}
	return m
}

// wireResult is a rule's answer as the wire carries it.
func wireResult(v reflect.Value) any {
	if rr, ok := ruleResults[v.Type()]; ok {
		return rr.id(v)
	}
	return v.Interface()
}

func exported(n string) string {
	if n == "" {
		return n
	}
	b := []byte(n)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 'a' - 'A'
	}
	return string(b)
}
