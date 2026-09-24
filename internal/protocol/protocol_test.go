package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

var update = flag.Bool("update", false, "rewrite schema.json from the Go types")

// loop is a writer that hands each request line to a server in the same
// process and keeps what it answers for a reader: the protocol without
// a process or a pipe between them.
type loop struct {
	srv *Server
	out bytes.Buffer
}

func (l *loop) Write(p []byte) (int, error) {
	for _, line := range l.srv.Handle(bytes.TrimRight(p, "\n")) {
		l.out.Write(line)
		l.out.WriteByte('\n')
	}
	return len(p), nil
}

// loopClient is an RPCClient on a server in the same process.
func loopClient(t *testing.T) (*RPCClient, *Server) {
	t.Helper()
	srv, err := NewServer(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	l := &loop{srv: srv}
	return NewRPCClient(l, &l.out), srv
}

// direct is the baseline: the same client calls made on an
// engine.Session in this process, no protocol between them, its events
// encoded as the wire encodes them. It serves only what Play calls.
type direct struct {
	s      *engine.Session
	events []json.RawMessage
}

func newDirect(t *testing.T) *direct {
	t.Helper()
	s, err := engine.New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	d := &direct{s: s}
	s.Subscribe(func(e events.Event) {
		raw, err := EventJSON(e, s.World().Day)
		if err != nil {
			t.Fatal(err)
		}
		d.events = append(d.events, raw)
	})
	return d
}

func (d *direct) Events() []json.RawMessage { return d.events }

func (d *direct) Call(method string, ps []any, result any) error {
	var out any
	var err error
	switch method {
	case "new_run":
		d.s.NewRun(ps[0].(uint64), game.Start{Character: ps[1].(string), HardDA: ps[2].(bool)})
		out = d.s.View()
	case "view":
		out = d.s.View()
	case "choose":
		out, err = d.s.Choose(ps[0].(int))
	case "buy":
		out, err = d.s.Buy(ps[0].(string), ps[1].(string), ps[2].(int), ps[3].(bool))
	case "place_sell":
		dial, _ := events.ParseDial(ps[3].(string))
		err = d.s.PlaceSell(ps[0].(string), ps[1].(string), ps[2].(int), dial)
	case "end_day":
		evs := d.s.EndDay()
		out = DayResult{Day: d.s.World().Day, Events: len(evs)}
	default:
		return errors.New("direct: no " + method)
	}
	if err != nil {
		return &Error{Code: CodeRefused, Message: err.Error()}
	}
	if result == nil {
		return nil
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, result)
}

// finalView is the run's last view as JSON, asked for through the client.
func finalView(t *testing.T, c Client) []byte {
	t.Helper()
	var raw json.RawMessage
	if err := c.Call("view", nil, &raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

// sameRun fails unless two clients saw the same run: every event byte
// for byte, and the last view.
func sameRun(t *testing.T, a, b Client, va, vb []byte) {
	t.Helper()
	ea, eb := a.Events(), b.Events()
	if len(ea) != len(eb) {
		t.Fatalf("%d events against %d", len(ea), len(eb))
	}
	for i := range ea {
		if !bytes.Equal(ea[i], eb[i]) {
			t.Fatalf("event %d differs:\n%s\n%s", i, ea[i], eb[i])
		}
	}
	if !bytes.Equal(va, vb) {
		t.Fatal("the last views differ")
	}
}

const playDays = 400

// TestProtocolIsTheSession (#300): the reference client's game played on
// a session directly and through the protocol (a server in this
// process) is the same game: the same events byte for byte and the
// same last view. It reaches an ending.
func TestProtocolIsTheSession(t *testing.T) {
	t.Parallel()
	d := newDirect(t)
	vd, err := Play(d, 7, playDays)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := loopClient(t)
	vc, err := Play(c, 7, playDays)
	if err != nil {
		t.Fatal(err)
	}
	if vd.Over == nil || vc.Over == nil {
		t.Fatalf("no ending in %d days: day %d / %d", playDays, vd.Day, vc.Day)
	}
	t.Logf("the run ended on day %d: %s, %d events", vc.Over.Day, vc.Over.Cause, len(c.Events()))
	sameRun(t, d, c, finalView(t, d), finalView(t, c))
	var last engine.View
	if err := json.Unmarshal(c.LastView(), &last); err != nil || last.Day != vc.Day {
		t.Fatalf("the last view notification is not the run's last day: %v, day %d", err, last.Day)
	}
}

// TestOverStdio (#300): the same game through a kingpind process over
// stdin and stdout.
func TestOverStdio(t *testing.T) {
	if testing.Short() {
		t.Skip("builds kingpind")
	}
	t.Parallel()
	bin := filepath.Join(t.TempDir(), "kingpind")
	build := exec.Command("go", "build", "-o", bin, "../../cmd/kingpind")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), "KINGPIN_HOME="+t.TempDir())
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	c := NewRPCClient(in, out)
	v, err := Play(c, 7, playDays)
	if err != nil {
		t.Fatal(err)
	}
	vc := finalView(t, c)
	in.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("kingpind: %v", err)
	}
	if v.Over == nil {
		t.Fatalf("no ending in %d days over stdio", playDays)
	}
	d := newDirect(t)
	if _, err := Play(d, 7, playDays); err != nil {
		t.Fatal(err)
	}
	sameRun(t, d, c, finalView(t, d), vc)
}

// TestErrors (#300): every way a call goes wrong has its code.
func TestErrors(t *testing.T) {
	t.Parallel()
	c, srv := loopClient(t)
	code := func(err error) int {
		var e *Error
		if errors.As(err, &e) {
			return e.Code
		}
		return 0
	}
	if err := c.Call("view", nil, nil); code(err) != CodeNoRun {
		t.Errorf("view before a run: %v", err)
	}
	if err := c.Call("no_such", nil, nil); code(err) != CodeNoMethod {
		t.Errorf("no_such: %v", err)
	}
	if err := c.Call("new_run", []any{7}, nil); code(err) != CodeInvalidParams {
		t.Errorf("new_run with one param: %v", err)
	}
	if err := c.Call("new_run", []any{7, "", false}, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Call("place_sell", []any{"eastside", "weed", 1, "loud"}, nil); code(err) != CodeInvalidParams || !strings.Contains(err.Error(), "aggressive") {
		t.Errorf("a dial by a name it does not have: %v", err)
	}
	if err := c.Call("travel", []any{"atlantis"}, nil); code(err) != CodeRefused {
		t.Errorf("travel to nowhere: %v", err)
	}
	if err := c.Call("buy_front", []any{"nope"}, nil); code(err) != CodeRefused || !strings.Contains(err.Error(), engine.ErrNoOffer.Error()) {
		t.Errorf("buy_front of nothing on offer: %v", err)
	}
	for _, bad := range []string{`not json`, `{"method":"view"}`, `{"jsonrpc":"2.0","id":9,"method":"view","params":{"a":1}}`} {
		lines := srv.Handle([]byte(bad))
		var r Response
		if err := json.Unmarshal(lines[len(lines)-1], &r); err != nil || r.Error == nil {
			t.Errorf("%s: answered %s", bad, lines[len(lines)-1])
			continue
		}
		want := map[string]int{`not json`: CodeParse, `{"method":"view"}`: CodeInvalidRequest}[bad]
		if want == 0 {
			want = CodeInvalidParams
		}
		if r.Error.Code != want {
			t.Errorf("%s: code %d, want %d", bad, r.Error.Code, want)
		}
	}
}

// TestEverySessionMethodIsClassed (#300): every method a session has is
// on the wire as a command or a query, or named in unserved with why,
// so a command added to the engine is served or refused on purpose.
func TestEverySessionMethodIsClassed(t *testing.T) {
	t.Parallel()
	classed := map[string]bool{}
	for _, n := range commands {
		classed[n] = true
	}
	for _, n := range queries {
		classed[n] = true
	}
	for n := range unserved {
		classed[n] = true
	}
	st := reflect.TypeFor[*engine.Session]()
	for i := 0; i < st.NumMethod(); i++ {
		if n := st.Method(i).Name; !classed[n] {
			t.Errorf("engine.Session.%s is on the wire nowhere: add it to commands or queries, or say why in unserved", n)
		}
	}
	for n := range classed {
		if _, ok := st.MethodByName(n); !ok {
			t.Errorf("%s is classed but engine.Session has no such method", n)
		}
	}
	if len(commands) != 70 {
		t.Errorf("%d commands on the wire, docs/engine.md says 70", len(commands))
	}
}

// TestSchemaIsCurrent (#300): schema.json is what Schema generates from
// the Go types; `go test ./internal/protocol -run TestSchemaIsCurrent
// -update` rewrites it.
func TestSchemaIsCurrent(t *testing.T) {
	got, err := Schema()
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if *update {
		if err := os.WriteFile("schema.json", got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile("schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("schema.json is stale: run go test ./internal/protocol -run TestSchemaIsCurrent -update")
	}
	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatal(err)
	}
	ms := doc["methods"].(map[string]any)
	if len(ms) != len(methods) || ms["buy"] == nil || ms["end_day"] == nil || ms["view"] == nil {
		t.Fatalf("the schema lists %d methods of %d", len(ms), len(methods))
	}
}

// TestHireFromTheView (#332): a client that sees only the view hires
// over the protocol: the pool's first id, and the next view has them on
// the payroll and out of the pool.
func TestHireFromTheView(t *testing.T) {
	t.Parallel()
	c, _ := loopClient(t)
	var v engine.View
	if err := c.Call("new_run", []any{7, "", false}, &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Pool) == 0 {
		t.Fatal("nobody in the view's pool on day 0")
	}
	id := v.Pool[0].ID
	if err := c.Call("hire", []any{id}, nil); err != nil {
		t.Fatalf("hire %d: %v", id, err)
	}
	var after engine.View
	if err := json.Unmarshal(c.LastView(), &after); err != nil {
		t.Fatal(err)
	}
	onPayroll := false
	for _, m := range after.Crew {
		onPayroll = onPayroll || m.ID == id
	}
	for _, m := range after.Pool {
		if m.ID == id {
			t.Error("hired and still in the pool")
		}
	}
	if !onPayroll {
		t.Errorf("hired %d and not on the payroll: %+v", id, after.Crew)
	}
}

// A buy past the room is refused as no_room with what fits (#356), and
// max_buy is what a buy takes without a refusal.
func TestNoRoomIsTyped(t *testing.T) {
	t.Parallel()
	c, srv := loopClient(t)
	if err := c.Call("new_run", []any{7, "", false}, nil); err != nil {
		t.Fatal(err)
	}
	w := srv.sess.World()
	w.Player.DirtyCash = 1_000_000_000
	sup := w.StreetSupplier(w.Player.Location)
	sup.Cap = 1_000_000
	product := w.Products[0]
	var room engine.BuyRoom
	if err := c.Call("max_buy", []any{sup.ID, product, false}, &room); err != nil {
		t.Fatal(err)
	}
	free := room.Capacity - room.Held
	if room.Max != free || free <= 0 {
		t.Fatalf("max_buy with the cash to fill the stash: %+v", room)
	}
	err := c.Call("buy", []any{sup.ID, product, free + 1, false}, nil)
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeNoRoom || e.Data == nil || e.Data.Free != free || !Refused(err) {
		t.Fatalf("a buy one past the room: %v (%+v)", err, e)
	}
	if err := c.Call("buy", []any{sup.ID, product, room.Max, false}, nil); err != nil {
		t.Fatalf("a buy of max_buy's %d: %v", room.Max, err)
	}
	var plan []game.RestockLine
	if err := c.Call("restock_plan", []any{w.Player.Location, 2.0}, &plan); err != nil || plan == nil {
		t.Fatalf("restock_plan on a full stash: %v %v", plan, err)
	}
}

// The presets on the wire (#357): the list, a diff that changes
// nothing, and apply_preset returning the same review; every op a
// preset issues is a command the wire serves under that name, so a
// client can issue a preset's commands itself.
func TestPresetsOnTheWire(t *testing.T) {
	t.Parallel()
	for _, op := range engine.Ops() {
		if m, ok := methods[op]; !ok || !m.changes {
			t.Errorf("a preset's op %q is no command on the wire", op)
		}
	}
	c, srv := loopClient(t)
	if err := c.Call("new_run", []any{7, "", false}, nil); err != nil {
		t.Fatal(err)
	}
	w := srv.sess.World()
	var list []engine.Preset
	if err := c.Call("presets", nil, &list); err != nil || len(list) == 0 || list[0].ID != "quiet" {
		t.Fatalf("presets: %+v %v", list, err)
	}
	if err := c.Call("set_launder_dial", []any{"greedy"}, nil); err != nil {
		t.Fatal(err)
	}
	var diff, applied engine.Review
	if err := c.Call("preset_diff", []any{"quiet"}, &diff); err != nil || len(diff.Changes) != 1 || diff.Changes[0].To != "careful" {
		t.Fatalf("preset_diff: %+v %v", diff, err)
	}
	if w.Laundering.Dial != events.LaunderGreedy {
		t.Fatal("preset_diff turned the dial")
	}
	if err := c.Call("apply_preset", []any{"quiet"}, &applied); err != nil || !reflect.DeepEqual(diff, applied) {
		t.Fatalf("apply_preset: %+v %v", applied, err)
	}
	if w.Laundering.Dial != events.LaunderCareful {
		t.Fatal("apply_preset left the dial")
	}
	if err := c.Call("preset_diff", []any{"nothing"}, nil); !Refused(err) {
		t.Errorf("an unknown preset: %v", err)
	}
}
