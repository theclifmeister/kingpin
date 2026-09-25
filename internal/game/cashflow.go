package game

// Pools is an amount of cash in each pile: dirty, off the street, and
// clean, out of the wash. A flow line is signed, money in positive.
type Pools struct {
	Dirty int
	Clean int
}

// Total is the two piles together, World.Cash's sense.
func (p Pools) Total() int { return p.Dirty + p.Clean }

// Add is p and q summed pile by pile.
func (p Pools) Add(q Pools) Pools { return Pools{Dirty: p.Dirty + q.Dirty, Clean: p.Clean + q.Clean} }

// The cash flow's categories (#351), in the order the money moves
// through a night: what came in off the street, what went out on
// product, the road and the errands, the payroll, the wash, what was
// put into the operation, what was lost, the tax, and the cards and
// the rival's takings. A category is a word in a save and on the wire;
// a new one is appended and reads as zero on an old flow.
const (
	FlowSales       = "sales"       // the street and the buyers, net of the cut the crew and the lieutenants keep
	FlowPurchases   = "purchases"   // product: the connects, the contracts, the cuts, the cook, a debt paid down
	FlowRoutes      = "routes"      // the road and the errands: lots and fares, checkpoints, signing fees, the rent on the houses, investigations, scouts, cops, bail
	FlowWages       = "wages"       // the payroll, and the loyalty bought on top
	FlowLaundering  = "laundering"  // the wash (dirty out, clean in), the upkeep, what the fronts earn, the offshore account's fee
	FlowInvestments = "investments" // upgrades, fronts, assets, levels, houses, deeds and their rent, the cities funded, campaigns, bribes, the rival paid or paying
	FlowLosses      = "losses"      // robbed, skimmed, seized by the police or the auditors, a buyer collecting
	FlowTax         = "tax"         // the free corners of a city you hold (#231)
	FlowOther       = "other"       // a card's cash, the rival's takings your enforcers boosted
	FlowOffshore    = "offshore"    // clean cash put in the offshore account (#422): still yours, so no night's profit counts it
)

// FlowCats is every category in order: CashFlow.Lines holds one line
// each, zero or not.
var FlowCats = []string{FlowSales, FlowPurchases, FlowRoutes, FlowWages, FlowLaundering, FlowInvestments, FlowLosses, FlowTax, FlowOther, FlowOffshore}

// FlowLine is one category of a night's flow, signed by pile.
type FlowLine struct {
	Cat string
	Pools
}

// CashFlow is one night's money explained (#351): the piles the day
// opened on, what each category moved, and the piles it closed on. It
// reconciles by construction, pile by pile (Opening + Sum = Closing):
// the news sim reads the closing off the world and the lines off the
// night's totals, and works the opening back, the way CASH BEFORE
// always was. That the opening is the night before's closing is what
// TestCashFlowReconciles holds across every policy. It is a report:
// no sim reads it.
type CashFlow struct {
	Day     int
	Opening Pools
	Lines   []FlowLine
	Closing Pools
}

// Line is the category's amounts, zero where the flow has no such line.
func (f CashFlow) Line(cat string) Pools {
	for _, l := range f.Lines {
		if l.Cat == cat {
			return l.Pools
		}
	}
	return Pools{}
}

// Sum is every line together, pile by pile.
func (f CashFlow) Sum() Pools {
	var s Pools
	for _, l := range f.Lines {
		s = s.Add(l.Pools)
	}
	return s
}

// Net is what the night moved, both piles: closing less opening.
func (f CashFlow) Net() int { return f.Closing.Total() - f.Opening.Total() }

// Reconciles says the opening and the lines make the closing, dirty and
// clean each.
func (f CashFlow) Reconciles() bool {
	return f.Opening.Add(f.Sum()) == f.Closing
}

// NewCashFlow is a night's flow from its lines and the piles it closed
// on: the opening is worked back, pile by pile, so it reconciles. The
// lines are laid out in FlowCats' order, a category lines misses as zero.
func NewCashFlow(day int, lines map[string]Pools, closing Pools) CashFlow {
	f := CashFlow{Day: day, Closing: closing, Lines: make([]FlowLine, 0, len(FlowCats))}
	for _, c := range FlowCats {
		f.Lines = append(f.Lines, FlowLine{Cat: c, Pools: lines[c]})
	}
	s := f.Sum()
	f.Opening = Pools{Dirty: closing.Dirty - s.Dirty, Clean: closing.Clean - s.Clean}
	return f
}

// FlowLabel is a category as the report and the ledger print it.
func FlowLabel(cat string) string {
	switch cat {
	case FlowSales:
		return "Sales, net of cuts"
	case FlowPurchases:
		return "Purchases"
	case FlowRoutes:
		return "Road, hires, fees" // #465: a signing fee on a night no route ran read as the road's
	case FlowWages:
		return "Wages"
	case FlowLaundering:
		return "Laundering"
	case FlowInvestments:
		return "Investments, tribute" // #465: homage in read as a return on an investment
	case FlowLosses:
		return "Losses"
	case FlowTax:
		return "Tax"
	case FlowOther:
		return "Cards and takings"
	case FlowOffshore:
		return "Offshore"
	}
	return cat
}

// Big says a line moved more than share of the cash the night opened
// on, either way: the report picks it out (headlines.toml [flow]
// big_share). On an empty opening any line that moved is big.
func (f CashFlow) Big(l FlowLine, share float64) bool {
	n := l.Total()
	if n < 0 {
		n = -n
	}
	return n > 0 && float64(n) > share*float64(max(f.Opening.Total(), 0))
}
