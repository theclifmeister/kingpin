package engine

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The stop rule (#541, docs/engine.md): what a fast-forward stops for,
// written once so a fix is checked against it and not against the last
// playtest. F stops only for
//
//   - a danger: something that can end the run within a night or two;
//   - an ending: one of the endings' doors opening, closing or at risk;
//   - an answer: something that needs a move before the next night runs
//     (a card, an offer, a deadline, a trouble the night repeats);
//   - a turn: something you held gone or hit, or the street, the law or
//     the table changing hands, said once.
//
// Everything else is a notice: the report or the dashboard says it and
// it never stops. An alert stops once by its key, an event the morning
// it happens (bar Stop's serves and news); when several land on one
// morning a danger shows first. Every alert kind and every event kind
// has a class here, and TestEveryKindIsClassed fails on one that has
// none, the way events.All works: a new kind is classed on purpose.

// Class is where a kind stands against the stop rule.
type Class string

// The classes, most severe first.
const (
	ClassDanger Class = "danger" // can end the run within a night or two
	ClassEnding Class = "ending" // an ending's door opening, closing or at risk
	ClassAnswer Class = "answer" // needs a move before the next night runs
	ClassTurn   Class = "turn"   // something you held gone or hit, or a hand changed
	ClassNotice Class = "notice" // said on the report or the dashboard; never stops
)

// alertClasses is every alert kind's class. Two depend on the alert,
// in Alert.Class: a crew_line under the informant or flip line and a
// war_muscle at the last corner are dangers.
var alertClasses = map[AlertKind]Class{
	AlertArrest: ClassDanger, AlertBroke: ClassDanger, AlertTalking: ClassDanger, AlertPages: ClassDanger,
	AlertTaskForce: ClassDanger, AlertFile: ClassDanger, AlertInvestigation: ClassDanger,

	AlertRetire: ClassEnding, AlertStraight: ClassEnding, AlertVanish: ClassEnding, AlertReign: ClassEnding,

	AlertWarMuscle: ClassAnswer, AlertContractDue: ClassAnswer, AlertDebtDue: ClassAnswer, AlertHeat: ClassAnswer,
	AlertWages: ClassAnswer, AlertCrewLine: ClassAnswer, AlertSkim: ClassAnswer, AlertUnposted: ClassAnswer,
	AlertIdleCorner: ClassAnswer, AlertScouts: ClassAnswer, AlertHouseKnown: ClassAnswer, AlertFavour: ClassAnswer,
	AlertExposure: ClassAnswer,

	AlertNoCorner: ClassTurn, AlertFrontShut: ClassTurn,

	AlertFloat: ClassNotice, AlertTill: ClassNotice, AlertStashFull: ClassNotice, AlertLanded: ClassNotice,
	AlertGate: ClassNotice, AlertPort: ClassNotice, AlertExports: ClassNotice, AlertDARace: ClassNotice,
	AlertPlan: ClassNotice,
}

// Class is the alert's class under the stop rule.
func (a Alert) Class() Class {
	switch {
	case a.Kind == AlertCrewLine && a.Cross == "under":
		return ClassDanger // a member that low talks (#492)
	case a.Kind == AlertWarMuscle && a.Count <= 1:
		return ClassDanger // the war takes the last corner (#520)
	}
	return alertClasses[a.Kind]
}

// eventLists is every event kind's class when it stops, by class. Some
// stop only on a condition, in EventClass, and are notices otherwise.
var eventLists = map[Class][]events.Event{
	ClassDanger: {
		events.WarrantSigned{}, events.TaskForceFormed{}, events.InvestigationOpened{},
		events.Enforcement{}, // past a patrol
	},
	ClassEnding: {
		events.ReignBegan{}, // the run's first
		events.ReignBroken{}, events.HoldBegan{}, events.StraightOpened{}, events.StraightLapsed{},
		events.QuietBroken{}, // where Stop's serves says retiring is in play
	},
	ClassAnswer: {
		events.DealOffered{}, events.ContractOffered{}, events.RivalEyeing{}, events.RivalScouting{},
		events.RivalRecruiting{}, events.SupplyShort{}, events.StandingShort{},
		events.CrewPaid{}, // short
	},
	ClassTurn: {
		events.CornerStruck{}, // unless a war night that held
		events.RivalBoosted{}, // failed
		events.CornerTaken{},  // from you
		events.CornerLost{},   // yours
		events.CrewShot{},     // yours, dead
		events.CrewPoached{},  // gone over
		events.WarEnded{},     // not called off by you
		events.RivalMovedIn{}, events.RivalRaided{}, events.RivalAbandoned{}, events.RivalAbsorbed{},
		events.CrewQuit{}, events.CrewDefected{}, events.CrewArrested{}, events.CrewRetired{},
		events.SpyFound{}, events.IntelFalse{}, events.LieutenantWalked{},
		events.FrontAudited{}, events.ShipmentSeized{}, events.ExportSeized{}, events.AssetSeized{},
		events.TrophySeized{}, events.DeedSeized{}, events.TunnelFound{},
		events.HouseRobbed{}, events.HouseRaided{}, events.HouseLost{},
		events.DealBroken{}, events.ChiefReplaced{}, events.DAElected{},
		events.BribeBackfired{}, events.RaidFellThrough{}, events.LeadsFiled{}, events.OfficialsCold{},
	},
	ClassNotice: {
		events.DayEnded{}, events.Headline{}, events.PriceShock{}, events.Unlocked{}, events.PriceMove{},
		events.PlayerSold{}, events.HeatChanged{}, events.FileChanged{}, events.LaidLow{}, events.GameOver{},
		events.CrewHired{}, events.CrewFired{}, events.CrewTurnedInformant{}, events.InvestigationRun{},
		events.CrewSkimmed{}, events.CrewPaidOff{}, events.CornerClaimed{}, events.CornerRobbed{},
		events.RivalOutbid{}, events.RivalPushed{}, events.RivalTippedPolice{}, events.RivalUndercut{},
		events.WarEscalated{}, events.UpgradeBought{}, events.FallGuyBurned{}, events.ReputationShifted{},
		events.FrontBought{}, events.FrontFrozen{}, events.FrontInvested{}, events.FrontGrew{}, events.Reserved{},
		events.CashLaundered{}, events.Incident{}, events.DilemmaDrawn{}, events.DilemmaAnswered{},
		events.WholesaleBought{}, events.SupplyBought{}, events.SupplierBought{}, events.CreditTaken{},
		events.DebtPaid{}, events.DebtLate{}, events.SupplierFrozen{}, events.SupplierWarned{},
		events.SupplierCollected{}, events.ShipmentSent{}, events.RouteIdle{}, events.ShipmentArrived{},
		events.CrewBailed{}, events.CrewReleased{}, events.CrewRecovered{}, events.KinLooking{},
		events.DealAccepted{}, events.DealRefused{}, events.DealEnded{}, events.DealEnding{},
		events.ClaimDeterred{}, events.Taxed{}, events.TributePaid{}, events.FactionPushed{},
		events.RivalWithdrew{}, events.ScoutsHit{}, events.ScoutsMissed{}, events.StrikeCalledOff{},
		events.RivalLeaderArrested{}, events.TrustSpread{}, events.LieutenantFlipped{}, events.LieutenantActed{},
		events.PressureShifted{}, events.CityFunded{}, events.CampaignBacked{}, events.CampaignLost{},
		events.CampaignHedged{}, events.BribeAccepted{}, events.BribeRefused{}, events.LeadFound{},
		events.CheckpointBought{}, events.ContractAccepted{}, events.ContractDelivered{}, events.HandoffHeld{},
		events.ContractFailed{}, events.ContractExpired{}, events.PlayerUndercut{}, events.RivalScouted{},
		events.PoliceTipped{}, events.RivalMusclePoached{}, events.TierReached{}, events.HouseBought{},
		events.HouseCompromised{}, events.StockMoved{}, events.RentPaid{}, events.Overdose{}, events.StockCut{},
		events.CookOrdered{}, events.Cooked{}, events.DeedBought{}, events.DeedsBought{}, events.DeedRent{},
		events.AssetBought{}, events.AssetFrozen{}, events.AssetUpkeepPaid{}, events.FrontsClosed{},
		events.WarrantLapsed{}, events.ExportShipped{}, events.ExportLanded{}, events.TrophyBought{},
		events.CashRotted{}, events.RichListed{}, events.IntelGained{}, events.SpyPlanted{},
		events.InvestigationClosed{}, events.CrewTrait{}, events.CaptainActed{},
	},
}

// eventClasses is eventLists by kind.
var eventClasses = classes(eventLists)

// classes turns the lists into a table by kind.
func classes(by map[Class][]events.Event) map[string]Class {
	out := map[string]Class{}
	for c, evs := range by {
		for _, e := range evs {
			out[e.Kind()] = c
		}
	}
	return out
}

// EventClass is the event's class under the stop rule: its kind's, or a
// notice where the kind stops only on a condition the event does not
// meet. Stop's serves and news filter it further by the world and the
// nights before.
func EventClass(e events.Event) Class {
	stops := true
	switch ev := e.(type) {
	case events.Enforcement:
		stops = ev.Level != content.Patrol
	case events.CornerStruck:
		stops = !ev.War || ev.Taken // a war night that held runs past (#229)
	case events.RivalBoosted:
		stops = !ev.Taken
	case events.CornerTaken:
		stops = ev.From == game.OwnerPlayer
	case events.CornerLost:
		stops = ev.Owner == game.OwnerPlayer // nobody worked it (#345), or the police cleared it (#469)
	case events.CrewShot:
		stops = ev.Dead && !ev.Theirs
	case events.CrewPoached:
		stops = !ev.Stayed // one who stayed is a loyalty dip, in the report (#541)
	case events.ReignBegan:
		stops = !ev.Again // the first reign of the run (#399); one begun again runs past
	case events.CrewPaid:
		stops = ev.Short > 0 // a missed payroll (#518)
	case events.WarEnded:
		stops = !ev.Called // one you called off yourself is in the report and runs past (#520)
	}
	c, ok := eventClasses[e.Kind()]
	if !ok || !stops {
		return ClassNotice
	}
	return c
}
