package engine_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// freshSession is a new run on seed 7, day 0.
func freshSession(t *testing.T) (*engine.Session, *game.World) {
	t.Helper()
	s, err := engine.New(content.MustLoad())
	if err != nil {
		t.Fatal(err)
	}
	return s, s.NewRun(7, game.Start{})
}

// TestViewNeverMarksTheUnnamedInformant (#550): an informant on the
// payroll changes nothing in the view, byte for byte, until an
// investigation names them (Crew.Exposed); then, and only then, the
// member is marked exposed.
func TestViewNeverMarksTheUnnamedInformant(t *testing.T) {
	t.Parallel()
	s, w := freshSession(t)
	w.Player.DirtyCash += 1_000_000
	pool := s.View().Pool
	if len(pool) == 0 {
		t.Fatal("nobody to hire on day 0")
	}
	hired, err := s.Hire(pool[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	member := func() *game.CrewMember {
		for i := range w.Crew.Members {
			if w.Crew.Members[i].ID == hired.ID {
				return &w.Crew.Members[i]
			}
		}
		t.Fatal("the hire is not on the payroll")
		return nil
	}
	clean, err := json.Marshal(s.View())
	if err != nil {
		t.Fatal(err)
	}
	member().Informant = true
	talking, err := json.Marshal(s.View())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(clean, talking) {
		t.Fatal("an unnamed informant changes the view")
	}
	w.Crew.Exposed = hired.ID
	for _, m := range s.View().Crew {
		if m.Exposed != (m.ID == hired.ID) {
			t.Errorf("%s (%d): exposed %v with %d named", m.Name, m.ID, m.Exposed, hired.ID)
		}
	}
}

// TestViewCarriesTheStateTheScreensRead (#550): each field view 15
// added reads the world's state, and is absent while that state is.
func TestViewCarriesTheStateTheScreensRead(t *testing.T) {
	t.Parallel()
	s, w := freshSession(t)
	cfg := s.Config()
	home := w.CityOrder[0]
	day := w.Day

	// Absent on a fresh run.
	v := s.View()
	if v.Stage != nil || v.Proposal != nil || v.You.War != "" || v.You.Score != 0 || v.Law.SellCap != 0 || len(v.Orders) != 0 || len(v.Standing) != 0 || len(v.Supply) != 0 || len(v.Assets) != 0 || len(v.Cooks) != 0 {
		t.Fatalf("day 0: %+v %+v %+v", v.Stage, v.Proposal, v.You)
	}

	// The crew.
	if len(v.Routes) == 0 {
		t.Fatal("no route open on day 0")
	}
	route := v.Routes[0].ID
	w.Crew.Members = append(w.Crew.Members,
		game.CrewMember{ID: 901, Name: "Wounded", Role: game.RoleRunner, WoundedUntil: day + 3},
		game.CrewMember{ID: 902, Name: "Jailed", Role: game.RoleRunner, JailedUntil: day + 2, Bailed: true},
		game.CrewMember{ID: 903, Name: "Lieutenant", Role: game.RoleLieutenant, City: home, Assigned: day},
		game.CrewMember{ID: 904, Name: "Driver", Role: game.RoleDriver},
		game.CrewMember{ID: 905, Name: "Chemist", Role: game.RoleChemist, Skill: 90},
		game.CrewMember{ID: 906, Name: "Second", Role: game.RoleChemist, Skill: 10},
	)
	{
		rs := w.Route(route)
		rs.Driver = 904
		rs.Target = map[string]int{w.Products[0]: 40}
		rs.Days = map[string]int{w.Products[0]: 3}
		rs.Bought = day + 5
		if w.Routes == nil {
			w.Routes = map[string]game.RouteSetting{}
		}
		w.Routes[route] = rs
	}
	w.Crew.Cooks = append(w.Crew.Cooks, game.Cook{ID: 1, City: home, Product: w.Products[0], Units: 30, Quality: 70, Ordered: day, Ready: day + 2, Chemist: "Chemist"})

	// The law.
	w.Law.Favours, w.Law.CampaignOpen, w.Law.Leads = 2, true, 1
	w.Law.ChiefBought, w.Law.DABought = day+4, day+6
	w.Cities[home].Campaign = game.Campaign{Ticket: "reform", Cash: 5000, Hedged: true}
	w.Heat.SellCap, w.Heat.SellCapDays, w.Heat.SellCapCity = 0.5, 3, home

	// The money.
	w.Fronts = append(w.Fronts, game.Front{ID: "laundromat", Name: "Laundromat", Bought: day, FrozenUntil: day + 7, Unpaid: 250, Audited: day})
	w.Assets = append(w.Assets, game.Asset{ID: "plane", Name: "A plane", Effect: "route", City: home, Cost: 1000, Upkeep: 10, Bought: day, FrozenUntil: day + 2})
	w.Laundering.Till = 60_000
	w.Laundering.Sweep = game.OffshoreSweep{On: true, Keep: 20_000}
	w.Offshore, w.Stats.Bodies = 300_000, 2

	// The orders.
	p := w.Products[0]
	w.Today.Orders = map[string]game.SellOrder{game.OrderKey(home, p): {City: home, Product: p, Qty: 12, Dial: events.DialQuiet}}
	w.Standing = map[string]game.SellOrder{game.OrderKey(home, p): {City: home, Product: p, Qty: 30, Dial: events.DialAggressive, All: true}}
	w.Supply = map[string]game.SupplyContract{game.OrderKey(home, p): {City: home, Product: p, Units: 50, Since: day}}
	w.DelegatedSupply = map[string]game.SupplyContract{game.OrderKey(home, p): {City: home, Product: p, Units: 20, Since: day}}

	// The table.
	r := w.Rival()
	w.War = r.Faction()
	r.Heat, r.LastRaid = 45, day
	r.Deals = append(r.Deals, game.Deal{Kind: game.DealTruce, Terms: game.Terms{Days: 10}, Since: day, Until: day + 10, Offered: true, Faction: r.Faction()})
	w.Today.Proposal = &game.Deal{Kind: game.DealTribute, Terms: game.Terms{PerDay: 100}, Faction: r.Faction()}

	// The stage.
	w.Reach(2, day)

	v = s.View()
	crew := map[int]engine.MemberView{}
	for _, m := range v.Crew {
		crew[m.ID] = m
	}
	if m := crew[901]; m.Wounded != 3 {
		t.Errorf("wounded: %+v", m)
	}
	if m := crew[902]; !m.Jailed || m.JailedUntil != day+2 || !m.Bailed {
		t.Errorf("jailed: %+v", m)
	}
	if m := crew[903]; m.Assigned != day || m.City != home {
		t.Errorf("lieutenant: %+v", m)
	}
	if m := crew[904]; m.Route != route {
		t.Errorf("driver: %+v, want %q", m, route)
	}
	if !crew[905].Lab || crew[906].Lab {
		t.Errorf("chemists: %+v, %+v", crew[905], crew[906])
	}
	if len(v.Cooks) != 1 || v.Cooks[0].Units != 30 || v.Cooks[0].Ready != day+2 {
		t.Errorf("cooks: %+v", v.Cooks)
	}
	{
		for _, rv := range v.Routes {
			if rv.ID == route && (rv.Target[p] != 40 || rv.DaysTarget[p] != 3 || rv.CheckpointUntil != day+5) {
				t.Errorf("route: %+v", rv)
			}
		}
	}

	l := v.Law
	if l.Favours != 2 || !l.CampaignOpen || l.Leads != 1 || l.ChiefBought != day+4 || l.DABought != day+6 || l.ChiefTermEnds != s.Rules().Law.ChiefTermEnds(w) {
		t.Errorf("the law: %+v", l)
	}
	if l.SellCap != 0.5 || l.SellCapDays != 3 || l.SellCapCity != home {
		t.Errorf("the patrol cap: %+v", l)
	}
	if c := v.Cities[0].Campaign; c == nil || *c != (engine.CampaignView{Ticket: "reform", Cash: 5000, Hedged: true}) {
		t.Errorf("the campaign: %+v", c)
	}

	if len(v.Fronts) != 1 {
		t.Fatalf("fronts: %+v", v.Fronts)
	}
	if f := v.Fronts[0]; !f.Frozen || f.FrozenUntil != day+7 || f.Unpaid != 250 || f.Audited != day || f.Bought != day || f.City != home {
		t.Errorf("the front: %+v", f)
	}
	if len(v.Assets) != 1 || v.Assets[0].FrozenUntil != day+2 || v.Assets[0].Upkeep != 10 {
		t.Errorf("assets: %+v", v.Assets)
	}
	y := v.You
	if y.Till != 60_000 || !y.SweepOn || y.SweepKeep != 20_000 || y.War != r.Faction() || y.Score != 100_000 || y.Bodies != 2 {
		t.Errorf("you: %+v", y)
	}

	if len(v.Orders) != 1 || v.Orders[0] != (engine.OrderView{City: home, Product: p, Qty: 12, Dial: "quiet"}) {
		t.Errorf("orders: %+v", v.Orders)
	}
	if len(v.Standing) != 1 || v.Standing[0] != (engine.OrderView{City: home, Product: p, Qty: 30, Dial: "aggressive", All: true}) {
		t.Errorf("standing: %+v", v.Standing)
	}
	if len(v.Supply) != 2 || v.Supply[0].Units != 50 || v.Supply[0].Lieutenant != 0 || v.Supply[1].Units != 20 || v.Supply[1].Lieutenant != 903 {
		t.Errorf("supply: %+v", v.Supply)
	}

	var f *engine.FactionView
	for i := range v.Factions {
		if v.Factions[i].ID == r.Faction() {
			f = &v.Factions[i]
		}
	}
	if f == nil {
		t.Fatal("the rival is not in the view")
	}
	if f.Police != 45 || f.LastRaid != day || len(f.DealTerms) != 1 {
		t.Fatalf("the faction: %+v", f)
	}
	if d := f.DealTerms[0]; d.Kind != game.DealTruce || d.Terms.Days != 10 || d.Until != day+10 || d.Left != 9 || !d.Theirs {
		t.Errorf("the deal: %+v", d)
	}
	if pr := v.Proposal; pr == nil || pr.Faction != r.Faction() || pr.Kind != game.DealTribute || pr.Terms.PerDay != 100 {
		t.Errorf("the proposal: %+v", pr)
	}

	tier := cfg.Progression.Tier(2)
	if st := v.Stage; st == nil || st.Pending != 2 || st.Name != tier.Name || len(st.Opened) != len(tier.Opens) || st.Next != s.StageNext(2) || st.Next == "" {
		t.Errorf("the stage: %+v", v.Stage)
	}
	s.SeeStage(2)
	if s.View().Stage != nil {
		t.Error("the stage is still pending once seen")
	}
}
