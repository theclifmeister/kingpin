package engine

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The cues (#301, docs/engine.md): what a front end animates. A
// graphical client draws the view and moves its sprites on the day's
// events; CueOf reads an event as that movement, in ids and never in
// words: which corner changed hands and whose it was, which route a
// shipment took, where the police came, who on the crew left. An event
// with no cue is the report's and the journal's (a price move, a debt
// paid, a deal refused). Every kind in events.All is one or the other
// (TestEveryKindIsCuedOrNot), and the table in docs/engine.md is
// generated from cueKinds.

// CueKind is what a front end animates.
type CueKind string

// The cues.
const (
	CueCornerClaimed CueKind = "corner_claimed" // Corner is yours now
	CueCornerFlip    CueKind = "corner_flip"    // Corner changed hands: From -> To (owners), Faction the faction in it
	CueStrike        CueKind = "strike"         // muscle hit Corner and it held
	CueRivalMove     CueKind = "rival_move"     // Faction set up on, eyes, pushes or boosts at Corner
	CueRobbery       CueKind = "robbery"        // Corner or House robbed
	CuePolice        CueKind = "police"         // Level at City (House where the stock came from, Corner for a rival's)
	CueTaskForce     CueKind = "task_force"     // the task force in City: formed, took Asset, found the tunnel
	CueShipment      CueKind = "shipment"       // Shipment on Route From -> To: Phase sent, landed or seized
	CueCrewJoined    CueKind = "crew_joined"    // Member joined
	CueCrewLeft      CueKind = "crew_left"      // Member left: quit, fired, defected (Faction), retired, walked
	CueCrewDown      CueKind = "crew_down"      // Member arrested (City, Corner or Route) or shot (Dead) at Corner
	CueCrewBack      CueKind = "crew_back"      // Member back: bailed, released, recovered
	CueSale          CueKind = "sale"           // Units of Product sold in City
	CueMarket        CueKind = "market"         // Product's price in City shocked (Phase shock or slump)
	CueProperty      CueKind = "property"       // House or Corner's deed bought, lost or seized (Phase)
	CueOverdose      CueKind = "overdose"       // a customer at Corner in City
	CueRun           CueKind = "run"            // the run turned: the reign began or broke, or it ended (Phase)
)

// Cue is one thing to animate. The fields are the kind's, as the
// constants say; the rest are zero.
type Cue struct {
	Kind     CueKind `json:"kind"`
	Day      int     `json:"day"`
	City     string  `json:"city,omitempty"`
	Corner   string  `json:"corner,omitempty"`
	House    string  `json:"house,omitempty"`
	Route    string  `json:"route,omitempty"`
	Shipment int     `json:"shipment,omitempty"`
	Member   int     `json:"member,omitempty"`
	Faction  string  `json:"faction,omitempty"`
	Asset    string  `json:"asset,omitempty"`
	Product  string  `json:"product,omitempty"`
	Units    int     `json:"units,omitempty"`
	From     string  `json:"from,omitempty"` // corner_flip: the owner before; shipment: the city it left
	To       string  `json:"to,omitempty"`   // corner_flip: the owner after; shipment: the city it goes to
	Level    string  `json:"level,omitempty"`
	Phase    string  `json:"phase,omitempty"` // the kind's step: sent | landed | seized, bought | lost | seized, shock | slump, began | broke | ended, quit | fired | defected | retired | walked, bailed | released | recovered
	Dead     bool    `json:"dead,omitempty"`
}

// cueKinds is the cue each kind that animates gives (CueOf may pick the
// other flip of a strike that took its corner): the table
// docs/engine.md prints. A kind not here has no cue.
var cueKinds = map[string]CueKind{
	"CornerClaimed": CueCornerClaimed,
	"CornerTaken":   CueCornerFlip, "CornerLost": CueCornerFlip, "RivalAbandoned": CueCornerFlip, "RivalRaided": CueCornerFlip,
	"CornerStruck": CueStrike,
	"RivalMovedIn": CueRivalMove, "RivalEyeing": CueRivalMove, "RivalPushed": CueRivalMove, "RivalBoosted": CueRivalMove, "FactionPushed": CueRivalMove,
	"CornerRobbed": CueRobbery, "HouseRobbed": CueRobbery,
	"Enforcement": CuePolice, "HouseRaided": CuePolice,
	"TaskForceFormed": CueTaskForce, "AssetSeized": CueTaskForce, "TunnelFound": CueTaskForce,
	"ShipmentSent": CueShipment, "ShipmentArrived": CueShipment, "ShipmentSeized": CueShipment,
	"CrewHired": CueCrewJoined,
	"CrewQuit":  CueCrewLeft, "CrewFired": CueCrewLeft, "CrewDefected": CueCrewLeft, "CrewRetired": CueCrewLeft, "LieutenantWalked": CueCrewLeft,
	"CrewArrested": CueCrewDown, "CrewShot": CueCrewDown,
	"CrewBailed": CueCrewBack, "CrewReleased": CueCrewBack, "CrewRecovered": CueCrewBack,
	"PlayerSold":  CueSale,
	"PriceShock":  CueMarket,
	"HouseBought": CueProperty, "HouseLost": CueProperty, "DeedBought": CueProperty, "DeedSeized": CueProperty,
	"Overdose":   CueOverdose,
	"ReignBegan": CueRun, "ReignBroken": CueRun, "GameOver": CueRun,
}

// CueKindOf is the cue an event kind gives, or "" for one the report
// and the journal carry alone.
func CueKindOf(kind string) CueKind { return cueKinds[kind] }

// CueKinds is every cue, sorted: what a front end's animation table
// must cover (#328: the web client's test holds its table to it).
func CueKinds() []CueKind {
	return []CueKind{
		CueCornerClaimed, CueCornerFlip, CueCrewBack, CueCrewDown, CueCrewJoined, CueCrewLeft, CueMarket, CueOverdose,
		CuePolice, CueProperty, CueRivalMove, CueRobbery, CueRun, CueSale, CueShipment, CueStrike, CueTaskForce,
	}
}

// CueOf is the event as a front end animates it, or false for one with
// no cue.
func CueOf(e events.Event) (Cue, bool) {
	switch ev := e.(type) {
	case events.CornerClaimed:
		return Cue{Kind: CueCornerClaimed, Day: ev.Day, Corner: ev.Corner}, true
	case events.CornerTaken:
		from := ev.From
		if from == "" {
			from = game.OwnerNone
		}
		return Cue{Kind: CueCornerFlip, Day: ev.Day, Corner: ev.Corner, Faction: ev.Faction, From: from, To: game.OwnerRival}, true
	case events.CornerLost:
		from := ev.Owner
		if from == "" {
			from = game.OwnerPlayer
		}
		return Cue{Kind: CueCornerFlip, Day: ev.Day, Corner: ev.Corner, Faction: ev.Faction, From: from, To: game.OwnerNone, Level: ev.Reason}, true
	case events.RivalAbandoned:
		return Cue{Kind: CueCornerFlip, Day: ev.Day, Corner: ev.Corner, Faction: ev.Faction, From: game.OwnerRival, To: game.OwnerNone}, true
	case events.RivalRaided:
		return Cue{Kind: CueCornerFlip, Day: ev.Day, Corner: ev.Corner, Faction: ev.Faction, From: game.OwnerRival, To: game.OwnerNone, Level: content.Raid}, true
	case events.CornerStruck:
		if ev.Taken {
			return Cue{Kind: CueCornerFlip, Day: ev.Day, Corner: ev.Corner, Faction: ev.Faction, From: game.OwnerRival, To: game.OwnerPlayer}, true
		}
		return Cue{Kind: CueStrike, Day: ev.Day, Corner: ev.Corner, Faction: ev.Faction}, true
	case events.RivalMovedIn:
		return Cue{Kind: CueRivalMove, Day: ev.Day, Corner: ev.Corner, Faction: ev.Faction, Phase: "moved_in"}, true
	case events.RivalEyeing:
		return Cue{Kind: CueRivalMove, Day: ev.Day, Corner: ev.Corner, Faction: ev.Faction, Phase: "eyeing"}, true
	case events.RivalPushed:
		return Cue{Kind: CueRivalMove, Day: ev.Day, Corner: ev.Corner, Faction: ev.Faction, Phase: "pushed"}, true
	case events.RivalBoosted:
		return Cue{Kind: CueRivalMove, Day: ev.Day, Corner: ev.Corner, Faction: ev.Faction, Phase: "boosted"}, true
	case events.FactionPushed:
		return Cue{Kind: CueRivalMove, Day: ev.Day, City: ev.City, Corner: ev.Corner, Faction: ev.Faction, Phase: "pushed"}, true
	case events.CornerRobbed:
		return Cue{Kind: CueRobbery, Day: ev.Day, Corner: ev.Corner}, true
	case events.HouseRobbed:
		return Cue{Kind: CueRobbery, Day: ev.Day, City: ev.City, Corner: ev.Corner, House: ev.House}, true
	case events.Enforcement:
		return Cue{Kind: CuePolice, Day: ev.Day, City: ev.City, Level: ev.Level, House: ev.House}, true
	case events.HouseRaided:
		return Cue{Kind: CuePolice, Day: ev.Day, City: ev.City, House: ev.House, Level: ev.Level}, true
	case events.TaskForceFormed:
		return Cue{Kind: CueTaskForce, Day: ev.Day, City: ev.City, Phase: "formed"}, true
	case events.AssetSeized:
		return Cue{Kind: CueTaskForce, Day: ev.Day, City: ev.City, Asset: ev.Asset, Phase: "seized"}, true
	case events.TunnelFound:
		return Cue{Kind: CueTaskForce, Day: ev.Day, Route: ev.Route, Asset: ev.Asset, Product: ev.Product, Units: ev.Units, Phase: "found"}, true
	case events.ShipmentSent:
		return Cue{Kind: CueShipment, Day: ev.Day, Shipment: ev.ID, Route: ev.Route, From: ev.From, To: ev.To, Product: ev.Product, Units: ev.Units, Member: ev.Driver, Phase: "sent"}, true
	case events.ShipmentArrived:
		return Cue{Kind: CueShipment, Day: ev.Day, Shipment: ev.ID, Route: ev.Route, From: ev.From, To: ev.To, Product: ev.Product, Units: ev.Units, Phase: "landed"}, true
	case events.ShipmentSeized:
		return Cue{Kind: CueShipment, Day: ev.Day, Shipment: ev.ID, Route: ev.Route, From: ev.From, To: ev.To, Product: ev.Product, Units: ev.Units, Member: ev.Driver, Phase: "seized"}, true
	case events.CrewHired:
		return Cue{Kind: CueCrewJoined, Day: ev.Day, Member: ev.ID}, true
	case events.CrewQuit:
		return Cue{Kind: CueCrewLeft, Day: ev.Day, Member: ev.ID, Phase: "quit"}, true
	case events.CrewFired:
		return Cue{Kind: CueCrewLeft, Day: ev.Day, Member: ev.ID, Phase: "fired"}, true
	case events.CrewDefected:
		return Cue{Kind: CueCrewLeft, Day: ev.Day, Member: ev.ID, Faction: ev.Faction, Corner: ev.Corner, Phase: "defected"}, true
	case events.CrewRetired:
		return Cue{Kind: CueCrewLeft, Day: ev.Day, Member: ev.ID, Phase: "retired"}, true
	case events.LieutenantWalked:
		return Cue{Kind: CueCrewLeft, Day: ev.Day, Member: ev.ID, City: ev.City, Faction: ev.Faction, Phase: "walked"}, true
	case events.CrewArrested:
		return Cue{Kind: CueCrewDown, Day: ev.Day, Member: ev.ID, City: ev.City, Corner: ev.Corner, Route: ev.Route, Phase: "arrested"}, true
	case events.CrewShot:
		return Cue{Kind: CueCrewDown, Day: ev.Day, Member: ev.ID, City: ev.City, Corner: ev.Corner, Faction: ev.Faction, Dead: ev.Dead, Phase: "shot"}, true
	case events.CrewBailed:
		return Cue{Kind: CueCrewBack, Day: ev.Day, Member: ev.ID, Phase: "bailed"}, true
	case events.CrewReleased:
		return Cue{Kind: CueCrewBack, Day: ev.Day, Member: ev.ID, Phase: "released"}, true
	case events.CrewRecovered:
		return Cue{Kind: CueCrewBack, Day: ev.Day, Member: ev.ID, Phase: "recovered"}, true
	case events.PlayerSold:
		return Cue{Kind: CueSale, Day: ev.Day, City: ev.City, Product: ev.Product, Units: ev.Sold}, true
	case events.PriceShock:
		phase := "shock"
		if ev.Slump {
			phase = "slump"
		}
		return Cue{Kind: CueMarket, Day: ev.Day, City: ev.City, Product: ev.Product, Phase: phase}, true
	case events.HouseBought:
		return Cue{Kind: CueProperty, Day: ev.Day, City: ev.City, House: ev.House, Phase: "bought"}, true
	case events.HouseLost:
		return Cue{Kind: CueProperty, Day: ev.Day, City: ev.City, House: ev.House, Units: ev.Units, Phase: "lost"}, true
	case events.DeedBought:
		return Cue{Kind: CueProperty, Day: ev.Day, City: ev.City, Corner: ev.Corner, Phase: "bought"}, true
	case events.DeedSeized:
		return Cue{Kind: CueProperty, Day: ev.Day, City: ev.City, Corner: ev.Corner, Phase: "seized"}, true
	case events.Overdose:
		return Cue{Kind: CueOverdose, Day: ev.Day, City: ev.City, Corner: ev.Corner, Product: ev.Product}, true
	case events.ReignBegan:
		return Cue{Kind: CueRun, Day: ev.Day, City: ev.City, Phase: "began"}, true
	case events.ReignBroken:
		return Cue{Kind: CueRun, Day: ev.Day, Phase: "broke"}, true
	case events.GameOver:
		return Cue{Kind: CueRun, Day: ev.Day, Level: ev.Cause, Phase: "ended"}, true
	}
	return Cue{}, false
}
