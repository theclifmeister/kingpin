package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/format"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The copy of #537: words that say what the game does.

// Cash out says where the money goes, with the offshore key beside it
// in the ledger's KEYS, and its dialog says it goes nowhere offshore.
func TestCashOutSaysWhereTheMoneyGoes(t *testing.T) {
	m := richModel(t, 120, 40)
	keys := m.keysFor(screenLedger)
	at := -1
	for i, b := range keys {
		if b.key == "c" {
			at = i
			if b.label != "clean to dirty" {
				t.Errorf("c on the ledger reads %q", b.label)
			}
		}
	}
	if at < 1 || keys[at-1].key != "o" || !strings.Contains(keys[at-1].help, "offshore") {
		t.Fatalf("c is not beside the offshore key in the ledger's KEYS: %d in %v", at, keys)
	}
	m.w.Player.CleanCash = 200_000
	m.Update(key("7"))
	m.Update(key("c"))
	if v := stripANSI(m.View()); m.mode != modeCashOut || !strings.Contains(v, "CLEAN TO DIRTY") || !strings.Contains(v, "Nothing here goes offshore") {
		t.Fatalf("the cash-out dialog:\n%s", v)
	}
}

// A tribute says who pays whom (#537): the rivals table's terms, the
// report's words through game.Deal.String (TestAdviceFitsTheSetting has
// the offer's demand).
func TestTributeSaysWhoPays(t *testing.T) {
	m := richModel(t, 120, 40)
	if got := m.dealTerms(game.Deal{Kind: game.DealTribute, Terms: game.Terms{PerDay: 3000}}); got != "$3,000 a day from you" {
		t.Errorf("a tribute's terms: %q", got)
	}
	if got := m.dealTerms(game.Deal{Kind: game.DealHomage, Terms: game.Terms{PerDay: 3000}}); got != "$3,000 a day to you" {
		t.Errorf("a homage's terms: %q", got)
	}
	if got := (game.Deal{Kind: game.DealTribute, Terms: game.Terms{PerDay: 3000}}).String(); got != "tribute: you pay them $3,000 a day" {
		t.Errorf("a tribute in words: %q", got)
	}
}

// The crown's confirmation says what the reign pays a night (#537: it
// read `$0 a night`, the homage alone, beside a tax paying $30K): the
// homage and the tax off the free corners, with the total.
func TestCrownSaysTheReignsIncome(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	home := w.Home()
	for _, r := range w.Rivals {
		if r == nil {
			continue
		}
		r.Arrived = max(r.Arrived, 1)
		r.Deals = append(r.Deals, game.Deal{Kind: game.DealHomage, Terms: game.Terms{PerDay: 1_000}, Since: w.Day - 20, Until: w.Day + 100, Faction: r.Faction()})
	}
	for i := range home.Corners {
		c := &home.Corners[i]
		c.Owner, c.Faction = game.OwnerNone, ""
		if i < len(home.Corners)*3/4 {
			c.Owner = game.OwnerPlayer
		}
	}
	crews, homage := w.HomageDeals()
	corners, tax := 0, 0
	for _, cid := range w.CityOrder {
		n, a := m.rules.Territory.TaxDue(w, cid)
		corners += n
		tax += a
	}
	if crews == 0 || tax == 0 {
		t.Fatalf("fixture: %d crews paying $%d, %d corners taxed $%d", crews, homage, corners, tax)
	}
	got := m.reignIncome()
	for _, want := range []string{"~" + format.Money(homage+tax) + " a night", format.Money(homage) + " homage from " + plural(crews, "crew"), "~" + format.Money(tax) + " tax off " + plural(corners, "free corner")} {
		if !strings.Contains(got, want) {
			t.Errorf("the reign's income %q lacks %q", got, want)
		}
	}
}

// The stage's NEXT line lists only what is not open yet (#537: STAGE 2
// on day 31 said `Move $25K: the laundromat opens` of one open since
// day 26): a next stage whose line is crossed opens in the morning.
func TestStageNextIsWhatIsNotOpen(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	crew := m.cfg.Progression.Tier(2)
	territory := m.cfg.Progression.Tier(3)
	if crew == nil || territory == nil {
		t.Fatal("no crew or territory tier")
	}
	w.Stats.PeakCash = 0
	if got := m.stageNext(2); got != crew.Next {
		t.Errorf("under the line: %q, want the file's %q", got, crew.Next)
	}
	w.Stats.PeakCash = territory.Enter.PeakCashMin + 1
	if got := m.stageNext(2); got == crew.Next || !strings.Contains(got, territory.Name) {
		t.Errorf("over the line: %q", got)
	}
	for _, l := range m.stageLines(2) {
		if strings.Contains(stripANSI(l), "the rival, from day 10") {
			t.Errorf("the crew stage lists the rival as opened: %q", l)
		}
	}
}

// The checkpoint's confirmation shows the rate the file knows (#537:
// `seized ?` beside a rate on the intel screen), and says when it knows
// none.
func TestCheckpointShowsTheKnownRate(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	m.Update(key("5"))
	m.onRoutes, m.routeCursor = true, 0
	r := m.selectedRoute()
	if r == nil {
		t.Fatal("no route")
	}
	w.Intel = nil
	if body := stripANSI(strings.Join(m.checkpointConfirm(), "\n")); !strings.Contains(body, "not known") || strings.Contains(body, "?") {
		t.Errorf("with nothing known:\n%s", body)
	}
	w.Learn(game.Fact{Subject: r.ID, Kind: game.FactRisk, Value: "~2%/day", Number: 0.02, Confidence: 1, Day: w.Day, Source: game.SourceSeen})
	body := stripANSI(strings.Join(m.checkpointConfirm(), "\n"))
	if !strings.Contains(body, "~2% a day known") || strings.Contains(body, "seized now  ?") {
		t.Errorf("with the rate known:\n%s", body)
	}
}

// The typos and forms of #537: a page that goes, a run's first day on
// the payroll, a count with separators, the contract's stop in words.
func TestCopyFormsAgree(t *testing.T) {
	m := richModel(t, 120, 40)
	w := m.w
	w.Laundering.Structured = game.Structuring{Day: w.Day, Amount: 1_000_000, Lots: 1}
	if pages := m.sess.PagesDue(); pages != 1 {
		t.Fatalf("fixture: %d pages due", pages)
	}
	for _, r := range m.exitRows() {
		if strings.Contains(r.short, "transfer") {
			if !strings.Contains(r.short, "1 page from last night's transfer goes") {
				t.Errorf("one page: %q", r.short)
			}
		}
	}
	if err := m.sess.Retire(); err == nil || !strings.Contains(err.Error(), "1 page goes") {
		t.Errorf("the session's refusal: %v", err)
	}
	w.Laundering.Structured = game.Structuring{}
	w.Crew.Fallen = nil
	w.Crew.Members = []game.CrewMember{{ID: 1, Name: "Dre", Role: game.RoleRunner, Hired: w.Day - 1, Loyalty: 80}}
	if got := m.bestCrew(); !strings.Contains(got, "1 day on the payroll") {
		t.Errorf("a member hired yesterday: %q", got)
	}
	w.Stats.TotalRevenue, w.Stats.UnitsSold = 3_600_000, 16_007
	w.Over = w.End(content.CauseRetired, w.Day, "")
	m.mode = modeOver
	if view := stripANSI(strings.Join(m.summaryLines(), "\n")); !strings.Contains(view, "off 16,007 units") {
		t.Errorf("the summary's units:\n%s", view)
	}
	w.Over = nil
	for why, want := range map[string]string{events.SupplyRoad: "waiting on what is on the road", "room": "short of room", "cash": "short of cash"} {
		if got := m.stopEvent(events.SupplyShort{City: w.Player.Location, Product: w.Products[0], Why: why}); !strings.Contains(got, want) || strings.Contains(got, "short of road") {
			t.Errorf("a contract short for %q stops on %q", why, got)
		}
	}
	if got := fmt.Sprint(format.Plural(16_007, "unit")); got != "16,007 units" {
		t.Errorf("a count: %q", got)
	}
}
