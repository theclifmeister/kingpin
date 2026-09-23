package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

// TestSaveBytesRoundTrip (#327): a front end with no slots keeps the run
// as export_save's bytes, and import_save on another server plays on
// from it exactly as the run that was never saved. Nothing goes to the
// slots, and bytes that are not a save are refused.
func TestSaveBytesRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("KINGPIN_HOME", home)
	play := func(c *RPCClient, days int) []json.RawMessage {
		t.Helper()
		from := len(c.Events())
		for d := 0; d < days; d++ {
			if err := c.Call("end_day", nil, nil); err != nil {
				t.Fatal(err)
			}
		}
		return c.Events()[from:]
	}
	a, _ := loopClient(t)
	if err := a.Call("new_run", []any{7, "", false}, nil); err != nil {
		t.Fatal(err)
	}
	play(a, 20)
	var save []byte
	if err := a.Call("export_save", nil, &save); err != nil {
		t.Fatal(err)
	}
	if len(save) < 1000 {
		t.Fatalf("a save of %d bytes", len(save))
	}
	want := play(a, 20)

	b, _ := loopClient(t)
	if err := b.Call("import_save", []any{save}, nil); err != nil {
		t.Fatal(err)
	}
	got := play(b, 20)
	if len(got) != len(want) || len(got) == 0 {
		t.Fatalf("%d events after the import, %d in the run never saved", len(got), len(want))
	}
	for i := range got {
		if !bytes.Equal(got[i], want[i]) {
			t.Fatalf("event %d after the import: %s, the run never saved: %s", i, got[i], want[i])
		}
	}
	if !bytes.Equal(a.LastView(), b.LastView()) {
		t.Error("the views part after the import")
	}
	if entries, _ := os.ReadDir(home); len(entries) != 0 {
		t.Errorf("export and import wrote %d files to the slots", len(entries))
	}

	c, _ := loopClient(t)
	var e *Error
	if err := c.Call("import_save", []any{[]byte("not a save")}, nil); !errors.As(err, &e) || e.Code != CodeRefused {
		t.Errorf("import of rubbish: %v, want refused", err)
	}
	if err := c.Call("export_save", nil, nil); !errors.As(err, &e) || e.Code != CodeNoRun {
		t.Errorf("export with no run: %v, want no run", err)
	}
}
