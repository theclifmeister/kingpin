package content

import (
	"strings"
	"testing"
)

// The deed table (#194): the file's is on and sane, an empty one is
// off and passes, and push_mul of 0 with deeds on sale is refused: a
// deed slows the rival and never stops it.
func TestDeedTuning(t *testing.T) {
	cfg := MustLoad()
	d := cfg.City.Deed
	if !d.On() || d.PushMul <= 0 || d.PushMul >= 1 || d.RobberyMul <= 0 || d.RaidMul <= 0 || d.Rent <= 0 || d.ForfeitRatio <= 0 || d.ForfeitEvidence <= 0 || d.HeadlineDeeds <= 0 {
		t.Fatalf("the file's deed table: %+v", d)
	}
	if err := (DeedTuning{}).validate(); err != nil {
		t.Fatalf("an empty table refused: %v", err)
	}
	bad := d
	bad.PushMul = 0
	if err := bad.validate(); err == nil || !strings.Contains(err.Error(), "push_mul") {
		t.Fatalf("push_mul 0 accepted, or refused for another reason: %v", err)
	}
	bad = d
	bad.Rent = -1
	if err := bad.validate(); err == nil {
		t.Fatal("a negative rent accepted")
	}
	city := cfg.City
	city.Deed.PushMul = 0
	if err := city.validate(); err == nil {
		t.Fatal("the city config accepted a deed table that stops the rival")
	}
}
