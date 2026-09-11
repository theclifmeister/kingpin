package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// kindPatterns is what a cell of each kind reads as once rendered: a
// number column may also hold `-` for none.
var kindPatterns = map[colKind]*regexp.Regexp{
	kInt:   regexp.MustCompile(`^(-|~?[+-]?\d+)$`),
	kCash:  regexp.MustCompile(`^(-|-?\$(\d{1,3}(,\d{3})*|\d+\.\d[KMBT]|\d{2,3}[KMBT]))$`),
	kMoney: regexp.MustCompile(`^(-|-?\$\d{1,3}(,\d{3})*)$`),
	kPrice: regexp.MustCompile(`^(-|\$\d{1,3}(,\d{3})*(\.\d\d)?)$`),
	kPct:   regexp.MustCompile(`^(-|~?[+-]?\d+(\.\d)?%)$`),
	kDays:  regexp.MustCompile(`^(-|\d+d|d\d+)$`),
	kBar:   regexp.MustCompile(`^(-|[█░┆]+ \d+|[▁▂▃▄▅▆▇█]+( [▲▼])?)$`),
	kDial:  regexp.MustCompile(`^(-|\d+ (quiet|normal|aggr\.)( \(lt\))?|off|slow|normal|fast)$`),
}

// cells splits one rendered table line into its cells by the widths
// the table was drawn at: the gutter, then each column two apart.
func cells(cols []col, line string) []string {
	rs := []rune(stripANSI(line))
	at := 2
	var out []string
	for i, c := range cols {
		if i > 0 {
			at += 2
		}
		end := min(len(rs), at+c.width)
		if at > len(rs) {
			out = append(out, "")
			continue
		}
		out = append(out, strings.TrimSpace(string(rs[at:end])))
		at = end
	}
	return out
}

// checkTable parses a rendered table, the header and its rows, and
// asserts that every column has a title, that its cells read as its
// declared kind, that no number or money cell is cut with …, and that
// `-` stands for none only in number columns.
func checkTable(t *testing.T, cols []col, lines []string) {
	t.Helper()
	if len(lines) == 0 {
		t.Fatal("table has no header")
	}
	if len(cols) == 0 {
		t.Fatal("table has no columns")
	}
	head := cells(cols, lines[0])
	for i, c := range cols {
		if c.title == "" {
			t.Errorf("column %d has no title (%q)", i, stripANSI(lines[0]))
		}
		if head[i] != c.title && !strings.HasSuffix(head[i], "…") {
			t.Errorf("column %d titled %q reads %q", i, c.title, head[i])
		}
	}
	for r, line := range lines[1:] {
		got := cells(cols, line)
		for i, c := range cols {
			cell := got[i]
			switch c.kind {
			case kText:
				if cell == "-" {
					t.Errorf("row %d, %s: a text column holds - (%q)", r, c.title, stripANSI(line))
				}
			default:
				if strings.HasSuffix(cell, "…") {
					t.Errorf("row %d, %s: a %s cell is cut: %q (%q)", r, c.title, kindName(c.kind), cell, stripANSI(line))
				} else if !kindPatterns[c.kind].MatchString(cell) {
					t.Errorf("row %d, %s: %q does not read as %s (%q)", r, c.title, cell, kindName(c.kind), stripANSI(line))
				}
			}
		}
	}
}

func kindName(k colKind) string {
	return [...]string{"text", "int", "cash", "money", "price", "pct", "days", "bar", "dial"}[k]
}

// TestTableFormatsByKind: every kind writes its cells its own way, nil
// is `-` or blank, text goes left and numbers right, the cursor's row is
// marked, and the last text column gives way to the width.
func TestTableFormatsByKind(t *testing.T) {
	cols := []col{{"name", kText, 0}, {"n", kInt, 0}, {"total", kCash, 0}, {"fee", kMoney, 0}, {"price", kPrice, 0}, {"share", kPct, 0}, {"left", kDays, 0}, {"loyalty", kBar, 4}, {"order", kDial, 0}, {"note", kText, 0}}
	rows := [][]any{
		{"Vasquez", 12, 1_234_567, 25_000, 19.5, 0.8, 3, gauge{0.5, nil, 58}, order{40, "aggr.", false}, "runs Bayport"},
		{"Books", signed{-3}, nil, styled{lipgloss.NewStyle(), 150}, 2500.0, signed{15.0}, day(0), spark{[]float64{1, 2, 3}, "▲"}, order{240, "normal", true}, nil},
		{nil, approx{40}, -1500, nil, nil, 12.34, nil, nil, nil, "a very long note that is cut"},
	}
	lines := table(cols, rows, 1, 90)
	plain := make([]string, len(lines))
	for i, l := range lines {
		plain[i] = stripANSI(l)
	}
	want := []string{
		"  name       n    total      fee   price  share  left  loyalty  order            note     ",
		"  Vasquez   12    $1.2M  $25,000  $19.50   0.8%    3d  ██░░ 58  40 aggr.         runs Bay…",
		"▸ Books     -3        -     $150  $2,500   +15%    d0  ▁▄█ ▲    240 normal (lt)           ",
		"           ~40  -$1,500        -       -    12%     -  -        -                a very l…",
	}
	for i, w := range want {
		if i >= len(plain) || plain[i] != w {
			t.Errorf("line %d:\n got %q\nwant %q", i, plain[i], w)
		}
	}
	for _, l := range lines {
		if lipgloss.Width(l) > 90 {
			t.Errorf("line wider than 90: %q", stripANSI(l))
		}
	}
	checkTable(t, drawnCols(cols, rows), lines)
	// A value a number column cannot read is shown as one, and read as
	// one by the check.
	if got, _ := cellText(kMoney, 0, "$65"); got != "?$65" || kindPatterns[kMoney].MatchString(got) {
		t.Errorf("a string in a money column: %q", got)
	}
	// A mark in the gutter shows where the cursor is not.
	got := stripANSI(table([]col{{"node", kText, 0}, {"cost", kCash, 0}}, [][]any{{mark("✓"), "Stash spot", 5000}, {mark("○"), "Lookouts", 12_000}}, 1, 0)[1])
	if got != "✓ Stash spot  $5,000" {
		t.Errorf("marked row: %q", got)
	}
}

// drawnCols is cols with the widths table drew them at.
func drawnCols(cols []col, rows [][]any) []col {
	var drawn []col
	hook := tableHook
	tableHook = func(c []col, _ []string) { drawn = c }
	table(cols, rows, -1, 90)
	tableHook = hook
	return drawn
}

// TestTablesAreConsistent: every table the rich fixture renders at 80
// and 120 columns passes checkTable.
func TestTablesAreConsistent(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		n := 0
		tableHook = func(cols []col, lines []string) {
			n++
			checkTable(t, cols, lines)
		}
		richFixture(t, sz, func(string, string) {})
		tableHook = nil
		if n < 50 {
			t.Errorf("%dx%d: only %d tables rendered", sz[0], sz[1], n)
		}
	}
}

// TestUpgradeCostReadsTheSame: the tree and the inspector print a node's
// cost through cash() and so agree.
func TestUpgradeCostReadsTheSame(t *testing.T) {
	m := newTestModel(t, 120, 40)
	m.Update(key("6"))
	for range m.upgradeRows() {
		sel, _ := m.upgradeSelected()
		v := stripANSI(m.View())
		if strings.Count(v, cash(sel.Cost)) < 2 {
			t.Errorf("%s: the tree and the inspector do not both print %s:\n%s", sel.Name, cash(sel.Cost), v)
		}
		if strings.Contains(v, money(sel.Cost)) && money(sel.Cost) != cash(sel.Cost) {
			t.Errorf("%s: %s printed as %s somewhere", sel.Name, cash(sel.Cost), money(sel.Cost))
		}
		m.Update(key("j"))
	}
}

// TestReportNumbers: the report's MONEY and PRICES lines carry money
// with separators and changes with →, never $25000 or ->.
func TestReportNumbers(t *testing.T) {
	bare := regexp.MustCompile(`\$\d{5}`)
	seen := 0
	m := newTestModel(t, 120, 40)
	m.w.Player.DirtyCash = 700_000
	m.w.Stats.PeakCash = 700_000
	m.Update(key("7"))
	for i := 0; i < 3; i++ {
		m.Update(key("b"))
		m.Update(key("enter"))
	}
	m.Update(key("6"))
	m.Update(key("enter"))
	m.Update(key("y"))
	m.w.Stash(m.w.Player.Location)[m.w.Products[0]] = 200
	m.Update(key("1"))
	m.Update(key("s"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("3"))
	m.Update(key("enter"))
	endDay(t, m)
	r := m.w.Report
	for _, l := range append(append([]string(nil), r.Money...), r.Prices...) {
		seen++
		if bare.MatchString(l) || strings.Contains(l, "->") {
			t.Errorf("report line %q", l)
		}
	}
	if seen < 6 {
		t.Fatalf("only %d money and price lines: %+v", seen, r)
	}
	v := stripANSI(m.View())
	if bare.MatchString(v) || strings.Contains(v, "->") {
		t.Errorf("the report modal:\n%s", v)
	}
	if !strings.Contains(strings.Join(r.Money, "\n"), "Stash spot -$5,000") || !strings.Contains(strings.Join(r.Money, "\n"), "Bought Laundromat -$25,000") {
		t.Errorf("money lines: %v", r.Money)
	}
}
