package engine

import (
	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The rules a front end reads (#297): one interface per sim, the read
// methods the TUI calls on it, each satisfied by the sim as it is. The
// parameter names are the wire's too (#325): internal/protocol serves
// every method as rules.<sim>.<method> and names its parameters after
// these, so a name here is part of the protocol.
type MarketRules interface {
	Band(rel float64) int
	Bands() int
	BaseRatio(w *game.World) float64
	BuyersTuning() content.BuyersTuning
	Capacity(w *game.World, city string, product string, d events.Dial) int
	ContractPrice(w *game.World, c game.Contract) float64
	CookCost(product string) int
	Cooks(product string) bool
	Cut() float64
	CutCost(product string) int
	CutMax(product string) float64
	Dial(d events.Dial) content.DialConfig
	Due(w *game.World, city string, product string) int
	Markup() float64
	PriceCut() float64
	QualityMul(quality float64) float64
	RatioAt(w *game.World, sup *game.Supplier, band int) float64
	Steal(w *game.World, c game.Corner, dial events.Dial) float64
	SupplierRatio(w *game.World, sup *game.Supplier) float64
	SuppliersTuning() content.SupplierTuning
	SupplyPrice(w *game.World, city string, product string) float64
	Tuning() content.QualityTuning
	UndercutUnits(w *game.World, c game.Corner, dial events.Dial) float64
}
type LogisticsRules interface {
	Budget(w *game.World) int
	Capacity(w *game.World, r content.RouteConfig) int
	Customs(r content.RouteConfig) bool
	Days(w *game.World, r content.RouteConfig, d events.Ship) int
	DaysTarget(w *game.World, r content.RouteConfig, product string, days int) int
	DealCut(r content.RouteConfig) float64
	DriverCut(skill int) float64
	ExportCost(w *game.World, product string) float64
	ExportPrice(w *game.World, l content.LaneConfig, product string) float64
	Fare(w *game.World, r content.RouteConfig) float64
	Idle(w *game.World, r content.RouteConfig) string
	Lane(id string) *content.LaneConfig
	LaneCapacity(w *game.World, l content.LaneConfig) int
	LaneOpen(w *game.World, l content.LaneConfig) bool
	LaneRisk(w *game.World, l content.LaneConfig, day int) float64
	Lanes() []content.LaneConfig
	LanesOpen(w *game.World) []content.LaneConfig
	LoadTonight(w *game.World, l content.LaneConfig, budget int) (units int, cost int)
	RiskFrom(w *game.World, r content.RouteConfig, d events.Ship, base float64) float64
	Route(id string) *content.RouteConfig
	RoutesOpen(w *game.World, city string) []content.RouteConfig
	Target(w *game.World, r content.RouteConfig, product string) int
	Watched(w *game.World, day int) bool
}
type TerritoryRules interface {
	DeedPrice(w *game.World, c game.Corner) int
	DeedRent(w *game.World, c game.Corner, price int) int
	Deeds() content.DeedTuning
	DriftDays(w *game.World) int
	HouseRobberyChance(w *game.World, h *game.House) float64
	RentDays() int
	RobberyChance(w *game.World, c *game.Corner) float64
	TaxDue(w *game.World, city string) (corners int, amount int)
}
type RivalsRules interface {
	Allies(w *game.World) []*game.RivalState
	ArriveDay(w *game.World, r *game.RivalState) int
	Books() content.BooksTuning
	BoostHeat(c *game.Corner) float64
	BoostTake(w *game.World, c game.Corner) int
	BoostTuning() content.BoostTuning
	Chance(w *game.World, r *game.RivalState, d game.Deal) float64
	CornerIncome(w *game.World, c game.Corner) int
	Cut(w *game.World, r *game.RivalState, share float64) int
	DefenceAt(w *game.World, r *game.RivalState, muscle int) float64
	Diplomacy() content.DiplomacyTuning
	Distrusted(r *game.RivalState, day int) bool
	Expansion() content.ExpansionTuning
	EyeingBy(w *game.World, r *game.RivalState) *game.Corner
	Factions() content.FactionsTuning
	MusclePrice(w *game.World, r *game.RivalState) int
	OddsOnAt(w *game.World, r *game.RivalState, c *game.Corner, force events.Force, muscle int) float64
	PoachTuning() content.PoachTuning
	PushOddsAt(w *game.World, r *game.RivalState, c *game.Corner, muscle int) float64
	RaidReady(r *game.RivalState, day int) bool
	ScoutCost() int
	ScoutOdds(w *game.World, r *game.RivalState) float64
	Stale(w *game.World, r *game.RivalState, day int) bool
	StrikeHeat(c *game.Corner, force events.Force) float64
	TipTuning() content.TipTuning
	TributeBase(w *game.World, r *game.RivalState) float64
	Tuning() content.RivalsTuning
	WarTarget(w *game.World, r *game.RivalState) *game.Corner
}
type CrewRules interface {
	BailCost(m game.CrewMember) int
	Batch(w *game.World) int
	BatchIn(w *game.World, city string) int
	BatchOf(skill int) int
	Birthday(m game.CrewMember, day int) int
	CanCaptain(w *game.World, m game.CrewMember) bool
	Captaincy() content.CaptainTuning
	ChemistName(w *game.World) string
	ChemistQuality(w *game.World) float64
	CookCostIn(w *game.World, city string, cost int) int
	CookDays() int
	Cut() float64
	CutBonus(w *game.World) float64
	Dial(m game.CrewMember) events.Dial
	FlipLine() float64
	InvestigateCost() int
	InvestigateOdds(w *game.World) float64
	Lab(w *game.World, city string) *content.AssetConfig
	Lieutenancy() content.LieutenantTerms
	LieutenantsWanted(w *game.World) bool
	Life() content.LifeTuning
	MaxCrew(w *game.World) int
	PayoffCost(m game.CrewMember) int
	PayoffLoyalty() float64
	PoolDays(w *game.World) int
	QualityIn(w *game.World, city string) float64
	QualityOf(skill int) float64
	Retiring(m game.CrewMember) bool
	RevealDays() int
	Trait(name string) content.Trait
	TraitDays() int
	Tuning() content.CrewTuning
	WageAt(w *game.World, m game.CrewMember, p events.Pay) int
	Wages(w *game.World, p events.Pay) int
}
type HeatRules interface {
	BribeCooldown() int
	BribeDecayMul() float64
	ContractHeat(w *game.World, city string, product string, units int, mul float64) float64
	Cover(w *game.World) int
	PileHeatOf(w *game.World, pile int) float64
	DirtyCashThreshold(w *game.World) int
	Due(w *game.World) string
	EvidenceArrest(w *game.World) int
	ExposureLine(w *game.World) int
	ForfeitEvidence() int
	Hottest(w *game.World) *game.City
	Ladder(w *game.World, city *game.City) []content.ResponseConfig
	Rungs(w *game.World, city *game.City) []content.ResponseConfig
	MoveHeat(w *game.World, city string, product string, units int) float64
	SaleHeat(w *game.World, city string, product string, wanted int, dial events.Dial) float64
	Sloppiness(w *game.World, city string) float64
	SloppyHeat(w *game.World, city string, units int) float64
	StructureEvidence() int
	TaskForceForming(w *game.World) bool
	ThresholdsIn(w *game.World, city *game.City) []content.ResponseConfig
}
type LawRules interface {
	Bribes() content.BribeTuning
	Campaign() content.CampaignTuning
	ChiefTermEnds(w *game.World) int
	DAOdds(w *game.World, amount int) float64
	DAPrice(w *game.World) int
	DeedLimit(w *game.World) int
	Forfeits(w *game.World) bool
	Goodwill(amount int) float64
	NextElection(w *game.World) int
	Tuning() content.LawTuning
}
type LaunderingRules interface {
	AnyAuditRisk(w *game.World) float64
	AssetOffer(id string) (game.AssetOffer, bool)
	AssetOffers() []game.AssetOffer
	AssetUpkeep(w *game.World) int
	AuditRisk(w *game.World, f game.Front) float64
	CanGoStraight(w *game.World) bool
	CanRetire(w *game.World) bool
	Capacity(w *game.World) int
	CashOutFee(amount int) int
	Dial(d events.Launder) content.LaunderConfig
	Fee(w *game.World, amount int) int
	FrontUpkeep(w *game.World, f game.Front) int
	Growth() content.GrowthConfig
	Income(f game.Front) int
	LegitIncome(w *game.World) int
	LevelCost(f game.Front, n int) int
	Levels(f game.Front, n int) game.LevelOffer
	Lots(amount int) int
	MaxLevel(f game.Front) int
	Offers() []game.FrontOffer
	Offshore() content.OffshoreConfig
	Rot(pile int) int
	RotLine() int
	Throughput(w *game.World, f game.Front) int
	TrophyOffer(trophy string) (game.TrophyOffer, bool)
	TrophyOffers() []game.TrophyOffer
	Tuning() content.LaunderingTuning
	Upkeep(w *game.World) int
}

type Rules struct {
	Market     MarketRules
	Logistics  LogisticsRules
	Territory  TerritoryRules
	Rivals     RivalsRules
	Crew       CrewRules
	Heat       HeatRules
	Law        LawRules
	Laundering LaunderingRules
}
