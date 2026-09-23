package events

// All is every event kind's zero value, in the order events.go declares
// them (#274). It is the one list a test walks when it must see every
// kind: TestAllListsEveryKind parses events.go and fails when a type
// with a Kind method is missing here, so the list cannot drift, and
// news's TestEveryEmittedEventHasTemplate walks it, so a new kind has a
// headline template or is named report-only, never neither. Nothing in
// the game reads it.
var All = []Event{
	DayEnded{}, Headline{}, PriceShock{}, Unlocked{}, PriceMove{}, PlayerSold{},
	StandingShort{}, HeatChanged{}, Enforcement{}, LaidLow{}, GameOver{}, CrewHired{},
	CrewFired{}, CrewQuit{}, CrewTurnedInformant{}, CrewDefected{}, InvestigationRun{},
	CrewSkimmed{}, CrewPaidOff{}, CrewPaid{}, CornerClaimed{}, CornerLost{}, CornerRobbed{},
	RivalMovedIn{}, CornerTaken{}, RivalEyeing{}, RivalOutbid{}, RivalPushed{},
	CornerStruck{}, WarEnded{}, RivalTippedPolice{}, RivalUndercut{}, WarEscalated{},
	UpgradeBought{}, FallGuyBurned{}, ReputationShifted{}, FrontBought{}, FrontAudited{},
	FrontFrozen{}, FrontInvested{}, FrontGrew{}, Reserved{}, CashLaundered{}, Incident{},
	DilemmaDrawn{}, DilemmaAnswered{}, WholesaleBought{}, SupplyBought{}, SupplierBought{},
	CreditTaken{}, DebtPaid{}, DebtLate{}, SupplierFrozen{}, SupplierWarned{},
	SupplierCollected{}, SupplyShort{}, ShipmentSent{}, ShipmentArrived{}, ShipmentSeized{},
	CrewArrested{}, CrewBailed{}, CrewReleased{}, CrewShot{}, CrewRecovered{}, CrewRetired{},
	KinLooking{}, DealOffered{}, DealAccepted{}, DealRefused{}, DealBroken{}, DealEnded{},
	ClaimDeterred{}, Taxed{}, ReignBegan{}, ReignBroken{}, TributePaid{}, FactionPushed{},
	RivalAbsorbed{}, RivalScouting{}, RivalRecruiting{}, RivalWithdrew{}, ScoutsHit{}, RivalLeaderArrested{}, CrewPoached{}, TrustSpread{},
	LieutenantFlipped{}, LieutenantWalked{}, LieutenantActed{}, DAElected{}, ChiefReplaced{},
	PressureShifted{}, CityFunded{}, CampaignBacked{}, CampaignLost{}, CampaignHedged{},
	BribeAccepted{}, RaidFellThrough{}, BribeRefused{}, BribeBackfired{}, LeadFound{},
	LeadsFiled{}, OfficialsCold{}, CheckpointBought{}, ContractOffered{}, ContractAccepted{},
	ContractDelivered{}, ContractFailed{}, ContractExpired{}, PlayerUndercut{},
	RivalAbandoned{}, RivalScouted{}, RivalBoosted{}, PoliceTipped{}, RivalRaided{},
	RivalMusclePoached{}, TierReached{}, HouseBought{}, HouseRobbed{}, HouseRaided{},
	HouseLost{}, HouseCompromised{}, StockMoved{}, RentPaid{}, Overdose{}, StockCut{},
	CookOrdered{}, Cooked{}, DeedBought{}, DeedsBought{}, DeedRent{}, DeedSeized{},
	AssetBought{}, AssetFrozen{}, TaskForceFormed{}, AssetSeized{}, TunnelFound{},
	IntelGained{}, SpyPlanted{}, SpyFound{}, IntelFalse{},
}
