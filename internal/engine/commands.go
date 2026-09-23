package engine

import (
	"errors"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The commands (#297, docs/engine.md): every action a player takes, one
// method each, the way a front end acts on the run. A command is the
// World method it wraps, never a second copy of its rules; where the
// World method takes a number the sims own (a price, a cap, a cost, the
// chemist's name, the tree), the command reads it off the sims itself
// and takes only what the player chose, so a front end can neither get
// it wrong nor name a price of its own. A command needs a run: called
// before NewRun, Load or Attach it panics, as a World method on a nil
// world would.
//
// The TUI acts through these and calls no World action itself
// (TestUIActsThroughTheSession).

// ErrNoOffer is what a buy by id returns when nothing is on offer under
// that id.
var ErrNoOffer = errors.New("nothing on offer by that name")

// Travel goes to the city (World.Travel).
func (s *Session) Travel(city string) error { return s.w.Travel(city) }

// SetLieLow lies low today, or stops (World.SetLieLow).
func (s *Session) SetLieLow(on bool) { s.w.SetLieLow(on) }

// SeeStage marks the stage the morning showed as seen (World.SeeStage).
func (s *Session) SeeStage(n int) { s.w.SeeStage(n) }

// Choose answers the dilemma card on the table (World.Choose).
func (s *Session) Choose(i int) (game.Answer, error) { return s.w.Choose(i) }

// ---- the market: buying, selling, the routines

// Buy buys qty of a product from a supplier, on credit or not, at the
// pressure the market reads off the tree (World.Buy).
func (s *Session) Buy(supplier, product string, qty int, credit bool) (game.Purchase, error) {
	return s.w.Buy(supplier, product, qty, credit, s.set.Market.BuyPressure(s.w))
}

// Return gives back today's cash buys (World.Return).
func (s *Session) Return(city, product string, qty int) (int, error) {
	return s.w.Return(city, product, qty)
}

// ReturnCredit gives back today's buys on credit (World.ReturnCredit).
func (s *Session) ReturnCredit(city, product string, qty int) (int, error) {
	return s.w.ReturnCredit(city, product, qty)
}

// ReturnSupplied gives back this morning's supply-contract receipts
// (World.ReturnSupplied).
func (s *Session) ReturnSupplied(city, product string, qty int) (int, error) {
	return s.w.ReturnSupplied(city, product, qty)
}

// PlaceSell queues tonight's sell order (World.PlaceSell).
func (s *Session) PlaceSell(city, product string, qty int, dial events.Dial) error {
	return s.w.PlaceSell(city, product, qty, dial)
}

// CancelSell drops tonight's sell order (World.CancelSell).
func (s *Session) CancelSell(city, product string) { s.w.CancelSell(city, product) }

// PlaceStanding sets a standing order (World.PlaceStanding).
func (s *Session) PlaceStanding(city, product string, qty int, dial events.Dial) error {
	return s.w.PlaceStanding(city, product, qty, dial)
}

// CancelStanding ends a standing order (World.CancelStanding).
func (s *Session) CancelStanding(city, product string) { s.w.CancelStanding(city, product) }

// SetSupply sets a supply contract (World.SetSupply).
func (s *Session) SetSupply(city, product string, units int) error {
	return s.w.SetSupply(city, product, units)
}

// ClearSupply ends a supply contract (World.ClearSupply).
func (s *Session) ClearSupply(city, product string) { s.w.ClearSupply(city, product) }

// AcceptContract takes a buyer's contract (World.AcceptContract).
func (s *Session) AcceptContract(id int) error { return s.w.AcceptContract(id) }

// DeclineContract turns a buyer's contract down (World.DeclineContract).
func (s *Session) DeclineContract(id int) error { return s.w.DeclineContract(id) }

// Deliver sends units against a contract tonight (World.Deliver).
func (s *Session) Deliver(id, units int) error { return s.w.Deliver(id, units) }

// Cut cuts a product where it is held by ratio, at the market's most and
// cost and the chemist's bonus (World.Cut).
func (s *Session) Cut(city, product string, ratio float64) (game.CutRecord, error) {
	mk, cr := s.set.Market, s.set.Crew
	return s.w.Cut(city, product, ratio, mk.CutMax(product), mk.CutCost(product), cr.CutBonus(s.w), cr.ChemistName(s.w))
}

// Cook puts a cook order in for units of a product in the city, at the
// cost, the days, the quality and the batch the chemist and the lab
// there give (World.CookOrder).
func (s *Session) Cook(city, product string, units int) (game.Cook, error) {
	mk, cr := s.set.Market, s.set.Crew
	return s.w.CookOrder(city, product, units, cr.CookCostIn(s.w, city, mk.CookCost(product)), cr.CookDays(), cr.QualityIn(s.w, city), cr.BatchIn(s.w, city), cr.ChemistName(s.w))
}

// ---- the street: corners, houses, stock

// Post puts a member (or you, game.You) on a corner (World.Post).
func (s *Session) Post(corner string, id int) error { return s.w.Post(corner, id) }

// Abandon gives up a corner (World.Abandon).
func (s *Session) Abandon(corner string) error { return s.w.Abandon(corner) }

// SendEnforcers sends the muscle at a corner (World.SendEnforcers).
func (s *Session) SendEnforcers(corner string, force events.Force) error {
	return s.w.SendEnforcers(corner, force)
}

// Boost puts muscle behind a push on a corner (World.Boost).
func (s *Session) Boost(corner string, force events.Force) error { return s.w.Boost(corner, force) }

// Undercut undercuts a rival corner at the dial (World.Undercut).
func (s *Session) Undercut(corner string, dial events.Dial) error { return s.w.Undercut(corner, dial) }

// CancelUndercut stops an undercut (World.CancelUndercut).
func (s *Session) CancelUndercut(corner string) { s.w.CancelUndercut(corner) }

// Tip tips the police off about a rival corner (World.Tip).
func (s *Session) Tip(corner string) error { return s.w.Tip(corner) }

// BuyDeed buys the block under a corner at the territory's price for it
// (World.BuyDeed).
func (s *Session) BuyDeed(corner string) error {
	price := 0
	if c := s.w.Corner(corner); c != nil {
		price = s.set.Territory.DeedPrice(s.w, *c)
	}
	return s.w.BuyDeed(corner, price)
}

// HouseOffers is every house the city file lets, as offers.
func (s *Session) HouseOffers() []game.HouseOffer {
	var offers []game.HouseOffer
	for _, o := range s.cfg.Houses.Offers {
		offers = append(offers, game.HouseOffer{ID: o.ID, Name: o.Name, City: o.City, Corner: o.Corner, Capacity: o.Capacity, Price: o.Price, Rent: o.Rent, UnlockCash: o.UnlockCash})
	}
	return offers
}

// BuyHouse takes the lease on the house offered under id
// (World.BuyHouse).
func (s *Session) BuyHouse(id string) (game.House, error) {
	for _, o := range s.HouseOffers() {
		if o.ID == id {
			return s.w.BuyHouse(o)
		}
	}
	return game.House{}, ErrNoOffer
}

// Drop gives up a house (World.Drop).
func (s *Session) Drop(id string) (game.House, error) { return s.w.Drop(id) }

// Guard puts an enforcer on a house (World.Guard).
func (s *Session) Guard(house string, id int) error { return s.w.Guard(house, id) }

// Move moves stock between the street and the houses in a city
// (World.Move).
func (s *Session) Move(city, from, to, product string, units int) (int, error) {
	return s.w.Move(city, from, to, product, units)
}

// ---- the crew

// Hire hires a candidate, up to the crew's cap (World.Hire).
func (s *Session) Hire(id int) (game.CrewMember, error) {
	return s.w.Hire(id, s.set.Crew.MaxCrew(s.w))
}

// Fire lets a member go (World.Fire).
func (s *Session) Fire(id int) (game.CrewMember, error) { return s.w.Fire(id) }

// SetPay sets the pay dial (World.SetPay).
func (s *Session) SetPay(p events.Pay) error { return s.w.SetPay(p) }

// Investigate looks for who is talking, at the crew's price
// (World.Investigate).
func (s *Session) Investigate() error {
	return s.w.Investigate(s.set.Crew.InvestigateCost())
}

// PayOff pays a member to keep quiet at the crew's price and loyalty for
// them (World.PayOff).
func (s *Session) PayOff(id int) (game.CrewMember, error) {
	cost := 0
	if c := s.w.Crew.Member(id); c != nil {
		cost = s.set.Crew.PayoffCost(*c)
	}
	return s.w.PayOff(id, cost, s.set.Crew.PayoffLoyalty())
}

// Bail bails a member out at the crew's price for them (World.Bail).
func (s *Session) Bail(id int) (game.CrewMember, error) {
	cost := 0
	if c := s.w.Crew.Member(id); c != nil {
		cost = s.set.Crew.BailCost(*c)
	}
	return s.w.Bail(id, cost)
}

// Assign sends a lieutenant to run a city (World.Assign).
func (s *Session) Assign(id int, city string) error { return s.w.Assign(id, city) }

// Unassign brings a lieutenant home (World.Unassign).
func (s *Session) Unassign(id int) error { return s.w.Unassign(id) }

// ---- the road

// SetRoute turns a route's dial (World.SetRoute).
func (s *Session) SetRoute(id string, d events.RouteDial) error { return s.w.SetRoute(id, d) }

// SetRouteTarget sets a route's target in units (World.SetRouteTarget).
func (s *Session) SetRouteTarget(id, product string, units int) error {
	return s.w.SetRouteTarget(id, product, units)
}

// SetRouteDays sets a route's target in days of stock
// (World.SetRouteDays).
func (s *Session) SetRouteDays(id, product string, days int) error {
	return s.w.SetRouteDays(id, product, days)
}

// SetRouteDriver seats a driver on a route (World.SetRouteDriver).
func (s *Session) SetRouteDriver(route string, id int) error { return s.w.SetRouteDriver(route, id) }

// BuyCheckpoint buys the deal on a route, a customs agent on a boat or
// plane edge and a checkpoint on the road, at the law's price and term
// (World.BuyCheckpoint).
func (s *Session) BuyCheckpoint(route string) error {
	tun := s.set.Law.Bribes()
	price := tun.CheckpointPrice
	if r := s.set.Logistics.Route(route); r != nil && s.set.Logistics.Customs(*r) {
		price = tun.CustomsPrice
	}
	return s.w.BuyCheckpoint(route, price, tun.CheckpointDays)
}

// ---- the money: fronts, assets, the account, the upgrades

// SetLaunderDial turns the launder dial (World.SetLaunderDial).
func (s *Session) SetLaunderDial(d events.Launder) error { return s.w.SetLaunderDial(d) }

// BuyFront buys the front offered under id (World.BuyFront).
func (s *Session) BuyFront(id string) (game.Front, error) {
	for _, o := range s.set.Laundering.Offers() {
		if o.ID == id {
			return s.w.BuyFront(o)
		}
	}
	return game.Front{}, ErrNoOffer
}

// Invest buys levels on a front at the laundering's price for them
// (World.Invest).
func (s *Session) Invest(front string, levels int) error {
	f := s.w.Front(front)
	if f == nil {
		return ErrNoOffer
	}
	return s.w.Invest(s.set.Laundering.Levels(*f, levels))
}

// BuyAsset buys the asset offered under id (World.BuyAsset).
func (s *Session) BuyAsset(id string) (game.Asset, error) {
	o, ok := s.set.Laundering.AssetOffer(id)
	if !ok {
		return game.Asset{}, ErrNoOffer
	}
	return s.w.BuyAsset(o)
}

// Reserve moves clean cash into the offshore account (World.Reserve).
func (s *Session) Reserve(amount int) error { return s.w.Reserve(amount) }

// BuyUpgrade buys a node of the tree (World.BuyUpgrade).
func (s *Session) BuyUpgrade(id string) (content.UpgradeConfig, error) {
	return s.w.BuyUpgrade(s.cfg.Upgrades, id)
}

// ---- the law

// Bribe puts an envelope in front of the chief or the DA (World.Bribe).
func (s *Session) Bribe(target string, amount int) error { return s.w.Bribe(target, amount) }

// Fund gives a city clean cash for goodwill (World.Fund).
func (s *Session) Fund(city string, amount int) error { return s.w.Fund(city, amount) }

// Back puts clean cash behind a ticket in a city's campaign (World.Back).
func (s *Session) Back(city, ticket string, amount int) error { return s.w.Back(city, ticket, amount) }

// CallFavour calls in a favour with the police, due when heat's response
// is on its way (World.CallFavour).
func (s *Session) CallFavour() error { return s.w.CallFavour(s.set.Heat.Due(s.w) != "") }

// PayCop pays a cop for a word on the police (World.PayCop).
func (s *Session) PayCop(amount int) error { return s.w.PayCop(amount) }

// ---- the table: factions, diplomacy, intel

// ScoutFaction reads a faction's books at the rivals' price
// (World.ScoutFaction).
func (s *Session) ScoutFaction(faction string) error {
	return s.w.ScoutFaction(faction, s.set.Rivals.ScoutCost())
}

// PlantSpy sends a member under with a faction (World.PlantSpy).
func (s *Session) PlantSpy(faction string, id int) error { return s.w.PlantSpy(faction, id) }

// BuyOffFrom pays a faction's muscle to go home, units heads at its
// price a head (World.BuyOffFrom).
func (s *Session) BuyOffFrom(faction string, units int) error {
	price := 0
	if r := s.w.Faction(faction); r != nil {
		price = s.set.Rivals.MusclePrice(s.w, r)
	}
	return s.w.BuyOffFrom(faction, units, units*price)
}

// ProposeTo puts a deal to a faction (World.ProposeTo).
func (s *Session) ProposeTo(faction, kind string, terms game.Terms) error {
	return s.w.ProposeTo(faction, kind, terms)
}

// Accept takes a faction's offer (World.Accept).
func (s *Session) Accept(id int) (game.Offer, error) { return s.w.Accept(id) }

// Decline turns a faction's offer down (World.Decline).
func (s *Session) Decline(id int) (game.Offer, error) { return s.w.Decline(id) }

// CallOff ends the deal on the table (World.CallOff).
func (s *Session) CallOff() { s.w.CallOff() }

// DeclareWar declares war on a faction (World.DeclareWar).
func (s *Session) DeclareWar(faction string) error { return s.w.DeclareWar(faction) }

// CallOffWar ends the war (World.CallOffWar).
func (s *Session) CallOffWar() error { return s.w.CallOffWar() }

// HitScouts sends the enforcers after a faction's scouts tonight
// (World.HitScouts, #341).
func (s *Session) HitScouts(faction string) error { return s.w.HitScouts(faction) }

// Withdraw pulls the muscle back (World.Withdraw).
func (s *Session) Withdraw() { s.w.Withdraw() }

// ---- the endings

// Retire walks away on the account at the laundering's terms
// (World.Retire).
func (s *Session) Retire() error { return s.set.Laundering.Retire(s.w) }

// Vanish walks away on a new identity, at what the tree owned does for
// it (World.Vanish).
func (s *Session) Vanish() error { return s.w.Vanish(game.FoldEffects(s.w, s.cfg.Upgrades)) }

// Crown takes the city (World.Crown).
func (s *Session) Crown() error { return s.w.Crown() }
