package game

import "testing"

// TestNewCashFlowWorksTheOpeningBack (#351): the flow lays every
// category out in order, a missing one as zero, and works the opening
// back from the close pile by pile, so it reconciles; the wash moves
// money between the piles and nothing on the total.
func TestNewCashFlowWorksTheOpeningBack(t *testing.T) {
	f := NewCashFlow(4, map[string]Pools{
		FlowSales:      {Dirty: 900},
		FlowLaundering: {Dirty: -500, Clean: 450},
		FlowLosses:     {Dirty: -100, Clean: -20},
	}, Pools{Dirty: 1300, Clean: 430})
	if len(f.Lines) != len(FlowCats) || f.Lines[0].Cat != FlowSales || f.Lines[len(f.Lines)-1].Cat != FlowOther {
		t.Fatalf("lines %+v, want one a category in FlowCats' order", f.Lines)
	}
	if f.Opening != (Pools{Dirty: 1000, Clean: 0}) || !f.Reconciles() {
		t.Fatalf("opening %+v, reconciles %v; want 1000 dirty, 0 clean", f.Opening, f.Reconciles())
	}
	if f.Line(FlowWages) != (Pools{}) || f.Line(FlowLaundering).Total() != -50 || f.Net() != 730 {
		t.Fatalf("wages %+v, laundering %+v, net %d", f.Line(FlowWages), f.Line(FlowLaundering), f.Net())
	}
	if !f.Big(f.Lines[0], 0.25) || f.Big(FlowLine{Cat: FlowLosses, Pools: f.Line(FlowLosses)}, 0.25) {
		t.Fatal("sales of 90% of the opening are big and losses of 12% are not, at a quarter")
	}
}

// TestChooseRecordsTheCash (#351): a card's answer keeps what the choice
// did to the piles, after the clamp, for the report's flow.
func TestChooseRecordsTheCash(t *testing.T) {
	w := &World{Player: Player{DirtyCash: 300, CleanCash: 50}}
	w.Dilemmas.Pending = &Card{ID: "c", Choices: []Choice{{Label: "pay", Effects: map[string]float64{"dirty_cash": -500, "clean_cash": 40}}}}
	a, err := w.Choose(0)
	if err != nil {
		t.Fatal(err)
	}
	if a.Cash != (Pools{Dirty: -300, Clean: 40}) || w.Dilemmas.Answered.Cash != a.Cash {
		t.Fatalf("cash %+v, want the clamp's -300 dirty and +40 clean", a.Cash)
	}
}
