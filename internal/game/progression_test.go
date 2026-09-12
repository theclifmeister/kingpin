package game

import "testing"

// The stage pending (#149) is the highest tier reached and not seen,
// never tier 1, and seeing it marks every tier under it seen too, so an
// old save with three tiers reached is shown one stage.
func TestStagePending(t *testing.T) {
	w := &World{}
	if w.StagePending() != 0 {
		t.Fatalf("a fresh world has stage %d pending", w.StagePending())
	}
	w.Reach(2, 3)
	if w.StagePending() != 2 {
		t.Fatalf("tier 2 reached: pending %d", w.StagePending())
	}
	w.SeeStage(2)
	if w.StagePending() != 0 || !w.Progression.Seen[2] {
		t.Fatalf("tier 2 seen: pending %d seen %v", w.StagePending(), w.Progression.Seen)
	}
	w.Reach(3, 8)
	w.Reach(4, 20)
	if w.StagePending() != 4 {
		t.Fatalf("tiers 3 and 4 reached: pending %d", w.StagePending())
	}
	w.SeeStage(4)
	if w.StagePending() != 0 || !w.Progression.Seen[3] || !w.Progression.Seen[4] {
		t.Fatalf("tier 4 seen: pending %d seen %v", w.StagePending(), w.Progression.Seen)
	}
	// Seen is never stamped for a tier not reached.
	if len(w.Progression.Seen) != 3 {
		t.Fatalf("seen %v", w.Progression.Seen)
	}
}
