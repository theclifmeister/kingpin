package protocol

import "testing"

// TestFastForwardSaysDanger (#550): fast_forward's answer says whether
// its stop is a danger (engine.Stop.Danger), so a front end styles an
// event stop, which carries no alert to read it off, as the TUI does. A
// talking informant on the payroll is run into until the danger stops
// the days; every stop on the way agrees with its alert.
func TestFastForwardSaysDanger(t *testing.T) {
	t.Parallel()
	c, srv := loopClient(t)
	if err := c.Call("new_run", []any{uint64(7), "", false}, nil); err != nil {
		t.Fatal(err)
	}
	w := srv.sess.World()
	w.Player.DirtyCash += 100_000
	pool := srv.sess.View().Pool
	if len(pool) == 0 {
		t.Fatal("nobody to hire")
	}
	if _, err := srv.sess.Hire(pool[0].ID); err != nil {
		t.Fatal(err)
	}
	for i := range w.Crew.Members {
		w.Crew.Members[i].Informant = true
	}
	for range 40 {
		var r FastResult
		if err := c.Call("fast_forward", []any{30}, &r); err != nil {
			t.Fatal(err)
		}
		if r.Alert != nil && r.Danger != r.Alert.Danger() {
			t.Fatalf("day %d: danger %v on the %s alert", r.Day, r.Danger, r.Alert.Kind)
		}
		if r.Danger {
			t.Logf("day %d: stopped on a danger (%s %s)", r.Day, r.Stop, r.Event)
			return
		}
		if r.Stop == "over" || r.Stop == "" {
			break
		}
		for srv.sess.World().Dilemmas.Pending != nil {
			if err := c.Call("choose", []any{0}, nil); err != nil {
				t.Fatal(err)
			}
		}
		if n := w.StagePending(); n > 0 {
			srv.sess.SeeStage(n)
		}
	}
	t.Fatal("an informant talked for the whole run and no stop was a danger")
}
