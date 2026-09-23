package protocol

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// method is one method on the wire.
type method struct {
	name    string
	params  []param
	result  reflect.Type // nil for none
	run     bool         // needs a run
	changes bool         // may change the run: a view follows the response
	call    func(s *Server, params []json.RawMessage) (any, error)
}

// param is one positional parameter.
type param struct {
	name string
	typ  reflect.Type
}

// commands are the session's commands (engine/commands.go), each served
// under its name in snake_case with its parameters in order: travel,
// buy, place_sell, hire, … A dial goes by name ("aggressive"), terms as
// an object (TermsParams).
var commands = []string{
	"Travel", "SetLieLow", "SeeStage", "Choose",
	"Buy", "Return", "ReturnCredit", "ReturnSupplied", "PlaceSell", "CancelSell", "PlaceStanding", "CancelStanding",
	"SetSupply", "ClearSupply", "AcceptContract", "DeclineContract", "Deliver", "Cut", "Cook",
	"Post", "Abandon", "SendEnforcers", "Boost", "Undercut", "CancelUndercut", "Tip", "BuyDeed", "BuyHouse", "Drop", "Guard", "Move",
	"Hire", "Fire", "SetPay", "Investigate", "PayOff", "Bail", "Assign", "Unassign",
	"SetRoute", "SetRouteTarget", "SetRouteDays", "SetRouteDriver", "BuyCheckpoint",
	"SetLaunderDial", "BuyFront", "Invest", "BuyAsset", "Reserve", "BuyUpgrade",
	"Bribe", "Fund", "Back", "CallFavour", "PayCop",
	"ScoutFaction", "PlantSpy", "BuyOffFrom", "ProposeTo", "Accept", "Decline", "CallOff", "DeclareWar", "CallOffWar", "HitScouts", "Withdraw",
	"Retire", "Vanish", "Crown",
}

// queries are the session's reads served as they are: they change
// nothing.
var queries = []string{"View", "Alerts", "GatesAhead", "NextGates", "FrontOffers", "AssetOffers", "HouseOffers", "FloatMatters", "ExportSave", "MaxBuy", "RestockPlan"}

// unserved are the session's methods the wire does not carry, and why:
// TestEverySessionMethodIsClassed fails on one in no list, so a new
// command cannot miss the wire by accident.
var unserved = map[string]string{
	"Attach":      "a world built in the caller's process",
	"Config":      "the tuning is the server's",
	"Rules":       "served rule by rule as rules.<sim>.<method> (rules.go, #325)",
	"Sims":        "the harness's and the tests'",
	"World":       "a pointer into the run; the view is its wire form",
	"Subscribe":   "events are notifications",
	"Stop":        "fast_forward weighs the days itself",
	"NewRun":      "new_run, by hand: the start as three positions",
	"Load":        "load, by hand",
	"Save":        "save, by hand",
	"ImportSave":  "import_save, by hand: the save as base64, like load",
	"EndDay":      "end_day, by hand: the events go out as notifications",
	"FastForward": "fast_forward, by hand: the days are weighed server side",
}

// methods is every method on the wire, by name.
var methods = map[string]method{}

func init() {
	for _, name := range commands {
		register(reflected(name, true))
	}
	for _, name := range queries {
		register(reflected(name, false))
	}
	register(method{
		name:    "new_run",
		params:  []param{{"seed", reflect.TypeFor[uint64]()}, {"character", reflect.TypeFor[string]()}, {"hard_da", reflect.TypeFor[bool]()}},
		result:  reflect.TypeFor[engine.View](),
		changes: true,
		call: func(s *Server, ps []json.RawMessage) (any, error) {
			var seed uint64
			var start game.Start
			if err := decode(ps, 3, &seed, &start.Character, &start.HardDA); err != nil {
				return nil, err
			}
			s.sess.NewRun(seed, start)
			return s.sess.View(), nil
		},
	})
	register(method{
		name:    "load",
		params:  []param{{"slot", reflect.TypeFor[int]()}},
		result:  reflect.TypeFor[engine.View](),
		changes: true,
		call: func(s *Server, ps []json.RawMessage) (any, error) {
			var slot int
			if err := decode(ps, 1, &slot); err != nil {
				return nil, err
			}
			if _, err := s.sess.Load(slot); err != nil {
				return nil, err
			}
			return s.sess.View(), nil
		},
	})
	register(method{
		name:    "import_save",
		params:  []param{{"save", reflect.TypeFor[[]byte]()}},
		result:  reflect.TypeFor[engine.View](),
		changes: true,
		call: func(s *Server, ps []json.RawMessage) (any, error) {
			var b []byte
			if err := decode(ps, 1, &b); err != nil {
				return nil, err
			}
			if _, err := s.sess.ImportSave(b); err != nil {
				return nil, err
			}
			return s.sess.View(), nil
		},
	})
	register(method{
		name:   "save",
		params: []param{{"slot", reflect.TypeFor[int]()}},
		run:    true,
		call: func(s *Server, ps []json.RawMessage) (any, error) {
			var slot int
			if err := decode(ps, 1, &slot); err != nil {
				return nil, err
			}
			return nil, s.sess.Save(slot)
		},
	})
	register(method{
		name:    "end_day",
		result:  reflect.TypeFor[DayResult](),
		run:     true,
		changes: true,
		call: func(s *Server, ps []json.RawMessage) (any, error) {
			if err := decode(ps, 0); err != nil {
				return nil, err
			}
			evs := s.sess.EndDay()
			return DayResult{Day: s.sess.World().Day, Events: len(evs)}, nil
		},
	})
	register(method{
		name:    "fast_forward",
		params:  []param{{"days", reflect.TypeFor[int]()}},
		result:  reflect.TypeFor[FastResult](),
		run:     true,
		changes: true,
		call: func(s *Server, ps []json.RawMessage) (any, error) {
			var days int
			if err := decode(ps, 1, &days); err != nil {
				return nil, err
			}
			ran, st, _ := s.sess.FastForward(days, nil)
			r := FastResult{Ran: ran, Day: s.sess.World().Day, Stop: string(st.Kind)}
			if st.Kind == engine.StopAlert {
				a := st.Alert
				r.Alert = &a
			}
			if st.Kind == engine.StopEvent {
				r.Event = st.Event.Kind()
			}
			return r, nil
		},
	})
}

// DayResult is end_day's answer: the day it is now and how many events
// the night published (each went out as an `event` before it).
type DayResult struct {
	Day    int `json:"day"`
	Events int `json:"events"`
}

// FastResult is fast_forward's answer: the days run, the day it is now
// and why it stopped (stage, card, alert, event, over, cap), with the
// alert or the event's kind that stopped it.
type FastResult struct {
	Ran   int           `json:"ran"`
	Day   int           `json:"day"`
	Stop  string        `json:"stop"`
	Alert *engine.Alert `json:"alert,omitempty"`
	Event string        `json:"event,omitempty"`
}

// TermsParams is game.Terms as the wire spells it.
type TermsParams struct {
	Days    int      `json:"days,omitempty"`
	PerDay  int      `json:"per_day,omitempty"`
	Corners []string `json:"corners,omitempty"`
	Route   string   `json:"route,omitempty"`
	Units   int      `json:"units,omitempty"`
}

func register(m method) {
	if _, dup := methods[m.name]; dup {
		panic("protocol: two methods named " + m.name)
	}
	methods[m.name] = m
}

// snake is a Go name in snake_case: BuyCheckpoint is buy_checkpoint.
func snake(name string) string {
	var b strings.Builder
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

var (
	sessionType = reflect.TypeFor[*engine.Session]()
	errorType   = reflect.TypeFor[error]()
	termsType   = reflect.TypeFor[game.Terms]()
)

// reflected is a session method served under its snake_case name, its
// parameters decoded by position and its results mapped: a trailing
// error is the call's error, the value before it the result.
func reflected(name string, changes bool) method {
	fn, ok := sessionType.MethodByName(name)
	if !ok {
		panic("protocol: engine.Session has no method " + name)
	}
	t := fn.Type
	m := method{name: snake(name), run: true, changes: changes}
	names := paramNames[name]
	for i := 1; i < t.NumIn(); i++ {
		pn := fmt.Sprintf("arg%d", i)
		if i-1 < len(names) {
			pn = names[i-1]
		}
		m.params = append(m.params, param{pn, t.In(i)})
	}
	outs := t.NumOut()
	if outs > 0 && t.Out(outs-1) == errorType {
		outs--
	}
	if outs == 1 {
		m.result = t.Out(0)
	}
	m.call = func(s *Server, ps []json.RawMessage) (any, error) {
		if len(ps) != len(m.params) {
			return nil, &Error{Code: CodeInvalidParams, Message: fmt.Sprintf("%s takes %d params, got %d", m.name, len(m.params), len(ps))}
		}
		args := []reflect.Value{reflect.ValueOf(s.sess)}
		for i, p := range m.params {
			v, err := decodeArg(ps[i], p.typ)
			if err != nil {
				return nil, &Error{Code: CodeInvalidParams, Message: fmt.Sprintf("%s: %s: %v", m.name, p.name, err)}
			}
			args = append(args, v)
		}
		out := fn.Func.Call(args)
		if n := len(out); n > 0 && t.Out(n-1) == errorType {
			if err, _ := out[n-1].Interface().(error); err != nil {
				return nil, err
			}
			out = out[:n-1]
		}
		if len(out) == 1 {
			return out[0].Interface(), nil
		}
		return nil, nil
	}
	return m
}

// decodeArg is one parameter: a dial by its name, terms as TermsParams,
// anything else as encoding/json reads it.
func decodeArg(raw json.RawMessage, t reflect.Type) (reflect.Value, error) {
	if names := dialNames(t); names != nil {
		var name string
		if err := json.Unmarshal(raw, &name); err != nil {
			return reflect.Value{}, fmt.Errorf("a %s goes by name, one of %s", t.Name(), strings.Join(names, ", "))
		}
		for i, n := range names {
			if n == name {
				return reflect.ValueOf(i).Convert(t), nil
			}
		}
		return reflect.Value{}, fmt.Errorf("no %s named %q: one of %s", t.Name(), name, strings.Join(names, ", "))
	}
	if t == termsType {
		var tp TermsParams
		if err := json.Unmarshal(raw, &tp); err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(game.Terms{Days: tp.Days, PerDay: tp.PerDay, Corners: tp.Corners, Route: tp.Route, Units: tp.Units}), nil
	}
	v := reflect.New(t)
	if err := json.Unmarshal(raw, v.Interface()); err != nil {
		return reflect.Value{}, err
	}
	return v.Elem(), nil
}

// dialNames is a dial type's names in the order of its values, or nil
// for a type that is not one: the events package's tables, read through
// String until it repeats its default for a value off the table.
func dialNames(t reflect.Type) []string {
	if t.PkgPath() != reflect.TypeFor[events.Dial]().PkgPath() || t.Kind() != reflect.Int {
		return nil
	}
	if _, ok := reflect.New(t).Elem().Interface().(fmt.Stringer); !ok {
		return nil
	}
	var names []string
	seen := map[string]bool{}
	for i := 0; i < 32; i++ {
		n := reflect.ValueOf(i).Convert(t).Interface().(fmt.Stringer).String()
		if seen[n] {
			break
		}
		seen[n] = true
		names = append(names, n)
	}
	return names
}

// decode reads exactly n positional params into ptrs.
func decode(ps []json.RawMessage, n int, ptrs ...any) error {
	if len(ps) != n {
		return &Error{Code: CodeInvalidParams, Message: fmt.Sprintf("takes %d params, got %d", n, len(ps))}
	}
	for i, p := range ptrs {
		if err := json.Unmarshal(ps[i], p); err != nil {
			return &Error{Code: CodeInvalidParams, Message: fmt.Sprintf("param %d: %v", i+1, err)}
		}
	}
	return nil
}

// paramNames are the parameters' names, for the schema and the errors:
// reflection cannot see them.
var paramNames = map[string][]string{
	"Travel": {"city"}, "SetLieLow": {"on"}, "SeeStage": {"stage"}, "Choose": {"choice"},
	"Buy": {"supplier", "product", "qty", "credit"}, "MaxBuy": {"supplier", "product", "credit"}, "RestockPlan": {"city", "days"}, "Return": {"city", "product", "qty"},
	"ReturnCredit": {"city", "product", "qty"}, "ReturnSupplied": {"city", "product", "qty"},
	"PlaceSell": {"city", "product", "qty", "dial"}, "CancelSell": {"city", "product"},
	"PlaceStanding": {"city", "product", "qty", "dial"}, "CancelStanding": {"city", "product"},
	"SetSupply": {"city", "product", "units"}, "ClearSupply": {"city", "product"},
	"AcceptContract": {"contract"}, "DeclineContract": {"contract"}, "Deliver": {"contract", "units"},
	"Cut": {"city", "product", "ratio"}, "Cook": {"city", "product", "units"},
	"Post": {"corner", "member"}, "Abandon": {"corner"}, "SendEnforcers": {"corner", "force"},
	"Boost": {"corner", "force"}, "Undercut": {"corner", "dial"}, "CancelUndercut": {"corner"},
	"Tip": {"corner"}, "BuyDeed": {"corner"}, "BuyHouse": {"house"}, "Drop": {"house"},
	"Guard": {"house", "member"}, "Move": {"city", "from", "to", "product", "units"},
	"Hire": {"candidate"}, "Fire": {"member"}, "SetPay": {"pay"}, "PayOff": {"member"}, "Bail": {"member"},
	"Assign": {"member", "city"}, "Unassign": {"member"},
	"SetRoute": {"route", "dial"}, "SetRouteTarget": {"route", "product", "units"},
	"SetRouteDays": {"route", "product", "days"}, "SetRouteDriver": {"route", "member"}, "BuyCheckpoint": {"route"},
	"SetLaunderDial": {"dial"}, "BuyFront": {"front"}, "Invest": {"front", "levels"}, "BuyAsset": {"asset"},
	"Reserve": {"amount"}, "BuyUpgrade": {"upgrade"},
	"Bribe": {"target", "amount"}, "Fund": {"city", "amount"}, "Back": {"city", "ticket", "amount"}, "PayCop": {"amount"},
	"ScoutFaction": {"faction"}, "PlantSpy": {"faction", "member"}, "BuyOffFrom": {"faction", "units"},
	"ProposeTo": {"faction", "kind", "terms"}, "Accept": {"offer"}, "Decline": {"offer"}, "DeclareWar": {"faction"}, "HitScouts": {"faction"},
}
